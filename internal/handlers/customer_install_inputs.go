package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) GetInstallInputs(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	install := installInterface.(*models.Install)

	// Load install with both InstallLink.NuonOrg and Org (handles published-app installs)
	if err := h.loadInstallWithOrg(install); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	// Create Nuon client using org credentials
	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil || nuonOrg.APIToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization credentials not configured"})
		return
	}

	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Fetch current inputs from Nuon API
	// Note: This returns 404 if no inputs have been set yet - that's OK, we still want to show the form
	currentInputs, err := nuonClient.GetInstallCurrentInputs(c.Request.Context(), install.NuonInstallID)
	if err != nil {
		// Log the error but don't fail - we can still show the input form with empty values
		// The input config will tell us what fields to display
		currentInputs = nil
	}
	// Fetch app input config for field definitions
	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), install.GetAppID())
	if err != nil {
		// Input config may not exist, that's okay - return empty
		inputConfig = nil
	}

	// Fetch local customer input config for this app
	var localConfig models.AppInputConfig
	localOrgID := install.OrgID
	if install.InstallLinkID != nil {
		localOrgID = install.InstallLink.OrgID
	}
	h.db.Where("org_id = ? AND app_id = ?", localOrgID, install.GetAppID()).First(&localConfig)
	customerInputNames := localConfig.GetCustomerInputNames()
	groupOrder := localConfig.GetGroupOrder()
	inputOrder := localConfig.GetInputOrder()

	// Filter input config to only show customer-facing inputs and apply ordering
	if inputConfig != nil && len(customerInputNames) > 0 {
		jsonBytes, err := json.Marshal(inputConfig)
		if err == nil {
			var configMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
				inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeCustomer)
				inputConfig = applyInputOrdering(inputConfig, groupOrder, inputOrder)
			}
		}
	}

	// Build inputs map from current inputs
	// Use RedactedValues if Values is empty (RedactedValues contains all input values)
	inputs := make(map[string]string)
	if currentInputs != nil {
		if len(currentInputs.Values) > 0 {
			inputs = currentInputs.Values
		} else if len(currentInputs.RedactedValues) > 0 {
			inputs = currentInputs.RedactedValues
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"inputs":       inputs,
		"input_config": inputConfig,
	})
}

// UpdateInstallInputs handles updating inputs for an install via Nuon API

func (h *Handler) UpdateInstallInputs(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	install := installInterface.(*models.Install)

	var req struct {
		Inputs map[string]string `json:"inputs" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Load install with both InstallLink.NuonOrg and Org (handles published-app installs)
	if err := h.loadInstallWithOrg(install); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	// Create Nuon client using org credentials
	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil || nuonOrg.APIToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization credentials not configured"})
		return
	}

	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Update inputs via Nuon API
	workflowID, err := nuonClient.UpdateInstallInputs(c.Request.Context(), install.NuonInstallID, req.Inputs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update inputs: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"workflow_id": workflowID,
		"message":     "Inputs updated successfully. A new workflow has been triggered.",
	})
}

// installStatus holds the combined status flags for an install, determined by a single set of API calls.
