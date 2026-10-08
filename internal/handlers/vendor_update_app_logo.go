package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
)

func (h *Handler) UpdateAppLogo(c *gin.Context) {
	appID := c.Param("app_id")

	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	var req struct {
		LogoLightBase64 string `json:"logo_light_base64"`
		LogoDarkBase64  string `json:"logo_dark_base64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Find or create the PublishedApp record
	var pa models.PublishedApp
	result := h.db.Unscoped().Where("org_id = ? AND app_id = ?", org.ID, appID).First(&pa)
	if result.Error != nil {
		pa = models.PublishedApp{OrgID: org.ID, AppID: appID}
	}

	if req.LogoLightBase64 == "REMOVE" {
		pa.LogoLightBase64 = ""
	} else if strings.HasPrefix(req.LogoLightBase64, "data:image/") {
		pa.LogoLightBase64 = req.LogoLightBase64
	}

	if req.LogoDarkBase64 == "REMOVE" {
		pa.LogoDarkBase64 = ""
	} else if strings.HasPrefix(req.LogoDarkBase64, "data:image/") {
		pa.LogoDarkBase64 = req.LogoDarkBase64
	}

	if result.Error != nil {
		h.db.Create(&pa)
	} else {
		h.db.Unscoped().Model(&pa).Updates(map[string]interface{}{
			"logo_light_base64": pa.LogoLightBase64,
			"logo_dark_base64":  pa.LogoDarkBase64,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// LoginSettingsPage renders the login settings page
