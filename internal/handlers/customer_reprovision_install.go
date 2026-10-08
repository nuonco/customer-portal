package handlers

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/pkg/nuon"
	"go.uber.org/zap"
)

func (h *Handler) ReprovisionInstall(c *gin.Context) {
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

	// Initialize Nuon client to reprovision the install using global API URL
	nuonClient, err := nuon.NewClientWithURL(nuonOrgDep.APIToken, nuonOrgDep.NuonOrgID, h.nuonAPIURLForOrg(nuonOrgDep))
	if err != nil {
		h.redirectOverviewWithAlert(c, install.ID, "error", "Failed to initialize Nuon client")
		return
	}

	// Reprovision the install via Nuon API
	if err := nuonClient.ReprovisionInstall(c.Request.Context(), install.NuonInstallID); err != nil {
		h.logger.Warn("reprovision failed",
			zap.String("install_id", install.ID),
			zap.String("nuon_install_id", install.NuonInstallID),
			zap.Error(err))
		h.redirectOverviewWithAlert(c, install.ID, "error", reprovisionErrorMessage(err))
		return
	}

	h.redirectOverviewWithAlert(c, install.ID, "success", "Install reprovisioning initiated successfully")
}

// redirectOverviewWithAlert redirects to the install overview page with alert query params.
func (h *Handler) redirectOverviewWithAlert(c *gin.Context, installID, alertType, alertMsg string) {
	redirectURL := fmt.Sprintf("%s/installs/%s/overview?alert_type=%s&alert_msg=%s",
		h.basePath, installID, alertType, url.QueryEscape(alertMsg))
	h.alertResponse(c, alertType, alertMsg, redirectURL)
}

// alertResponse answers JSON-preferring callers with the alert as a JSON body,
// and everything else with the existing redirect.
//
// fetch() follows a 302 transparently, so redirecting the React client landed it
// on whatever serves the redirect target. That URL is now the SPA fallback, which
// returns a JSON 404 for JSON callers — so every outcome surfaced as
// "Failed to ... (404)": a real failure lost its message, and a success was
// reported as a failure.
func (h *Handler) alertResponse(c *gin.Context, alertType, alertMsg, redirectURL string) {
	if wantsJSON(c) {
		status := http.StatusOK
		if alertType == "error" {
			// These call sites do not distinguish their failure modes, so use a
			// generic server error; the message carries the detail.
			status = http.StatusInternalServerError
		}
		c.JSON(status, gin.H{"status": alertType, "message": alertMsg})
		return
	}
	h.redirectOrHXRedirect(c, redirectURL)
}

// redirectBackWithAlert redirects to the originating page with alert query params.
// Falls back to the install overview workflow page if no originating URL is available.
func (h *Handler) redirectBackWithAlert(c *gin.Context, installID, workflowID, alertType, alertMsg string) {
	// Default to overview page
	redirectPath := fmt.Sprintf("%s/installs/%s/overview/workflows/%s", h.basePath, installID, workflowID)

	// Use the originating page path if available (so wizard stays on wizard, overview stays on overview)
	params := url.Values{}
	if currentURL := c.GetHeader("HX-Current-URL"); currentURL != "" {
		if parsed, err := url.Parse(currentURL); err == nil {
			redirectPath = parsed.Path
			params = parsed.Query()
		}
	} else if referer := c.GetHeader("Referer"); referer != "" {
		if parsed, err := url.Parse(referer); err == nil {
			redirectPath = parsed.Path
			params = parsed.Query()
		}
	}

	// Remove stale alert params and partial param, then set fresh alert
	params.Del("alert_type")
	params.Del("alert_msg")
	params.Del("partial")
	params.Set("alert_type", alertType)
	params.Set("alert_msg", alertMsg)

	h.alertResponse(c, alertType, alertMsg, redirectPath+"?"+params.Encode())
}

// redirectOrHXRedirect sends an HX-Redirect header for HTMX requests (so hx-boost
// triggers a full page navigation), or a standard 302 redirect for plain requests.
func (h *Handler) redirectOrHXRedirect(c *gin.Context, url string) {
	if isHTMXRequest(c) {
		c.Header("HX-Redirect", url)
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusFound, url)
}

// reprovisionErrorMessage surfaces what the Nuon API said, instead of a flat
// "Failed to reprovision install" that hides the cause.
func reprovisionErrorMessage(err error) string {
	if nuon.IsUnreachable(err) {
		return "Could not reach the Nuon API. Please try again."
	}
	if apiErr := nuon.ParseAPIError(err); apiErr.Description != "" {
		return "Failed to reprovision install: " + apiErr.Description
	}
	return "Failed to reprovision install"
}
