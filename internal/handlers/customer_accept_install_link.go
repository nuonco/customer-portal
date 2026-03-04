package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) AcceptInstallLink(c *gin.Context) {
	// REQUIRE authentication - user must be logged in first
	customer := h.tryGetLoggedInUser(c)
	if customer == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Please log in first"})
		return
	}

	var req struct {
		SHA      string            `json:"sha" binding:"required"`
		Region   string            `json:"region"`   // AWS region (customer chooses)
		Location string            `json:"location"` // Azure location (customer chooses)
		Inputs   map[string]string `json:"inputs"`   // Customer-facing inputs
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find the install link
	var link models.InstallLink
	if err := h.db.Preload("NuonOrg").Where("sha = ?", req.SHA).First(&link).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install link not found"})
		return
	}

	if link.Used {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Install link already used"})
		return
	}

	// Validate region or location is provided
	region := req.Region
	location := req.Location
	if region == "" && location == "" {
		region = "us-east-1" // Default to AWS us-east-1
	}

	// Get vendor inputs from link
	vendorInputs, err := link.GetVendorInputs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read vendor inputs"})
		return
	}

	// Merge vendor and customer inputs (customer inputs take precedence if overlap)
	mergedInputs := make(map[string]string)
	for k, v := range vendorInputs {
		mergedInputs[k] = v
	}
	for k, v := range req.Inputs {
		mergedInputs[k] = v
	}

	// Initialize Nuon client with the org's credentials
	nuonClient, err := nuon.NewClientWithURL(link.NuonOrg.APIToken, link.NuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to initialize Nuon client: %v", err)})
		return
	}

	// Use the vendor-provided install name from the link
	installName := link.Name

	// Create the install via Nuon API with merged inputs
	nuonInstall, err := nuonClient.CreateInstallWithCustomName(c.Request.Context(), link.AppID, link.AppName, installName, region, location, mergedInputs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create install via Nuon API: %v", err)})
		return
	}

	// Create local Install record with customer as owner
	linkID := link.ID
	install := &models.Install{
		OrgID:             link.OrgID,   // Link to vendor's org
		UserID:            customer.ID,  // Customer owns the install
		CreatedByVendorID: &link.UserID, // Track original vendor
		InstallLinkID:     &linkID,
		NuonInstallID:     nuonInstall.ID,
		Name:              installName,
		Status:            models.StatusPending,
		Region:            region,
		Visibility:        models.VisibilityAccount,
	}

	// Associate with customer's active account if they have one
	var members []models.CustomerAccountMember
	if err := h.db.Where("user_id = ? AND org_id = ? AND deleted_at IS NULL", customer.ID, link.OrgID).Find(&members).Error; err == nil && len(members) > 0 {
		selected := middleware.SelectActiveMember(c, members)
		install.CustomerAccountID = &selected.AccountID
	}

	if err := h.db.Create(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store install locally"})
		return
	}

	// Mark link as used
	link.Used = true
	if err := h.db.Save(&link).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update install link"})
		return
	}

	// Generate JWT token for the customer (customer is already *models.User)
	token, _, err := h.auth.TokenGenerator(customer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate authentication token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Install accepted successfully",
		"token":   token,
		"install": install,
	})
}

// InstallsPage renders the customer installs page with tab-based pagination
