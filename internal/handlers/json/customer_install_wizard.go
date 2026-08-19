package jsonhandlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	localmodels "github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	wizardpartials "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials/wizard"
	workflowpartials "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials/workflows"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon/sdks/nuon-go/models"
	"gorm.io/gorm"
)

type CustomerInstallWizardHandler struct {
	db                  *gorm.DB
	customerBaseURL     string
	subdomainBaseDomain string
	nuonAPIURL          string
}

func NewCustomerInstallWizardHandler(db *gorm.DB, customerBaseURL, subdomainBaseDomain, nuonAPIURL string) *CustomerInstallWizardHandler {
	return &CustomerInstallWizardHandler{
		db:                  db,
		customerBaseURL:     customerBaseURL,
		subdomainBaseDomain: subdomainBaseDomain,
		nuonAPIURL:          nuonAPIURL,
	}
}

type customerWizardStateResponse struct {
	App        customerWizardAppResponse      `json:"app"`
	WorkflowID string                         `json:"workflow_id,omitempty"`
	Form       *customerWizardFormResponse    `json:"form,omitempty"`
	Workflow   *customerWizardWorkflowSummary `json:"workflow,omitempty"`
}

type customerWizardAppResponse struct {
	AppID       string `json:"app_id"`
	DisplayName string `json:"display_name"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
	LogoLight   string `json:"logo_light"`
	LogoDark    string `json:"logo_dark"`
	Platform    string `json:"platform"`
}

type customerWizardFormResponse struct {
	InstallName string                             `json:"install_name,omitempty"`
	Platform    string                             `json:"platform"`
	InputGroups []customerWizardInputGroupResponse `json:"input_groups"`
}

type customerWizardInputGroupResponse struct {
	Name        string                             `json:"name"`
	DisplayName string                             `json:"display_name"`
	Description string                             `json:"description"`
	Inputs      []customerWizardInputFieldResponse `json:"inputs"`
}

type customerWizardInputFieldResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive"`
	Index       int    `json:"index"`
}

type customerWizardWorkflowSummary struct {
	Install             customerWizardInstallResponse      `json:"install"`
	Groups              []customerWizardGroupResponse      `json:"groups"`
	PolicyTotals        customerWizardPolicyTotalsResponse `json:"policy_totals"`
	PlanSummary         *customerWizardPlanSummaryResponse `json:"plan_summary,omitempty"`
	StackSetup          *customerWizardStackSetupResponse  `json:"stack_setup,omitempty"`
	HasApprovalAwaiting bool                               `json:"has_approval_awaiting"`
	IsStepComplete      bool                               `json:"is_step_complete"`
	IsStepError         bool                               `json:"is_step_error"`
	NextStep            string                             `json:"next_step,omitempty"`
	OverviewPath        string                             `json:"overview_path,omitempty"`
}

type customerWizardStackSetupResponse struct {
	Platform           string `json:"platform"`
	CloudFormationLink string `json:"cloudformation_link,omitempty"`
	TemplateURL        string `json:"template_url,omitempty"`
	StackName          string `json:"stack_name,omitempty"`
	Region             string `json:"region,omitempty"`
	TfvarsContent      string `json:"tfvars_content,omitempty"`
	NuonInstallID      string `json:"nuon_install_id,omitempty"`
	AzureTemplateURL   string `json:"azure_template_url,omitempty"`
	AzureLocation      string `json:"azure_location,omitempty"`
}

type customerWizardInstallResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Region    string    `json:"region"`
	CreatedAt time.Time `json:"created_at"`
}

type customerWizardGroupResponse struct {
	ID                    string                             `json:"id"`
	Title                 string                             `json:"title"`
	Domain                string                             `json:"domain"`
	Status                string                             `json:"status"`
	ActiveStepName        string                             `json:"active_step_name,omitempty"`
	ActiveStepDescription string                             `json:"active_step_description,omitempty"`
	CanApprove            bool                               `json:"can_approve"`
	CanRetry              bool                               `json:"can_retry"`
	Approval              *customerWizardApprovalResponse    `json:"approval,omitempty"`
	RetryStepID           string                             `json:"retry_step_id,omitempty"`
	PlanSummary           *customerWizardPlanSummaryResponse `json:"plan_summary,omitempty"`
	Steps                 []customerWizardGroupStepResponse  `json:"steps"`
	ImageURL              string                             `json:"image_url,omitempty"`
	ImageTag              string                             `json:"image_tag,omitempty"`
	TargetStatus          string                             `json:"target_status,omitempty"`
	TargetDescription     string                             `json:"target_description,omitempty"`
}

type customerWizardApprovalResponse struct {
	StepID     string `json:"step_id"`
	ApprovalID string `json:"approval_id"`
	Type       string `json:"type"`
}

type customerWizardGroupStepResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ExecutionType   string `json:"execution_type"`
	Status          string `json:"status"`
	StatusHumanDesc string `json:"status_human_description,omitempty"`
	Retryable       bool   `json:"retryable"`
	Finished        bool   `json:"finished"`
}

type customerWizardPolicyTotalsResponse struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Deny int `json:"deny"`
}

type customerWizardPlanSummaryResponse struct {
	Kind         string `json:"kind"`
	CreateCount  int    `json:"create_count"`
	UpdateCount  int    `json:"update_count"`
	DeleteCount  int    `json:"delete_count"`
	ReplaceCount int    `json:"replace_count"`
	AddCount     int    `json:"add_count"`
	ChangeCount  int    `json:"change_count"`
	DestroyCount int    `json:"destroy_count"`
	TotalChanges int    `json:"total_changes"`
}

type customerWizardCreateRequest struct {
	Name     string            `json:"name"`
	Region   string            `json:"region"`
	Location string            `json:"location"`
	Inputs   map[string]string `json:"inputs"`
}

