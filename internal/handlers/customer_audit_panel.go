package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/internal/views/customerui/theme/partials"
	"github.com/nuonco/customer-portal/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon/sdks/nuon-go/models"
)

const stackSubTabDefault = "outputs"
const sandboxSubTabDefault = "outputs"
const componentsSubTabDefault = "components"

const auditPageSize = 10

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

// buildStackInfo fetches install stack data from the Nuon API and maps it to a StackInfo for display.
func (h *Handler) buildStackInfo(ctx context.Context, apiClient *nuon.Client, install *models.Install) *partials.StackInfo {
	stack, err := apiClient.GetInstallStack(ctx, install.NuonInstallID)
	if err != nil || stack == nil {
		return nil
	}

	// Determine status from the latest stack version
	var status string
	if len(stack.Versions) > 0 && stack.Versions[0].CompositeStatus != nil {
		status = string(stack.Versions[0].CompositeStatus.Status)
	}

	// Keep card in empty state until provisioning begins
	if status == "" || status == "queued" {
		return nil
	}

	info := &partials.StackInfo{
		ID:        stack.ID,
		Status:    status,
		CreatedAt: partials.FormatAPITime(stack.CreatedAt),
		UpdatedAt: partials.FormatAPITime(stack.UpdatedAt),
	}

	outputs := stack.InstallStackOutputs
	if outputs == nil {
		return info
	}

	// Prefer DataContents (interface{}) over Data (map[string]string) so nested
	// values are JSON-encoded instead of Go-formatted "map[...]" strings.
	if dc := outputs.DataContents; len(dc) > 0 {
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
	if err != nil || nuonInstall == nil || nuonInstall.Sandbox == nil || nuonInstall.Sandbox.Status == "queued" {
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
		if outputMap := runs[0].Outputs; len(outputMap) > 0 {
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
