package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon-go/models"
	"go.uber.org/zap"
)

const stackSubTabDefault = "outputs"
const sandboxSubTabDefault = "outputs"
const componentsSubTabDefault = "components"

const auditPageSize = 10

// AuditPanel renders the audit panel with stack runs, sandbox runs, and deploys sub-tabs.
func (h *Handler) AuditPanel(c *gin.Context) {
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()

	activeTab := c.DefaultQuery("tab", "stack")

	stackSubTab := c.DefaultQuery("stack_sub", stackSubTabDefault)

	props := partials.AuditPanelProps{
		Install:     install,
		BasePath:    h.basePath,
		ActiveTab:   activeTab,
		StackSubTab: stackSubTab,
	}

	if nuonOrg != nil && nuonOrg.APIToken != "" {
		apiClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
		if err == nil {
			ctx := c.Request.Context()
			installID := install.NuonInstallID

			switch activeTab {
			case "stack":
				// Always fetch stack info for the info sub-tab
				props.StackInfo = h.buildStackInfo(ctx, apiClient, install)

				// Fetch stack runs for the history sub-tab
				offset := queryInt(c, "stack_offset", 0)
				if runs, err := apiClient.GetInstallStackRuns(ctx, installID); err == nil {
					// Build version ID → status lookup from the stack
					if stack, err := apiClient.GetInstallStack(ctx, installID); err == nil && stack != nil {
						versionStatus := make(map[string]string)
						versionTemplateURL := make(map[string]string)
						for _, v := range stack.Versions {
							if v.CompositeStatus != nil {
								versionStatus[v.ID] = string(v.CompositeStatus.Status)
							}
							if v.TemplateURL != "" {
								versionTemplateURL[v.ID] = v.TemplateURL
							}
						}
						for i := range runs {
							vid := runs[i].InstallStackVersionID
							if s, ok := versionStatus[vid]; ok {
								runs[i].VersionStatus = s
							}
							if u, ok := versionTemplateURL[vid]; ok {
								runs[i].TemplateURL = u
							}
						}
					}
					if len(runs) > 0 {
						props.ActiveStackRun = &runs[0]
					}
					props.StackPagination = auditPagination(len(runs), offset)
					props.StackRuns = paginateStackRuns(runs, offset)
				} else {
					zap.L().Warn("failed to fetch stack runs", zap.Error(err))
				}
			case "sandbox":
				props.SandboxSubTab = c.DefaultQuery("sandbox_sub", sandboxSubTabDefault)

				// Always fetch sandbox info for the info sub-tab
				sandboxInfo, workspaceID := h.buildSandboxInfo(ctx, apiClient, install)
				props.SandboxInfo = sandboxInfo

				// Fetch sandbox runs for the history sub-tab
				offset := queryInt(c, "sandbox_offset", 0)
				if runs, err := apiClient.GetInstallSandboxRuns(ctx, installID); err == nil {
					if len(runs) > 0 {
						props.ActiveSandboxRun = runs[0]
					}
					props.SandboxPagination = auditPagination(len(runs), offset)
					props.SandboxRuns = paginateSandboxRuns(runs, offset)
				} else {
					zap.L().Warn("failed to fetch sandbox runs", zap.Error(err))
				}

				// Fetch terraform state resources for the resources sub-tab
				if workspaceID != "" {
					if states, err := apiClient.GetTerraformWorkspaceStates(ctx, workspaceID); err == nil && len(states) > 0 {
						if resources, err := apiClient.GetTerraformWorkspaceStateResources(ctx, workspaceID, states[0].ID); err == nil {
							props.SandboxResources = resources
						} else {
							zap.L().Warn("failed to fetch sandbox state resources", zap.Error(err))
						}
					} else if err != nil {
						zap.L().Warn("failed to fetch sandbox workspace states", zap.Error(err))
					}
				}

				// Fetch policy reports for the policy evaluations sub-tab
				if reports, err := apiClient.GetInstallPolicyReports(ctx, installID, "install_sandbox_runs"); err == nil {
					resolvePolicyReportNames(ctx, apiClient, install.GetAppID(), reports)
					props.SandboxPolicyReports = reports
				} else {
					zap.L().Warn("failed to fetch sandbox policy reports", zap.Error(err))
				}
			case "components":
				props.ComponentsSubTab = c.DefaultQuery("components_sub", componentsSubTabDefault)

				// Fetch deploys first — used for history sub-tab and component status
				offset := queryInt(c, "components_offset", 0)
				deployStatus := make(map[string]string)
				if deploys, err := apiClient.GetInstallDeploys(ctx, installID); err == nil {
					props.DeploysPagination = auditPagination(len(deploys), offset)
					props.Deploys = paginateDeploys(deploys, offset)

					// Build component_id → latest deploy status (deploys are newest-first)
					for _, d := range deploys {
						if _, exists := deployStatus[d.ComponentID]; !exists && d.ComponentID != "" {
							if d.StatusV2 != nil && d.StatusV2.Status != "" {
								deployStatus[d.ComponentID] = string(d.StatusV2.Status)
							} else if d.Status != "" {
								deployStatus[d.ComponentID] = d.Status
							}
						}
					}
				} else {
					zap.L().Warn("failed to fetch deploys", zap.Error(err))
				}

				// Fetch install components for the info sub-tab
				if components, err := apiClient.GetInstallComponents(ctx, installID); err == nil {
					props.Components = components
					props.ComponentInfos = h.buildComponentInfos(ctx, apiClient, install, components)

					// Merge deploy status into ComponentInfos
					for i := range props.ComponentInfos {
						if props.ComponentInfos[i].Status == "" && i < len(components) {
							if s, ok := deployStatus[components[i].ComponentID]; ok {
								props.ComponentInfos[i].Status = s
							} else {
								props.ComponentInfos[i].Status = "not deployed"
							}
						}
					}
				} else {
					zap.L().Warn("failed to fetch install components", zap.Error(err))
				}

				// Fetch policy reports for the policy evaluations sub-tab
				if reports, err := apiClient.GetInstallPolicyReports(ctx, installID, "install_deploys"); err == nil {
					resolvePolicyReportNames(ctx, apiClient, install.GetAppID(), reports)
					props.ComponentsPolicyReports = reports
				} else {
					zap.L().Warn("failed to fetch component policy reports", zap.Error(err))
				}
			case "roles":
				props.StackInfo = h.buildStackInfo(ctx, apiClient, install)
			case "audit":
				props.AuditSubTab = c.DefaultQuery("audit_sub", "workflows")
				offset := queryInt(c, "audit_offset", 0)

				switch props.AuditSubTab {
				case "workflows":
					if workflows, hasMore, err := apiClient.GetInstallWorkflows(ctx, installID, offset, auditPageSize); err == nil {
						props.Workflows = workflows
						props.WorkflowsPagination = partials.AuditTabPagination{
							Total:      offset + len(workflows),
							HasPrev:    offset > 0,
							HasNext:    hasMore,
							PrevOffset: max(0, offset-auditPageSize),
							NextOffset: offset + auditPageSize,
						}
					} else {
						zap.L().Warn("failed to fetch workflows", zap.Error(err))
					}
				case "stack":
					if runs, err := apiClient.GetInstallStackRuns(ctx, installID); err == nil {
						if stack, err := apiClient.GetInstallStack(ctx, installID); err == nil && stack != nil {
							versionStatus := make(map[string]string)
							for _, v := range stack.Versions {
								if v.CompositeStatus != nil {
									versionStatus[v.ID] = string(v.CompositeStatus.Status)
								}
							}
							for i := range runs {
								if s, ok := versionStatus[runs[i].InstallStackVersionID]; ok {
									runs[i].VersionStatus = s
								}
							}
						}
						props.StackPagination = auditPagination(len(runs), offset)
						props.StackRuns = paginateStackRuns(runs, offset)
					} else {
						zap.L().Warn("failed to fetch stack runs", zap.Error(err))
					}
				case "sandbox":
					if runs, err := apiClient.GetInstallSandboxRuns(ctx, installID); err == nil {
						props.SandboxPagination = auditPagination(len(runs), offset)
						props.SandboxRuns = paginateSandboxRuns(runs, offset)
					} else {
						zap.L().Warn("failed to fetch sandbox runs", zap.Error(err))
					}
				case "components":
					if components, err := apiClient.GetInstallComponents(ctx, installID); err == nil {
						props.Components = components
					}
					if deploys, err := apiClient.GetInstallDeploys(ctx, installID); err == nil {
						props.DeploysPagination = auditPagination(len(deploys), offset)
						props.Deploys = paginateDeploys(deploys, offset)
					} else {
						zap.L().Warn("failed to fetch deploys", zap.Error(err))
					}
				case "actions":
					if workflows, hasMore, err := apiClient.GetInstallWorkflowsByType(ctx, installID, offset, auditPageSize, "action_workflow_run"); err == nil {
						props.ActionWorkflows = workflows
						props.ActionsPagination = partials.AuditTabPagination{
							Total:      offset + len(workflows),
							HasPrev:    offset > 0,
							HasNext:    hasMore,
							PrevOffset: max(0, offset-auditPageSize),
							NextOffset: offset + auditPageSize,
						}
					} else {
						zap.L().Warn("failed to fetch action workflows", zap.Error(err))
					}
				}
			}
		}
	}
	h.RenderTempl(c, http.StatusOK, partials.AuditPanel(props))
}