type customerWizardCreateResponse struct {
	InstallID   string `json:"install_id"`
	WorkflowID  string `json:"workflow_id"`
	CurrentStep string `json:"current_step"`
	NextURL     string `json:"next_url"`
}

type customerWizardFormData struct {
	platform    string
	inputGroups []customerWizardInputGroupResponse
}

func (h *CustomerInstallWizardHandler) State(c *gin.Context) {
	step := c.DefaultQuery("step", "inputs")
	if step == "confirm" || step == "preparing" {
		step = "inputs"
	}

	org, err := h.getOrgForCustomerPortal(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	appID := c.Param("app_id")
	publishedApp, err := h.getPublishedApp(org.ID, appID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found or not published"})
		return
	}

	if publishedApp.Status == localmodels.AppStatusComingSoon {
		c.JSON(http.StatusForbidden, gin.H{"error": "This app is not yet available for installation"})
		return
	}

	client, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	ctx := c.Request.Context()
	appName, platform, summary := h.resolveAppDetails(ctx, client, appID, publishedApp)

	response := customerWizardStateResponse{
		App: customerWizardAppResponse{
			AppID:       appID,
			DisplayName: appName,
			Summary:     summary,
			Status:      publishedApp.Status,
			LogoLight:   publishedApp.LogoLightBase64,
			LogoDark:    publishedApp.LogoDarkBase64,
			Platform:    platform,
		},
	}

	if step == "inputs" {
		formData, err := h.getPublishedAppFormData(c, org, appID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		response.Form = &customerWizardFormResponse{
			Platform:    formData.platform,
			InputGroups: formData.inputGroups,
		}

		if installID := c.Query("install_id"); installID != "" {
			var install localmodels.Install
			if err := h.db.Where("id = ?", installID).First(&install).Error; err == nil {
				response.Form.InstallName = install.Name
			}
		}

		c.JSON(http.StatusOK, response)
		return
	}

	user := middleware.GetCurrentUser(c)
	install, err := h.getInstallForWizard(c, user, org.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found or access denied"})
		return
	}

	workflowID := strings.TrimSpace(c.Query("workflow_id"))
	if workflowID == "" {
		workflowID, err = h.resolveLatestWorkflowID(ctx, client, install.NuonInstallID)
		if err != nil {
			response.Workflow = &customerWizardWorkflowSummary{
				Install: customerWizardInstallResponse{
					ID:        install.ID,
					Name:      install.Name,
					Status:    string(install.Status),
					Region:    install.Region,
					CreatedAt: install.CreatedAt,
				},
				Groups:              []customerWizardGroupResponse{},
				PolicyTotals:        customerWizardPolicyTotalsResponse{},
				HasApprovalAwaiting: false,
				IsStepComplete:      false,
				IsStepError:         false,
				NextStep:            nextWizardStep(step),
				OverviewPath:        "/installs/" + install.ID + "/overview",
			}
			c.JSON(http.StatusOK, response)
			return
		}
		response.WorkflowID = workflowID
	}

	allGroups, err := client.GetWorkflowStepGroups(ctx, workflowID)
	if err != nil {
		latestWorkflowID, latestErr := h.resolveLatestWorkflowID(ctx, client, install.NuonInstallID)
		if latestErr == nil && latestWorkflowID != "" && latestWorkflowID != workflowID {
			workflowID = latestWorkflowID
			response.WorkflowID = workflowID
			allGroups, err = client.GetWorkflowStepGroups(ctx, workflowID)
		}
		if err != nil {
			response.Workflow = &customerWizardWorkflowSummary{
				Install: customerWizardInstallResponse{
					ID:        install.ID,
					Name:      install.Name,
					Status:    string(install.Status),
					Region:    install.Region,
					CreatedAt: install.CreatedAt,
				},
				Groups:              []customerWizardGroupResponse{},
				PolicyTotals:        customerWizardPolicyTotalsResponse{},
				HasApprovalAwaiting: false,
				IsStepComplete:      false,
				IsStepError:         false,
				NextStep:            nextWizardStep(step),
				OverviewPath:        "/installs/" + install.ID + "/overview",
			}
			c.JSON(http.StatusOK, response)
			return
		}
	}

	componentNames, configByComponent := h.getComponentMetadata(ctx, client, appID)
	wizardpartials.BackfillStepGroupLabels(allGroups, componentNames)

	activeGroups := filterGroupsForStep(step, allGroups)
	noOpComponentsStep := isNoOpComponentsStep(step, activeGroups, allGroups)
	policyReports, planSummary, groupPlans, targetStatuses := h.buildWorkflowDecorations(ctx, client, step, appID, install, activeGroups)
	pass, warn, deny := policyTotals(policyReports)

	workflow := &customerWizardWorkflowSummary{
		Install: customerWizardInstallResponse{
			ID:        install.ID,
			Name:      install.Name,
			Status:    string(install.Status),
			Region:    install.Region,
			CreatedAt: install.CreatedAt,
		},
		Groups:              make([]customerWizardGroupResponse, 0, len(activeGroups)),
		PolicyTotals:        customerWizardPolicyTotalsResponse{Pass: pass, Warn: warn, Deny: deny},
		PlanSummary:         planSummary,
		HasApprovalAwaiting: hasApprovalAwaiting(activeGroups),
		IsStepComplete:      isStepComplete(activeGroups) || noOpComponentsStep,
		IsStepError:         isStepError(activeGroups),
		NextStep:            nextWizardStep(step),
		OverviewPath:        "/installs/" + install.ID + "/overview",
	}

	if step == "stack" {
		if stackSetup := h.buildStackSetup(ctx, client, install, platform, allGroups); stackSetup != nil {
			workflow.StackSetup = stackSetup
		}
	}

	for _, group := range activeGroups {
		groupPlan := groupPlans[group.ID]
		responseGroup := mapGroupResponse(group, groupPlan, targetStatuses[group.ID], configByComponent[group.Labels["component_name"]])
		workflow.Groups = append(workflow.Groups, responseGroup)
	}

	response.Workflow = workflow
	response.WorkflowID = workflowID

	c.JSON(http.StatusOK, response)
}

func (h *CustomerInstallWizardHandler) buildStackSetup(
	ctx context.Context,
	client *nuon.Client,
	install *localmodels.Install,
	platform string,
	groups []nuon.WorkflowStepGroup,
) *customerWizardStackSetupResponse {
	hasAwaitInstallStackStep := false
	for _, group := range groups {
		for _, step := range group.Steps {
			if step != nil && strings.EqualFold(strings.TrimSpace(step.Name), "await install stack") {
				hasAwaitInstallStackStep = true
				break
			}
		}
		if hasAwaitInstallStackStep {
			break
		}
	}

	if !hasAwaitInstallStackStep {
		return nil
	}

	setup := &customerWizardStackSetupResponse{
		Platform:      platform,
		NuonInstallID: install.NuonInstallID,
	}

	stack, err := client.GetInstallStack(ctx, install.NuonInstallID)
	if err != nil || stack == nil || len(stack.Versions) == 0 {
		return setup
	}

	version := stack.Versions[0]
	if version.TemplateURL != "" {
		setup.TemplateURL = version.TemplateURL
	}

	switch platform {
	case "gcp":
		setup.TfvarsContent = parseWizardTfvars(version.Contents)
		return setup
	case "azure":
		setup.AzureTemplateURL = version.TemplateURL
		setup.AzureLocation = install.Region
		return setup
	default:
		setup.CloudFormationLink = version.QuickLinkURL
		setup.StackName = "nuon-" + install.NuonInstallID
		setup.Region = install.Region

		if setup.CloudFormationLink != "" {
			if parsed, parseErr := url.Parse(setup.CloudFormationLink); parseErr == nil {
				fragment := parsed.Fragment
				if idx := strings.Index(fragment, "?"); idx >= 0 {
					if values, queryErr := url.ParseQuery(fragment[idx+1:]); queryErr == nil {
						if stackName := values.Get("stackName"); stackName != "" {
							setup.StackName = stackName
						}
						if region := values.Get("region"); region != "" {
							setup.Region = region
						}
					}
				}
			}
		}

		if setup.Region == "" {
			setup.Region = "us-east-1"
		}
		return setup
	}
}

func parseWizardTfvars(contents interface{}) string {
	if contents == nil {
		return ""
	}

	var raw interface{}
	switch value := contents.(type) {
	case string:
		if err := json.Unmarshal([]byte(value), &raw); err != nil {
			decoded, decodeErr := base64.StdEncoding.DecodeString(value)
			if decodeErr != nil {
				decoded, decodeErr = base64.RawStdEncoding.DecodeString(value)
			}
			if decodeErr != nil {
				return ""
			}
			if err := json.Unmarshal(decoded, &raw); err != nil {
				return ""
			}
		}
	case map[string]interface{}:
		raw = value
	case json.RawMessage:
		if err := json.Unmarshal(value, &raw); err != nil {
			return ""
		}
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		if err := json.Unmarshal(encoded, &raw); err != nil {
			return ""
		}
	}

	mapped, ok := raw.(map[string]interface{})
	if !ok {
		return ""
	}

	tfvars, ok := mapped["tfvars"]
	if !ok {
		return ""
	}

	return fmt.Sprintf("%v", tfvars)
}

func (h *CustomerInstallWizardHandler) Create(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	resolvedUser, err := h.ensureLocalWizardUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve local user"})
		return
	}

	org, err := h.getOrgForCustomerPortal(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	appID := c.Param("app_id")
	publishedApp, err := h.getPublishedApp(org.ID, appID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found or not published"})
		return
	}

	if publishedApp.Status == localmodels.AppStatusComingSoon {
		c.JSON(http.StatusForbidden, gin.H{"error": "This app is not yet available for installation"})
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req customerWizardCreateRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Install name is required"})
		return
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err == nil {
		req.Inputs = extractWizardBracketInputs(raw, req.Inputs)
	}

	client, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	ctx := c.Request.Context()
	appName, platform, _ := h.resolveAppDetails(ctx, client, appID, publishedApp)
	region := strings.TrimSpace(req.Region)
	location := strings.TrimSpace(req.Location)
	if region == "" && location == "" && platform != "gcp" && platform != "azure" {
		region = "us-east-1"
	}

	mergedInputs := h.mergeDefaultInputs(ctx, client, appID, org.ID, req.Inputs)
	nuonInstall, err := client.CreateInstallWithCustomName(ctx, appID, appName, req.Name, region, location, platform, mergedInputs)
	if err != nil {
		status, message := buildWizardCreateInstallError(err, req.Name)
		c.JSON(status, gin.H{"error": message})
		return
	}

	install := &localmodels.Install{
		OrgID:             org.ID,
		UserID:            resolvedUser.ID,
		InstallLinkID:     nil,
		NuonAppID:         appID,
		AppName:           appName,
		NuonInstallID:     nuonInstall.ID,
		Name:              req.Name,
		Status:            localmodels.StatusPending,
		Region:            region,
		Visibility:        localmodels.VisibilityAccount,
		CreatedByVendorID: nil,
	}

	if selected := h.selectCustomerAccountMember(c, resolvedUser.ID, org.ID); selected != nil {
		install.CustomerAccountID = &selected.AccountID
	}

	if err := h.db.Create(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store install locally"})
		return
	}

	workflowID, ok := waitForWizardWorkflow(ctx, client, nuonInstall.ID, 30*time.Second)
	if workflowID == "" || !ok {
		c.JSON(http.StatusCreated, customerWizardCreateResponse{
			InstallID:   install.ID,
			WorkflowID:  workflowID,
			CurrentStep: "stack",
			NextURL:     fmt.Sprintf("/apps/%s/install?step=stack&install_id=%s", appID, install.ID),
		})
		return
	}

	c.JSON(http.StatusCreated, customerWizardCreateResponse{
		InstallID:   install.ID,
		WorkflowID:  workflowID,
		CurrentStep: "stack",
		NextURL:     fmt.Sprintf("/apps/%s/install?step=stack&install_id=%s&workflow_id=%s", appID, install.ID, workflowID),
	})
}

func buildWizardCreateInstallError(err error, installName string) (int, string) {
	if nuon.IsConflict(err) {
		name := strings.TrimSpace(installName)
		if name == "" {
			return http.StatusConflict, "An install with that name already exists. Please choose a different name."
		}
		return http.StatusConflict, fmt.Sprintf("Install name %q is already in use. Please choose a different name.", name)
	}

	parsed := nuon.ParseAPIError(err)
	errText := err.Error()
	if parsed.Description != "" && (strings.Contains(errText, "Error:") || strings.Contains(errText, "Description:")) {
		return http.StatusInternalServerError, parsed.Description
	}

	return http.StatusInternalServerError, "Unable to create this install right now. Please try again."
}

func (h *CustomerInstallWizardHandler) ensureLocalWizardUser(user *localmodels.User) (*localmodels.User, error) {
	if user == nil {
		return nil, fmt.Errorf("missing authenticated user claims")
	}

	claimID := strings.TrimSpace(user.ID)
	email := strings.TrimSpace(user.Email)
	name := strings.TrimSpace(user.Name)
	role := user.Role

	if claimID == "" && email == "" {
		return nil, fmt.Errorf("missing authenticated user claims")
	}

	// Prefer email reconciliation first to match legacy behavior where the
	// authenticated principal maps to an existing local user row.
	if email != "" {
		var byEmail localmodels.User
		err := h.db.Unscoped().Where("email = ?", email).First(&byEmail).Error
		if err == nil {
			updates := map[string]interface{}{
				"name":       name,
				"role":       role,
				"deleted_at": nil,
			}
			if uErr := h.db.Model(&byEmail).Updates(updates).Error; uErr != nil {
				return nil, uErr
			}
			byEmail.Name = name
			byEmail.Role = role
			byEmail.DeletedAt = gorm.DeletedAt{}
			return &byEmail, nil
		}
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
	}

	if claimID != "" {
		var byID localmodels.User
		err := h.db.Unscoped().Where("id = ?", claimID).First(&byID).Error
		if err == nil {
			updates := map[string]interface{}{
				"name":       name,
				"role":       role,
				"deleted_at": nil,
			}
			if email != "" {
				updates["email"] = email
			}
			if uErr := h.db.Model(&byID).Updates(updates).Error; uErr != nil {
				return nil, uErr
			}
			byID.Name = name
			if email != "" {
				byID.Email = email
			}
			byID.Role = role
			byID.DeletedAt = gorm.DeletedAt{}
			return &byID, nil
		}
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
	}

	if email == "" {
		return nil, fmt.Errorf("missing authenticated user email")
	}

	newUser := &localmodels.User{
		Name:  name,
		Email: email,
		Role:  role,
	}

	// Only trust the claim ID when it matches the local short ID format.
	if shortid.IsValid(claimID) {
		newUser.ID = claimID
	}

	if cErr := h.db.Create(newUser).Error; cErr != nil {
		return nil, cErr
	}

	return newUser, nil
}

func (h *CustomerInstallWizardHandler) getOrgForCustomerPortal(c *gin.Context) (*localmodels.NuonOrg, error) {
	subdomain := strings.TrimSpace(c.Query("subdomain"))
	if subdomain == "" {
		subdomainValue, exists := c.Get("subdomain")
		if !exists {
			return nil, gorm.ErrRecordNotFound
		}
		contextSubdomain, ok := subdomainValue.(string)
		if !ok {
			return nil, gorm.ErrRecordNotFound
		}
		subdomain = strings.TrimSpace(contextSubdomain)
	}
	if subdomain == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var org localmodels.NuonOrg
	if err := h.db.Where("subdomain = ?", subdomain).First(&org).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

func (h *CustomerInstallWizardHandler) getPublishedApp(orgID, appID string) (*localmodels.PublishedApp, error) {
	var publishedApp localmodels.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ? AND status IN ?", orgID, appID, []string{localmodels.AppStatusPublished, localmodels.AppStatusComingSoon}).First(&publishedApp).Error; err != nil {
		return nil, err
	}
	return &publishedApp, nil
}

func (h *CustomerInstallWizardHandler) resolveAppDetails(ctx context.Context, client *nuon.Client, appID string, publishedApp *localmodels.PublishedApp) (string, string, string) {
	appName := appID
	platform := "aws"
	summary := markdownSummary(publishedApp.OverviewMarkdown)

	app, err := client.GetApp(ctx, appID)
	if err != nil || app == nil {
		return appName, platform, summary
	}

	if app.DisplayName != "" {
		appName = app.DisplayName
	} else if app.Name != "" {
		appName = app.Name
	}

	if app.RunnerConfig != nil {
		switch string(app.RunnerConfig.AppRunnerType) {
		case "gcp":
			platform = "gcp"
		case "azure", "azure-aks", "azure-acs":
			platform = "azure"
		default:
			platform = "aws"
		}
	}

	return appName, platform, summary
}

func (h *CustomerInstallWizardHandler) getPublishedAppFormData(c *gin.Context, org *localmodels.NuonOrg, appID string) (*customerWizardFormData, error) {
	client, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		return nil, err
	}

	app, err := client.GetApp(c.Request.Context(), appID)
	if err != nil {
		return nil, err
	}

	platform := "aws"
	if app != nil && app.RunnerConfig != nil {
		switch string(app.RunnerConfig.AppRunnerType) {
		case "gcp":
			platform = "gcp"
		case "azure", "azure-aks", "azure-acs":
			platform = "azure"
		}
	}

	inputConfig, err := client.GetAppInputConfig(c.Request.Context(), appID)
	if err != nil {
		inputConfig = nil
	}

	var localConfig localmodels.AppInputConfig
	configExists := h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&localConfig).Error == nil
	customerInputNames := localConfig.GetCustomerInputNames()
	if !configExists {
		customerInputNames = nil
	}
	groupOrder := localConfig.GetGroupOrder()
	inputOrder := localConfig.GetInputOrder()

	jsonBytes, err := json.Marshal(inputConfig)
	if err == nil {
		var configMap map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
			inputConfig = filterWizardInputConfigByLocalConfig(configMap, customerInputNames)
			inputConfig = applyWizardInputOrdering(inputConfig, groupOrder, inputOrder)
		}
	}

	return &customerWizardFormData{
		platform:    platform,
		inputGroups: parseWizardInputGroups(inputConfig),
	}, nil
}

