package handlers

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"github.com/nuonco/nuon-go/client/operations"
	nuonmodels "github.com/nuonco/nuon-go/models"
	"go.uber.org/zap"
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
			h.nuonAPIURLForOrg(nuonOrg),
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
	var stackSetup partials.StackSetupData
	var rawWorkflow *nuonmodels.AppWorkflow
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		provClient, provErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURLForOrg(nuonOrg),
		)
		if provErr == nil {
			ctx := c.Request.Context()
			bestWf, bestPanel := h.findMostRecentProvisionWorkflow(ctx, provClient, install.NuonInstallID)
			if bestPanel != nil {
				provisionWorkflow = bestPanel
				rawWorkflow = bestWf

				// Only check for stack setup data on active workflows
				if isActiveWorkflowStatus(provisionWorkflow.Status) && bestWf != nil {
					stackSetup = h.getStackSetupData(ctx, provClient, install, bestWf)
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
		phases := groupStepsIntoPhases(rawWorkflow, provisionWorkflow.IsReprovision)
		if len(phases) == 0 {
			// Workflow exists but steps haven't populated yet — show placeholder phases
			if provisionWorkflow.IsReprovision {
				phases = []partials.ProvisionPhase{
					{Name: "Update stack", Status: "not_started"},
					{Name: "Update sandbox", Status: "not_started"},
					{Name: "Update app", Status: "not_started"},
				}
			} else {
				phases = []partials.ProvisionPhase{
					{Name: "Install stack", Status: "not_started"},
					{Name: "Provision sandbox", Status: "not_started"},
					{Name: "Deploy app", Status: "not_started"},
				}
			}
		}
		h.RenderTempl(c, http.StatusOK, partials.ProvisionAccordion(
			phases,
			provisionWorkflow,
			stackSetup,
			install.ID,
			h.basePath,
			primaryColor,
		))
		return
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
			placeholderPhases, placeholderWf, partials.StackSetupData{}, install.ID, h.basePath, primaryColor,
		))
		return
	}

	// Fallback: terminal workflow → re-fetch full panel so overview cards appear;
	// no workflow yet → keep polling for workflow-status.
	c.Header("Content-Type", "text/html; charset=utf-8")
	if provisionWorkflow != nil && isTerminalWorkflowStatus(provisionWorkflow.Status) {
		// Trigger an immediate full panel re-render so the overview cards replace the accordion.
		c.String(http.StatusOK, fmt.Sprintf(`<div id="active-provision-banner" hx-get="%s/installs/%s/panel" hx-trigger="load" hx-target="#panel-content-overview" hx-swap="innerHTML"></div>`,
			h.basePath, install.ID))
	} else {
		c.String(http.StatusOK, fmt.Sprintf(`<div id="active-provision-banner" hx-get="%s/installs/%s/workflow-status" hx-trigger="every 5s" hx-swap="outerHTML"></div>`,
			h.basePath, install.ID))
	}
}

// provisionWorkflowTypes is the set of workflow types that represent provision/reprovision operations.
var provisionWorkflowTypes = map[string]bool{
	"provision":           true,
	"provision_sandbox":   true,
	"reprovision":         true,
	"reprovision_sandbox": true,
}

// findMostRecentProvisionWorkflow finds the most recently created provision/reprovision workflow.
// Uses a single API call to fetch recent workflows and filters client-side.
func (h *Handler) findMostRecentProvisionWorkflow(ctx context.Context, client *nuon.Client, nuonInstallID string) (*nuonmodels.AppWorkflow, *partials.WorkflowDataPanel) {
	// Fetch a small batch of recent workflows (sorted by created_at desc by the API)
	workflows, _, err := client.GetInstallWorkflowsByType(ctx, nuonInstallID, 0, 10, "")
	if err != nil || len(workflows) == 0 {
		return nil, nil
	}

	// Find the most recent provision-type workflow
	for _, wf := range workflows {
		if wf.Status == nil {
			continue
		}
		if !provisionWorkflowTypes[string(wf.Type)] {
			continue
		}
		processed := processWorkflowForCustomer(wf)
		processed["current_step_role"] = resolveCurrentStepRole(ctx, client, nuonInstallID, processed)
		panel := ginHToWorkflowDataPanel(processed)
		panel.IsReprovision = strings.Contains(string(wf.Type), "reprovision")
		if panel.IsReprovision {
			panel.Name = "Reprovision"
		}
		return wf, &panel
	}
	return nil, nil
}

