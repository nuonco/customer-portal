package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

func (h *Handler) UnpublishApp(c *gin.Context) {
	appID := c.Param("app_id")
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	// Set status to unpublished (keep record for sort order)
	result := h.db.Model(&models.PublishedApp{}).
		Where("org_id = ? AND app_id = ?", org.ID, appID).
		Update("status", models.AppStatusUnpublished)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unpublish app"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "App unpublished successfully"})
}

// UpdateAppOrder updates the sort_order of published apps for an org
