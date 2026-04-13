package handlers

import (
	"encoding/json"
	"fmt"
	"io"
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

	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
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
	// Fetch app input config as raw JSON to preserve all fields (including source)
	rawConfig, err := nuonClient.GetAppInputConfigRaw(c.Request.Context(), install.GetAppID())
	var inputConfig interface{} = rawConfig
	if err != nil {
		inputConfig = nil
	}

	// Build set of install_stack input names from top-level inputs array.
	// Inputs with source="customer" are managed by the install stack and
	// cannot be updated via the API after install creation.
	installStackInputs := make(map[string]bool)
	if rawConfig != nil {
		if topInputs, ok := rawConfig["inputs"].([]interface{}); ok {
			for _, inp := range topInputs {
				if im, ok := inp.(map[string]interface{}); ok {
					source, _ := im["source"].(string)
					name, _ := im["name"].(string)
					if source == "customer" && name != "" {
						installStackInputs[name] = true
					}
				}
			}
		}
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
		if configMap, ok := inputConfig.(map[string]interface{}); ok {
			inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeCustomer)
			inputConfig = applyInputOrdering(inputConfig, groupOrder, inputOrder)
		}
	}

	// Remove install_stack inputs from input_groups
	if len(installStackInputs) > 0 {
		if configMap, ok := inputConfig.(map[string]interface{}); ok {
			inputConfig = filterInputsByName(configMap, installStackInputs)
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

	// Strip install_stack inputs from values map
	for name := range installStackInputs {
		delete(inputs, name)
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

	// Read raw body so we can parse both nested and bracket-notation inputs
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req struct {
		Inputs map[string]string `json:"inputs"`
	}

	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// HTMX json-enc sends inputs as flat "inputs[name]" keys; extract them
	var raw map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err == nil {
		req.Inputs = extractBracketInputs(raw, req.Inputs)
	}

	if len(req.Inputs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No inputs provided"})
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

	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Update inputs via Nuon API
	workflowID, err := nuonClient.UpdateInstallInputs(c.Request.Context(), install.NuonInstallID, req.Inputs)
	if err != nil {
		if c.GetHeader("HX-Request") != "" || c.Request.Header.Get("Accept") == "text/html" {
			h.redirectOverviewWithAlert(c, install.ID, "error", fmt.Sprintf("Failed to update inputs: %v", err))
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update inputs: %v", err)})
		return
	}

	// POST-Redirect-GET for HTML/HTMX requests
	if c.GetHeader("HX-Request") != "" || c.Request.Header.Get("Accept") == "text/html" {
		h.redirectOverviewWithAlert(c, install.ID, "success", "Inputs updated successfully. A new workflow has been triggered.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"workflow_id": workflowID,
		"message":     "Inputs updated successfully. A new workflow has been triggered.",
	})
}

// installStatus holds the combined status flags for an install, determined by a single set of API calls.
