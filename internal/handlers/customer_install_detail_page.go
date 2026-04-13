package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"github.com/nuonco/nuon-go/client/operations"
	nuonmodels "github.com/nuonco/nuon-go/models"
	"go.uber.org/zap"
)

// buildInstallPageProps loads install data and builds shared props for install tab pages.
func (h *Handler) buildInstallPageProps(c *gin.Context, activeTab string) (*customerpages.InstallPageProps, error) {
	user := h.tryGetLoggedInUser(c)

	installInterface, exists := c.Get("install")
	if !exists {
		return nil, errors.New("install not found")
	}

	install := installInterface.(*models.Install)

	if err := h.loadInstallWithOrg(install); err != nil {
		return nil, err
	}

	nuonOrg := install.GetNuonOrg()

	// Check if install still exists in Nuon API
	var apiDeletedError bool
	var nuonClientErr error
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		checkClient, checkErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURLForOrg(nuonOrg),
		)
		nuonClientErr = checkErr
		if checkErr == nil {
			_, apiErr := checkClient.GetInstall(context.Background(), install.NuonInstallID)
			if apiErr != nil {
				var notFoundErr *operations.GetInstallNotFound
				if errors.As(apiErr, &notFoundErr) {
					apiDeletedError = true
					h.logger.Warn("install deleted from API but exists locally",
						zap.String("install_id", install.ID),
						zap.String("nuon_install_id", install.NuonInstallID),
					)
					if !install.APIDeleted {
						h.db.Model(install).Update("api_deleted", true)
					}
				} else {
					nuonClientErr = apiErr
				}
			} else if install.APIDeleted {
				h.db.Model(install).Update("api_deleted", false)
			}
		}
	}

	// Fetch app name
	var appName string
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		appClient, err := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURLForOrg(nuonOrg),
		)
		if err == nil {
			app, err := appClient.GetApp(context.Background(), install.GetAppID())
			if err == nil && app != nil {
				appName = appDisplayName(app)
			}
		}
	}

	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	acctActive, acctOthers := h.getCustomerAccountsFromContext(c)

	title := install.Name
	if appName != "" {
		title = install.Name + " — " + appName
	}

	layoutProps := h.buildCustomerLayoutProps(title, user, theme, h.getOrgForLayout(c), acctActive, acctOthers)
	layoutProps.HasPublishedApps = h.orgHasPublishedApps(orgID)
	layoutProps.ActiveNav = "installs"
	if nuonClientErr != nil {
		layoutProps.NuonAPIError = "The app is experiencing network issues, and is not able to access app or install data. Please contact support for assistance."
	}
	layoutProps.CurrentInstallID = install.ID
	layoutProps.ActiveTab = activeTab

	tabTitles := map[string]string{
		"overview":   "Overview",
		"stack":      "Stack",
		"sandbox":    "Sandbox",
		"components": "Components",
		"roles":      "Roles",
		"policies":   "Policies",
		"history":    "History",
		"readme":     "README",
		"audit":      "Audit Log",
	}

	tabIcons := map[string]string{
		"overview":   "house-simple",
		"stack":      "stack",
		"sandbox":    "shipping-container",
		"components": "cards",
		"roles":      "file-lock",
		"policies":   "shield-check",
		"audit":      "clock-counter-clockwise",
		"readme":     "book-open",
	}

	return &customerpages.InstallPageProps{
		LayoutProps:     layoutProps,
		Install:         install,
		AppName:         appName,
		APIDeletedError: apiDeletedError,
		PageTitle:       tabTitles[activeTab],
		PageIcon:        tabIcons[activeTab],
	}, nil
}

// InstallDetailPage redirects to the overview tab.
func (h *Handler) InstallDetailPage(c *gin.Context) {
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}
	install := installInterface.(*models.Install)
	dest := h.basePath + "/installs/" + install.ID + "/overview"
	if qs := c.Request.URL.RawQuery; qs != "" {
		dest += "?" + qs
	}
	c.Redirect(http.StatusFound, dest)
}

