package jsonhandlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *VendorHandler) ImportInstall(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	vendor := middleware.GetCurrentUser(c)
	nuonInstallID := strings.TrimSpace(c.PostForm("nuon_install_id"))
	appID := strings.TrimSpace(c.PostForm("app_id"))
	customerEmail := strings.TrimSpace(c.PostForm("customer_email"))

	if nuonInstallID == "" || customerEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Install ID and customer email are required."})
		return
	}

	var existing models.Install
	if err := h.db.Where("nuon_install_id = ? AND org_id = ?", nuonInstallID, org.ID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "This install has already been imported."})
		return
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client."})
		return
	}

	nuonInstall, err := nuonClient.GetInstall(c.Request.Context(), nuonInstallID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Install not found in Nuon API: %v", err)})
		return
	}

	appName := ""
	if appID != "" {
		nuonApp, appErr := nuonClient.GetApp(c.Request.Context(), appID)
		if appErr == nil {
			appName = nuonApp.Name
		}
	}

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

	var customerUser models.User
	if err := h.db.Where("email = ?", customerEmail).First(&customerUser).Error; err != nil {
		customerUser = models.User{Email: customerEmail, Name: customerEmail, Role: models.RoleCustomer}
		if createErr := h.db.Create(&customerUser).Error; createErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create customer: %v", createErr)})
			return
		}
	}

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save install: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Install imported successfully"})
}
