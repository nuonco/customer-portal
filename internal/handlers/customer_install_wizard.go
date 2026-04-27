package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials/wizard"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials/workflows"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon-go/models"
)

// fetchComponentTargetStatus fetches the most recent step target for a component
// step group and returns the latest entry from its status_v2.history.
func (h *Handler) fetchComponentTargetStatus(ctx context.Context, apiClient *nuon.Client, install *models.Install, g nuon.WorkflowStepGroup) (wizard.TargetStatus, bool) {
	var targetStep *nuon.WorkflowStep
	for _, s := range g.Steps {
		if s == nil || s.StepTargetID == "" {
			continue
		}
		switch s.StepTargetType {
		case "install_deploys", "install_sandbox_runs", "install_action_workflow_runs":
			targetStep = s
		}
	}
	if targetStep == nil {
		return wizard.TargetStatus{}, false
	}

	var statusV2 *nuonmodels.AppCompositeStatus
	switch targetStep.StepTargetType {
	case "install_deploys":
		if d, err := apiClient.GetInstallDeploy(ctx, install.NuonInstallID, targetStep.StepTargetID); err == nil && d != nil {
			statusV2 = d.StatusV2
		}
	case "install_sandbox_runs":
		if r, err := apiClient.GetInstallSandboxRun(ctx, install.NuonInstallID, targetStep.StepTargetID); err == nil && r != nil {
			statusV2 = r.StatusV2
		}
	case "install_action_workflow_runs":
		if r, err := apiClient.GetInstallActionWorkflowRun(ctx, install.NuonInstallID, targetStep.StepTargetID); err == nil && r != nil {
			statusV2 = r.StatusV2
		}
	}
	if statusV2 == nil || len(statusV2.History) == 0 {
		return wizard.TargetStatus{}, false
	}
	last := statusV2.History[len(statusV2.History)-1]
	if last == nil {
		return wizard.TargetStatus{}, false
	}
	// Only surface the v2 status when the target has actually failed —
	// the history array contains every transition, so for in-flight
	// components the tail can be a stale "in-progress" entry that does
	// not reflect the component's true current state.
	if string(last.Status) != "error" {
		return wizard.TargetStatus{}, false
	}
	return wizard.TargetStatus{
		Status:      string(last.Status),
		Description: last.StatusHumanDescription,
	}, true
}

// wizardSteps defines the ordered wizard steps.
var wizardSteps = []string{"inputs", "stack", "sandbox", "components"}

// nextWizardStep returns the step after the given one, or "" if it's the last.
func nextWizardStep(current string) string {
	for i, s := range wizardSteps {
		if s == current && i+1 < len(wizardSteps) {
			return wizardSteps[i+1]
		}
	}
	return ""
}

// wizardStepIndex returns the 0-based index of a step name.
func wizardStepIndex(step string) int {
	for i, s := range wizardSteps {
		if s == step {
			return i
		}
	}
	return 0
}

