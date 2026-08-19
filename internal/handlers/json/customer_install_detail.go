package jsonhandlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	localmodels "github.com/nuonco/mono/services/customer-dashboard/internal/models"
	wizardpartials "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials/wizard"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"github.com/nuonco/nuon/sdks/nuon-go/client/operations"
	nuonmodels "github.com/nuonco/nuon/sdks/nuon-go/models"
	"gorm.io/gorm"
)

type CustomerInstallDetailHandler struct {
	db         *gorm.DB
	nuonAPIURL string
}

func NewCustomerInstallDetailHandler(db *gorm.DB, nuonAPIURL string) *CustomerInstallDetailHandler {
	return &CustomerInstallDetailHandler{db: db, nuonAPIURL: nuonAPIURL}
}

type customerInstallDetailResponse struct {
	Install         customerInstallSummaryResponse    `json:"install"`
	App             customerInstallAppSummaryResponse `json:"app"`
	APIDeletedError bool                              `json:"api_deleted_error"`
	NuonAPIError    string                            `json:"nuon_api_error,omitempty"`
	Overview        customerInstallOverviewResponse   `json:"overview"`
	Stack           customerInstallStackResponse      `json:"stack"`
	Sandbox         customerInstallSandboxResponse    `json:"sandbox"`
	Components      customerInstallComponentsResponse `json:"components"`
	Roles           customerInstallRolesResponse      `json:"roles"`
	Policies        customerInstallPoliciesResponse   `json:"policies"`
	Audit           customerInstallAuditResponse      `json:"audit"`
	Readme          customerInstallReadmeResponse     `json:"readme"`
	LegacyBasePath  string                            `json:"legacy_base_path"`
}

type customerInstallSummaryResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Visibility string    `json:"visibility"`
	CreatedAt  time.Time `json:"created_at"`
	Region     string    `json:"region"`
	AppID      string    `json:"app_id"`
	AppName    string    `json:"app_name"`
}

type customerInstallAppSummaryResponse struct {
	DisplayName string `json:"display_name"`
	Summary     string `json:"summary"`
	Platform    string `json:"platform"`
	LogoLight   string `json:"logo_light"`
	LogoDark    string `json:"logo_dark"`
}

type customerInstallOverviewResponse struct {
	Stack           customerInstallStackSummaryResponse   `json:"stack"`
	Sandbox         customerInstallSandboxSummaryResponse `json:"sandbox"`
	Components      []customerInstallComponentResponse    `json:"components"`
	RecentWorkflows []customerInstallWorkflowResponse     `json:"recent_workflows"`
	InputFields     []customerInstallInputFieldResponse   `json:"input_fields"`
}

type customerInstallStackSummaryResponse struct {
	Status    string `json:"status"`
	Region    string `json:"region"`
	AccountID string `json:"account_id"`
}

type customerInstallSandboxSummaryResponse struct {
	Status     string `json:"status"`
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`
	RepoPublic bool   `json:"repo_public"`
}

type customerInstallComponentResponse struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Repo       string `json:"repo"`
	Directory  string `json:"directory"`
	Branch     string `json:"branch"`
	RepoPublic bool   `json:"repo_public"`
}

type customerInstallInputFieldResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Value       string `json:"value"`
	Sensitive   bool   `json:"sensitive"`
}

type customerInstallWorkflowResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	FinishedAt string `json:"finished_at"`
}

type customerInstallStackResponse struct {
	Status     string                      `json:"status"`
	Region     string                      `json:"region"`
	AccountID  string                      `json:"account_id"`
	VPC        string                      `json:"vpc"`
	Outputs    map[string]string           `json:"outputs"`
	RecentRuns []customerInstallRunSummary `json:"recent_runs"`
}

type customerInstallSandboxResponse struct {
	Status     string                      `json:"status"`
	Repo       string                      `json:"repo"`
	Directory  string                      `json:"directory"`
	Branch     string                      `json:"branch"`
	RepoPublic bool                        `json:"repo_public"`
	Outputs    map[string]string           `json:"outputs"`
	RecentRuns []customerInstallRunSummary `json:"recent_runs"`
}

type customerInstallComponentsResponse struct {
	Items   []customerInstallComponentResponse `json:"items"`
	Deploys []customerInstallDeploySummary     `json:"deploys"`
}

type customerInstallRunSummary struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	VersionStatus string `json:"version_status,omitempty"`
	CreatedAt     string `json:"created_at"`
	FinishedAt    string `json:"finished_at"`
}

type customerInstallDeploySummary struct {
	ID          string `json:"id"`
	ComponentID string `json:"component_id"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	FinishedAt  string `json:"finished_at"`
}

