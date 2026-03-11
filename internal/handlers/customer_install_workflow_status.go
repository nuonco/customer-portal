package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

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
				// Install deleted from API - return empty state
				theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
				primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)
				h.RenderTempl(c, http.StatusOK, partials.ProvisionBanner(nil, "", install.ID, h.basePath, primaryColor, false))
				return
			}
		}
	}

	// Fetch most recent provision workflow (any status)
	var provisionWorkflow *partials.WorkflowDataPanel
	var cloudFormationLink string
	var apiError bool
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		provClient, provErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if provErr != nil {
			apiError = true
		} else {
			ctx := c.Request.Context()
			bestWf, bestPanel := h.findMostRecentProvisionWorkflow(ctx, provClient, install.NuonInstallID)
			if bestPanel != nil {
				provisionWorkflow = bestPanel

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

	// Always render banner (pass nil if no workflow, component handles it)
	h.RenderTempl(c, http.StatusOK, partials.ProvisionBanner(
		provisionWorkflow,
		cloudFormationLink,
		install.ID,
		h.basePath,
		primaryColor,
		apiError,
	))
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

// InstallReadmeStatus handles HTMX polling for readme display
