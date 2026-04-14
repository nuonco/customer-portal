package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials/workflows"
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
	partial := c.Query("partial")
	isHTMX := c.GetHeader("HX-Request") != ""

	// Fast path: HTMX shimmer shell. Skip all API calls — build props from DB only.
	if partial == "" && isHTMX {
		h.renderOverviewShimmer(c)
		return
	}

	// All other paths need full page props (includes API calls for install check + app name).
	pageProps, err := h.buildInstallPageProps(c, "overview")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	install := pageProps.Install
	nuonOrg := install.GetNuonOrg()

	switch partial {
	case "content":
		overviewProps := h.buildOverviewData(c, pageProps, install, nuonOrg)
		overviewProps.HasData = true
		h.RenderTempl(c, http.StatusOK, customerpages.OverviewContent(overviewProps))
	case "panel":
		overviewProps := h.buildOverviewData(c, pageProps, install, nuonOrg)
		overviewProps.HasData = true
		h.RenderTempl(c, http.StatusOK, customerpages.OverviewWorkflowPanel(overviewProps))
	default:
		// No-JS fallback: full data fetch
		overviewProps := h.buildOverviewData(c, pageProps, install, nuonOrg)
		overviewProps.HasData = true
		h.RenderTempl(c, http.StatusOK, customerpages.InstallOverviewPage(overviewProps))
	}
}

// renderOverviewShimmer serves the overview page shell with shimmer placeholders.
// No API calls — only DB reads for install data and theme.
func (h *Handler) renderOverviewShimmer(c *gin.Context) {
	user := h.tryGetLoggedInUser(c)

	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}
	install := installInterface.(*models.Install)

	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	acctActive, acctOthers := h.getCustomerAccountsFromContext(c)

	layoutProps := h.buildCustomerLayoutProps(install.Name, user, theme, h.getOrgForLayout(c), acctActive, acctOthers)
	layoutProps.HasPublishedApps = h.orgHasPublishedApps(orgID)
	layoutProps.ActiveNav = "installs"
	layoutProps.CurrentInstallID = install.ID
	layoutProps.ActiveTab = "overview"

	shellProps := customerpages.InstallOverviewPageProps{
		LayoutProps:     layoutProps,
		Install:         install,
		APIDeletedError: install.APIDeleted,
		AlertType:       c.Query("alert_type"),
		AlertMsg:        c.Query("alert_msg"),
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallOverviewPage(shellProps))
}

