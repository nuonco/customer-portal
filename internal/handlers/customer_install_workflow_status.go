package handlers

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"github.com/nuonco/nuon-go/client/operations"
	nuonmodels "github.com/nuonco/nuon-go/models"
)

func (h *Handler) InstallWorkflowStatus(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	// Load install with both InstallLink.NuonOrg and Org (handles published-app installs)
	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()

	// Check if install still exists in API
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		checkClient, checkErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if checkErr == nil {
			_, apiErr := checkClient.GetInstall(context.Background(), install.NuonInstallID)
			var notFoundErr *operations.GetInstallNotFound
			if errors.As(apiErr, &notFoundErr) {
				// Persist the flag
				if !install.APIDeleted {
					h.db.Model(install).Update("api_deleted", true)
				}
				// Install deleted from API - return empty div that stops polling
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.String(http.StatusOK, `<div id="active-provision-banner"></div>`)
				return
			}
		}
	}

	// Fetch most recent provision workflow (any status)
	var provisionWorkflow *partials.WorkflowDataPanel
	var cloudFormationLink string
	var rawWorkflow *nuonmodels.AppWorkflow
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		provClient, provErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if provErr == nil {
			ctx := c.Request.Context()
			bestWf, bestPanel := h.findMostRecentProvisionWorkflow(ctx, provClient, install.NuonInstallID)
			if bestPanel != nil {
				provisionWorkflow = bestPanel
				rawWorkflow = bestWf

				// Only check for CloudFormation link on active workflows
				if isActiveWorkflowStatus(provisionWorkflow.Status) && bestWf != nil {
					cloudFormationLink = h.getCloudFormationLink(ctx, provClient, install, bestWf)
				}
			}
		}
	}

	// Get theme color
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)

	// If active or failed provision, render accordion; otherwise render banner
	if provisionWorkflow != nil && (isActiveWorkflowStatus(provisionWorkflow.Status) || provisionWorkflow.Status == "error") && rawWorkflow != nil {
		phases := groupStepsIntoPhases(rawWorkflow)
		if len(phases) > 0 {
			h.RenderTempl(c, http.StatusOK, partials.ProvisionAccordion(
				phases,
				provisionWorkflow,
				cloudFormationLink,
				install.ID,
				h.basePath,
				primaryColor,
			))
			return
		}
	}

	// If the most recent provision workflow reached a terminal state, transition install status
	if provisionWorkflow != nil && (install.Status == models.StatusPending || install.Status == models.StatusProvisioning) {
		switch provisionWorkflow.Status {
		case "completed", "success":
			h.db.Model(install).Update("status", models.StatusActive)
			install.Status = models.StatusActive
		case "cancelled":
			h.db.Model(install).Update("status", models.StatusFailed)
			install.Status = models.StatusFailed
		}
	}

	// If install is pending and no active provision was rendered above, show placeholder
	if install.Status == models.StatusPending {
		placeholderWf := &partials.WorkflowDataPanel{
			Name:            "Provision",
			Status:          "pending",
			StatusClass:     "badge-neutral",
			CurrentStepName: "Preparing to provision",
			CurrentStepType: "initializing",
		}
		placeholderPhases := []partials.ProvisionPhase{
			{Name: "Install stack", Status: "not_started"},
			{Name: "Provision sandbox", Status: "not_started"},
			{Name: "Deploy app", Status: "not_started"},
		}
		h.RenderTempl(c, http.StatusOK, partials.ProvisionAccordion(
			placeholderPhases, placeholderWf, "", install.ID, h.basePath, primaryColor,
		))
		return
	}

	// Fallback: return empty polling div
	// For terminal workflows, poll slowly; for no-workflow-yet, poll every 5s
	trigger := "every 5s"
	if provisionWorkflow != nil && isTerminalWorkflowStatus(provisionWorkflow.Status) {
		trigger = "every 30s"
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, fmt.Sprintf(`<div id="active-provision-banner" hx-get="%s/installs/%s/workflow-status" hx-trigger="%s" hx-swap="outerHTML"></div>`,
		h.basePath, install.ID, trigger))
}

// findMostRecentProvisionWorkflow finds the most recently created workflow across all provision types.
func (h *Handler) findMostRecentProvisionWorkflow(ctx context.Context, client *nuon.Client, nuonInstallID string) (*nuonmodels.AppWorkflow, *partials.WorkflowDataPanel) {
	provisionTypes := []string{"provision", "provision_sandbox", "reprovision", "reprovision_sandbox"}

	var bestWf *nuonmodels.AppWorkflow
	var bestPanel *partials.WorkflowDataPanel
	var bestCreatedAt string

	for _, wfType := range provisionTypes {
		workflows, _, err := client.GetInstallWorkflowsByType(ctx, nuonInstallID, 0, 1, wfType)
		if err != nil || len(workflows) == 0 {
			continue
		}
		wf := workflows[0]
		if wf.Status == nil {
			continue
		}
		if bestWf == nil || wf.CreatedAt > bestCreatedAt {
			bestCreatedAt = wf.CreatedAt
			bestWf = wf
			processed := processWorkflowForCustomer(wf)
			panel := ginHToWorkflowDataPanel(processed)
			bestPanel = &panel
		}
	}
	return bestWf, bestPanel
}