func (h *Handler) InstallOverviewPage(c *gin.Context) {
	pageProps, err := h.buildInstallPageProps(c, "overview")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	install := pageProps.Install
	nuonOrg := install.GetNuonOrg()
	overviewProps := h.buildOverviewData(c, pageProps, install, nuonOrg)

	switch c.Query("partial") {
	case "panel":
		h.RenderTempl(c, http.StatusOK, customerpages.OverviewWorkflowPanel(overviewProps))
	case "content":
		h.RenderTempl(c, http.StatusOK, customerpages.OverviewContent(overviewProps))
	default:
		h.RenderTempl(c, http.StatusOK, customerpages.InstallOverviewPage(overviewProps))
	}
}

// buildOverviewData fetches all overview summary data and returns populated props.
func (h *Handler) buildOverviewData(c *gin.Context, pageProps *customerpages.InstallPageProps, install *models.Install, nuonOrg *models.NuonOrg) customerpages.InstallOverviewPageProps {
	overviewProps := customerpages.InstallOverviewPageProps{
		LayoutProps:     pageProps.LayoutProps,
		Install:         install,
		AppName:         pageProps.AppName,
		APIDeletedError: pageProps.APIDeletedError,
		ActiveTab:       c.Query("tab"),
		AlertType:       c.Query("alert_type"),
		AlertMsg:        c.Query("alert_msg"),
	}

	if nuonOrg != nil && nuonOrg.APIToken != "" {
		apiClient, apiErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURLForOrg(nuonOrg),
		)
		if apiErr == nil {
			ctx := c.Request.Context()

			// Stack summary — keep card in empty state until provisioning begins
			if stack, err := apiClient.GetInstallStack(ctx, install.NuonInstallID); err == nil && stack != nil {
				status := ""
				if len(stack.Versions) > 0 && stack.Versions[0].CompositeStatus != nil {
					status = string(stack.Versions[0].CompositeStatus.Status)
				}
				if status != "queued" {
					overviewProps.StackStatus = status
					if outputs := stack.InstallStackOutputs; outputs != nil && outputs.Aws != nil {
						overviewProps.StackRegion = outputs.Aws.Region
						overviewProps.StackAccountID = outputs.Aws.AccountID
					}
				}
			}

			// Sandbox summary — keep card in empty state until provisioning begins
			sandboxStarted := false
			if nuonInst, err := apiClient.GetInstall(ctx, install.NuonInstallID); err == nil && nuonInst != nil && nuonInst.Sandbox != nil {
				if nuonInst.Sandbox.Status != "queued" {
					overviewProps.SandboxStatus = nuonInst.Sandbox.Status
					sandboxStarted = true
				}
			}
			if sandboxStarted {
				if cfg, err := apiClient.GetAppSandboxLatestConfig(ctx, install.GetAppID()); err == nil && cfg != nil {
					if gh := cfg.ConnectedGithubVcsConfig; gh != nil {
						overviewProps.SandboxRepo = gh.Repo
						overviewProps.SandboxBranch = gh.Branch
					} else if pg := cfg.PublicGitVcsConfig; pg != nil {
						overviewProps.SandboxRepo = pg.Repo
						overviewProps.SandboxBranch = pg.Branch
						overviewProps.SandboxRepoPublic = true
					}
				}
			}

			// Components summary
			if comps, err := apiClient.GetInstallComponents(ctx, install.NuonInstallID); err == nil {
				overviewProps.Components = h.buildComponentInfos(ctx, apiClient, install, comps)
			}

			// Recent workflows
			if wfs, _, err := apiClient.GetInstallWorkflows(ctx, install.NuonInstallID, 0, 20); err == nil {
				overviewProps.RecentWorkflows = wfs
			}

			// Current input values
			currentInputs, _ := apiClient.GetInstallCurrentInputs(ctx, install.NuonInstallID)
			rawConfig, _ := apiClient.GetAppInputConfigRaw(ctx, install.GetAppID())
			overviewProps.InputFields = h.buildInputFields(currentInputs, rawConfig)
		}
	}

	return overviewProps
}