type customerInstallRolesResponse struct {
	Roles []customerInstallRoleResponse `json:"roles"`
}

type customerInstallRoleResponse struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Arn  string `json:"arn"`
}

type customerInstallPoliciesResponse struct {
	Totals  customerInstallPolicyTotals     `json:"totals"`
	Items   []customerInstallPolicyResponse `json:"items"`
	Reports []customerInstallPolicyReport   `json:"reports"`
}

type customerInstallPolicyTotals struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Deny int `json:"deny"`
}

type customerInstallPolicyResponse struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Engine string `json:"engine"`
}

type customerInstallPolicyReport struct {
	ID         string `json:"id"`
	PolicyName string `json:"policy_name"`
	PassCount  int    `json:"pass_count"`
	WarnCount  int    `json:"warn_count"`
	DenyCount  int    `json:"deny_count"`
	CreatedAt  string `json:"created_at"`
}

type customerInstallAuditResponse struct {
	Workflows       []customerInstallWorkflowResponse `json:"workflows"`
	ActionWorkflows []customerInstallWorkflowResponse `json:"action_workflows"`
}

type customerInstallReadmeResponse struct {
	Markdown string `json:"markdown"`
}

func (h *CustomerInstallDetailHandler) Detail(c *gin.Context) {
	org, err := h.getOrgForCustomerPortal(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	user := middleware.GetCurrentUser(c)
	install, err := h.getInstallForCustomer(c, user, org.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found or access denied"})
		return
	}

	response := customerInstallDetailResponse{
		Install: customerInstallSummaryResponse{
			ID:         install.ID,
			Name:       install.Name,
			Status:     string(install.Status),
			Visibility: string(install.Visibility),
			CreatedAt:  install.CreatedAt,
			Region:     install.Region,
			AppID:      install.GetAppID(),
			AppName:    install.AppName,
		},
		App: customerInstallAppSummaryResponse{
			DisplayName: install.AppName,
			Summary:     "",
			Platform:    "aws",
		},
		LegacyBasePath: "/installs/" + install.ID,
	}

	var publishedApp localmodels.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ?", org.ID, install.GetAppID()).First(&publishedApp).Error; err == nil {
		if response.App.DisplayName == "" {
			response.App.DisplayName = publishedApp.AppID
		}
		response.App.Summary = markdownSummary(publishedApp.OverviewMarkdown)
		response.App.LogoLight = publishedApp.LogoLightBase64
		response.App.LogoDark = publishedApp.LogoDarkBase64
		response.Readme.Markdown = publishedApp.OverviewMarkdown
	}

	if org.APIToken == "" {
		c.JSON(http.StatusOK, response)
		return
	}

	client, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		response.NuonAPIError = "Unable to initialize Nuon API client."
		c.JSON(http.StatusOK, response)
		return
	}

	ctx := c.Request.Context()

	if apiInstall, apiErr := client.GetInstall(ctx, install.NuonInstallID); apiErr != nil {
		var notFoundErr *operations.GetInstallNotFound
		if errors.As(apiErr, &notFoundErr) {
			response.APIDeletedError = true
		} else {
			response.NuonAPIError = "Unable to load install details from Nuon API."
		}
	} else if apiInstall != nil {
		if apiInstall.Sandbox != nil {
			response.Overview.Sandbox.Status = apiInstall.Sandbox.Status
			response.Sandbox.Status = apiInstall.Sandbox.Status
		}
	}

	h.populateAppDetails(ctx, client, install.GetAppID(), &response)
	h.populateStack(ctx, client, install, &response)
	h.populateSandbox(ctx, client, install, &response)
	h.populateComponents(ctx, client, install, &response)
	h.populateWorkflows(ctx, client, install, &response)
	h.populateInputs(ctx, client, install, &response)
	h.populatePolicies(ctx, client, install, &response)

	c.JSON(http.StatusOK, response)
}

func (h *CustomerInstallDetailHandler) populateAppDetails(
	ctx context.Context,
	client *nuon.Client,
	appID string,
	response *customerInstallDetailResponse,
) {
	app, err := client.GetApp(ctx, appID)
	if err != nil || app == nil {
		return
	}

	if app.DisplayName != "" {
		response.App.DisplayName = app.DisplayName
	} else if app.Name != "" {
		response.App.DisplayName = app.Name
	}

	if app.RunnerConfig != nil {
		switch string(app.RunnerConfig.AppRunnerType) {
		case "gcp":
			response.App.Platform = "gcp"
		case "azure", "azure-aks", "azure-acs":
			response.App.Platform = "azure"
		default:
			response.App.Platform = "aws"
		}
	}
}