// getCloudFormationLink checks for an active "await install stack" step and returns the CF link.
func (h *Handler) getCloudFormationLink(ctx context.Context, client *nuon.Client, install *models.Install, wf *nuonmodels.AppWorkflow) string {
	if wf.Steps == nil {
		return ""
	}

	for _, step := range wf.Steps {
		if step.Name != "await install stack" {
			continue
		}
		stepStatus := ""
		if step.Status != nil {
			stepStatus = string(step.Status.Status)
		}
		if stepStatus != "" && stepStatus != "pending" && stepStatus != "in-progress" && stepStatus != "approval-awaiting" {
			return ""
		}

		// Fetch CloudFormation link
		var cloudFormationLink string
		stack, stackErr := client.GetInstallStack(ctx, install.NuonInstallID)
		if stackErr == nil && stack != nil && stack.Versions != nil && len(stack.Versions) > 0 {
			if stack.Versions[0].QuickLinkURL != "" {
				cloudFormationLink = stack.Versions[0].QuickLinkURL
			}
		}
		// Append customer inputs to CF URL
		if cloudFormationLink != "" {
			inputConfig, inputErr := client.GetAppInputConfig(ctx, install.GetAppID())
			if inputErr == nil && inputConfig != nil {
				var configMap map[string]interface{}
				jsonBytes, jerr := json.Marshal(inputConfig)
				if jerr == nil {
					if json.Unmarshal(jsonBytes, &configMap) == nil {
						inputMappings := extractCustomerInputMappings(configMap)
						if len(inputMappings) > 0 {
							currentInputs, ciErr := client.GetInstallCurrentInputs(ctx, install.NuonInstallID)
							if ciErr == nil && currentInputs != nil && currentInputs.Values != nil {
								cloudFormationLink = appendInputsToCloudFormationURL(
									cloudFormationLink,
									currentInputs.Values,
									inputMappings,
								)
							}
						}
					}
				}
			}
		}
		return cloudFormationLink
	}
	return ""
}

// isActiveWorkflowStatus returns true if the workflow status indicates it's still running.
func isActiveWorkflowStatus(status string) bool {
	return status == "in-progress" || status == "approval-awaiting" || status == "pending"
}

// isTerminalWorkflowStatus returns true if the workflow is in a final state.
func isTerminalWorkflowStatus(status string) bool {
	return status == "completed" || status == "success" || status == "error" || status == "cancelled"
}

// groupStepsIntoPhases groups workflow steps into logical provision phases.
func groupStepsIntoPhases(wf *nuonmodels.AppWorkflow) []partials.ProvisionPhase {
	if wf == nil || len(wf.Steps) == 0 {
		return nil
	}

	// Sort steps by index
	steps := make([]*nuonmodels.AppWorkflowStep, len(wf.Steps))
	copy(steps, wf.Steps)
	slices.SortFunc(steps, func(a, b *nuonmodels.AppWorkflowStep) int {
		return cmp.Compare(a.Idx, b.Idx)
	})

	phaseNames := []string{"Install stack", "Provision sandbox", "Deploy app"}

	// Assign each step to a phase by index: 0-4 stack, 5-9 sandbox, 10+ components
	phaseSteps := make([][]partials.PhaseStep, len(phaseNames))

	for i, step := range steps {
		stepStatus := ""
		if step.Status != nil {
			stepStatus = string(step.Status.Status)
		}

		ps := partials.PhaseStep{
			Name:      formatStepName(step.Name),
			Status:    stepStatus,
			ID:        step.ID,
			Retryable: step.Retryable,
		}

		switch {
		case i < 5:
			phaseSteps[0] = append(phaseSteps[0], ps)
		case i < 10:
			phaseSteps[1] = append(phaseSteps[1], ps)
		default:
			phaseSteps[2] = append(phaseSteps[2], ps)
		}
	}

	// Build phases — always include all three for consistent layout
	var phases []partials.ProvisionPhase
	for i, name := range phaseNames {
		phase := partials.ProvisionPhase{
			Name:  name,
			Steps: phaseSteps[i],
		}

		if len(phaseSteps[i]) == 0 {
			phase.Status = "not_started"
			phases = append(phases, phase)
			continue
		}

		// Derive phase status from steps
		allCompleted := true
		activeStepIdx := -1
		for j, s := range phase.Steps {
			switch s.Status {
			case "error":
				phase.Status = "failed"
				phase.ActiveStepName = s.Name
				allCompleted = false
			case "in-progress", "approval-awaiting":
				if phase.Status != "failed" {
					phase.Status = "in_progress"
					phase.ActiveStepName = s.Name
					activeStepIdx = j
				}
				allCompleted = false
			case "completed", "success", "approved":
				// continue
			default:
				allCompleted = false
			}
		}

		if allCompleted && phase.Status == "" {
			phase.Status = "completed"
		} else if phase.Status == "" {
			phase.Status = "not_started"
		}

		if activeStepIdx >= 0 {
			phase.StepProgress = fmt.Sprintf("Step %d of %d", activeStepIdx+1, len(phase.Steps))
		}

		phases = append(phases, phase)
	}

	return phases
}

// InstallReadmeStatus handles HTMX polling for readme display