// getStackSetupData checks for an active "await install stack" step and returns platform-specific setup data.
func (h *Handler) getStackSetupData(ctx context.Context, client *nuon.Client, install *models.Install, wf *nuonmodels.AppWorkflow) partials.StackSetupData {
	if wf.Steps == nil {
		return partials.StackSetupData{}
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
			return partials.StackSetupData{}
		}

		// Determine platform from app runner type
		platform := h.detectPlatform(ctx, client, install)
		zap.L().Debug("getStackSetupData: detected platform", zap.String("platform", platform), zap.String("installID", install.NuonInstallID))

		stack, stackErr := client.GetInstallStack(ctx, install.NuonInstallID)
		if stackErr != nil {
			zap.L().Debug("getStackSetupData: failed to fetch install stack", zap.Error(stackErr))
		}

		switch platform {
		case "gcp":
			setup := partials.StackSetupData{
				Platform:      "gcp",
				NuonInstallID: install.NuonInstallID,
			}
			if stackErr == nil && stack != nil && stack.Versions != nil && len(stack.Versions) > 0 {
				setup.TfvarsContent = parseTfvars(stack.Versions[0].Contents)
				zap.L().Debug("getStackSetupData: parsed tfvars", zap.Bool("hasTfvars", setup.TfvarsContent != ""))
			} else {
				zap.L().Debug("getStackSetupData: no stack versions available for tfvars")
			}
			return setup

		default: // AWS and others use CloudFormation
			setup := partials.StackSetupData{Platform: platform}
			if stackErr == nil && stack != nil && stack.Versions != nil && len(stack.Versions) > 0 {
				if stack.Versions[0].QuickLinkURL != "" {
					setup.CloudFormationLink = stack.Versions[0].QuickLinkURL
				}
			}
			// Append customer inputs to CF URL
			if setup.CloudFormationLink != "" {
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
									setup.CloudFormationLink = appendInputsToCloudFormationURL(
										setup.CloudFormationLink,
										currentInputs.Values,
										inputMappings,
									)
								}
							}
						}
					}
				}
			}
			return setup
		}
	}
	return partials.StackSetupData{}
}

// detectPlatform determines the cloud platform from the app's runner type.
func (h *Handler) detectPlatform(ctx context.Context, client *nuon.Client, install *models.Install) string {
	appID := install.GetAppID()
	app, err := client.GetApp(ctx, appID)
	if err != nil {
		zap.L().Debug("detectPlatform: failed to fetch app", zap.String("appID", appID), zap.Error(err))
		return "aws" // default
	}
	if app == nil || app.RunnerConfig == nil {
		zap.L().Debug("detectPlatform: app or runner config is nil", zap.String("appID", appID))
		return "aws" // default
	}
	runnerType := string(app.RunnerConfig.AppRunnerType)
	zap.L().Debug("detectPlatform: runner type", zap.String("appID", appID), zap.String("runnerType", runnerType))
	switch runnerType {
	case "gcp":
		return "gcp"
	case "azure":
		return "azure"
	default:
		return "aws"
	}
}

