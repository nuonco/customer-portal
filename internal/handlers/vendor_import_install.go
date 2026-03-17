package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) ImportInstall(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.String(http.StatusBadRequest, "Organization context not found")
		return
	}

	vendor := h.GetFreshUser(c)

	nuonInstallID := strings.TrimSpace(c.PostForm("nuon_install_id"))
	appID := strings.TrimSpace(c.PostForm("app_id"))
	customerEmail := strings.TrimSpace(c.PostForm("customer_email"))

	if nuonInstallID == "" || customerEmail == "" {
		c.String(http.StatusBadRequest, "Install ID and customer email are required.")
		return
	}

	// Check if already imported
	var existing models.Install
	if err := h.db.Where("nuon_install_id = ? AND org_id = ?", nuonInstallID, org.ID).First(&existing).Error; err == nil {
		c.String(http.StatusConflict, "This install has already been imported.")
		return
	}

	// Fetch install from Nuon API
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to initialize Nuon client.")
		return
	}

	nuonInstall, err := nuonClient.GetInstall(c.Request.Context(), nuonInstallID)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("Install not found in Nuon API: %v", err))
		return
	}

	// Fetch app name from Nuon API
	appName := ""
	if appID != "" {
		nuonApp, appErr := nuonClient.GetApp(c.Request.Context(), appID)
		if appErr == nil {
			appName = nuonApp.Name
		}
	}

	// Resolve install name, region and status
	installName := nuonInstall.Name
	region := ""
	if nuonInstall.AwsAccount != nil && nuonInstall.AwsAccount.Region != "" {
		region = nuonInstall.AwsAccount.Region
	} else if nuonInstall.AzureAccount != nil && nuonInstall.AzureAccount.Location != "" {
		region = nuonInstall.AzureAccount.Location
	}
	installStatus := models.StatusActive
	switch nuonInstall.Status {
	case "provisioning":
		installStatus = models.StatusProvisioning
	case "failed":
		installStatus = models.StatusFailed
	case "deprovisioning":
		installStatus = models.StatusDeprovisioning
	}

	// Find or create customer user
	var customerUser models.User
	if err := h.db.Where("email = ?", customerEmail).First(&customerUser).Error; err != nil {
		// Create new customer record
		customerUser = models.User{
			Email: customerEmail,
			Name:  customerEmail,
			Role:  models.RoleCustomer,
		}
		if createErr := h.db.Create(&customerUser).Error; createErr != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create customer: %v", createErr))
			return
		}
	}

	// Create local install record
	vendorID := vendor.ID
	install := models.Install{
		OrgID:             org.ID,
		UserID:            customerUser.ID,
		CreatedByVendorID: &vendorID,
		InstallLinkID:     nil,
		NuonInstallID:     nuonInstallID,
		NuonAppID:         appID,
		AppName:           appName,
		Name:              installName,
		Status:            installStatus,
		Region:            region,
	}
	if err := h.db.Create(&install).Error; err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to save install: %v", err))
		return
	}

	// Redirect to installs page
	redirectURL := fmt.Sprintf("%s/orgs/%s/installs", h.basePath, org.ID)
	if isHTMXRequest(c) {
		c.Header("HX-Redirect", redirectURL)
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusFound, redirectURL)
}

// AdminForgetInstall removes an install from the portal database (vendor-only action).
// This does NOT call any Nuon APIs — it only deletes the local record.
