package jsonhandlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/pkg/nuon"
	"go.uber.org/zap"
)

func (h *VendorHandler) SearchNuonInstalls(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	q := strings.TrimSpace(c.Query("q"))
	if len(q) < 3 {
		c.JSON(http.StatusOK, gin.H{"results": []interface{}{}})
		return
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	apps, err := nuonClient.ListApps(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch apps"})
		return
	}

	type searchResult struct {
		InstallID   string
		InstallName string
		AppID       string
		AppName     string
		Status      string
		Region      string
	}

	var results []searchResult
	for _, app := range apps {
		if len(results) >= 20 {
			break
		}
		installs, listErr := nuonClient.ListAppInstalls(c.Request.Context(), app.ID, q)
		if listErr != nil {
			zap.L().Warn("failed to search installs for app", zap.String("app_id", app.ID), zap.Error(listErr))
			continue
		}
		for _, install := range installs {
			if len(results) >= 20 {
				break
			}
			region := ""
			if install.AwsAccount != nil && install.AwsAccount.Region != "" {
				region = install.AwsAccount.Region
			} else if install.AzureAccount != nil && install.AzureAccount.Location != "" {
				region = install.AzureAccount.Location
			}
			results = append(results, searchResult{
				InstallID:   install.ID,
				InstallName: install.Name,
				AppID:       app.ID,
				AppName:     app.Name,
				// `status` was removed from app.Install; use the component rollup.
				Status: install.CompositeComponentStatus,
				Region: region,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}