func (h *CustomerInstallWizardHandler) getInstallForWizard(c *gin.Context, user *localmodels.User, orgID string) (*localmodels.Install, error) {
	installID := strings.TrimSpace(c.Query("install_id"))
	if installID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	selected := h.selectCustomerAccountMember(c, user.ID, orgID)
	query := h.db.Where("id = ? AND org_id = ? AND user_id = ?", installID, orgID, user.ID)
	if selected != nil {
		query = h.db.Where("id = ? AND org_id = ? AND ((user_id = ?) OR (customer_account_id = ? AND visibility = ?))",
			installID, orgID, user.ID, selected.AccountID, localmodels.VisibilityAccount,
		)
	}

	var install localmodels.Install
	if err := query.First(&install).Error; err != nil {
		return nil, err
	}
	return &install, nil
}

func (h *CustomerInstallWizardHandler) selectCustomerAccountMember(c *gin.Context, userID, orgID string) *localmodels.CustomerAccountMember {
	if member := middleware.GetCustomerAccountMember(c); member != nil {
		return member
	}

	members := middleware.GetCustomerAccounts(c)
	if len(members) == 0 {
		h.db.Preload("Account").Where("user_id = ? AND org_id = ? AND deleted_at IS NULL", userID, orgID).Find(&members)
	}
	if len(members) == 0 {
		return nil
	}
	return middleware.SelectActiveMember(c, members)
}

