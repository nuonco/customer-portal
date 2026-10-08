package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
	"gorm.io/gorm"
)

func (h *Handler) UpdateAppCustomerInputConfig(c *gin.Context) {
	appID := c.Param("app_id")

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}
	orgID := org.ID

	// Parse request body
	var req struct {
		CustomerInputNames []string            `json:"customer_input_names"`
		GroupOrder         []string            `json:"group_order"`
		InputOrder         map[string][]string `json:"input_order"`
		CollapsedGroups    []string            `json:"collapsed_groups"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Find or create the AppInputConfig
	var config models.AppInputConfig
	result := h.db.Where("org_id = ? AND app_id = ?", orgID, appID).First(&config)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Create new config
			config = models.AppInputConfig{
				OrgID: orgID,
				AppID: appID,
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch input config"})
			return
		}
	}

	// Update customer input names
	if err := config.SetCustomerInputNames(req.CustomerInputNames); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set customer input names"})
		return
	}

	// Update group order
	if err := config.SetGroupOrder(req.GroupOrder); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set group order"})
		return
	}

	// Update input order
	if err := config.SetInputOrder(req.InputOrder); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set input order"})
		return
	}

	// Update collapsed groups
	if err := config.SetCollapsedGroups(req.CollapsedGroups); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set collapsed groups"})
		return
	}

	// Save the config
	if result.Error == gorm.ErrRecordNotFound {
		if err := h.db.Create(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create input config"})
			return
		}
	} else {
		if err := h.db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update input config"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"customer_input_names": config.GetCustomerInputNames(),
		"group_order":          config.GetGroupOrder(),
		"input_order":          config.GetInputOrder(),
		"collapsed_groups":     config.GetCollapsedGroups(),
	})
}

// CustomersPage displays all customers who have installed apps from this org