// resolvePolicyReportNames populates PolicyName on each report by cross-referencing
// PolicyIds with the app's policies config.
func resolvePolicyReportNames(ctx context.Context, apiClient *nuon.Client, appID string, reports []nuon.PolicyReport) {
	policies, err := apiClient.GetLatestAppPoliciesConfigFull(ctx, appID)
	if err != nil || len(policies) == 0 {
		return
	}
	nameByID := make(map[string]string, len(policies))
	for _, p := range policies {
		if p.ID != "" {
			nameByID[p.ID] = p.Name
		}
	}
	for i := range reports {
		if len(reports[i].PolicyIds) > 0 {
			var names []string
			for _, pid := range reports[i].PolicyIds {
				if name, ok := nameByID[pid]; ok {
					names = append(names, name)
				}
			}
			if len(names) > 0 {
				reports[i].PolicyName = strings.Join(names, ", ")
			}
		}
	}
}

// AuditRoleDetailPanel renders a single role's detail content for the secondary sliding panel.
func (h *Handler) AuditRoleDetailPanel(c *gin.Context) {
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	roleIndex, err := strconv.Atoi(c.Param("role_index"))
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid role index")
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil || nuonOrg.APIToken == "" {
		c.String(http.StatusNotFound, "Role not found")
		return
	}

	apiClient, apiErr := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
	if apiErr != nil {
		c.String(http.StatusInternalServerError, "Failed to create API client")
		return
	}

	info := h.buildStackInfo(c.Request.Context(), apiClient, install)
	if info == nil || roleIndex < 0 || roleIndex >= len(info.Roles) {
		c.String(http.StatusNotFound, "Role not found")
		return
	}

	h.RenderTempl(c, http.StatusOK, partials.AuditRoleDetailContent(info.Roles[roleIndex]))
}

