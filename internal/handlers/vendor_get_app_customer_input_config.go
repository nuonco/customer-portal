package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
	"gorm.io/gorm"
)

func (h *Handler) GetAppCustomerInputConfig(c *gin.Context) {
	appID := c.Param("app_id")

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}
	orgID := org.ID

	// Fetch or create the AppInputConfig
	var config models.AppInputConfig
	result := h.db.Where("org_id = ? AND app_id = ?", orgID, appID).First(&config)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Return empty config if none exists
			c.JSON(http.StatusOK, gin.H{
				"customer_input_names": []string{},
				"group_order":          []string{},
				"input_order":          map[string][]string{},
				"collapsed_groups":     []string{},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch input config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"customer_input_names": config.GetCustomerInputNames(),
		"group_order":          config.GetGroupOrder(),
		"input_order":          config.GetInputOrder(),
		"collapsed_groups":     config.GetCollapsedGroups(),
	})
}

// UpdateAppCustomerInputConfig updates the local customer-facing input configuration for an app
