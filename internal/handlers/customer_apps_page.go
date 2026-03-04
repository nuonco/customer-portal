package handlers

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
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
	if err := h.db.Where("org_id = ?", org.ID).Order("sort_order ASC, created_at ASC").Find(&publishedApps).Error; err != nil {
		zap.L().Warn("failed to fetch published apps", zap.Error(err))
	}

	theme, _ := models.GetOrCreateAppTheme(h.db, org.ID)

	appDisplays := make([]customerpages.PublishedAppDisplay, len(publishedApps))
	if len(publishedApps) > 0 {
		nuonClient, nuonClientErr := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)

		// Fetch only app name/platform/description in parallel (no heavy detail calls)
		var wg sync.WaitGroup
		for i, pa := range publishedApps {
			wg.Add(1)
			go func(i int, pa models.PublishedApp) {
				defer wg.Done()
				display := customerpages.PublishedAppDisplay{
					AppID:           pa.AppID,
					AppName:         pa.AppID,
					Platform:        "unknown",
					LogoLightBase64: pa.LogoLightBase64,
					LogoDarkBase64:  pa.LogoDarkBase64,
				}
				if nuonClientErr == nil {
					if app, err := nuonClient.GetApp(c.Request.Context(), pa.AppID); err == nil && app != nil {
						if app.Name != "" {
							display.AppName = app.Name
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
	}

	acctActive, acctOthers := h.getCustomerAccountsFromContext(c)
	layoutProps := h.buildCustomerLayoutProps("App Catalog", user, theme, h.getOrgForLayout(c), acctActive, acctOthers)
	layoutProps.HasPublishedApps = len(publishedApps) > 0
	layoutProps.ActiveNav = "apps"

	props := customerpages.CustomerAppsPageProps{
		LayoutProps: layoutProps,
		Apps:        appDisplays,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.CustomerAppsPage(props))
}

// buildAppDisplay builds a PublishedAppDisplay for a single app, fetching details from the Nuon API.
func (h *Handler) buildAppDisplay(c *gin.Context, appID string, orgID string, nuonClient *nuon.Client, nuonClientErr error) customerpages.PublishedAppDisplay {
	display := customerpages.PublishedAppDisplay{
		AppID:    appID,
		AppName:  appID, // fallback to AppID if name can't be fetched
		Platform: "unknown",
	}
	if nuonClientErr != nil {
		return display
	}

	ctx := c.Request.Context()

	// Fan out all 7 API calls concurrently
	var (
		wg          sync.WaitGroup
		app         *nuonmodels.AppApp
		appErr      error
		comps       []*nuonmodels.AppComponent
		compErr     error
		sbCfg       *nuonmodels.AppAppSandboxConfig
		sbErr       error
		permCfg     *nuonmodels.AppAppPermissionsConfig
		permErr     error
		policies    []nuon.AppPoliciesConfigPolicy
		polErr      error
		secretsCfg  *nuonmodels.AppAppSecretsConfig
		secErr      error
		inputConfig interface{}
		inputErr    error
	)

	wg.Add(7)
	go func() { defer wg.Done(); app, appErr = nuonClient.GetApp(ctx, appID) }()
	go func() { defer wg.Done(); comps, compErr = nuonClient.GetAppComponents(ctx, appID) }()
	go func() { defer wg.Done(); sbCfg, sbErr = nuonClient.GetAppSandboxLatestConfig(ctx, appID) }()
	go func() { defer wg.Done(); permCfg, permErr = nuonClient.GetLatestAppPermissionsConfig(ctx, appID) }()
	go func() { defer wg.Done(); policies, polErr = nuonClient.GetLatestAppPoliciesConfigFull(ctx, appID) }()
	go func() { defer wg.Done(); secretsCfg, secErr = nuonClient.GetAppSecretsConfig(ctx, appID) }()
	go func() { defer wg.Done(); inputConfig, inputErr = nuonClient.GetAppInputConfig(ctx, appID) }()
	wg.Wait()

	// Process app info
	if appErr == nil && app != nil {
		if app.Name != "" {
			display.AppName = app.Name
		}
		if app.RunnerConfig != nil {
			display.Platform = string(app.RunnerConfig.AppRunnerType)
		}
		display.Description = app.Description
	}

	// Process components
	if compErr != nil {
		zap.L().Warn("failed to fetch app components", zap.String("app_id", appID), zap.Error(compErr))
	} else {
		for _, comp := range comps {
			display.Components = append(display.Components, customerpages.ComponentDisplay{
				Name: comp.Name,
				Type: string(comp.Type),
			})
		}
	}

	// Process sandbox config
	if sbErr != nil {
		zap.L().Warn("failed to fetch app sandbox config", zap.String("app_id", appID), zap.Error(sbErr))
	} else if sbCfg != nil {
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

	// Process permissions config
	if permErr != nil {
		zap.L().Warn("failed to fetch app permissions config", zap.String("app_id", appID), zap.Error(permErr))
	} else if permCfg != nil {
		addRole := func(role *nuonmodels.AppAppAWSIAMRoleConfig, label string) {
			if role == nil || role.Name == "" {
				return
			}
			name := role.DisplayName
			if name == "" {
				name = role.Name
			}
			display.Permissions = append(display.Permissions, customerpages.IAMRoleDisplay{
				Label:       label,
				Name:        name,
				Description: role.Description,
			})
		}
		if permCfg.ProvisionAwsIamRole.Name != "" {
			prov := permCfg.ProvisionAwsIamRole.AppAppAWSIAMRoleConfig
			addRole(&prov, "Provision")
		}
		addRole(permCfg.DeprovisionAwsIamRole, "Deprovision")
		addRole(permCfg.MaintenanceAwsIamRole, "Maintenance")
		addRole(permCfg.BreakGlassAwsIamRole, "Break Glass")
		for _, r := range permCfg.AwsIamRoles {
			label := r.DisplayName
			if label == "" {
				label = r.Name
			}
			addRole(r, label)
		}
	}

	// Process policies config
	if polErr != nil {
		zap.L().Warn("failed to fetch app policies config", zap.String("app_id", appID), zap.Error(polErr))
	} else {
		for _, p := range policies {
			display.Policies = append(display.Policies, customerpages.PolicyDisplay{
				Name:        p.Name,
				Type:        p.Type,
				Engine:      p.Engine,
				Description: p.Description,
			})
		}
	}

	// Process secrets config
	if secErr != nil {
		zap.L().Warn("failed to fetch app secrets config", zap.String("app_id", appID), zap.Error(secErr))
	} else if secretsCfg != nil && len(secretsCfg.Secrets) > 0 {
		for _, s := range secretsCfg.Secrets {
			display.Secrets = append(display.Secrets, customerpages.SecretDisplay{
				Name:         s.Name,
				DisplayName:  s.DisplayName,
				Description:  s.Description,
				AutoGenerate: s.AutoGenerate,
			})
		}
	}

	// Process input config using pre-fetched DB data
	if inputErr != nil {
		zap.L().Warn("failed to fetch app input config", zap.String("app_id", appID), zap.Error(inputErr))
	} else if inputConfig != nil {
		var localConfig models.AppInputConfig
		h.db.Where("org_id = ? AND app_id = ?", orgID, appID).First(&localConfig)
		customerInputNames := localConfig.GetCustomerInputNames()

		customerInputSet := make(map[string]bool)
		for _, name := range customerInputNames {
			customerInputSet[name] = true
		}

		// Marshal/unmarshal to work with the untyped config
		jsonBytes, _ := json.Marshal(inputConfig)
		var configMap map[string]interface{}
		if json.Unmarshal(jsonBytes, &configMap) == nil {
			if inputGroups, ok := configMap["input_groups"].([]interface{}); ok {
				for _, group := range inputGroups {
					groupMap, ok := group.(map[string]interface{})
					if !ok {
						continue
					}
					appInputs, ok := groupMap["app_inputs"].([]interface{})
					if !ok {
						continue
					}
					gd := customerpages.InputGroupDisplay{
						Name:        strVal(groupMap, "name"),
						DisplayName: strVal(groupMap, "display_name"),
					}
					for _, input := range appInputs {
						inputMap, ok := input.(map[string]interface{})
						if !ok {
							continue
						}
						inputName := strVal(inputMap, "name")
						configuredBy := "vendor"
						if customerInputSet[inputName] {
							configuredBy = "customer"
						}
						gd.Inputs = append(gd.Inputs, customerpages.InputDisplay{
							Name:         inputName,
							DisplayName:  strVal(inputMap, "display_name"),
							Description:  strVal(inputMap, "description"),
							Type:         strVal(inputMap, "input_type"),
							Required:     boolVal(inputMap, "required"),
							Sensitive:    boolVal(inputMap, "sensitive"),
							Default:      strVal(inputMap, "default"),
							ConfiguredBy: configuredBy,
						})
					}
					if len(gd.Inputs) > 0 {
						display.InputGroups = append(display.InputGroups, gd)
					}
				}
			}
		}
	}

	return display
}

// CustomerAppDetailPage renders the detail page for a single published app.