// buildOverviewData fetches all overview summary data in parallel and returns populated props.
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

	if nuonOrg == nil || nuonOrg.APIToken == "" {
		return overviewProps
	}

	apiClient, apiErr := nuon.NewClientWithURL(
		nuonOrg.APIToken,
		nuonOrg.NuonOrgID,
		h.nuonAPIURLForOrg(nuonOrg),
	)
	if apiErr != nil {
		return overviewProps
	}

	ctx := c.Request.Context()
	var wg sync.WaitGroup

	// Stack summary
	var stackStatus, stackRegion, stackAccountID string
	wg.Add(1)
	go func() {
		defer wg.Done()
		if stack, err := apiClient.GetInstallStack(ctx, install.NuonInstallID); err == nil && stack != nil {
			status := ""
			if len(stack.Versions) > 0 && stack.Versions[0].CompositeStatus != nil {
				status = string(stack.Versions[0].CompositeStatus.Status)
			}
			if status != "queued" {
				stackStatus = status
				if outputs := stack.InstallStackOutputs; outputs != nil && outputs.Aws != nil {
					stackRegion = outputs.Aws.Region
					stackAccountID = outputs.Aws.AccountID
				}
			}
		}
	}()

	// Sandbox summary (status + config fetched together since config depends on status)
	var sandboxStatus, sandboxRepo, sandboxBranch string
	var sandboxRepoPublic bool
	wg.Add(1)
	go func() {
		defer wg.Done()
		nuonInst, err := apiClient.GetInstall(ctx, install.NuonInstallID)
		if err != nil || nuonInst == nil || nuonInst.Sandbox == nil || nuonInst.Sandbox.Status == "queued" {
			return
		}
		sandboxStatus = nuonInst.Sandbox.Status
		if cfg, err := apiClient.GetAppSandboxLatestConfig(ctx, install.GetAppID()); err == nil && cfg != nil {
			if gh := cfg.ConnectedGithubVcsConfig; gh != nil {
				sandboxRepo = gh.Repo
				sandboxBranch = gh.Branch
			} else if pg := cfg.PublicGitVcsConfig; pg != nil {
				sandboxRepo = pg.Repo
				sandboxBranch = pg.Branch
				sandboxRepoPublic = true
			}
		}
	}()

	// Components summary
	var components []partials.ComponentInfo
	wg.Add(1)
	go func() {
		defer wg.Done()
		if comps, err := apiClient.GetInstallComponents(ctx, install.NuonInstallID); err == nil {
			components = h.buildComponentInfos(ctx, apiClient, install, comps)
		}
	}()

	// Recent workflows
	var recentWorkflows []*nuonmodels.AppWorkflow
	wg.Add(1)
	go func() {
		defer wg.Done()
		if wfs, _, err := apiClient.GetInstallWorkflows(ctx, install.NuonInstallID, 0, 20); err == nil {
			recentWorkflows = wfs
		}
	}()

	// Current input values
	var inputFields []partials.InputFieldRow
	wg.Add(1)
	go func() {
		defer wg.Done()
		currentInputs, _ := apiClient.GetInstallCurrentInputs(ctx, install.NuonInstallID)
		rawConfig, _ := apiClient.GetAppInputConfigRaw(ctx, install.GetAppID())
		inputFields = h.buildInputFields(currentInputs, rawConfig)
	}()

	wg.Wait()

	overviewProps.StackStatus = stackStatus
	overviewProps.StackRegion = stackRegion
	overviewProps.StackAccountID = stackAccountID
	overviewProps.SandboxStatus = sandboxStatus
	overviewProps.SandboxRepo = sandboxRepo
	overviewProps.SandboxBranch = sandboxBranch
	overviewProps.SandboxRepoPublic = sandboxRepoPublic
	overviewProps.Components = components
	overviewProps.RecentWorkflows = recentWorkflows
	overviewProps.InputFields = inputFields

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

	partial := c.Query("partial")

	// For partial=panel requests (clicking a workflow row), we only need workflow data, not overview data.
	// For partial=content, we need overview data but not workflow data.
	// For full page loads, we need both (or shimmer for HTMX).

	var overviewProps customerpages.InstallOverviewPageProps

	needsOverviewData := partial == "content" || partial == "" && c.GetHeader("HX-Request") == ""
	if needsOverviewData {
		overviewProps = h.buildOverviewData(c, pageProps, install, nuonOrg)
		overviewProps.HasData = true
	} else {
		overviewProps = customerpages.InstallOverviewPageProps{
			LayoutProps:     pageProps.LayoutProps,
			Install:         install,
			AppName:         pageProps.AppName,
			APIDeletedError: pageProps.APIDeletedError,
			AlertType:       c.Query("alert_type"),
			AlertMsg:        c.Query("alert_msg"),
		}
	}
	overviewProps.Expanded = c.Query("expanded") == "true"

	// Fetch workflow detail data (needed for panel and full page, but not shimmer or content)
	workflowID := c.Param("workflow_id")
	if nuonOrg != nil && nuonOrg.APIToken != "" && workflowID != "" && partial != "content" && partial != "panel-shimmer" {
		apiClient, apiErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURLForOrg(nuonOrg),
		)
		if apiErr == nil {
			ctx := c.Request.Context()
			workflow, wfErr := apiClient.GetWorkflow(ctx, workflowID)
			if wfErr == nil && workflow != nil {
				panel := BuildWorkflowDataPanel(workflow)
				panel.CurrentStepRole = resolveCurrentStepRole(ctx, apiClient, install.NuonInstallID, workflow)
				overviewProps.SelectedWorkflow = &panel

				// Build WorkflowOverview props
				stepGroups := workflows.BuildStepGroups(workflow.Steps)
				selectedGroup := -1
				if groupParam := c.Query("group"); groupParam != "" {
					fmt.Sscanf(groupParam, "%d", &selectedGroup)
				}
				if selectedGroup < 0 || selectedGroup >= len(stepGroups) {
					selectedGroup = autoSelectGroup(stepGroups)
				}
				platform := h.detectPlatform(ctx, apiClient, install)

				var stackSetup workflows.StackSetupData
				if selectedGroup >= 0 && selectedGroup < len(stepGroups) && stepGroups[selectedGroup].Type == workflows.StepGroupStack {
					stackSetup = h.getStackSetupData(ctx, apiClient, install, workflow)
				}

				overviewProps.WorkflowOverview = &workflows.WorkflowOverviewProps{
					StepGroups:       stepGroups,
					SelectedGroup:    selectedGroup,
					InstallID:        install.ID,
					BasePath:         h.basePath,
					WorkflowID:       workflowID,
					WorkflowFinished: workflow.Finished,
					ShowApproveAll:   string(workflow.ApprovalOption) == "prompt" && !workflow.Finished,
					Platform:         platform,
					StackSetup:       stackSetup,
					PageBaseURL:      fmt.Sprintf("%s/installs/%s/overview/workflows/%s", h.basePath, install.ID, workflowID),
					PanelPartialURL:  fmt.Sprintf("%s/installs/%s/overview/workflows/%s?partial=panel", h.basePath, install.ID, workflowID),
					Expanded:         overviewProps.Expanded,
				}
			}
		}
	}

	// Fetch theme for primary color
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)
	overviewProps.PrimaryColor = primaryColor

	switch partial {
	case "panel-shimmer":
		h.RenderTempl(c, http.StatusOK, customerpages.OverviewWorkflowPanelShimmer(overviewProps, workflowID))
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