// buildStackInfo fetches install stack data from the Nuon API and maps it to a StackInfo for display.
func (h *Handler) buildStackInfo(ctx context.Context, apiClient *nuon.Client, install *models.Install) *partials.StackInfo {
	stack, err := apiClient.GetInstallStack(ctx, install.NuonInstallID)
	if err != nil || stack == nil {
		return nil
	}

	info := &partials.StackInfo{
		ID:        stack.ID,
		CreatedAt: partials.FormatAPITime(stack.CreatedAt),
		UpdatedAt: partials.FormatAPITime(stack.UpdatedAt),
	}

	// Get status from the latest stack version
	if len(stack.Versions) > 0 && stack.Versions[0].CompositeStatus != nil {
		info.Status = string(stack.Versions[0].CompositeStatus.Status)
	}

	outputs := stack.InstallStackOutputs
	if outputs == nil {
		return info
	}

	// Prefer DataContents (interface{}) over Data (map[string]string) so nested
	// values are JSON-encoded instead of Go-formatted "map[...]" strings.
	if dc, ok := outputs.DataContents.(map[string]interface{}); ok && len(dc) > 0 {
		info.Outputs = make(map[string]string, len(dc))
		for k, v := range dc {
			switch s := v.(type) {
			case string:
				info.Outputs[k] = prettyIfJSON(s)
			default:
				if b, err := json.MarshalIndent(s, "", "  "); err == nil {
					info.Outputs[k] = string(b)
				} else {
					info.Outputs[k] = fmt.Sprintf("%v", v)
				}
			}
		}
	} else if len(outputs.Data) > 0 {
		info.Outputs = make(map[string]string, len(outputs.Data))
		for k, v := range outputs.Data {
			info.Outputs[k] = prettyIfJSON(v)
		}
	}

	if aws := outputs.Aws; aws != nil {
		info.Region = aws.Region
		info.AccountID = aws.AccountID
		info.VPC = aws.VpcID

		// Build a map of role type → config from the app permissions config
		roleConfigMap := map[string]*nuonmodels.AppAppAWSIAMRoleConfig{}
		app, appErr := apiClient.GetApp(ctx, install.GetAppID())
		appConfigID := ""
		if appErr == nil && app != nil && len(app.AppConfigs) > 0 {
			appConfigID = app.AppConfigs[0].ID
		}
		if appConfigID != "" {
			if cfg, err := apiClient.GetAppConfigFull(ctx, install.GetAppID(), appConfigID); err == nil && cfg != nil && cfg.Permissions != nil {
				perms := cfg.Permissions
				if perms.ProvisionAwsIamRole.ID != "" {
					rc := perms.ProvisionAwsIamRole.AppAppAWSIAMRoleConfig
					roleConfigMap["Provision"] = &rc
				}
				if perms.DeprovisionAwsIamRole != nil {
					roleConfigMap["Deprovision"] = perms.DeprovisionAwsIamRole
				}
				if perms.MaintenanceAwsIamRole != nil {
					roleConfigMap["Maintenance"] = perms.MaintenanceAwsIamRole
				}
				for _, r := range perms.AwsIamRoles {
					if r != nil && r.DisplayName != "" {
						roleConfigMap[r.DisplayName] = r
					}
				}
			}
		}

		decodeBase64 := func(s string) string {
			if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
				return string(decoded)
			}
			return s
		}

		prettyJSON := func(s string) string {
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(s), &raw); err == nil {
				if pretty, err := json.MarshalIndent(raw, "", "  "); err == nil {
					return string(pretty)
				}
			}
			return s
		}

		extractPolicies := func(rc *nuonmodels.AppAppAWSIAMRoleConfig) []partials.RolePolicy {
			if rc == nil {
				return nil
			}
			var policies []partials.RolePolicy
			for _, p := range rc.Policies {
				if p != nil {
					contents := p.Contents
					if contents != "" {
						contents = prettyJSON(decodeBase64(contents))
					}
					policies = append(policies, partials.RolePolicy{
						Name:              p.Name,
						ManagedPolicyName: p.ManagedPolicyName,
						Contents:          contents,
					})
				}
			}
			return policies
		}

		addRole := func(name, roleType, arn string) {
			if arn == "" {
				return
			}
			role := partials.StackInfoRole{
				Name:   name,
				Type:   roleType,
				Arn:    arn,
				Active: true,
			}
			if rc, ok := roleConfigMap[name]; ok {
				pb := rc.PermissionsBoundary
				if pb != "" {
					pb = prettyJSON(decodeBase64(pb))
				}
				role.PermissionsBoundary = pb
				role.Policies = extractPolicies(rc)
			}
			info.Roles = append(info.Roles, role)
		}
		addRole("Provision", "Runner Provision", aws.ProvisionIamRoleArn)
		addRole("Deprovision", "Runner Deprovision", aws.DeprovisionIamRoleArn)
		addRole("Maintenance", "Runner Maintenance", aws.MaintenanceIamRoleArn)
		addRole("Runner", "Runner", aws.RunnerIamRoleArn)
		for name, arn := range aws.BreakGlassRoleArns {
			addRole(fmt.Sprintf("Breakglass – %s", name), "Breakglass", arn)
		}
	}

	return info
}

