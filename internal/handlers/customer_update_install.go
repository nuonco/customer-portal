package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
)

func (h *Handler) UpdateInstall(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	install := installInterface.(*models.Install)

	var req struct {
		Region     string `json:"region,omitempty"`
		Visibility string `json:"visibility,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := middleware.GetCurrentUser(c)

	// Update allowed fields
	if req.Region != "" {
		install.Region = req.Region
	}

	// Only the install owner can change visibility
	if req.Visibility != "" && install.UserID == user.ID {
		if req.Visibility == string(models.VisibilityAccount) || req.Visibility == string(models.VisibilityPrivate) {
			install.Visibility = models.InstallVisibility(req.Visibility)
		}
	}

	if err := h.db.Save(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update install"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Install updated successfully",
		"install": install,
	})
}

// DeleteInstall handles customer install deletion (deprovision)