func (h *CustomerInstallWizardHandler) resolveLatestWorkflowID(ctx context.Context, client *nuon.Client, nuonInstallID string) (string, error) {
	workflows, _, err := client.GetInstallWorkflowsV2(ctx, nuonInstallID, 0, 1)
	if err != nil || len(workflows) == 0 || workflows[0] == nil || workflows[0].ID == "" {
		return "", fmt.Errorf("latest workflow not found")
	}
	return workflows[0].ID, nil
}

func (h *CustomerInstallWizardHandler) getComponentMetadata(ctx context.Context, client *nuon.Client, appID string) ([]string, map[string]customerWizardComponentConfig) {
	componentNames := []string{}
	configByName := map[string]customerWizardComponentConfig{}
	compNameToID := make(map[string]string)

	appComponents, err := client.GetAppComponents(ctx, appID)
	if err == nil {
		for _, comp := range appComponents {
			componentNames = append(componentNames, comp.Name)
			compNameToID[comp.Name] = comp.ID
		}
	}

	app, err := client.GetApp(ctx, appID)
	if err != nil || app == nil || len(app.AppConfigs) == 0 {
		return componentNames, configByName
	}

	config, err := client.GetAppConfigFull(ctx, appID, app.AppConfigs[0].ID)
	if err != nil || config == nil {
		return componentNames, configByName
	}

	for _, conn := range config.ComponentConfigConnections {
		if conn == nil || conn.ExternalImage == nil {
			continue
		}
		for compName, compID := range compNameToID {
			if compID == conn.ComponentID {
				configByName[compName] = customerWizardComponentConfig{
					ImageURL: conn.ExternalImage.ImageURL,
					ImageTag: conn.ExternalImage.Tag,
				}
			}
		}
	}

	return componentNames, configByName
}

