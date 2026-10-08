package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/models"
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
		if c.GetHeader("HX-Request") != "" {
			c.String(http.StatusInternalServerError, "Failed to forget install")
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to forget install"})
		}
		return
	}

	if c.GetHeader("HX-Request") != "" {
		// Return empty string so HTMX removes the row
		c.String(http.StatusOK, "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Install forgotten successfully",
	})
}

// loadInstallWithOrg reloads install with both InstallLink.NuonOrg and Org preloaded,
// required for published-app installs where InstallLinkID is nil.
