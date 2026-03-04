package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

func (h *Handler) UpdateAppOrder(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	var body struct {
		AppIDs          []string `json:"app_ids"`
		PublishedAppIDs []string `json:"published_app_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	publishedSet := make(map[string]bool, len(body.PublishedAppIDs))
	for _, id := range body.PublishedAppIDs {
		publishedSet[id] = true
	}

	for i, appID := range body.AppIDs {
		if publishedSet[appID] {
			var pa models.PublishedApp
			result := h.db.Unscoped().Where("org_id = ? AND app_id = ?", org.ID, appID).First(&pa)
			updates := map[string]interface{}{"deleted_at": nil, "sort_order": i}
			if result.Error != nil {
				newPA := models.PublishedApp{OrgID: org.ID, AppID: appID, SortOrder: i}
				h.db.Create(&newPA)
			} else {
				h.db.Unscoped().Model(&pa).Updates(updates)
			}
		} else {
			h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).Delete(&models.PublishedApp{})
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "configuration updated"})
}

// AppDetailRedirect redirects app detail to inputs page
