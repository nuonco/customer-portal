package jsonhandlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
	vendorpages "github.com/nuonco/customer-portal/internal/views/vendorui/pages"
	"github.com/nuonco/customer-portal/pkg/nuon"
	"go.uber.org/zap"
)

func (h *VendorHandler) CustomerInstalls(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	filterEmail := c.Query("email")
	filterApp := c.Query("app")
	filterPlatform := c.Query("platform")

	type installRow struct {
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

	var rows []installRow
	if err := query.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch installs: %v", err)})
		return
	}

	platformMap := make(map[string]string)
	appNameMap := make(map[string]string)
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err == nil {
		apps, appsErr := nuonClient.ListApps(c.Request.Context())
		if appsErr != nil {
			zap.L().Warn("failed to fetch apps from Nuon API for platform/name resolution", zap.String("org_id", org.ID), zap.Error(appsErr))
		} else {
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

	adminInstalls := make([]vendorpages.AdminInstall, 0, len(rows))
	for _, row := range rows {
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
				if row.AppName == "" {
					h.db.Model(&models.Install{}).Where("id = ?", row.ID).Update("app_name", name)
				}
			}
		}
		if effectiveAppName == "" && nuonClient != nil && effectiveAppID != "" {
			if app, appErr := nuonClient.GetApp(c.Request.Context(), effectiveAppID); appErr == nil && app != nil && app.Name != "" {
				effectiveAppName = app.Name
				if row.AppName == "" {
					h.db.Model(&models.Install{}).Where("id = ?", row.ID).Update("app_name", app.Name)
				}
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

		if filterApp != "" && !strings.Contains(strings.ToLower(effectiveAppName), strings.ToLower(filterApp)) {
			continue
		}

		platform := "aws"
		if value, ok := platformMap[effectiveAppID]; ok {
			platform = value
		}
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

	c.JSON(http.StatusOK, gin.H{
		"installs":    adminInstalls,
		"org_id":      org.ID,
		"nuon_org_id": org.NuonOrgID,
	})
}