type customerWizardComponentConfig struct {
	ImageURL string
	ImageTag string
}

func (h *CustomerInstallWizardHandler) buildWorkflowDecorations(ctx context.Context, client *nuon.Client, step, appID string, install *localmodels.Install, groups []nuon.WorkflowStepGroup) ([]nuon.PolicyReport, *customerWizardPlanSummaryResponse, map[string]*customerWizardPlanSummaryResponse, map[string]wizardpartials.TargetStatus) {
	groupPlans := map[string]*customerWizardPlanSummaryResponse{}
	targetStatuses := map[string]wizardpartials.TargetStatus{}
	var planSummary *customerWizardPlanSummaryResponse
	var policyReports []nuon.PolicyReport

	if len(groups) == 0 {
		return policyReports, planSummary, groupPlans, targetStatuses
	}

	if step == "sandbox" || step == "components" {
		ownerType := "install_sandbox_runs"
		if step == "components" {
			ownerType = "install_deploys"
		}
		reports, err := client.GetInstallPolicyReports(ctx, install.NuonInstallID, ownerType)
		if err == nil {
			policyReports = reports
		}
	}

	for _, group := range groups {
		for _, stepItem := range group.Steps {
			if stepItem == nil || stepItem.Approval == nil {
				continue
			}
			raw, err := client.GetApprovalContents(ctx, stepItem.WorkflowID, stepItem.ID, stepItem.Approval.ID)
			if err != nil || raw == nil {
				continue
			}

			switch stepItem.Approval.Type {
			case "helm_approval":
				plan := workflowpartials.ParseHelmPlan(raw)
				groupPlans[group.ID] = helmPlanSummaryResponse(plan)
			case "kubernetes_manifest_approval":
				plan := workflowpartials.ParseKubernetesPlan(raw)
				groupPlans[group.ID] = helmPlanSummaryResponse(plan)
			default:
				plan := workflowpartials.ParseTerraformPlan(raw)
				groupPlans[group.ID] = terraformPlanSummaryResponse(plan)
			}
		}

		if step == "components" {
			if ts, ok := fetchWizardComponentTargetStatus(ctx, client, install, group); ok {
				targetStatuses[group.ID] = ts
			}
		}
	}

	if step == "sandbox" && len(groups) > 0 {
		planSummary = groupPlans[groups[0].ID]
	}

	return policyReports, planSummary, groupPlans, targetStatuses
}

