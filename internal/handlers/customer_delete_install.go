package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) DeleteInstall(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		h.redirectOverviewWithAlert(c, c.Param("install_id"), "error", "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	// Load install with org relationships
	if err := h.db.Preload("InstallLink.NuonOrg").Preload("Org").Where("id = ?", install.ID).First(install).Error; err != nil {
		h.redirectOverviewWithAlert(c, install.ID, "error", "Failed to load install details")
		return
	}

	nuonOrgDep := install.GetNuonOrg()
	if nuonOrgDep == nil {
		h.redirectOverviewWithAlert(c, install.ID, "error", "Organization information not found")
		return
	}

	// Initialize Nuon client to deprovision the install using global API URL
	nuonClient, err := nuon.NewClientWithURL(nuonOrgDep.APIToken, nuonOrgDep.NuonOrgID, h.nuonAPIURLForOrg(nuonOrgDep))
	if err != nil {
		h.redirectOverviewWithAlert(c, install.ID, "error", "Failed to initialize Nuon client")
		return
	}

	// Deprovision the install via Nuon API
	if err := nuonClient.DeprovisionInstall(c.Request.Context(), install.NuonInstallID); err != nil {
		h.redirectOverviewWithAlert(c, install.ID, "error", "Failed to deprovision install")
		return
	}

	// Update status to deprovisioning
	install.Status = models.StatusDeprovisioning
	if err := h.db.Save(install).Error; err != nil {
		h.redirectOverviewWithAlert(c, install.ID, "error", "Failed to update install status")
		return
	}

	h.redirectOverviewWithAlert(c, install.ID, "success", "Install deprovisioning initiated successfully")
}

// ForgetInstall handles customer install forgetting (local database removal only)