// InstallWizardPage handles all install wizard steps (inputs, stack, sandbox, components).
func (h *Handler) InstallWizardPage(c *gin.Context) {
	appID := c.Param("app_id")
	step := c.Query("step")
	if step == "" {
		step = "inputs"
	}
	installID := c.Query("install_id")
	workflowID := c.Query("workflow_id")
	partial := c.Query("partial")
	wizardTab := c.Query("wizard_tab")

	loggedInUser := h.tryGetLoggedInUser(c)

	ctx := c.Request.Context()
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	acctActive, acctOthers := h.getCustomerAccountsFromContext(c)

	// --- Inputs step: may have no install yet ---
	if step == "inputs" {
		h.installWizardInputsStep(c, appID, installID, workflowID, partial, loggedInUser, theme, acctActive, acctOthers)
		return
	}

	// --- Confirm / Preparing step: GET-only renders fall back to Configure (form
	// data lives in localStorage and the POST-driven flow renders the step inline).
	if step == "confirm" || step == "preparing" {
		c.Redirect(http.StatusFound, h.basePath+"/apps/"+appID+"/install")
		return
	}

	// --- Deployment steps (stack, sandbox, components): require an install ---
	if loggedInUser == nil {
		c.Redirect(http.StatusFound, h.basePath+"/login")
		return
	}

	var install models.Install
	if err := h.db.Where("id = ?", installID).First(&install).Error; err != nil {
		h.renderWizardError(c, appID, loggedInUser, "Install not found")
		return
	}

	if err := h.loadInstallWithOrg(&install); err != nil {
		h.renderWizardError(c, appID, loggedInUser, "Failed to load install data")
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil || nuonOrg.APIToken == "" {
		h.renderWizardError(c, appID, loggedInUser, "Organization configuration error")
		return
	}

	apiClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
	if err != nil {
		h.renderWizardError(c, appID, loggedInUser, "Failed to connect to API")
		return
	}

	// Fetch app components up front so we can backfill step group labels
	// (the API doesn't populate them in prod yet) and reuse the lookup
	// below for the components step.
	compNameToID := make(map[string]string)
	componentNames := []string{}
	if appComps, err := apiClient.GetAppComponents(ctx, appID); err == nil {
		for _, comp := range appComps {
			compNameToID[comp.Name] = comp.ID
			componentNames = append(componentNames, comp.Name)
		}
	}

	// Fetch step groups from the API
	var stepGroups []nuon.WorkflowStepGroup
	var activeGroups []nuon.WorkflowStepGroup

	allGroups, sgErr := apiClient.GetWorkflowStepGroups(ctx, workflowID)
	if sgErr == nil {
		wizard.BackfillStepGroupLabels(allGroups, componentNames)
		stepGroups = allGroups

		for i := range stepGroups {
			if wizard.MatchesWizardStep(step, stepGroups[i]) {
				activeGroups = append(activeGroups, stepGroups[i])
			}
		}
	}

	// Detect platform and get stack setup data for the stack step
	var platform string
	var stackSetup workflows.StackSetupData
	if step == "stack" && len(allGroups) > 0 {
		var allSteps []*nuon.WorkflowStep
		for _, g := range allGroups {
			allSteps = append(allSteps, g.Steps...)
		}
		platform = h.detectPlatform(ctx, apiClient, &install)
		stackSetup = h.getStackSetupDataFromSteps(ctx, apiClient, &install, allSteps)
	}

	// Fetch workspace resources and outputs for sandbox apply card
	var applyResources []nuon.TerraformResource
	var applyOutputs map[string]string
	if step == "sandbox" {
		if nuonInstall, err := apiClient.GetInstall(ctx, install.NuonInstallID); err == nil &&
			nuonInstall != nil && nuonInstall.Sandbox != nil &&
			nuonInstall.Sandbox.TerraformWorkspace != nil {
			workspaceID := nuonInstall.Sandbox.TerraformWorkspace.ID
			if states, err := apiClient.GetTerraformWorkspaceStates(ctx, workspaceID); err == nil && len(states) > 0 {
				stateID := states[0].ID
				if resources, err := apiClient.GetTerraformWorkspaceStateResources(ctx, workspaceID, stateID); err == nil {
					applyResources = resources
				}
				if outputs, err := apiClient.GetTerraformWorkspaceStateOutputs(ctx, workspaceID, stateID); err == nil {
					applyOutputs = outputs
				}
			}
		}
	}

	// Fetch plan summary and policy reports for sandbox/components approval steps
	var planSummary *workflows.PlanSummary
	var policyReports []nuon.PolicyReport
	componentData := make(map[string]*wizard.ComponentWizardData)
	targetStatus := make(map[string]wizard.TargetStatus)
	if (step == "sandbox" || step == "components") && len(activeGroups) > 0 {
		if step == "components" {
			configByCompID := make(map[string]*nuonmodels.AppComponentConfigConnection)
			if len(compNameToID) > 0 {
				if appObj, err := apiClient.GetApp(ctx, appID); err == nil && appObj != nil && len(appObj.AppConfigs) > 0 {
					if cfg, err := apiClient.GetAppConfigFull(ctx, appID, appObj.AppConfigs[0].ID); err == nil && cfg != nil {
						for _, conn := range cfg.ComponentConfigConnections {
							if conn != nil {
								configByCompID[conn.ComponentID] = conn
							}
						}
					}
				}
			}

			for _, g := range activeGroups {
				cd := &wizard.ComponentWizardData{}

				compName := g.Labels["component_name"]
				if compID, ok := compNameToID[compName]; ok {
					if cc, ok := configByCompID[compID]; ok {
						if cc.ExternalImage != nil {
							cd.ImageURL = cc.ExternalImage.ImageURL
							cd.ImageTag = cc.ExternalImage.Tag
						}
					}
				}

				for _, s := range g.Steps {
					if s.Approval != nil {
						cd.ApprovalType = s.Approval.Type
						raw, err := apiClient.GetApprovalContents(ctx, workflowID, s.ID, s.Approval.ID)
						if err == nil && raw != nil {
							switch s.Approval.Type {
							case "helm_approval", "kubernetes_manifest_approval":
								if s.Approval.Type == "helm_approval" {
									cd.HelmPlan = workflows.ParseHelmPlan(raw)
								} else {
									cd.HelmPlan = workflows.ParseKubernetesPlan(raw)
								}
							default:
								cd.TerraformPlan = workflows.ParseTerraformPlan(raw)
							}
						}
					}
				}
				componentData[g.ID] = cd

				if ts, ok := h.fetchComponentTargetStatus(ctx, apiClient, &install, g); ok {
					targetStatus[g.ID] = ts
				}
			}
		} else {
			for _, g := range activeGroups {
				for _, s := range g.Steps {
					if s.Approval != nil {
						raw, err := apiClient.GetApprovalContents(ctx, workflowID, s.ID, s.Approval.ID)
						if err == nil && raw != nil {
							planSummary = workflows.ParseTerraformPlan(raw)
						}
					}
				}
			}
		}
		ownerType := "install_sandbox_runs"
		if step == "components" {
			ownerType = "install_deploys"
		}
		reports, err := apiClient.GetInstallPolicyReports(ctx, install.NuonInstallID, ownerType)
		if err == nil {
			resolvePolicyReportNames(ctx, apiClient, appID, reports)
			policyReports = reports
		}
	}

	// Get app name
	appName := appID
	app, appErr := apiClient.GetApp(ctx, appID)
	if appErr == nil && app != nil {
		if dn := appDisplayName(app); dn != "" {
			appName = dn
		}
	}

	// Build props
	layoutProps := h.buildCustomerLayoutProps("Install "+appName, loggedInUser, theme, h.getOrgForLayout(c), acctActive, acctOthers)
	layoutProps.HasPublishedApps = h.orgHasPublishedApps(orgID)
	layoutProps.ActiveNav = "apps"
	layoutProps.SidebarMinimized = true

	primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)

	props := customerpages.InstallWizardProps{
		LayoutProps:    layoutProps,
		CurrentStep:    step,
		AppID:          appID,
		AppName:        appName,
		InstallID:      installID,
		WorkflowID:     workflowID,
		StepGroups:     stepGroups,
		ActiveGroups:   activeGroups,
		Platform:       platform,
		StackSetup:     stackSetup,
		PrimaryColor:   primaryColor,
		PlanSummary:    planSummary,
		PolicyReports:  policyReports,
		WizardTab:      wizardTab,
		ApplyResources: applyResources,
		ApplyOutputs:   applyOutputs,
		ComponentData:  componentData,
		TargetStatus:   targetStatus,
	}

	if props.WizardTab == "" {
		props.WizardTab = "resources"
	}

	switch partial {
	case "poll":
		h.RenderTempl(c, http.StatusOK, customerpages.WizardStepContent(props))
	case "content":
		h.RenderTempl(c, http.StatusOK, customerpages.WizardStepContentWithIndicator(props))
	default:
		h.RenderTempl(c, http.StatusOK, customerpages.InstallWizardPage(props))
	}
}

