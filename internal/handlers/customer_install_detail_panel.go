package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/markdown"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"github.com/nuonco/nuon-go/client/operations"
	"go.uber.org/zap"
)

func (h *Handler) InstallDetailPanel(c *gin.Context) {
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

	// Check if install still exists in Nuon API
	var apiDeletedError bool
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		checkClient, checkErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if checkErr == nil {
			_, apiErr := checkClient.GetInstall(context.Background(), install.NuonInstallID)
			if apiErr != nil {
				// Check if error is specifically a 404 NotFound
				var notFoundErr *operations.GetInstallNotFound
				if errors.As(apiErr, &notFoundErr) {
					apiDeletedError = true
					h.logger.Warn("install deleted from API but exists locally",
						zap.String("install_id", install.ID),
						zap.String("nuon_install_id", install.NuonInstallID),
					)
				}
			}
		}
	}

	// Fetch app config version info and app name
	var appName string
	var appConfigVersion int64
	var appConfigUpdatedAt string
	var installConfigVersion int64
	var installConfigUpdatedAt string
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		appClient, err := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if err == nil {
			app, err := appClient.GetApp(context.Background(), install.GetAppID())
			if err == nil && app != nil {
				appName = app.Name
				if len(app.AppConfigs) > 0 {
					appConfigVersion = app.AppConfigs[0].Version
					appConfigUpdatedAt = app.AppConfigs[0].UpdatedAt

					// Get install's current config version by matching AppConfigID
					nuonInstall, instErr := appClient.GetInstall(context.Background(), install.NuonInstallID)
					if instErr == nil && nuonInstall.AppConfigID != "" {
						for _, cfg := range app.AppConfigs {
							if cfg.ID == nuonInstall.AppConfigID {
								installConfigVersion = cfg.Version
								installConfigUpdatedAt = cfg.UpdatedAt
								break
							}
						}
					}
				}
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
			provisionTypes := []string{"provision", "provision_sandbox"}
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

	// Fetch install readme only if no active provision workflow
	var installReadme *partials.ReadmeDataPanel
	if activeProvisionWorkflow == nil {
		if nuonOrg != nil && nuonOrg.APIToken != "" {
			readmeClient, readmeErr := nuon.NewClientWithURL(
				nuonOrg.APIToken,
				nuonOrg.NuonOrgID,
				h.nuonAPIURL,
			)
			if readmeErr == nil {
				readme, err := readmeClient.GetInstallReadme(c.Request.Context(), install.NuonInstallID)
				if err != nil {
					h.logger.Warn("failed to fetch install readme",
						zap.String("install_id", install.NuonInstallID),
						zap.Error(err),
					)
					// Continue without readme - non-blocking
				} else if readme != nil {
					// Convert nuon.ServiceReadme to partials.ReadmeDataPanel
					warnings := []string{}
					if readme.Warnings != nil {
						warnings = readme.Warnings
					}

					// Convert markdown to HTML
					renderedHTML := readme.Readme
					if readme.Readme != "" {
						html, err := markdown.ToHTML(readme.Readme)
						if err != nil {
							h.logger.Warn("failed to render markdown to HTML",
								zap.String("install_id", install.NuonInstallID),
								zap.Error(err),
							)
							// Fall back to raw markdown if conversion fails
							renderedHTML = readme.Readme
						} else {
							renderedHTML = html
						}
					}
					installReadme = &partials.ReadmeDataPanel{
						Readme:   renderedHTML,
						Original: readme.Original,
						Warnings: warnings,
					}
				}
			}
		}
	}

	// Get theme colors
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)
	secondaryColor, _ := GetPrimaryColors(theme.SecondaryColor)

	props := partials.InstallDetailPanelProps{
		Install:                 install,
		AppName:                 appName,
		APIDeletedError:         apiDeletedError,
		BasePath:                h.basePath,
		PrimaryColor:            primaryColor,
		SecondaryColor:          secondaryColor,
		AppConfigVersion:        appConfigVersion,
		AppConfigUpdatedAt:      appConfigUpdatedAt,
		InstallConfigVersion:    installConfigVersion,
		InstallConfigUpdatedAt:  installConfigUpdatedAt,
		ActiveProvisionWorkflow: activeProvisionWorkflow,
		InstallReadme:           installReadme,
		CloudFormationLink:      cloudFormationLink,
	}
	h.RenderTempl(c, http.StatusOK, partials.InstallDetailPanel(props))
}

// InstallWorkflowStatus returns the active provision workflow banner for HTMX polling
// This endpoint is called by HTMX polling every 5 seconds
// It must return HTML (ActiveProvisionBanner template) for HTMX to swap
// Authentication is handled by JWT middleware which returns HX-Trigger: auth-error on failure