// parseTfvars extracts the tfvars string from stack version contents.
// Contents may be a JSON string containing a "tfvars" key.
func parseTfvars(contents interface{}) string {
	if contents == nil {
		zap.L().Debug("parseTfvars: contents is nil")
		return ""
	}

	zap.L().Debug("parseTfvars: contents type", zap.String("type", fmt.Sprintf("%T", contents)))

	var raw interface{}
	switch v := contents.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &raw); err != nil {
			// Fallback: try base64 decode, then JSON unmarshal
			decoded, b64Err := base64.StdEncoding.DecodeString(v)
			if b64Err != nil {
				decoded, b64Err = base64.RawStdEncoding.DecodeString(v)
			}
			if b64Err != nil {
				zap.L().Debug("parseTfvars: failed to unmarshal or base64-decode string contents", zap.Error(err))
				return ""
			}
			if err := json.Unmarshal(decoded, &raw); err != nil {
				zap.L().Debug("parseTfvars: base64-decoded but failed to unmarshal JSON", zap.Error(err))
				return ""
			}
		}
	case map[string]interface{}:
		raw = v
	case json.RawMessage:
		if err := json.Unmarshal(v, &raw); err != nil {
			zap.L().Debug("parseTfvars: failed to unmarshal RawMessage contents", zap.Error(err))
			return ""
		}
	default:
		// Try JSON round-trip for unknown types
		b, err := json.Marshal(v)
		if err != nil {
			zap.L().Debug("parseTfvars: unsupported contents type, marshal failed", zap.String("type", fmt.Sprintf("%T", v)))
			return ""
		}
		if err := json.Unmarshal(b, &raw); err != nil {
			zap.L().Debug("parseTfvars: unsupported contents type, unmarshal failed", zap.String("type", fmt.Sprintf("%T", v)))
			return ""
		}
	}

	if m, ok := raw.(map[string]interface{}); ok {
		if tfvars, ok := m["tfvars"]; ok {
			zap.L().Debug("parseTfvars: found tfvars key")
			return fmt.Sprintf("%v", tfvars)
		}
		zap.L().Debug("parseTfvars: no tfvars key in contents map", zap.Int("numKeys", len(m)))
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
func groupStepsIntoPhases(wf *nuonmodels.AppWorkflow, isReprovision ...bool) []partials.ProvisionPhase {
	if wf == nil || len(wf.Steps) == 0 {
		return nil
	}

	// Sort steps by index
	steps := make([]*nuonmodels.AppWorkflowStep, len(wf.Steps))
	copy(steps, wf.Steps)
	slices.SortFunc(steps, func(a, b *nuonmodels.AppWorkflowStep) int {
		return cmp.Compare(a.Idx, b.Idx)
	})

	// Determine phase layout based on workflow type
	type phaseLayout struct {
		names      []string
		phaseCount int // 1, 2, or 3
	}

	layout := phaseLayout{}
	wfType := string(wf.Type)

	switch wfType {
	case "provision":
		layout = phaseLayout{names: []string{"Install stack", "Provision sandbox", "Deploy app"}, phaseCount: 3}
	case "reprovision":
		layout = phaseLayout{names: []string{"Update stack", "Update sandbox", "Update app"}, phaseCount: 3}
	case "deprovision":
		layout = phaseLayout{names: []string{"Teardown components", "Deprovision sandbox", "Remove stack"}, phaseCount: 3}
	case "reprovision_sandbox", "drift_run_reprovision_sandbox":
		layout = phaseLayout{names: []string{"Provision sandbox", "Deploy app"}, phaseCount: 2}
	case "input_update":
		layout = phaseLayout{names: []string{"Update inputs", "Deploy components"}, phaseCount: 2}
	case "app_branches_manual_update", "app_branches_config_repo_update", "app_branches_component_repo_update":
		layout = phaseLayout{names: []string{"Update branches", "Deploy components"}, phaseCount: 2}
	case "deprovision_sandbox":
		layout = phaseLayout{names: []string{"Deprovision sandbox"}, phaseCount: 1}
	case "manual_deploy", "deploy_components":
		layout = phaseLayout{names: []string{"Deploy components"}, phaseCount: 1}
	case "teardown_component":
		layout = phaseLayout{names: []string{"Teardown component"}, phaseCount: 1}
	case "teardown_components":
		layout = phaseLayout{names: []string{"Teardown components"}, phaseCount: 1}
	case "action_workflow_run":
		layout = phaseLayout{names: []string{"Run action"}, phaseCount: 1}
	case "sync_secrets":
		layout = phaseLayout{names: []string{"Sync secrets"}, phaseCount: 1}
	case "drift_run":
		layout = phaseLayout{names: []string{"Drift check"}, phaseCount: 1}
	default:
		// Fallback: use isReprovision param or default provision layout
		reprov := len(isReprovision) > 0 && isReprovision[0]
		if reprov {
			layout = phaseLayout{names: []string{"Update stack", "Update sandbox", "Update app"}, phaseCount: 3}
		} else {
			layout = phaseLayout{names: []string{"Install stack", "Provision sandbox", "Deploy app"}, phaseCount: 3}
		}
	}

	// Assign steps to phases based on layout
	phaseSteps := make([][]partials.PhaseStep, layout.phaseCount)

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

		switch layout.phaseCount {
		case 1:
			phaseSteps[0] = append(phaseSteps[0], ps)
		case 2:
			if i < 5 {
				phaseSteps[0] = append(phaseSteps[0], ps)
			} else {
				phaseSteps[1] = append(phaseSteps[1], ps)
			}
		default: // 3
			switch {
			case i < 5:
				phaseSteps[0] = append(phaseSteps[0], ps)
			case i < 10:
				phaseSteps[1] = append(phaseSteps[1], ps)
			default:
				phaseSteps[2] = append(phaseSteps[2], ps)
			}
		}
	}

	// Build phases — include all defined phases for consistent layout
	var phases []partials.ProvisionPhase
	for i, name := range layout.names {
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