// buildSandboxInfo fetches install sandbox data from the Nuon API and maps it to a SandboxInfo for display.
// Returns the SandboxInfo and the terraform workspace ID (empty if unavailable).
func (h *Handler) buildSandboxInfo(ctx context.Context, apiClient *nuon.Client, install *models.Install) (*partials.SandboxInfo, string) {
	nuonInstall, err := apiClient.GetInstall(ctx, install.NuonInstallID)
	if err != nil || nuonInstall == nil || nuonInstall.Sandbox == nil {
		return nil, ""
	}

	sandbox := nuonInstall.Sandbox
	var workspaceID string
	if sandbox.TerraformWorkspace != nil {
		workspaceID = sandbox.TerraformWorkspace.ID
	}
	info := &partials.SandboxInfo{
		ID:        sandbox.ID,
		Status:    sandbox.Status,
		CreatedAt: partials.FormatAPITime(sandbox.CreatedAt),
		UpdatedAt: partials.FormatAPITime(sandbox.UpdatedAt),
	}

	// Fetch repo/directory/branch from sandbox config
	if cfg, err := apiClient.GetAppSandboxLatestConfig(ctx, install.GetAppID()); err == nil && cfg != nil {
		if gh := cfg.ConnectedGithubVcsConfig; gh != nil {
			info.Repo = gh.Repo
			info.Directory = gh.Directory
			info.Branch = gh.Branch
		} else if pg := cfg.PublicGitVcsConfig; pg != nil {
			info.Repo = pg.Repo
			info.Directory = pg.Directory
			info.Branch = pg.Branch
			info.RepoPublic = true
		}
	}

	// Fetch outputs from the latest sandbox run
	if runs, err := apiClient.GetInstallSandboxRuns(ctx, install.NuonInstallID); err == nil && len(runs) > 0 {
		if outputMap, ok := runs[0].Outputs.(map[string]interface{}); ok && len(outputMap) > 0 {
			info.Outputs = make(map[string]string, len(outputMap))
			for k, v := range outputMap {
				switch s := v.(type) {
				case string:
					info.Outputs[k] = prettyIfJSON(s)
				default:
					if b, err := json.MarshalIndent(s, "", "  "); err == nil {
						info.Outputs[k] = string(b)
					} else {
						info.Outputs[k] = fmt.Sprintf("%v", v)
					}
				}
			}
			// Extract structured AWS fields from nested output maps
			if acct, ok := outputMap["account"].(map[string]interface{}); ok {
				if id, ok := acct["id"].(string); ok {
					info.AccountID = id
				}
				if region, ok := acct["region"].(string); ok && info.Region == "" {
					info.Region = region
				}
			}
			if region, ok := outputMap["region"].(string); ok {
				info.Region = region
			}
			if vpc, ok := outputMap["vpc"].(map[string]interface{}); ok {
				if id, ok := vpc["id"].(string); ok {
					info.VPCID = id
				}
				if arn, ok := vpc["arn"].(string); ok {
					info.VPCARN = arn
				}
			}
			if ecr, ok := outputMap["ecr"].(map[string]interface{}); ok {
				if id, ok := ecr["registry_id"].(string); ok {
					info.ECRID = id
				}
				if arn, ok := ecr["repository_arn"].(string); ok {
					info.ECRARN = arn
				}
			}
			if cluster, ok := outputMap["cluster"].(map[string]interface{}); ok {
				if arn, ok := cluster["arn"].(string); ok {
					info.ClusterARN = arn
				}
			}
		}
	}

	return info, workspaceID
}

