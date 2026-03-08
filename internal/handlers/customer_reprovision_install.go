package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) ReprovisionInstall(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	install := installInterface.(*models.Install)

	// Load install with org relationships
	if err := h.db.Preload("InstallLink.NuonOrg").Preload("Org").Where("id = ?", install.ID).First(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	nuonOrgDep := install.GetNuonOrg()
	if nuonOrgDep == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization information not found"})
		return
	}

	// Initialize Nuon client to reprovision the install using global API URL
	nuonClient, err := nuon.NewClientWithURL(nuonOrgDep.APIToken, nuonOrgDep.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Reprovision the install via Nuon API
	if err := nuonClient.ReprovisionInstall(c.Request.Context(), install.NuonInstallID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reprovision install"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Install reprovisioning initiated successfully",
		"install": install,
	})
}
