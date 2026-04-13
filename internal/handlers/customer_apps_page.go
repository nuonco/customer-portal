package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/markdown"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	customerpartials "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon-go/models"
	"go.uber.org/zap"
)

func (h *Handler) CustomerAppsPage(c *gin.Context) {
	user := h.tryGetLoggedInUser(c)

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		c.Redirect(http.StatusFound, h.basePath+"/installs")
		return
	}

	var publishedApps []models.PublishedApp
	if err := h.db.Where("org_id = ? AND status IN ?", org.ID, []string{models.AppStatusPublished, models.AppStatusComingSoon}).Order("sort_order ASC, created_at ASC").Find(&publishedApps).Error; err != nil {
		zap.L().Warn("failed to fetch published apps", zap.Error(err))
	}

	theme, _ := models.GetOrCreateAppTheme(h.db, org.ID)

	nuonClient, nuonClientErr := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	// Verify API connectivity
	if nuonClientErr == nil {
		if _, err := nuonClient.GetOrg(c.Request.Context()); err != nil {
			nuonClientErr = err
		}
	}
	appDisplays := make([]customerpartials.PublishedAppDisplay, len(publishedApps))
	if len(publishedApps) == 1 {
		// Single app: full detail fetch so the full-width card has all tab data
		pa := publishedApps[0]
		display := h.buildAppDisplay(c, pa.AppID, org.ID, nuonClient, nuonClientErr)
		display.LogoLightBase64 = pa.LogoLightBase64
		display.LogoDarkBase64 = pa.LogoDarkBase64
		if pa.OverviewMarkdown != "" {
			if html, err := markdown.Render([]byte(pa.OverviewMarkdown)); err == nil {
				display.ReadmeHTML = html
			}
		}
		display.Status = pa.Status
		if display.Status == "" {
			display.Status = "published"
		}
		if display.Platform == "unknown" {
			appDisplays = nil
		} else {
			appDisplays[0] = display
		}
	} else if len(publishedApps) > 1 {
		// Fetch only app name/platform/description in parallel (no heavy detail calls)
		var wg sync.WaitGroup
		for i, pa := range publishedApps {
			wg.Add(1)
			go func(i int, pa models.PublishedApp) {
				defer wg.Done()
				status := pa.Status
				if status == "" {
					status = "published"
				}
				display := customerpartials.PublishedAppDisplay{
					AppID:           pa.AppID,
					AppName:         pa.AppID,
					Platform:        "unknown",
					Status:          status,
					LogoLightBase64: pa.LogoLightBase64,
					LogoDarkBase64:  pa.LogoDarkBase64,
				}
				if nuonClientErr == nil {
					if app, err := nuonClient.GetApp(c.Request.Context(), pa.AppID); err == nil && app != nil {
						if dn := appDisplayName(app); dn != "" {
							display.AppName = dn
						}
						if app.RunnerConfig != nil {
							display.Platform = string(app.RunnerConfig.AppRunnerType)
						}
						display.Description = app.Description
					}
				}
				appDisplays[i] = display
			}(i, pa)
		}
		wg.Wait()

		// Filter out deleted apps (API returned no data, Platform remains "unknown")
		filtered := appDisplays[:0]
		for _, d := range appDisplays {
			if d.Platform != "unknown" {
				filtered = append(filtered, d)
			}
		}
		appDisplays = filtered
	}

	acctActive, acctOthers := h.getCustomerAccountsFromContext(c)
	layoutProps := h.buildCustomerLayoutProps("App Catalog", user, theme, h.getOrgForLayout(c), acctActive, acctOthers)
	layoutProps.HasPublishedApps = len(appDisplays) > 0
	layoutProps.ActiveNav = "apps"
	if nuonClientErr != nil {
		layoutProps.NuonAPIError = "The app is experiencing network issues, and is not able to access app or install data. Please contact support for assistance."
	}

	props := customerpages.CustomerAppsPageProps{
		LayoutProps: layoutProps,
		Apps:        appDisplays,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.CustomerAppsPage(props))
}

