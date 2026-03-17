package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) PublishApp(c *gin.Context) {
	appID := c.Param("app_id")
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	// Verify app exists in Nuon API
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	_, err = nuonClient.GetApp(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found"})
		return
	}

	// Find or create the PublishedApp record, then set status to published
	var publishedApp models.PublishedApp
	result := h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&publishedApp)
	if result.Error != nil {
		// Not found - create new
		publishedApp = models.PublishedApp{
			OrgID:  org.ID,
			AppID:  appID,
			Status: models.AppStatusPublished,
		}
		if err := h.db.Create(&publishedApp).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to publish app"})
			return
		}
	} else {
		h.db.Model(&publishedApp).Update("status", models.AppStatusPublished)
	}

	c.JSON(http.StatusOK, publishedApp)
}

// UnpublishApp removes a published app so customers can no longer discover it