func fetchWizardComponentTargetStatus(ctx context.Context, apiClient *nuon.Client, install *localmodels.Install, group nuon.WorkflowStepGroup) (wizardpartials.TargetStatus, bool) {
	var targetStep *nuon.WorkflowStep
	for _, step := range group.Steps {
		if step == nil || step.StepTargetID == "" {
			continue
		}
		switch step.StepTargetType {
		case "install_deploys", "install_sandbox_runs", "install_action_workflow_runs":
			targetStep = step
		}
	}
	if targetStep == nil {
		return wizardpartials.TargetStatus{}, false
	}

	var statusV2 *nuonmodels.AppCompositeStatus
	switch targetStep.StepTargetType {
	case "install_deploys":
		if deploy, err := apiClient.GetInstallDeploy(ctx, install.NuonInstallID, targetStep.StepTargetID); err == nil && deploy != nil {
			statusV2 = deploy.StatusV2
		}
	case "install_sandbox_runs":
		if run, err := apiClient.GetInstallSandboxRun(ctx, install.NuonInstallID, targetStep.StepTargetID); err == nil && run != nil {
			statusV2 = run.StatusV2
		}
	case "install_action_workflow_runs":
		if run, err := apiClient.GetInstallActionWorkflowRun(ctx, install.NuonInstallID, targetStep.StepTargetID); err == nil && run != nil {
			statusV2 = run.StatusV2
		}
	}
	if statusV2 == nil || len(statusV2.History) == 0 {
		return wizardpartials.TargetStatus{}, false
	}
	last := statusV2.History[len(statusV2.History)-1]
	if last == nil || string(last.Status) != "error" {
		return wizardpartials.TargetStatus{}, false
	}
	return wizardpartials.TargetStatus{Status: string(last.Status), Description: last.StatusHumanDescription}, true
}

func filterGroupsForStep(step string, groups []nuon.WorkflowStepGroup) []nuon.WorkflowStepGroup {
	filtered := make([]nuon.WorkflowStepGroup, 0, len(groups))
	for _, group := range groups {
		if wizardpartials.MatchesWizardStep(step, group) {
			filtered = append(filtered, group)
		}
	}
	return filtered
}

func isNoOpComponentsStep(step string, activeGroups, allGroups []nuon.WorkflowStepGroup) bool {
	return step == "components" && len(activeGroups) == 0 && aggregateWizardStatus(allGroups) == "completed"
}

func aggregateWizardStatus(groups []nuon.WorkflowStepGroup) string {
	if len(groups) == 0 {
		return ""
	}
	hasActive := false
	for _, group := range groups {
		switch wizardpartials.GroupStatus(group) {
		case "error":
			return "error"
		case "approval-awaiting":
			return "approval-awaiting"
		case "completed", "success":
		default:
			hasActive = true
		}
	}
	if hasActive {
		return "in-progress"
	}
	return "completed"
}

func hasApprovalAwaiting(groups []nuon.WorkflowStepGroup) bool {
	for _, group := range groups {
		for _, step := range group.Steps {
			if step != nil && step.Approval != nil && step.Approval.Response == nil {
				return true
			}
		}
	}
	return false
}

func isStepComplete(groups []nuon.WorkflowStepGroup) bool {
	status := aggregateWizardStatus(groups)
	return status == "completed" || status == "success"
}

func isStepError(groups []nuon.WorkflowStepGroup) bool {
	return aggregateWizardStatus(groups) == "error"
}

func mapGroupResponse(group nuon.WorkflowStepGroup, plan *customerWizardPlanSummaryResponse, targetStatus wizardpartials.TargetStatus, componentConfig customerWizardComponentConfig) customerWizardGroupResponse {
	title := group.Labels["display_name"]
	if title == "" {
		title = group.Labels["component_name"]
	}
	if title == "" {
		title = group.Labels["name"]
	}
	if title == "" {
		title = group.Name
	}

	response := customerWizardGroupResponse{
		ID:                group.ID,
		Title:             title,
		Domain:            group.Labels["domain"],
		Status:            wizardpartials.GroupStatus(group),
		CanApprove:        wizardpartials.GroupCanApprove(group),
		CanRetry:          wizardpartials.GroupCanRetry(group),
		RetryStepID:       wizardpartials.RetryableStepID(group),
		PlanSummary:       plan,
		Steps:             make([]customerWizardGroupStepResponse, 0, len(group.Steps)),
		ImageURL:          componentConfig.ImageURL,
		ImageTag:          componentConfig.ImageTag,
		TargetStatus:      targetStatus.Status,
		TargetDescription: targetStatus.Description,
	}

	if activeStep := wizardpartials.ActiveStep(group); activeStep != nil {
		response.ActiveStepName = activeStep.Name
		if activeStep.Status != nil {
			response.ActiveStepDescription = activeStep.Status.StatusHumanDescription
		}
	}

	if activeStep := wizardpartials.ActiveStep(group); activeStep != nil && activeStep.Approval != nil && activeStep.Status != nil && activeStep.Status.Status == "approval-awaiting" {
		response.Approval = &customerWizardApprovalResponse{
			StepID:     activeStep.ID,
			ApprovalID: activeStep.Approval.ID,
			Type:       activeStep.Approval.Type,
		}
	}

	for _, step := range group.Steps {
		if step == nil {
			continue
		}
		stepResponse := customerWizardGroupStepResponse{
			ID:            step.ID,
			Name:          step.Name,
			ExecutionType: step.ExecutionType,
			Retryable:     step.Retryable,
			Finished:      step.Finished,
		}
		if step.Status != nil {
			stepResponse.Status = step.Status.Status
			stepResponse.StatusHumanDesc = step.Status.StatusHumanDescription
		}
		response.Steps = append(response.Steps, stepResponse)
	}

	return response
}