func (h *Handler) InstallOverviewWorkflowPage(c *gin.Context) {
	pageProps, err := h.buildInstallPageProps(c, "overview")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	install := pageProps.Install
	nuonOrg := install.GetNuonOrg()

	overviewProps := h.buildOverviewData(c, pageProps, install, nuonOrg)
	overviewProps.Expanded = c.Query("expanded") == "true"

	// Fetch workflow detail data
	workflowID := c.Param("workflow_id")
	if nuonOrg != nil && nuonOrg.APIToken != "" && workflowID != "" {
		apiClient, apiErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURLForOrg(nuonOrg),
		)
		if apiErr == nil {
			ctx := c.Request.Context()
			workflow, wfErr := apiClient.GetWorkflow(ctx, workflowID)
			if wfErr == nil && workflow != nil {
				processed := processWorkflowForCustomer(workflow)
				processed["current_step_role"] = resolveCurrentStepRole(ctx, apiClient, install.NuonInstallID, processed)
				panel := ginHToWorkflowDataPanel(processed)
				overviewProps.SelectedWorkflow = &panel

				var stackSetup partials.StackSetupData
				if isActiveWorkflowStatus(panel.Status) {
					stackSetup = h.getStackSetupData(ctx, apiClient, install, workflow)
				}
				overviewProps.WorkflowStack = stackSetup
				overviewProps.WorkflowPhases = groupStepsIntoPhases(workflow, panel.IsReprovision)
			}
		}
	}

	// Fetch theme for primary color
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)
	overviewProps.PrimaryColor = primaryColor

	switch c.Query("partial") {
	case "panel":
		h.RenderTempl(c, http.StatusOK, customerpages.OverviewWorkflowPanel(overviewProps))
	case "content":
		h.RenderTempl(c, http.StatusOK, customerpages.OverviewContent(overviewProps))
	default:
		h.RenderTempl(c, http.StatusOK, customerpages.InstallOverviewPage(overviewProps))
	}
}

func (h *Handler) InstallStackPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "stack")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallStackPage(*props))
}

func (h *Handler) InstallSandboxPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "sandbox")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallSandboxPage(*props))
}

func (h *Handler) InstallComponentsPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "components")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallComponentsPage(*props))
}

func (h *Handler) InstallRolesPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "roles")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallRolesPage(*props))
}

func (h *Handler) InstallPoliciesPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "policies")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallPoliciesPage(*props))
}

func (h *Handler) InstallAppInfoPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "readme")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallAppInfoPage(*props))
}

func (h *Handler) InstallAuditPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "audit")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallAuditPage(*props))
}

// buildInputFields constructs input field rows from current values and raw config metadata.
func (h *Handler) buildInputFields(currentInputs *nuonmodels.AppInstallInputs, rawConfig map[string]interface{}) []partials.InputFieldRow {
	type fieldMeta struct {
		displayName string
		sensitive   bool
	}
	meta := map[string]fieldMeta{}
	if rawConfig != nil {
		if topInputs, ok := rawConfig["inputs"].([]interface{}); ok {
			for _, inp := range topInputs {
				if im, ok := inp.(map[string]interface{}); ok {
					name, _ := im["name"].(string)
					dn, _ := im["display_name"].(string)
					sensitive, _ := im["sensitive"].(bool)
					if name != "" {
						meta[name] = fieldMeta{displayName: dn, sensitive: sensitive}
					}
				}
			}
		}
	}

	var fields []partials.InputFieldRow
	if currentInputs != nil {
		for k, v := range currentInputs.Values {
			m := meta[k]
			fields = append(fields, partials.InputFieldRow{
				Name:        k,
				DisplayName: m.displayName,
				Value:       v,
				Sensitive:   m.sensitive,
			})
		}
	}
	return fields
}