func (h *CustomerInstallDetailHandler) populateStack(
	ctx context.Context,
	client *nuon.Client,
	install *localmodels.Install,
	response *customerInstallDetailResponse,
) {
	stack, err := client.GetInstallStack(ctx, install.NuonInstallID)
	if err == nil && stack != nil {
		if len(stack.Versions) > 0 && stack.Versions[0].CompositeStatus != nil {
			response.Overview.Stack.Status = string(stack.Versions[0].CompositeStatus.Status)
			response.Stack.Status = response.Overview.Stack.Status
		}

		if outputs := stack.InstallStackOutputs; outputs != nil {
			if outputs.Aws != nil {
				response.Overview.Stack.Region = outputs.Aws.Region
				response.Overview.Stack.AccountID = outputs.Aws.AccountID
				response.Stack.Region = outputs.Aws.Region
				response.Stack.AccountID = outputs.Aws.AccountID
				response.Stack.VPC = outputs.Aws.VpcID

				roles := []customerInstallRoleResponse{}
				addRole := func(name, roleType, arn string) {
					if arn == "" {
						return
					}
					roles = append(roles, customerInstallRoleResponse{Name: name, Type: roleType, Arn: arn})
				}

				addRole("Provision", "Runner Provision", outputs.Aws.ProvisionIamRoleArn)
				addRole("Deprovision", "Runner Deprovision", outputs.Aws.DeprovisionIamRoleArn)
				addRole("Maintenance", "Runner Maintenance", outputs.Aws.MaintenanceIamRoleArn)
				addRole("Runner", "Runner", outputs.Aws.RunnerIamRoleArn)
				for name, arn := range outputs.Aws.BreakGlassRoleArns {
					addRole(fmt.Sprintf("Breakglass - %s", name), "Breakglass", arn)
				}
				sort.Slice(roles, func(i, j int) bool {
					return roles[i].Name < roles[j].Name
				})
				response.Roles.Roles = roles
			}

			response.Stack.Outputs = prettyDataOutputs(outputs.DataContents, outputs.Data)
		}
	}

	stackRuns, err := client.GetInstallStackRuns(ctx, install.NuonInstallID)
	if err != nil {
		return
	}

	runs := make([]customerInstallRunSummary, 0, len(stackRuns))
	for _, run := range stackRuns {
		runs = append(runs, customerInstallRunSummary{
			ID:            run.ID,
			Status:        run.StatusDescription,
			VersionStatus: run.VersionStatus,
			CreatedAt:     stringifyTime(run.CreatedAt),
			FinishedAt:    stringifyTime(run.UpdatedAt),
		})
	}
	response.Stack.RecentRuns = limitRuns(runs, 20)
}

func (h *CustomerInstallDetailHandler) populateSandbox(
	ctx context.Context,
	client *nuon.Client,
	install *localmodels.Install,
	response *customerInstallDetailResponse,
) {
	if cfg, err := client.GetAppSandboxLatestConfig(ctx, install.GetAppID()); err == nil && cfg != nil {
		if gh := cfg.ConnectedGithubVcsConfig; gh != nil {
			response.Overview.Sandbox.Repo = gh.Repo
			response.Overview.Sandbox.Branch = gh.Branch
			response.Sandbox.Repo = gh.Repo
			response.Sandbox.Directory = gh.Directory
			response.Sandbox.Branch = gh.Branch
		} else if pg := cfg.PublicGitVcsConfig; pg != nil {
			response.Overview.Sandbox.Repo = pg.Repo
			response.Overview.Sandbox.Branch = pg.Branch
			response.Overview.Sandbox.RepoPublic = true
			response.Sandbox.Repo = pg.Repo
			response.Sandbox.Directory = pg.Directory
			response.Sandbox.Branch = pg.Branch
			response.Sandbox.RepoPublic = true
		}
	}

	runs, err := client.GetInstallSandboxRuns(ctx, install.NuonInstallID)
	if err != nil {
		return
	}

	sandboxRuns := make([]customerInstallRunSummary, 0, len(runs))
	for _, run := range runs {
		if run == nil {
			continue
		}
		sandboxRuns = append(sandboxRuns, customerInstallRunSummary{
			ID:         run.ID,
			Status:     run.Status,
			CreatedAt:  stringifyTime(run.CreatedAt),
			FinishedAt: stringifyTime(run.UpdatedAt),
		})
	}
	response.Sandbox.RecentRuns = limitRuns(sandboxRuns, 20)

	if len(runs) > 0 && runs[0] != nil {
		if outputMap := runs[0].Outputs; outputMap != nil {
			response.Sandbox.Outputs = prettyInterfaceOutputs(outputMap)
		}
	}
}

