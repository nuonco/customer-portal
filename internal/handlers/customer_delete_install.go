package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/pkg/nuon"
	"go.uber.org/zap"
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
		h.logger.Warn("deprovision failed",
			zap.String("install_id", install.ID),
			zap.String("nuon_install_id", install.NuonInstallID),
			zap.Error(err))
		h.redirectOverviewWithAlert(c, install.ID, "error", deprovisionErrorMessage(err))
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

// deprovisionErrorMessage surfaces what the Nuon API said rather than a flat
// message that hides the cause.
func deprovisionErrorMessage(err error) string {
	if nuon.IsUnreachable(err) {
		return "Could not reach the Nuon API. Please try again."
	}
	if apiErr := nuon.ParseAPIError(err); apiErr.Description != "" {
		return "Failed to deprovision install: " + apiErr.Description
	}
	return "Failed to deprovision install"
}