// installWizardInputsStep handles the inputs/configure step of the wizard.
// This step may be visited before an install exists (first visit from app catalog).
func (h *Handler) installWizardInputsStep(c *gin.Context, appID, installID, workflowID, partial string, loggedInUser *models.User, theme *models.AppTheme, acctActive *models.CustomerAccount, acctOthers []models.CustomerAccount) {
	ctx := c.Request.Context()

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		h.renderWizardError(c, appID, loggedInUser, "Organization not found")
		return
	}

	// Verify app is published
	var publishedApp models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ? AND status IN ?", org.ID, appID, []string{models.AppStatusPublished, models.AppStatusComingSoon}).First(&publishedApp).Error; err != nil {
		h.renderWizardError(c, appID, loggedInUser, "App not found or not published")
		return
	}

	if publishedApp.Status == "coming_soon" {
		c.Redirect(http.StatusFound, h.basePath+"/apps/"+appID)
		return
	}

	// Fetch app and org names from Nuon API
	appName := appID
	orgName := org.NuonOrgID
	var apiCallFailed bool
	nuonClient, clientErr := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if clientErr == nil {
		app, appErr := nuonClient.GetApp(ctx, appID)
		if appErr != nil {
			apiCallFailed = true
		} else if app != nil {
			if dn := appDisplayName(app); dn != "" {
				appName = dn
			}
		}
		apiOrg, orgErr := nuonClient.GetOrg(ctx)
		if orgErr == nil && apiOrg != nil && apiOrg.Name != "" {
			orgName = apiOrg.Name
		}
	}

	layoutProps := h.buildCustomerLayoutProps("Install "+appName, loggedInUser, theme, h.getOrgForLayout(c), acctActive, acctOthers)
	layoutProps.HasPublishedApps = h.orgHasPublishedApps(org.ID)
	layoutProps.ActiveNav = "apps"
	layoutProps.SidebarMinimized = true
	if clientErr != nil || apiCallFailed {
		layoutProps.NuonAPIError = "The app is experiencing network issues, and is not able to access app or install data. Please contact support for assistance."
	}

	var authURL string
	if loggedInUser == nil {
		if subdomain, ok := c.Get("subdomain"); ok && subdomain != nil && subdomain.(string) != "" {
			redirect := url.QueryEscape(h.basePath + "/apps/" + appID + "/install")
			authURL = fmt.Sprintf("%s/auth/login?return_to=%s&redirect=%s", h.customerBaseURL, subdomain.(string), redirect)
		}
	}

	// Build form props
	formProps := &wizard.ConfigureStepProps{
		LayoutProps:     layoutProps,
		AppID:           appID,
		AppName:         appName,
		OrgName:         orgName,
		LogoLightBase64: publishedApp.LogoLightBase64,
		LogoDarkBase64:  publishedApp.LogoDarkBase64,
		LoggedInUser:    loggedInUser,
		AuthURL:         authURL,
	}

	// If an install exists (returning from wizard), fetch install name and workflow data
	var stepGroups []nuon.WorkflowStepGroup
	if installID != "" {
		formProps.InstallID = installID
		formProps.WorkflowID = workflowID
		var install models.Install
		if err := h.db.Where("id = ?", installID).First(&install).Error; err == nil {
			formProps.InstallName = install.Name
		}
		if workflowID != "" && clientErr == nil {
			if allGroups, sgErr := nuonClient.GetWorkflowStepGroups(ctx, workflowID); sgErr == nil {
				stepGroups = allGroups
				formProps.StepGroups = allGroups
			}
		}
	}

	props := customerpages.InstallWizardProps{
		LayoutProps: layoutProps,
		CurrentStep: "inputs",
		AppID:       appID,
		AppName:     appName,
		InstallID:   installID,
		WorkflowID:  workflowID,
		StepGroups:  stepGroups,
		FormProps:   formProps,
	}

	switch partial {
	case "poll":
		h.RenderTempl(c, http.StatusOK, customerpages.WizardStepContent(props))
	case "content":
		h.RenderTempl(c, http.StatusOK, customerpages.WizardStepContentWithIndicator(props))
	default:
		h.RenderTempl(c, http.StatusOK, customerpages.InstallWizardPage(props))
	}
}