func (h *CustomerInstallDetailHandler) populateComponents(
	ctx context.Context,
	client *nuon.Client,
	install *localmodels.Install,
	response *customerInstallDetailResponse,
) {
	deploys, _ := client.GetInstallDeploys(ctx, install.NuonInstallID)
	items := make([]customerInstallDeploySummary, 0, len(deploys))
	deployStatusByComponentID := map[string]string{}
	for _, deploy := range deploys {
		if deploy == nil {
			continue
		}

		status := deploy.Status
		if deploy.StatusV2 != nil && deploy.StatusV2.Status != "" {
			status = string(deploy.StatusV2.Status)
		}

		items = append(items, customerInstallDeploySummary{
			ID:          deploy.ID,
			ComponentID: deploy.ComponentID,
			Status:      status,
			CreatedAt:   stringifyTime(deploy.CreatedAt),
			FinishedAt:  stringifyTime(deploy.UpdatedAt),
		})

		if deploy.ComponentID != "" {
			if _, exists := deployStatusByComponentID[deploy.ComponentID]; !exists {
				deployStatusByComponentID[deploy.ComponentID] = status
			}
		}
	}

	response.Components.Deploys = limitDeploys(items, 50)

	components, compErr := client.GetInstallComponents(ctx, install.NuonInstallID)
	if compErr == nil && len(components) > 0 {
		infos := h.buildComponentInfos(ctx, client, install, components)
		h.mergeComponentStatusesFromDeploys(components, infos, deployStatusByComponentID)
		response.Overview.Components = infos
		response.Components.Items = infos
		return
	}

	if appComponents, appErr := client.GetAppComponents(ctx, install.GetAppID()); appErr == nil && len(appComponents) > 0 {
		infos := h.buildComponentInfosFromAppComponents(ctx, client, install, appComponents, deployStatusByComponentID)
		response.Overview.Components = infos
		response.Components.Items = infos
		return
	}

	if workflowInfos := h.buildComponentInfosFromLatestWorkflow(ctx, client, install); len(workflowInfos) > 0 {
		response.Overview.Components = workflowInfos
		response.Components.Items = workflowInfos
		return
	}

	fallbackInfos := buildComponentInfosFromDeployStatusMap(deployStatusByComponentID)
	response.Overview.Components = fallbackInfos
	response.Components.Items = fallbackInfos
}

func (h *CustomerInstallDetailHandler) buildComponentInfosFromLatestWorkflow(
	ctx context.Context,
	apiClient *nuon.Client,
	install *localmodels.Install,
) []customerInstallComponentResponse {
	workflows, _, err := apiClient.GetInstallWorkflowsV2(ctx, install.NuonInstallID, 0, 1)
	if err != nil || len(workflows) == 0 || workflows[0] == nil || workflows[0].ID == "" {
		return nil
	}

	groups, err := apiClient.GetWorkflowStepGroups(ctx, workflows[0].ID)
	if err != nil || len(groups) == 0 {
		return nil
	}

	componentTypeByName := map[string]string{}
	componentTypeByID := map[string]string{}
	componentNameByID := map[string]string{}
	componentIDByName := map[string]string{}
	componentNames := []string{}
	if installComponents, installErr := apiClient.GetInstallComponents(ctx, install.NuonInstallID); installErr == nil {
		for _, component := range installComponents {
			if component == nil {
				continue
			}
			if component.ComponentID != "" {
				componentTypeByID[component.ComponentID] = ""
				if component.Component != nil {
					componentTypeByID[component.ComponentID] = string(component.Component.Type)
					if component.Component.Name != "" {
						componentNameByID[component.ComponentID] = component.Component.Name
						componentIDByName[component.Component.Name] = component.ComponentID
						componentNames = append(componentNames, component.Component.Name)
						componentTypeByName[component.Component.Name] = string(component.Component.Type)
					}
				}
			}
		}
	}
	if appComponents, appErr := apiClient.GetAppComponents(ctx, install.GetAppID()); appErr == nil {
		for _, component := range appComponents {
			if component == nil || component.Name == "" {
				continue
			}
			if _, exists := componentTypeByName[component.Name]; !exists {
				componentNames = append(componentNames, component.Name)
			}
			componentTypeByName[component.Name] = string(component.Type)
			if component.ID != "" {
				componentNameByID[component.ID] = component.Name
				componentTypeByID[component.ID] = string(component.Type)
				componentIDByName[component.Name] = component.ID
			}
		}
	}

	wizardpartials.BackfillStepGroupLabels(groups, componentNames)
	return buildComponentInfosFromWorkflowGroups(groups, componentTypeByName, componentTypeByID, componentNameByID, componentIDByName)
}