// buildAppDisplay builds a PublishedAppDisplay for a single app, fetching details from the Nuon API.
func (h *Handler) buildAppDisplay(c *gin.Context, appID string, orgID string, nuonClient *nuon.Client, nuonClientErr error) customerpartials.PublishedAppDisplay {
	display := customerpartials.PublishedAppDisplay{
		AppID:    appID,
		AppName:  appID, // fallback to AppID if name can't be fetched
		Platform: "unknown",
	}
	if nuonClientErr != nil || nuonClient == nil {
		display.APIError = true
		return display
	}

	ctx := c.Request.Context()

	// Step 1: Fetch app (need AppConfigs[0].ID for GetAppConfigFull)
	app, appErr := nuonClient.GetApp(ctx, appID)
	if appErr != nil {
		zap.L().Warn("failed to fetch app", zap.String("app_id", appID), zap.Error(appErr))
		display.APIError = true
		return display
	}
	if app == nil {
		display.APIError = true
		return display
	}

	if appDisplayName(app) != "" {
		display.AppName = appDisplayName(app)
	}
	if app.RunnerConfig != nil {
		display.Platform = string(app.RunnerConfig.AppRunnerType)
	}
	display.Description = app.Description

	// Step 2: Fan out remaining API calls concurrently
	// GetAppConfigFull (recurse=true) returns sandbox, secrets, input, permissions, and policies nested data.
	// We still need a separate call for policies because the SDK model drops the Policies array field.
	var (
		wg       sync.WaitGroup
		comps    []*nuonmodels.AppComponent
		compErr  error
		fullCfg  *nuonmodels.AppAppConfig
		cfgErr   error
		policies []nuon.AppPoliciesConfigPolicy
		polErr   error
	)

	wg.Add(3)
	go func() { defer wg.Done(); comps, compErr = nuonClient.GetAppComponents(ctx, appID) }()
	go func() {
		defer wg.Done()
		if len(app.AppConfigs) > 0 {
			fullCfg, cfgErr = nuonClient.GetAppConfigFull(ctx, appID, app.AppConfigs[0].ID)
		}
	}()
	go func() { defer wg.Done(); policies, polErr = nuonClient.GetLatestAppPoliciesConfigFull(ctx, appID) }()
	wg.Wait()

	// Process components
	if compErr != nil {
		zap.L().Warn("failed to fetch app components", zap.String("app_id", appID), zap.Error(compErr))
	} else {
		for _, comp := range comps {
			display.Components = append(display.Components, customerpartials.ComponentDisplay{
				Name: comp.Name,
				Type: string(comp.Type),
			})
		}
	}

	// Process full config (sandbox, secrets, input, roles)
	if cfgErr != nil {
		zap.L().Warn("failed to fetch full app config", zap.String("app_id", appID), zap.Error(cfgErr))
	} else if fullCfg != nil {
		// Sandbox
		if sbCfg := fullCfg.Sandbox; sbCfg != nil {
			display.SandboxPlatform = sbCfg.CloudPlatform
			if display.SandboxPlatform == "" {
				display.SandboxPlatform = display.Platform
			}
			display.SandboxTFVersion = sbCfg.TerraformVersion
			display.SandboxDrift = sbCfg.DriftSchedule
			if sbCfg.PublicGitVcsConfig != nil {
				display.SandboxRepoIsPublic = true
				display.SandboxRepoURL = sbCfg.PublicGitVcsConfig.Repo
				display.SandboxRepoBranch = sbCfg.PublicGitVcsConfig.Branch
				display.SandboxRepoDir = sbCfg.PublicGitVcsConfig.Directory
			}
		}

		// Secrets
		if secretsCfg := fullCfg.Secrets; secretsCfg != nil {
			for _, s := range secretsCfg.Secrets {
				display.Secrets = append(display.Secrets, customerpartials.SecretDisplay{
					Name:         s.Name,
					DisplayName:  s.DisplayName,
					Description:  s.Description,
					AutoGenerate: s.AutoGenerate,
				})
			}
		}

		// Input config
		if inputCfg := fullCfg.Input; inputCfg != nil {
			var localConfig models.AppInputConfig
			configExists := h.db.Where("org_id = ? AND app_id = ?", orgID, appID).First(&localConfig).Error == nil
			customerInputNames := localConfig.GetCustomerInputNames()
			customerInputSet := make(map[string]bool)
			for _, name := range customerInputNames {
				customerInputSet[name] = true
			}

			// Build a map of group ID → inputs, since the recursive config
			// returns inputs flat rather than nested within groups.
			inputsByGroup := make(map[string][]*nuonmodels.AppAppInput)
			for _, input := range inputCfg.Inputs {
				if input != nil && input.GroupID != "" {
					inputsByGroup[input.GroupID] = append(inputsByGroup[input.GroupID], input)
				}
			}

			for _, group := range inputCfg.InputGroups {
				if group == nil {
					continue
				}
				// Use nested AppInputs if present, otherwise fall back to flat Inputs joined by GroupID
				inputs := group.AppInputs
				if len(inputs) == 0 {
					inputs = inputsByGroup[group.ID]
				}
				gd := customerpartials.InputGroupDisplay{
					Name:        group.Name,
					DisplayName: group.DisplayName,
				}
				for _, input := range inputs {
					if input == nil {
						continue
					}
					// Only show customer-facing inputs in the catalog.
					// When no AppInputConfig row exists, show all inputs.
					// When configured with zero customer inputs, show none.
					if configExists && !customerInputSet[input.Name] {
						continue
					}
					gd.Inputs = append(gd.Inputs, customerpartials.InputDisplay{
						Name:         input.Name,
						DisplayName:  input.DisplayName,
						Description:  input.Description,
						Type:         input.Type,
						Required:     input.Required,
						Sensitive:    input.Sensitive,
						Default:      input.Default,
						ConfiguredBy: "customer",
					})
				}
				if len(gd.Inputs) > 0 {
					display.InputGroups = append(display.InputGroups, gd)
				}
			}
		}

		// Roles (from permissions)
		if fullCfg.Permissions != nil {
			addRole := func(role *nuonmodels.AppAppAWSIAMRoleConfig, label string) {
				if role == nil || role.Name == "" {
					return
				}
				name := role.DisplayName
				if name == "" {
					name = role.Name
				}
				permBoundary := role.PermissionsBoundary
				if permBoundary != "" {
					if decoded, err := base64.StdEncoding.DecodeString(permBoundary); err == nil {
						var prettyJSON bytes.Buffer
						if json.Indent(&prettyJSON, decoded, "", "  ") == nil {
							permBoundary = prettyJSON.String()
						}
					}
				}
				rd := customerpartials.IAMRoleDisplay{
					Label:               label,
					Name:                name,
					Description:         role.Description,
					Type:                string(role.Type),
					PermissionsBoundary: permBoundary,
				}
				for _, p := range role.Policies {
					if p == nil {
						continue
					}
					policyType := "Vendor defined"
					if p.ManagedPolicyName != "" {
						policyType = "AWS managed"
					}
					pName := p.Name
					if pName == "" {
						pName = p.ManagedPolicyName
					}
					rd.Policies = append(rd.Policies, customerpartials.IAMPolicyDisplay{
						Name:             pName,
						Type:             policyType,
						ManagedPolicyARN: p.ManagedPolicyName,
					})
				}
				display.Permissions = append(display.Permissions, rd)
			}

			permCfg := fullCfg.Permissions
			seen := map[string]bool{}
			trackAndAdd := func(role *nuonmodels.AppAppAWSIAMRoleConfig, label string) {
				if role == nil || role.Name == "" {
					return
				}
				seen[role.Name] = true
				addRole(role, label)
			}

			if permCfg.ProvisionAwsIamRole.Name != "" {
				prov := permCfg.ProvisionAwsIamRole.AppAppAWSIAMRoleConfig
				trackAndAdd(&prov, "Provision")
			}
			trackAndAdd(permCfg.DeprovisionAwsIamRole, "Deprovision")
			trackAndAdd(permCfg.MaintenanceAwsIamRole, "Maintenance")
			trackAndAdd(permCfg.BreakGlassAwsIamRole, "Break Glass")
			for _, r := range permCfg.AwsIamRoles {
				if seen[r.Name] {
					continue
				}
				label := r.DisplayName
				if label == "" {
					label = r.Name
				}
				addRole(r, label)
			}
		}

	}

	// Fall back to description if no readme
	if display.ReadmeHTML == "" && display.Description != "" {
		display.ReadmeHTML = "<p>" + display.Description + "</p>"
	}

	// Process policies config (separate call — SDK model drops nested Policies array)
	if polErr != nil {
		zap.L().Warn("failed to fetch app policies config", zap.String("app_id", appID), zap.Error(polErr))
	} else {
		for _, p := range policies {
			display.Policies = append(display.Policies, customerpartials.PolicyDisplay{
				Name:        p.Name,
				Type:        p.Type,
				Engine:      p.Engine,
				Description: p.Description,
				Contents:    p.Contents,
			})
		}
	}

	return display
}

// CustomerAppDetailPage renders the detail page for a single published app.