func queryInt(c *gin.Context, key string, def int) int {
	v, err := strconv.Atoi(c.DefaultQuery(key, strconv.Itoa(def)))
	if err != nil {
		return def
	}
	return v
}

func auditPagination(total, offset int) partials.AuditTabPagination {
	return partials.AuditTabPagination{
		Total:      total,
		HasPrev:    offset > 0,
		HasNext:    offset+auditPageSize < total,
		PrevOffset: max(0, offset-auditPageSize),
		NextOffset: offset + auditPageSize,
	}
}

func paginateStackRuns(runs []nuon.StackRun, offset int) []nuon.StackRun {
	if offset >= len(runs) {
		return nil
	}
	end := offset + auditPageSize
	if end > len(runs) {
		end = len(runs)
	}
	return runs[offset:end]
}

func paginateSandboxRuns(runs []*nuonmodels.AppInstallSandboxRun, offset int) []*nuonmodels.AppInstallSandboxRun {
	if offset >= len(runs) {
		return nil
	}
	end := offset + auditPageSize
	if end > len(runs) {
		end = len(runs)
	}
	return runs[offset:end]
}

func paginateDeploys(deploys []*nuonmodels.AppInstallDeploy, offset int) []*nuonmodels.AppInstallDeploy {
	if offset >= len(deploys) {
		return nil
	}
	end := offset + auditPageSize
	if end > len(deploys) {
		end = len(deploys)
	}
	return deploys[offset:end]
}

// prettyIfJSON pretty-prints a string if it's valid JSON, otherwise returns it unchanged.
func prettyIfJSON(s string) string {
	if len(s) < 2 {
		return s
	}
	if (s[0] == '{' && s[len(s)-1] == '}') || (s[0] == '[' && s[len(s)-1] == ']') {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(s), &raw); err == nil {
			if pretty, err := json.MarshalIndent(raw, "", "  "); err == nil {
				return string(pretty)
			}
		}
	}
	return s
}
