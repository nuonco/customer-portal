package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

func (h *Handler) ForgetInstall(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	install := installInterface.(*models.Install)

	// Simply delete the install from the local database
	// This does NOT call any Nuon APIs - it just removes the record locally
	if err := h.db.Delete(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to forget install"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Install forgotten successfully",
	})
}

// loadInstallWithOrg reloads install with both InstallLink.NuonOrg and Org preloaded,
// required for published-app installs where InstallLinkID is nil.