// renderWizardError renders an error page in the wizard context.
func (h *Handler) renderWizardError(c *gin.Context, appID string, user *models.User, msg string) {
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	acctActive, acctOthers := h.getCustomerAccountsFromContext(c)
	props := customerpages.ErrorPageProps{
		LayoutProps: h.buildCustomerLayoutProps("Error", user, theme, h.getOrgForLayout(c), acctActive, acctOthers),
		Error:       msg,
	}
	h.RenderTempl(c, http.StatusBadRequest, customerpages.ErrorPage(props))
}

// buildWizardURL constructs a wizard step URL.
func buildWizardURL(basePath, appID, step, installID, workflowID string) string {
	return basePath + "/apps/" + appID + "/install?step=" + step + "&install_id=" + installID + "&workflow_id=" + workflowID
}

// buildAppConfirmURL constructs the URL for the install Confirm step (app-install flow).
func buildAppConfirmURL(basePath, appID string) string {
	return basePath + "/apps/" + appID + "/install?step=confirm"
}

// buildLinkConfirmURL constructs the URL for the install Confirm step (install-link flow).
func buildLinkConfirmURL(basePath, sha string) string {
	return basePath + "/install-link/?sha=" + sha + "&step=confirm"
}

// buildAppPreparingURL constructs the URL for the install Preparing step (app-install flow).
func buildAppPreparingURL(basePath, appID string) string {
	return basePath + "/apps/" + appID + "/install?step=preparing"
}

// buildLinkPreparingURL constructs the URL for the install Preparing step (install-link flow).
func buildLinkPreparingURL(basePath, sha string) string {
	return basePath + "/install-link/?sha=" + sha + "&step=preparing"
}
