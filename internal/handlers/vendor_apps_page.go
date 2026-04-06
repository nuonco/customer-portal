package handlers

import (
	"fmt"
	"math"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"go.uber.org/zap"
)

func (h *Handler) AppsPage(c *gin.Context) {
	user := h.GetFreshUser(c) // Load from DB for topbar display

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		allOrgs := h.GetUserOrgs(user.ID)
		props := vendorpages.AppsPageProps{
			LayoutProps: vendorui.LayoutProps{
				Title:             org.Name + " - Apps",
				ActivePage:        "apps",
				User:              user,
				CurrentOrg:        org,
				Orgs:              allOrgs,
				Breadcrumbs:       []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: true}},
				BasePath:          h.basePath,
				PortalScheme:      h.schemeFromBaseURL(),
				DashboardURL:      h.dashboardURL,
				PortalBaseDomain:  h.subdomainBaseDomain,
				CSSPath:           assets.VendorCSSPath(),
				IsSuperuser:       h.isSuperuser(user),
				NuonAPIErrorTitle: nuon.ParseAPIError(err).Title,
				NuonAPIError:      nuon.ParseAPIError(err).Description,
			},
			Org: *org,
		}
		h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
		h.RenderTempl(c, http.StatusOK, vendorpages.AppsPage(props))
		return
	}

	// Fetch apps from Nuon API
	apps, err := nuonClient.ListApps(c.Request.Context())
	if err != nil {
		allOrgs := h.GetUserOrgs(user.ID)
		props := vendorpages.AppsPageProps{
			LayoutProps: vendorui.LayoutProps{
				Title:             org.Name + " - Apps",
				ActivePage:        "apps",
				User:              user,
				CurrentOrg:        org,
				Orgs:              allOrgs,
				Breadcrumbs:       []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: true}},
				BasePath:          h.basePath,
				PortalScheme:      h.schemeFromBaseURL(),
				DashboardURL:      h.dashboardURL,
				PortalBaseDomain:  h.subdomainBaseDomain,
				CSSPath:           assets.VendorCSSPath(),
				IsSuperuser:       h.isSuperuser(user),
				NuonAPIErrorTitle: nuon.ParseAPIError(err).Title,
				NuonAPIError:      nuon.ParseAPIError(err).Description,
			},
			Org: *org,
		}
		h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
		h.RenderTempl(c, http.StatusOK, vendorpages.AppsPage(props))
		return
	}

	// Get all published apps for this org
	var publishedApps []models.PublishedApp
	h.db.Where("org_id = ?", org.ID).Order("sort_order ASC, created_at ASC").Find(&publishedApps)
	h.logger.Info("AppsPage: loaded published apps from DB",
		zap.Int("count", len(publishedApps)),
	)
	for _, pa := range publishedApps {
		h.logger.Info("AppsPage: published app from DB",
			zap.String("app_id", pa.AppID),
			zap.Int("sort_order", pa.SortOrder),
			zap.String("status", pa.Status),
		)
	}
	appStatuses := make(map[string]string)
	sortOrderMap := make(map[string]int)
	logoLightMap := make(map[string]string)
	logoDarkMap := make(map[string]string)
	for _, pa := range publishedApps {
		status := pa.Status
		if status == "" {
			status = "published"
		}
		appStatuses[pa.AppID] = status
		sortOrderMap[pa.AppID] = pa.SortOrder
		if pa.LogoLightBase64 != "" {
			logoLightMap[pa.AppID] = pa.LogoLightBase64
		}
		if pa.LogoDarkBase64 != "" {
			logoDarkMap[pa.AppID] = pa.LogoDarkBase64
		}
	}

	// Build app list
	appsWithStatus := make([]AppInfo, len(apps))
	for i, app := range apps {
		// Extract platform from runner config (same as GetAppInputConfig)
		platform := "aws" // default
		if app.RunnerConfig != nil {
			runnerType := string(app.RunnerConfig.AppRunnerType)
			if runnerType == "azure" {
				platform = "azure"
			} else if runnerType == "gcp" {
				platform = "gcp"
			}
		}

		appsWithStatus[i] = AppInfo{
			ID:              app.ID,
			Name:            app.Name,
			Platform:        platform,
			LogoLightBase64: logoLightMap[app.ID],
			LogoDarkBase64:  logoDarkMap[app.ID],
		}
	}

	// Find orphaned published apps (deleted from Nuon API but still published)
	apiAppIDs := make(map[string]bool, len(apps))
	for _, app := range apps {
		apiAppIDs[app.ID] = true
	}
	for _, pa := range publishedApps {
		if !apiAppIDs[pa.AppID] {
			appsWithStatus = append(appsWithStatus, AppInfo{
				ID:              pa.AppID,
				Name:            "-",
				LogoLightBase64: pa.LogoLightBase64,
				LogoDarkBase64:  pa.LogoDarkBase64,
				Deleted:         true,
			})
		}
	}

	// Sort appsWithStatus by sort_order (published apps first, then unpublished)
	sort.SliceStable(appsWithStatus, func(i, j int) bool {
		oi, iHas := sortOrderMap[appsWithStatus[i].ID]
		oj, jHas := sortOrderMap[appsWithStatus[j].ID]
		if !iHas {
			oi = math.MaxInt32
		}
		if !jHas {
			oj = math.MaxInt32
		}
		return oi < oj
	})

	// Log final render order
	for idx, app := range appsWithStatus {
		h.logger.Info("AppsPage: final render order",
			zap.Int("index", idx),
			zap.String("app_id", app.ID),
			zap.String("name", app.Name),
		)
	}

	// Get user's orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	// Convert to templ type
	templApps := make([]vendorpages.AppWithHealthCheckStatus, len(appsWithStatus))
	for i, app := range appsWithStatus {
		templApps[i] = vendorpages.AppWithHealthCheckStatus{
			ID:              app.ID,
			Name:            app.Name,
			Platform:        app.Platform,
			LogoLightBase64: app.LogoLightBase64,
			LogoDarkBase64:  app.LogoDarkBase64,
			Deleted:         app.Deleted,
		}
	}

	props := vendorpages.AppsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:            org.Name + " - Apps",
			ActivePage:       "apps",
			User:             user,
			CurrentOrg:       org,
			Orgs:             allOrgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
			IsSuperuser:      h.isSuperuser(user),
		},
		Org:         *org,
		Apps:        templApps,
		AppStatuses: appStatuses,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.AppsPage(props))
}

// PublishApp publishes an app so customers can discover and install it without a link

func (h *Handler) orgHasPublishedApps(orgID string) bool {
	var count int64
	h.db.Model(&models.PublishedApp{}).Where("org_id = ? AND status IN ?", orgID, []string{models.AppStatusPublished, models.AppStatusComingSoon}).Count(&count)
	return count > 0
}

// AppsPage displays the vendor apps list
