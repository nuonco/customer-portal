package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

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

func (h *Handler) CustomerInstallsPage(c *gin.Context) {
	user := h.GetFreshUser(c)

	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}

	filterEmail := c.Query("email")
	filterApp := c.Query("app")
	filterPlatform := c.Query("platform")

	// Build query for installs joined with their owning user
	type InstallRow struct {
		ID                 string
		NuonInstallID      string
		NuonOrgID          string
		Name               string
		UserID             string
		CustomerEmail      string
		CustomerName       string
		AppID              string
		AppName            string
		NuonAppID          string
		Status             string
		Region             string
		CreatedAt          time.Time
		InstallLinkID      string
		InstallLinkAppID   string
		InstallLinkAppName string
		APIDeleted         bool
	}

	query := h.db.Table("installs").
		Select(`installs.id, installs.nuon_install_id, installs.user_id,
			installs.name, installs.app_id, installs.app_name,
			installs.nuon_app_id, installs.api_deleted,
			installs.status, installs.region, installs.created_at,
			COALESCE(installs.install_link_id::text, '') as install_link_id,
			users.email as customer_email, users.name as customer_name,
			COALESCE(install_links.app_id, '') as install_link_app_id,
			COALESCE(install_links.app_name, '') as install_link_app_name`).
		Joins("JOIN users ON users.id = installs.user_id").
		Joins("LEFT JOIN install_links ON install_links.id = installs.install_link_id").
		Where("installs.org_id = ? AND installs.deleted_at IS NULL", org.ID).
		Order("installs.created_at DESC")

	if filterEmail != "" {
		query = query.Where("users.email ILIKE ?", "%"+filterEmail+"%")
	}
	// Note: app filter is applied in Go after name resolution (see below)
	// because published-app installs get names from the Nuon API, not DB.

	var rows []InstallRow
	if err := query.Find(&rows).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch installs: %v", err))
		return
	}

	// Fetch apps from Nuon API to build platform and name maps
	platformMap := make(map[string]string)
	appNameMap := make(map[string]string)
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err == nil {
		apps, appsErr := nuonClient.ListApps(c.Request.Context())
		if appsErr != nil {
			zap.L().Warn("failed to fetch apps from Nuon API for platform/name resolution",
				zap.String("org_id", org.ID), zap.Error(appsErr))
		}
		if appsErr == nil {
			for _, app := range apps {
				platform := "aws"
				if app.RunnerConfig != nil && string(app.RunnerConfig.AppRunnerType) == "azure" {
					platform = "azure"
				}
				platformMap[app.ID] = platform
				appNameMap[app.ID] = app.Name
			}
		}
	}

	// Convert rows to AdminInstall, applying platform filter in Go
	adminInstalls := make([]vendorpages.AdminInstall, 0, len(rows))
	for _, row := range rows {
		// Resolve app ID and name from all sources
		effectiveAppID := row.AppID
		if effectiveAppID == "" {
			effectiveAppID = row.InstallLinkAppID
		}
		if effectiveAppID == "" {
			effectiveAppID = row.NuonAppID
		}
		effectiveAppName := row.AppName
		if effectiveAppName == "" {
			effectiveAppName = row.InstallLinkAppName
		}
		if effectiveAppName == "" {
			if name, ok := appNameMap[effectiveAppID]; ok {
				effectiveAppName = name
				// Backfill app_name for installs missing it
				if row.AppName == "" {
					h.db.Model(&models.Install{}).Where("id = ?", row.ID).Update("app_name", name)
				}
			}
		}
		if effectiveAppName == "" && nuonClient != nil && effectiveAppID != "" {
			if app, err := nuonClient.GetApp(c.Request.Context(), effectiveAppID); err == nil && app != nil && app.Name != "" {
				effectiveAppName = app.Name
				// Backfill for future loads
				if row.AppName == "" {
					h.db.Model(&models.Install{}).Where("id = ?", row.ID).Update("app_name", app.Name)
				}
				// Also populate maps for other installs with the same app
				appNameMap[effectiveAppID] = app.Name
				if app.RunnerConfig != nil && string(app.RunnerConfig.AppRunnerType) == "azure" {
					platformMap[effectiveAppID] = "azure"
				} else {
					platformMap[effectiveAppID] = "aws"
				}
			}
		}
		if effectiveAppName == "" {
			effectiveAppName = "Unknown App"
		}

		// Apply app name filter (done in Go to cover API-resolved names)
		if filterApp != "" && !strings.Contains(strings.ToLower(effectiveAppName), strings.ToLower(filterApp)) {
			continue
		}

		// Resolve platform
		platform := "aws"
		if p, ok := platformMap[effectiveAppID]; ok {
			platform = p
		}

		// Apply platform filter
		if filterPlatform != "" && platform != filterPlatform {
			continue
		}

		adminInstalls = append(adminInstalls, vendorpages.AdminInstall{
			ID:            row.ID,
			NuonInstallID: row.NuonInstallID,
			NuonOrgID:     org.NuonOrgID,
			Name:          row.Name,
			CustomerID:    row.UserID,
			CustomerEmail: row.CustomerEmail,
			CustomerName:  row.CustomerName,
			AppID:         effectiveAppID,
			AppName:       effectiveAppName,
			Platform:      platform,
			Status:        row.Status,
			InstallLinkID: row.InstallLinkID,
			CreatedAt:     row.CreatedAt.Format("Jan 2, 2006"),
			APIDeleted:    row.APIDeleted,
		})
	}

	// Return partial for HTMX filter requests (but not hx-boost navigations)
	if isHTMXPartialRequest(c) {
		h.RenderTempl(c, http.StatusOK, vendorpages.InstallsTableBody(adminInstalls, org.ID, h.basePath, org.NuonOrgID))
		return
	}

	allOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.CustomerInstallsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      org.Name + " - Installs",
			ActivePage: "installs",
			User:       user,
			CurrentOrg: org,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: "Installs", Path: fmt.Sprintf("%s/orgs/%s/installs", h.basePath, org.ID), Active: true},
			},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
			IsSuperuser:      h.isSuperuser(user),
		},
		Org:            *org,
		Installs:       adminInstalls,
		FilterEmail:    filterEmail,
		FilterApp:      filterApp,
		FilterPlatform: filterPlatform,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.CustomerInstallsPage(props))
}

// SearchNuonInstalls searches the Nuon API for installs matching a query string.
// Returns an HTMX HTML fragment of result rows for the import modal.