func buildComponentInfosFromWorkflowGroups(
	groups []nuon.WorkflowStepGroup,
	componentTypeByName map[string]string,
	componentTypeByID map[string]string,
	componentNameByID map[string]string,
	componentIDByName map[string]string,
) []customerInstallComponentResponse {
	if len(groups) == 0 {
		return nil
	}

	infosByKey := map[string]customerInstallComponentResponse{}
	for _, group := range groups {
		if !wizardpartials.MatchesWizardStep("components", group) {
			continue
		}

		componentID := strings.TrimSpace(group.Labels["component_id"])
		if componentID == "" {
			groupName := strings.TrimSpace(group.Name)
			if looksLikeOpaqueWorkflowComponentID(groupName) {
				componentID = groupName
			}
		}
		if componentID == "" {
			groupID := strings.TrimSpace(group.ID)
			if looksLikeOpaqueWorkflowComponentID(groupID) {
				componentID = groupID
			}
		}

		name := strings.TrimSpace(group.Labels["display_name"])
		componentNameLabel := strings.TrimSpace(group.Labels["component_name"])
		if name == "" {
			name = componentNameLabel
		}

		if componentID == "" && componentNameLabel != "" {
			if mappedID := strings.TrimSpace(componentIDByName[componentNameLabel]); mappedID != "" {
				componentID = mappedID
			}
		}
		if componentID == "" && name != "" {
			if mappedID := strings.TrimSpace(componentIDByName[name]); mappedID != "" {
				componentID = mappedID
			}
		}
		if name == "" && componentID != "" {
			if mappedName := strings.TrimSpace(componentNameByID[componentID]); mappedName != "" {
				name = mappedName
			}
		}
		if name == "" {
			candidate := strings.TrimSpace(group.Name)
			if candidate != "" && !looksLikeOpaqueWorkflowComponentID(candidate) {
				name = candidate
			}
		}
		if name == "" {
			name = "Unknown component"
		}

		if componentID == "" {
			if mappedID := strings.TrimSpace(componentIDByName[name]); mappedID != "" {
				componentID = mappedID
			}
		}

		key := componentID
		if key == "" {
			key = name
		}

		existing, hasExisting := infosByKey[key]
		componentType := componentTypeByName[name]
		if componentType == "" && componentID != "" {
			componentType = componentTypeByID[componentID]
		}
		candidate := customerInstallComponentResponse{
			ID:     componentID,
			Name:   name,
			Type:   componentType,
			Status: wizardpartials.GroupStatus(group),
		}

		if !hasExisting || existing.Status == "" || existing.Status == "not_started" || existing.ID == "" {
			infosByKey[key] = candidate
		}
	}

	if len(infosByKey) == 0 {
		return nil
	}

	keys := make([]string, 0, len(infosByKey))
	for key := range infosByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	infos := make([]customerInstallComponentResponse, 0, len(keys))
	for _, key := range keys {
		infos = append(infos, infosByKey[key])
	}

	return infos
}

func looksLikeOpaqueWorkflowComponentID(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "wsg")
}

func (h *CustomerInstallDetailHandler) populateWorkflows(
	ctx context.Context,
	client *nuon.Client,
	install *localmodels.Install,
	response *customerInstallDetailResponse,
) {
	workflows, _, err := client.GetInstallWorkflows(ctx, install.NuonInstallID, 0, 50)
	if err == nil {
		items := buildCustomerInstallWorkflows(workflows)
		response.Overview.RecentWorkflows = limitWorkflows(items, 20)
		response.Audit.Workflows = items
	}

	actionWorkflows, _, err := client.GetInstallWorkflowsByType(ctx, install.NuonInstallID, 0, 50, "action_workflow_run")
	if err == nil {
		response.Audit.ActionWorkflows = buildCustomerInstallWorkflows(actionWorkflows)
	}
}