func policyTotals(reports []nuon.PolicyReport) (int, int, int) {
	pass, warn, deny := 0, 0, 0
	for _, report := range reports {
		pass += report.PassCount
		warn += report.WarnCount
		deny += report.DenyCount
	}
	return pass, warn, deny
}

func terraformPlanSummaryResponse(plan *workflowpartials.PlanSummary) *customerWizardPlanSummaryResponse {
	if plan == nil {
		return nil
	}
	return &customerWizardPlanSummaryResponse{
		Kind:         "terraform",
		CreateCount:  plan.CreateCount,
		UpdateCount:  plan.UpdateCount,
		DeleteCount:  plan.DeleteCount,
		ReplaceCount: plan.ReplaceCount,
		TotalChanges: plan.TotalChanges(),
	}
}

func helmPlanSummaryResponse(plan *workflowpartials.HelmPlanSummary) *customerWizardPlanSummaryResponse {
	if plan == nil {
		return nil
	}
	return &customerWizardPlanSummaryResponse{
		Kind:         "helm",
		AddCount:     plan.AddCount,
		ChangeCount:  plan.ChangeCount,
		DestroyCount: plan.DestroyCount,
		TotalChanges: plan.TotalChanges(),
	}
}

func markdownSummary(markdown string) string {
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return ""
	}
	lines := strings.Split(markdown, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if trimmed != "" {
			return trimmed
		}
	}
	return markdown
}

func nextWizardStep(current string) string {
	steps := []string{"inputs", "stack", "sandbox", "components"}
	for index, step := range steps {
		if step == current && index+1 < len(steps) {
			return steps[index+1]
		}
	}
	return ""
}

func waitForWizardWorkflow(ctx context.Context, client *nuon.Client, installID string, timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	tick := 500 * time.Millisecond
	var workflowID string

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return workflowID, false
		}
		workflows, _, err := client.GetInstallWorkflowsV2(ctx, installID, 0, 1)
		if err == nil && len(workflows) > 0 && workflows[0] != nil && workflows[0].ID != "" {
			workflowID = workflows[0].ID
			break
		}
		time.Sleep(tick)
	}
	if workflowID == "" {
		return "", false
	}

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return workflowID, false
		}
		groups, err := client.GetWorkflowStepGroups(ctx, workflowID)
		if err == nil {
			for _, group := range groups {
				if len(group.Steps) > 0 {
					return workflowID, true
				}
			}
		}
		time.Sleep(tick)
	}

	return workflowID, false
}

func extractWizardBracketInputs(raw map[string]interface{}, existing map[string]string) map[string]string {
	if existing == nil {
		existing = make(map[string]string)
	}
	for key, value := range raw {
		if len(key) <= 7 || !strings.HasPrefix(key, "inputs[") || key[len(key)-1] != ']' {
			continue
		}
		name := key[7 : len(key)-1]
		switch typed := value.(type) {
		case string:
			existing[name] = typed
		case []interface{}:
			if len(typed) > 0 {
				if last, ok := typed[len(typed)-1].(string); ok {
					existing[name] = last
				}
			}
		}
	}
	return existing
}

func (h *CustomerInstallWizardHandler) mergeDefaultInputs(ctx context.Context, client *nuon.Client, appID, orgID string, inputs map[string]string) map[string]string {
	result := make(map[string]string)
	for key, value := range inputs {
		result[key] = value
	}

	inputConfig, err := client.GetAppInputConfig(ctx, appID)
	if err != nil || inputConfig == nil {
		return result
	}

	customerInputSet := make(map[string]bool)
	var localConfig localmodels.AppInputConfig
	if err := h.db.Where("org_id = ? AND app_id = ?", orgID, appID).First(&localConfig).Error; err == nil {
		for _, name := range localConfig.GetCustomerInputNames() {
			customerInputSet[name] = true
		}
	}

	jsonBytes, err := json.Marshal(inputConfig)
	if err != nil {
		return result
	}
	var configMap map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &configMap); err != nil {
		return result
	}

	groups, _ := configMap["input_groups"].([]interface{})
	for _, rawGroup := range groups {
		groupMap, _ := rawGroup.(map[string]interface{})
		appInputs, _ := groupMap["app_inputs"].([]interface{})
		for _, rawInput := range appInputs {
			inputMap, _ := rawInput.(map[string]interface{})
			name, _ := inputMap["name"].(string)
			defaultValue, _ := inputMap["default"].(string)
			if name == "" || defaultValue == "" {
				continue
			}
			if !customerInputSet[name] && result[name] == "" {
				result[name] = defaultValue
			}
		}
	}

	return result
}

