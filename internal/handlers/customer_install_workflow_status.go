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
				// Install deleted from API - return empty state
				theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
				primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)
				h.RenderTempl(c, http.StatusOK, partials.ActiveProvisionBanner(nil, "", install.ID, h.basePath, primaryColor))
				return
			}
		}
	}

	// Fetch active provision workflow (provision or provision_sandbox)
	var activeProvisionWorkflow *partials.WorkflowDataPanel
	var cloudFormationLink string
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		provClient, provErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if provErr == nil {
			ctx := c.Request.Context()
			provisionTypes := []string{"provision", "provision_sandbox", "reprovision", "reprovision_sandbox"}
			for _, wfType := range provisionTypes {
				workflows, _, err := provClient.GetInstallWorkflowsByType(ctx, install.NuonInstallID, 0, 1, wfType)
				if err != nil || len(workflows) == 0 {
					continue
				}
				wf := workflows[0]
				if wf.Status == nil {
					continue
				}
				status := string(wf.Status.Status)
				if status != "in-progress" && status != "approval-awaiting" && status != "pending" {
					continue
				}
				processed := processWorkflowForCustomer(wf)
				panel := ginHToWorkflowDataPanel(processed)
				activeProvisionWorkflow = &panel

				// Check if the "await install stack" step is active (pending, in-progress, or approval-awaiting)
				if wf.Steps != nil {
					for _, step := range wf.Steps {
						if step.Name != "await install stack" {
							continue
						}
						stepStatus := ""
						if step.Status != nil {
							stepStatus = string(step.Status.Status)
						}
						if stepStatus == "" || stepStatus == "pending" || stepStatus == "in-progress" || stepStatus == "approval-awaiting" {
							// Fetch CloudFormation link
							stack, stackErr := provClient.GetInstallStack(ctx, install.NuonInstallID)
							if stackErr == nil && stack != nil && stack.Versions != nil && len(stack.Versions) > 0 {
								if stack.Versions[0].QuickLinkURL != "" {
									cloudFormationLink = stack.Versions[0].QuickLinkURL
								}
							}
							// Append customer inputs to CF URL
							if cloudFormationLink != "" {
								inputConfig, inputErr := provClient.GetAppInputConfig(ctx, install.GetAppID())
								if inputErr == nil && inputConfig != nil {
									var configMap map[string]interface{}
									jsonBytes, jerr := json.Marshal(inputConfig)
									if jerr == nil {
										if json.Unmarshal(jsonBytes, &configMap) == nil {
											inputMappings := extractCustomerInputMappings(configMap)
											if len(inputMappings) > 0 {
												currentInputs, ciErr := provClient.GetInstallCurrentInputs(ctx, install.NuonInstallID)
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
							break
						}
					}
				}
				break
			}
		}
	}

	// Get theme color
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)

	// Always render banner (pass nil if no workflow, component handles it)
	h.RenderTempl(c, http.StatusOK, partials.ActiveProvisionBanner(
		activeProvisionWorkflow,
		cloudFormationLink,
		install.ID,
		h.basePath,
		primaryColor,
	))
}

// InstallReadmeStatus handles HTMX polling for readme display