func (h *CustomerInstallDetailHandler) populateInputs(
	ctx context.Context,
	client *nuon.Client,
	install *localmodels.Install,
	response *customerInstallDetailResponse,
) {
	currentInputs, _ := client.GetInstallCurrentInputs(ctx, install.NuonInstallID)
	rawConfig, _ := client.GetAppInputConfigRaw(ctx, install.GetAppID())

	type fieldMeta struct {
		displayName string
		sensitive   bool
	}

	meta := map[string]fieldMeta{}
	if rawConfig != nil {
		if topInputs, ok := rawConfig["inputs"].([]interface{}); ok {
			for _, inp := range topInputs {
				inputMap, ok := inp.(map[string]interface{})
				if !ok {
					continue
				}

				name, _ := inputMap["name"].(string)
				displayName, _ := inputMap["display_name"].(string)
				sensitive, _ := inputMap["sensitive"].(bool)
				if name != "" {
					meta[name] = fieldMeta{displayName: displayName, sensitive: sensitive}
				}
			}
		}
	}

	rows := []customerInstallInputFieldResponse{}
	if currentInputs != nil {
		for name, value := range currentInputs.Values {
			m := meta[name]
			rows = append(rows, customerInstallInputFieldResponse{
				Name:        name,
				DisplayName: m.displayName,
				Value:       value,
				Sensitive:   m.sensitive,
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})

	response.Overview.InputFields = rows
}

func (h *CustomerInstallDetailHandler) populatePolicies(
	ctx context.Context,
	client *nuon.Client,
	install *localmodels.Install,
	response *customerInstallDetailResponse,
) {
	policies, err := client.GetLatestAppPoliciesConfigFull(ctx, install.GetAppID())
	if err == nil {
		items := make([]customerInstallPolicyResponse, 0, len(policies))
		for _, policy := range policies {
			items = append(items, customerInstallPolicyResponse{
				Name:   policy.Name,
				Type:   policy.Type,
				Engine: policy.Engine,
			})
		}
		response.Policies.Items = items
	}

	reports, err := client.GetInstallPolicyReports(ctx, install.NuonInstallID, "")
	if err != nil {
		return
	}

	resolvePolicyReportNamesForJSON(ctx, client, install.GetAppID(), reports)

	reportItems := make([]customerInstallPolicyReport, 0, len(reports))
	for _, report := range reports {
		response.Policies.Totals.Pass += report.PassCount
		response.Policies.Totals.Warn += report.WarnCount
		response.Policies.Totals.Deny += report.DenyCount

		reportItems = append(reportItems, customerInstallPolicyReport{
			ID:         report.ID,
			PolicyName: report.PolicyName,
			PassCount:  report.PassCount,
			WarnCount:  report.WarnCount,
			DenyCount:  report.DenyCount,
			CreatedAt:  stringifyTime(report.CreatedAt),
		})
	}

	response.Policies.Reports = reportItems
}

func (h *CustomerInstallDetailHandler) getOrgForCustomerPortal(c *gin.Context) (*localmodels.NuonOrg, error) {
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

func (h *CustomerInstallDetailHandler) getInstallForCustomer(c *gin.Context, user *localmodels.User, orgID string) (*localmodels.Install, error) {
	installID := strings.TrimSpace(c.Param("install_id"))
	if installID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	selected := h.selectCustomerAccountMember(c, user.ID, orgID)
	query := h.db.Where("id = ? AND org_id = ? AND user_id = ?", installID, orgID, user.ID)
	if selected != nil {
		query = h.db.Where(
			"id = ? AND org_id = ? AND ((user_id = ?) OR (customer_account_id = ? AND visibility = ?))",
			installID,
			orgID,
			user.ID,
			selected.AccountID,
			localmodels.VisibilityAccount,
		)
	}

	var install localmodels.Install
	if err := query.First(&install).Error; err != nil {
		return nil, err
	}

	return &install, nil
}

func (h *CustomerInstallDetailHandler) selectCustomerAccountMember(c *gin.Context, userID, orgID string) *localmodels.CustomerAccountMember {
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

func (h *CustomerInstallDetailHandler) nuonAPIURLForOrg(org *localmodels.NuonOrg) string {
	if org != nil && org.APIURL != "" {
		return org.APIURL
	}
	return h.nuonAPIURL
}

func (h *CustomerInstallDetailHandler) buildComponentInfos(
	ctx context.Context,
	apiClient *nuon.Client,
	install *localmodels.Install,
	comps []*nuonmodels.AppInstallComponent,
) []customerInstallComponentResponse {
	ccMap := h.componentConfigConnectionsByID(ctx, apiClient, install.GetAppID())

	infos := make([]customerInstallComponentResponse, 0, len(comps))
	for _, comp := range comps {
		if comp == nil {
			continue
		}

		status := comp.Status
		if comp.StatusV2 != nil && comp.StatusV2.Status != "" {
			status = string(comp.StatusV2.Status)
		}
		if status == "" && len(comp.InstallDeploys) > 0 {
			latest := comp.InstallDeploys[0]
			if latest.StatusV2 != nil && latest.StatusV2.Status != "" {
				status = string(latest.StatusV2.Status)
			} else if latest.Status != "" {
				status = latest.Status
			}
		}

		info := customerInstallComponentResponse{
			ID:     comp.ComponentID,
			Status: status,
			Name:   "Component",
		}
		if comp.Component != nil {
			if comp.Component.Name != "" {
				info.Name = comp.Component.Name
			}
			info.Type = string(comp.Component.Type)
		}

		if cc, ok := ccMap[comp.ComponentID]; ok {
			type vcsSource struct {
				Public *nuonmodels.AppPublicGitVCSConfig
				GitHub *nuonmodels.AppConnectedGithubVCSConfig
			}
			var vcs vcsSource
			switch {
			case cc.TerraformModule != nil:
				vcs.Public = cc.TerraformModule.PublicGitVcsConfig
				vcs.GitHub = cc.TerraformModule.ConnectedGithubVcsConfig
			case cc.Helm != nil:
				vcs.Public = cc.Helm.PublicGitVcsConfig
				vcs.GitHub = cc.Helm.ConnectedGithubVcsConfig
			case cc.DockerBuild != nil:
				vcs.Public = cc.DockerBuild.PublicGitVcsConfig
				vcs.GitHub = cc.DockerBuild.ConnectedGithubVcsConfig
			}

			if vcs.Public != nil {
				info.Repo = vcs.Public.Repo
				info.Directory = vcs.Public.Directory
				info.Branch = vcs.Public.Branch
				info.RepoPublic = true
			} else if vcs.GitHub != nil {
				info.Repo = vcs.GitHub.Repo
				info.Directory = vcs.GitHub.Directory
				info.Branch = vcs.GitHub.Branch
			}
		}

		infos = append(infos, info)
	}

	return infos
}

func (h *CustomerInstallDetailHandler) buildComponentInfosFromAppComponents(
	ctx context.Context,
	apiClient *nuon.Client,
	install *localmodels.Install,
	appComponents []*nuonmodels.AppComponent,
	deployStatusByComponentID map[string]string,
) []customerInstallComponentResponse {
	ccMap := h.componentConfigConnectionsByID(ctx, apiClient, install.GetAppID())

	infos := make([]customerInstallComponentResponse, 0, len(appComponents))
	for _, comp := range appComponents {
		status := deployStatusByComponentID[comp.ID]
		if status == "" {
			status = "not deployed"
		}

		info := customerInstallComponentResponse{
			ID:     comp.ID,
			Name:   comp.Name,
			Type:   string(comp.Type),
			Status: status,
		}

		if cc, ok := ccMap[comp.ID]; ok {
			type vcsSource struct {
				Public *nuonmodels.AppPublicGitVCSConfig
				GitHub *nuonmodels.AppConnectedGithubVCSConfig
			}
			var vcs vcsSource
			switch {
			case cc.TerraformModule != nil:
				vcs.Public = cc.TerraformModule.PublicGitVcsConfig
				vcs.GitHub = cc.TerraformModule.ConnectedGithubVcsConfig
			case cc.Helm != nil:
				vcs.Public = cc.Helm.PublicGitVcsConfig
				vcs.GitHub = cc.Helm.ConnectedGithubVcsConfig
			case cc.DockerBuild != nil:
				vcs.Public = cc.DockerBuild.PublicGitVcsConfig
				vcs.GitHub = cc.DockerBuild.ConnectedGithubVcsConfig
			}

			if vcs.Public != nil {
				info.Repo = vcs.Public.Repo
				info.Directory = vcs.Public.Directory
				info.Branch = vcs.Public.Branch
				info.RepoPublic = true
			} else if vcs.GitHub != nil {
				info.Repo = vcs.GitHub.Repo
				info.Directory = vcs.GitHub.Directory
				info.Branch = vcs.GitHub.Branch
			}
		}

		infos = append(infos, info)
	}

	return infos
}

func (h *CustomerInstallDetailHandler) componentConfigConnectionsByID(ctx context.Context, apiClient *nuon.Client, appID string) map[string]*nuonmodels.AppComponentConfigConnection {
	ccMap := map[string]*nuonmodels.AppComponentConfigConnection{}
	app, err := apiClient.GetApp(ctx, appID)
	if err != nil || app == nil || len(app.AppConfigs) == 0 {
		return ccMap
	}

	appConfigID := app.AppConfigs[0].ID
	cfg, cfgErr := apiClient.GetAppConfigFull(ctx, appID, appConfigID)
	if cfgErr != nil || cfg == nil {
		return ccMap
	}

	for _, cc := range cfg.ComponentConfigConnections {
		if cc != nil && cc.ComponentID != "" {
			ccMap[cc.ComponentID] = cc
		}
	}

	return ccMap
}

func (h *CustomerInstallDetailHandler) mergeComponentStatusesFromDeploys(
	comps []*nuonmodels.AppInstallComponent,
	infos []customerInstallComponentResponse,
	deployStatusByComponentID map[string]string,
) {
	for i := range infos {
		if infos[i].Status != "" || i >= len(comps) || comps[i] == nil {
			continue
		}

		if s, ok := deployStatusByComponentID[comps[i].ComponentID]; ok && s != "" {
			infos[i].Status = s
		} else {
			infos[i].Status = "not deployed"
		}
	}
}

func buildComponentInfosFromDeployStatusMap(deployStatusByComponentID map[string]string) []customerInstallComponentResponse {
	if len(deployStatusByComponentID) == 0 {
		return []customerInstallComponentResponse{}
	}

	componentIDs := make([]string, 0, len(deployStatusByComponentID))
	for componentID := range deployStatusByComponentID {
		componentIDs = append(componentIDs, componentID)
	}
	sort.Strings(componentIDs)

	infos := make([]customerInstallComponentResponse, 0, len(componentIDs))
	for _, componentID := range componentIDs {
		infos = append(infos, customerInstallComponentResponse{
			ID:     componentID,
			Name:   componentID,
			Status: deployStatusByComponentID[componentID],
		})
	}

	return infos
}

func stringifyTime(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

func prettyDataOutputs(dataContents interface{}, fallback map[string]string) map[string]string {
	if outputMap, ok := dataContents.(map[string]interface{}); ok {
		return prettyInterfaceOutputs(outputMap)
	}

	if len(fallback) == 0 {
		return nil
	}

	result := make(map[string]string, len(fallback))
	for key, value := range fallback {
		result[key] = prettyIfJSON(value)
	}
	return result
}

func prettyInterfaceOutputs(outputMap map[string]interface{}) map[string]string {
	if len(outputMap) == 0 {
		return nil
	}

	result := make(map[string]string, len(outputMap))
	for key, value := range outputMap {
		switch typed := value.(type) {
		case string:
			result[key] = prettyIfJSON(typed)
		default:
			if bytes, err := json.MarshalIndent(value, "", "  "); err == nil {
				result[key] = string(bytes)
			} else {
				result[key] = fmt.Sprintf("%v", typed)
			}
		}
	}

	return result
}

func prettyIfJSON(value string) string {
	if len(value) < 2 {
		return value
	}

	if (value[0] != '{' || value[len(value)-1] != '}') && (value[0] != '[' || value[len(value)-1] != ']') {
		return value
	}

	var raw json.RawMessage
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return value
	}

	pretty, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return value
	}

	return string(pretty)
}

func limitRuns[T any](items []T, limit int) []T {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func limitWorkflows(items []customerInstallWorkflowResponse, limit int) []customerInstallWorkflowResponse {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func buildCustomerInstallWorkflows(workflows []*nuonmodels.AppWorkflow) []customerInstallWorkflowResponse {
	items := make([]customerInstallWorkflowResponse, 0, len(workflows))
	for _, workflow := range workflows {
		if workflow == nil {
			continue
		}

		name := workflow.Name
		if name == "" {
			name = getCustomerFriendlyWorkflowName(string(workflow.Type))
		}

		status := "unknown"
		if workflow.Status != nil {
			status = string(workflow.Status.Status)
		}

		items = append(items, customerInstallWorkflowResponse{
			ID:         workflow.ID,
			Name:       name,
			Status:     status,
			CreatedAt:  stringifyTime(workflow.CreatedAt),
			FinishedAt: stringifyTime(workflow.FinishedAt),
		})
	}

	return items
}

func limitDeploys(items []customerInstallDeploySummary, limit int) []customerInstallDeploySummary {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func getCustomerFriendlyWorkflowName(workflowType string) string {
	switch workflowType {
	case "create_install_workflow_run":
		return "Create install"
	case "deploy_workflow_run":
		return "Deploy"
	case "upgrade_workflow_run":
		return "Upgrade"
	case "deprovision_workflow_run":
		return "Deprovision"
	case "reprovision_workflow_run":
		return "Reprovision"
	default:
		if workflowType == "" {
			return "Workflow"
		}
		return strings.ReplaceAll(workflowType, "_", " ")
	}
}

func resolvePolicyReportNamesForJSON(ctx context.Context, apiClient *nuon.Client, appID string, reports []nuon.PolicyReport) {
	policies, err := apiClient.GetLatestAppPoliciesConfigFull(ctx, appID)
	if err != nil || len(policies) == 0 {
		return
	}

	nameByID := make(map[string]string, len(policies))
	for _, policy := range policies {
		if policy.ID != "" {
			nameByID[policy.ID] = policy.Name
		}
	}

	for i := range reports {
		if len(reports[i].PolicyIds) == 0 {
			continue
		}

		names := []string{}
		for _, policyID := range reports[i].PolicyIds {
			if name, ok := nameByID[policyID]; ok {
				names = append(names, name)
			}
		}

		if len(names) > 0 {
			reports[i].PolicyName = strings.Join(names, ", ")
		}
	}
}