func filterWizardInputConfigByLocalConfig(configMap map[string]interface{}, customerInputNames []string) interface{} {
	if customerInputNames == nil {
		return configMap
	}
	allowed := make(map[string]bool, len(customerInputNames))
	for _, name := range customerInputNames {
		allowed[name] = true
	}

	groups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return configMap
	}

	filteredGroups := make([]interface{}, 0, len(groups))
	for _, rawGroup := range groups {
		groupMap, ok := rawGroup.(map[string]interface{})
		if !ok {
			continue
		}
		appInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok {
			continue
		}
		filteredInputs := make([]interface{}, 0, len(appInputs))
		for _, rawInput := range appInputs {
			inputMap, ok := rawInput.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := inputMap["name"].(string)
			if allowed[name] {
				filteredInputs = append(filteredInputs, rawInput)
			}
		}
		if len(filteredInputs) == 0 {
			continue
		}
		filteredGroup := map[string]interface{}{}
		for key, value := range groupMap {
			filteredGroup[key] = value
		}
		filteredGroup["app_inputs"] = filteredInputs
		filteredGroups = append(filteredGroups, filteredGroup)
	}

	result := map[string]interface{}{}
	for key, value := range configMap {
		result[key] = value
	}
	result["input_groups"] = filteredGroups
	return result
}

func applyWizardInputOrdering(inputConfig interface{}, groupOrder []string, inputOrder map[string][]string) interface{} {
	if inputConfig == nil || (len(groupOrder) == 0 && len(inputOrder) == 0) {
		return inputConfig
	}
	configMap, ok := inputConfig.(map[string]interface{})
	if !ok {
		return inputConfig
	}
	groups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return inputConfig
	}

	groupOrderMap := map[string]int{}
	for index, name := range groupOrder {
		groupOrderMap[name] = index
	}
	inputOrderMap := map[string]map[string]int{}
	for groupName, inputNames := range inputOrder {
		inputOrderMap[groupName] = map[string]int{}
		for index, inputName := range inputNames {
			inputOrderMap[groupName][inputName] = index
		}
	}

	sort.SliceStable(groups, func(i, j int) bool {
		groupI, _ := groups[i].(map[string]interface{})
		groupJ, _ := groups[j].(map[string]interface{})
		nameI, _ := groupI["name"].(string)
		nameJ, _ := groupJ["name"].(string)
		orderI, okI := groupOrderMap[nameI]
		orderJ, okJ := groupOrderMap[nameJ]
		switch {
		case okI && okJ:
			return orderI < orderJ
		case okI:
			return true
		case okJ:
			return false
		default:
			return nameI < nameJ
		}
	})

	for _, rawGroup := range groups {
		groupMap, ok := rawGroup.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := groupMap["name"].(string)
		appInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok {
			continue
		}
		orders := inputOrderMap[name]
		sort.SliceStable(appInputs, func(i, j int) bool {
			inputI, _ := appInputs[i].(map[string]interface{})
			inputJ, _ := appInputs[j].(map[string]interface{})
			nameI, _ := inputI["name"].(string)
			nameJ, _ := inputJ["name"].(string)
			orderI, okI := orders[nameI]
			orderJ, okJ := orders[nameJ]
			switch {
			case okI && okJ:
				return orderI < orderJ
			case okI:
				return true
			case okJ:
				return false
			default:
				return nameI < nameJ
			}
		})
		groupMap["app_inputs"] = appInputs
	}

	configMap["input_groups"] = groups
	return configMap
}

func parseWizardInputGroups(inputConfig interface{}) []customerWizardInputGroupResponse {
	if inputConfig == nil {
		return nil
	}
	configMap, ok := inputConfig.(map[string]interface{})
	if !ok {
		return nil
	}
	rawGroups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return nil
	}

	groups := make([]customerWizardInputGroupResponse, 0, len(rawGroups))
	for _, rawGroup := range rawGroups {
		groupMap, ok := rawGroup.(map[string]interface{})
		if !ok {
			continue
		}
		group := customerWizardInputGroupResponse{
			Name:        wizardStrVal(groupMap, "name"),
			DisplayName: wizardStrVal(groupMap, "display_name"),
			Description: wizardStrVal(groupMap, "description"),
		}
		rawInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok || len(rawInputs) == 0 {
			continue
		}
		group.Inputs = make([]customerWizardInputFieldResponse, 0, len(rawInputs))
		for _, rawInput := range rawInputs {
			inputMap, ok := rawInput.(map[string]interface{})
			if !ok {
				continue
			}
			group.Inputs = append(group.Inputs, customerWizardInputFieldResponse{
				Name:        wizardStrVal(inputMap, "name"),
				DisplayName: wizardStrVal(inputMap, "display_name"),
				Description: wizardStrVal(inputMap, "description"),
				Type:        wizardStrVal(inputMap, "type"),
				Default:     wizardStrVal(inputMap, "default"),
				Required:    wizardBoolVal(inputMap, "required"),
				Sensitive:   wizardBoolVal(inputMap, "sensitive"),
				Index:       wizardIntVal(inputMap, "index"),
			})
		}
		sort.Slice(group.Inputs, func(i, j int) bool {
			return group.Inputs[i].Index < group.Inputs[j].Index
		})
		groups = append(groups, group)
	}

	return groups
}

func wizardStrVal(values map[string]interface{}, key string) string {
	value, _ := values[key].(string)
	return value
}

func wizardBoolVal(values map[string]interface{}, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func wizardIntVal(values map[string]interface{}, key string) int {
	value, ok := values[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func (h *CustomerInstallWizardHandler) nuonAPIURLForOrg(org *localmodels.NuonOrg) string {
	if org != nil && strings.TrimSpace(org.APIURL) != "" {
		return org.APIURL
	}
	return h.nuonAPIURL
}
