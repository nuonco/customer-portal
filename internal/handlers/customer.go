package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/markdown"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/overrides"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"github.com/nuonco/nuon-go/client/operations"
	nuonmodels "github.com/nuonco/nuon-go/models"
)

// FilterType represents the type of input filtering to apply
type FilterType string

const (
	FilterTypeVendor   FilterType = "vendor"
	FilterTypeCustomer FilterType = "customer"
)

// filterInputConfig filters the input config based on source field
// Vendor sees inputs where source == "vendor" (or not set for backward compatibility)
// Customer sees inputs where source == "customer"
func filterInputConfig(inputConfig interface{}, filterType FilterType) interface{} {
	if inputConfig == nil {
		return nil
	}

	// The input config is a map with input_groups
	configMap, ok := inputConfig.(map[string]interface{})
	if !ok {
		return inputConfig
	}

	inputGroups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return inputConfig
	}

	var filteredGroups []interface{}
	for _, group := range inputGroups {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}

		appInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok {
			continue
		}

		var filteredInputs []interface{}
		for _, input := range appInputs {
			inputMap, ok := input.(map[string]interface{})
			if !ok {
				continue
			}

			// Check source field (values: "vendor" or "customer")
			source, _ := inputMap["source"].(string)

			if filterType == FilterTypeCustomer {
				// Customer sees only source="customer"
				if source == "customer" {
					filteredInputs = append(filteredInputs, input)
				}
			} else {
				// Vendor sees source="vendor" or not set (backward compatibility)
				if source == "vendor" || source == "" {
					filteredInputs = append(filteredInputs, input)
				}
			}
		}

		// Only include groups that have inputs after filtering
		if len(filteredInputs) > 0 {
			filteredGroup := make(map[string]interface{})
			for k, v := range groupMap {
				filteredGroup[k] = v
			}
			filteredGroup["app_inputs"] = filteredInputs
			filteredGroups = append(filteredGroups, filteredGroup)
		}
	}

	// Return the filtered config
	result := make(map[string]interface{})
	for k, v := range configMap {
		result[k] = v
	}
	result["input_groups"] = filteredGroups
	return result
}

// filterInputConfigByLocalConfig filters the input config using the local customer input names
// For customer filtering: only include inputs that are in customerInputNames
// For vendor filtering: only include inputs that are NOT in customerInputNames
func filterInputConfigByLocalConfig(inputConfig interface{}, customerInputNames []string, filterType FilterType) interface{} {
	if inputConfig == nil {
		return nil
	}

	// Create a set for fast lookup
	customerInputSet := make(map[string]bool)
	for _, name := range customerInputNames {
		customerInputSet[name] = true
	}

	// The input config is a map with input_groups
	configMap, ok := inputConfig.(map[string]interface{})
	if !ok {
		return inputConfig
	}

	inputGroups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return inputConfig
	}

	var filteredGroups []interface{}
	for _, group := range inputGroups {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}

		appInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok {
			continue
		}

		var filteredInputs []interface{}
		for _, input := range appInputs {
			inputMap, ok := input.(map[string]interface{})
			if !ok {
				continue
			}

			// Get input name
			inputName, _ := inputMap["name"].(string)
			isCustomerFacing := customerInputSet[inputName]

			if filterType == FilterTypeCustomer {
				// Customer sees only inputs in customerInputNames
				if isCustomerFacing {
					filteredInputs = append(filteredInputs, input)
				}
			} else {
				// Vendor sees inputs NOT in customerInputNames
				if !isCustomerFacing {
					filteredInputs = append(filteredInputs, input)
				}
			}
		}

		// Only include groups that have inputs after filtering
		if len(filteredInputs) > 0 {
			filteredGroup := make(map[string]interface{})
			for k, v := range groupMap {
				filteredGroup[k] = v
			}
			filteredGroup["app_inputs"] = filteredInputs
			filteredGroups = append(filteredGroups, filteredGroup)
		}
	}

	// Return the filtered config
	result := make(map[string]interface{})
	for k, v := range configMap {
		result[k] = v
	}
	result["input_groups"] = filteredGroups
	return result
}

// strVal extracts a string value from a map.
func strVal(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

// boolVal extracts a bool value from a map.
func boolVal(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}

// applyInputOrdering sorts input groups and inputs within them based on saved ordering
func applyInputOrdering(inputConfig interface{}, groupOrder []string, inputOrder map[string][]string) interface{} {
	if inputConfig == nil {
		return nil
	}

	// If no ordering is specified, return as-is
	if len(groupOrder) == 0 && len(inputOrder) == 0 {
		return inputConfig
	}

	configMap, ok := inputConfig.(map[string]interface{})
	if !ok {
		return inputConfig
	}

	inputGroups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return inputConfig
	}

	// Sort groups if ordering is specified
	if len(groupOrder) > 0 {
		groupOrderMap := make(map[string]int)
		for i, name := range groupOrder {
			groupOrderMap[name] = i
		}
		sort.SliceStable(inputGroups, func(i, j int) bool {
			groupI, _ := inputGroups[i].(map[string]interface{})
			groupJ, _ := inputGroups[j].(map[string]interface{})
			nameI, _ := groupI["name"].(string)
			nameJ, _ := groupJ["name"].(string)
			orderI, okI := groupOrderMap[nameI]
			orderJ, okJ := groupOrderMap[nameJ]
			if !okI && !okJ {
				return false
			}
			if !okI {
				return false
			}
			if !okJ {
				return true
			}
			return orderI < orderJ
		})
	}

	// Sort inputs within each group if ordering is specified
	for _, group := range inputGroups {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}
		groupName, _ := groupMap["name"].(string)
		order, hasOrder := inputOrder[groupName]
		if !hasOrder || len(order) == 0 {
			continue
		}

		appInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok {
			continue
		}

		inputOrderMap := make(map[string]int)
		for i, name := range order {
			inputOrderMap[name] = i
		}
		sort.SliceStable(appInputs, func(a, b int) bool {
			inputA, _ := appInputs[a].(map[string]interface{})
			inputB, _ := appInputs[b].(map[string]interface{})
			nameA, _ := inputA["name"].(string)
			nameB, _ := inputB["name"].(string)
			orderA, okA := inputOrderMap[nameA]
			orderB, okB := inputOrderMap[nameB]
			if !okA && !okB {
				return false
			}
			if !okA {
				return false
			}
			if !okB {
				return true
			}
			return orderA < orderB
		})
		groupMap["app_inputs"] = appInputs
	}

	// Return with sorted groups
	result := make(map[string]interface{})
	for k, v := range configMap {
		result[k] = v
	}
	result["input_groups"] = inputGroups
	return result
}

// buildCustomerLayoutProps builds the layout props for customer pages
func (h *Handler) buildCustomerLayoutProps(title string, user *models.User, theme *models.AppTheme, org *models.NuonOrg) customerui.LayoutProps {
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)
	secondaryColor, secondaryColorDark := GetPrimaryColors(theme.SecondaryColor)

	orgName := ""
	portalDomain := ""
	if org != nil {
		orgName = org.Name
		if org.Subdomain != "" {
			portalDomain = org.Subdomain + "." + h.subdomainBaseDomain
		}
	}

	adminURL := h.customerBaseURL + "/admin/orgs"
	if org != nil {
		adminURL = h.customerBaseURL + "/admin/orgs/" + org.ID
	}

	headerTitle := ""
	if !theme.HeaderTitleHidden {
		headerTitle = theme.GetHeaderTitle(orgName)
	}

	return customerui.LayoutProps{
		Title:                  title,
		User:                   user,
		BasePath:               h.basePath,
		PrimaryColor:           primaryColor,
		PrimaryColorDark:       primaryColorDark,
		SecondaryColor:         secondaryColor,
		SecondaryColorDark:     secondaryColorDark,
		PrimaryColorDarkMode:   theme.PrimaryColorDark,
		SecondaryColorDarkMode: theme.SecondaryColorDark,
		WhiteColor:             theme.WhiteColor,
		BlackColor:             theme.BlackColor,
		WhiteColorDark:         theme.WhiteColorDark,
		BlackColorLight:        theme.BlackColorLight,
		HeadingFont:            theme.HeadingFont,
		BodyFont:               theme.BodyFont,
		HeadingFontBase64:      theme.HeadingFontBase64,
		BodyFontBase64:         theme.BodyFontBase64,
		LogoBase64:             theme.LogoLightBase64,
		LogoDarkBase64:         theme.LogoDarkBase64,
		FaviconBase64:          theme.FaviconBase64,
		RadiusClass:            theme.GetRadiusClass(),
		ThemeMode:              theme.GetThemeMode(),
		HeaderTitle:            headerTitle,
		CSSPath:                assets.CustomerCSSPath(),
		CustomCSSPath:          h.getCustomCSSPath(theme.OrgID),
		OrgName:                orgName,
		PortalDomain:           portalDomain,
		AdminURL:               adminURL,
	}
}

// GetInstallLinkAppConfig returns the app configuration for a customer install link
// This allows the customer-facing page to fetch input config without authentication
// Accepts optional ?filter=vendor|customer query parameter to filter inputs:
// - filter=vendor: returns inputs where user_configurable != true
// - filter=customer: returns inputs where user_configurable == true
// - no filter: returns all inputs (backward compatibility)
func (h *Handler) GetInstallLinkAppConfig(c *gin.Context) {
	sha := c.Param("sha")
	if sha == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing SHA parameter"})
		return
	}

	// Get optional filter parameter
	filterParam := c.Query("filter")

	// Find install link with org
	var link models.InstallLink
	if err := h.db.Preload("NuonOrg").Where("sha = ?", sha).First(&link).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install link not found"})
		return
	}

	if link.Used {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Install link already used"})
		return
	}

	// Create Nuon client using org credentials
	nuonClient, err := nuon.NewClientWithURL(link.NuonOrg.APIToken, link.NuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize client"})
		return
	}

	// Fetch app details to get platform
	app, err := nuonClient.GetApp(c.Request.Context(), link.AppID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch app details"})
		return
	}

	// Extract platform from app runner config
	var platform string
	if app.RunnerConfig != nil {
		platform = string(app.RunnerConfig.AppRunnerType)
	}

	// Fetch app input config
	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), link.AppID)
	if err != nil {
		// Input config may not exist, that's okay
		inputConfig = nil
	}

	// Fetch local customer input config for this app
	var localConfig models.AppInputConfig
	h.db.Where("org_id = ? AND app_id = ?", link.OrgID, link.AppID).First(&localConfig)
	customerInputNames := localConfig.GetCustomerInputNames()
	groupOrder := localConfig.GetGroupOrder()
	inputOrder := localConfig.GetInputOrder()

	// Apply filtering if requested
	// Convert typed struct to map for filtering
	if filterParam == "vendor" || filterParam == "customer" {
		jsonBytes, err := json.Marshal(inputConfig)
		if err == nil {
			var configMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
				// Use local config for filtering
				if filterParam == "vendor" {
					inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeVendor)
				} else {
					inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeCustomer)
				}
				// Apply ordering
				inputConfig = applyInputOrdering(inputConfig, groupOrder, inputOrder)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"platform":         platform,
		"input_config":     inputConfig,
		"app_name":         link.AppName,
		"collapsed_groups": localConfig.GetCollapsedGroups(),
	})
}

// CustomerRootRedirect redirects to /installs for logged-in users, /apps for logged-out users.
func (h *Handler) CustomerRootRedirect(c *gin.Context) {
	if user := h.tryGetLoggedInUser(c); user != nil {
		c.Redirect(http.StatusFound, "/installs")
	} else {
		c.Redirect(http.StatusFound, "/apps")
	}
}

// tryGetLoggedInUser attempts to extract a logged-in user from JWT cookie
// Returns nil if no valid JWT is present or if parsing fails
func (h *Handler) tryGetLoggedInUser(c *gin.Context) *models.User {
	// Try to parse token from the request (cookie, header, or query)
	token, err := h.auth.ParseToken(c)
	if err != nil || token == nil || !token.Valid {
		return nil
	}

	// Extract claims from the token
	claims := jwt.ExtractClaimsFromToken(token)
	if claims == nil {
		return nil
	}

	// Validate required claims exist
	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return nil
	}

	email, ok := claims["email"].(string)
	if !ok || email == "" {
		return nil
	}

	// Look up the user in the database to ensure they still exist
	var user models.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil
	}

	return &user
}

// InstallLinkPage renders the install link acceptance page
func (h *Handler) InstallLinkPage(c *gin.Context) {
	sha := c.Query("sha")
	if sha == "" {
		theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme, h.getOrgForLayout(c)),
			Error:       "Missing or invalid install link",
		}
		h.RenderTempl(c, http.StatusBadRequest, customerpages.ErrorPage(props))
		return
	}

	var link models.InstallLink
	if err := h.db.Preload("NuonOrg").Where("sha = ?", sha).First(&link).Error; err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme, h.getOrgForLayout(c)),
			Error:       "Install link not found or invalid",
		}
		h.RenderTempl(c, http.StatusNotFound, customerpages.ErrorPage(props))
		return
	}

	if link.Used {
		theme, _ := models.GetOrCreateAppTheme(h.db, link.OrgID)
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme, h.getOrgForLayout(c)),
			Error:       "This install link has already been used",
		}
		h.RenderTempl(c, http.StatusBadRequest, customerpages.ErrorPage(props))
		return
	}

	// Get global app theme for customer UI
	// Use install link's org ID (not getOrgIDForTheme which only works for vendor routes)
	orgID := link.OrgID
	theme, err := models.GetOrCreateAppTheme(h.db, orgID)
	if err != nil {
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme, h.getOrgForLayout(c)),
			Error:       "Failed to load theme settings",
		}
		h.RenderTempl(c, http.StatusInternalServerError, customerpages.ErrorPage(props))
		return
	}

	// REQUIRE authentication - redirect to login if not logged in
	loggedInUser := h.tryGetLoggedInUser(c)
	if loggedInUser == nil {
		// Build redirect URL to return to this install link after login
		redirectURL := url.QueryEscape(h.basePath + "/install-link?sha=" + sha)
		c.Redirect(http.StatusFound, h.basePath+"/login?redirect="+redirectURL)
		return
	}

	// Try template override first
	vendorInputs, _ := link.GetVendorInputs()
	vendorInputsInterface := make(map[string]interface{})
	for k, v := range vendorInputs {
		vendorInputsInterface[k] = v
	}

	pageData := overrides.InstallLinkPageData{
		SHA:          sha,
		AppName:      link.AppName,
		VendorInputs: vendorInputsInterface,
		SubmitURL:    h.basePath + "/install-link/",
	}

	ctx := h.buildTemplateContext(orgID, "Install "+link.AppName, loggedInUser, theme, pageData)
	if h.tryRenderOverride(c, orgID, "install_link", ctx) {
		return
	}

	// Fall back to default Templ template
	props := customerpages.InstallLinkPageProps{
		LayoutProps:  h.buildCustomerLayoutProps("Install "+link.AppName, nil, theme, h.getOrgForLayout(c)),
		Link:         &link,
		LoggedInUser: loggedInUser,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallLinkPage(props))
}

// AcceptInstallLink handles the install link acceptance
// Creates the Install via Nuon API with merged vendor + customer inputs
// Requires authentication - user must be logged in
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
func (h *Handler) InstallsPage(c *gin.Context) {
	user := h.tryGetLoggedInUser(c) // Returns nil if not logged in

	// For unauthenticated visitors, render empty page with login CTA
	if user == nil {
		orgID := h.getOrgIDForTheme(c)
		theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
		layoutProps := h.buildCustomerLayoutProps("Your Installs", nil, theme, h.getOrgForLayout(c))
		layoutProps.HasPublishedApps = h.orgHasPublishedApps(orgID)
		layoutProps.ActiveNav = "installs"
		props := customerpages.InstallsPageProps{
			LayoutProps: layoutProps,
		}
		h.RenderTempl(c, http.StatusOK, customerpages.InstallsPage(props))
		return
	}

	// Get optional install_id from path for detail view with panel open
	installIDParam := c.Param("install_id")
	var initialInstallID string

	if installIDParam != "" {
		// Validate install exists and belongs to user
		var install models.Install
		if err := h.db.Where("id = ? AND user_id = ?", installIDParam, user.ID).First(&install).Error; err != nil {
			// Install not found or doesn't belong to user - redirect to list
			c.Redirect(http.StatusFound, h.basePath+"/installs")
			return
		}
		initialInstallID = installIDParam
	}

	// Tab and pagination parameters
	const installsPerPage = 10
	currentTab := c.DefaultQuery("tab", "needs-attention")
	page := getPageFromQuery(c)
	offset := (page - 1) * installsPerPage

	// Get all installs owned by this user
	var allInstalls []models.Install
	query := h.db.Preload("InstallLink").Preload("InstallLink.NuonOrg").Preload("Org").Order("created_at DESC")
	query = query.Where("user_id = ?", user.ID)
	if err := query.Find(&allInstalls).Error; err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", user, theme, h.getOrgForLayout(c)),
			Error:       "Failed to load installs",
		}
		h.RenderTempl(c, http.StatusInternalServerError, customerpages.ErrorPage(props))
		return
	}

	// Convert to InstallWithApprovalStatus and separate by approval status, update status, and health
	var needsAttentionInstalls []customerui.InstallWithApprovalStatus
	var healthyInstalls []customerui.InstallWithApprovalStatus
	var updatingInstalls []customerui.InstallWithApprovalStatus

	for _, install := range allInstalls {
		hasPendingApprovals := h.checkHasPendingApprovals(c, &install)
		isUpdating := h.checkIsUpdating(c, &install)
		hasActiveProvision := h.checkHasActiveProvision(c, &install)

		installWithStatus := customerui.InstallWithApprovalStatus{
			Install:             install,
			HasPendingApprovals: hasPendingApprovals,
			IsUpdating:          isUpdating,
		}

		// Categorization logic (priority order):
		// 1. Needs Attention: pending approvals OR active provision
		// 2. Updating: in-progress workflow without approvals
		// 3. Healthy: everything else
		needsAttention := hasPendingApprovals || hasActiveProvision

		if needsAttention {
			needsAttentionInstalls = append(needsAttentionInstalls, installWithStatus)
		} else if isUpdating {
			updatingInstalls = append(updatingInstalls, installWithStatus)
		} else {
			healthyInstalls = append(healthyInstalls, installWithStatus)
		}
	}

	// Get counts for each tab
	needsAttentionCount := int64(len(needsAttentionInstalls))
	healthyCount := int64(len(healthyInstalls))
	updatingCount := int64(len(updatingInstalls))

	// If navigating directly to a specific install, switch to the tab that contains it
	if initialInstallID != "" && c.Query("tab") == "" {
		for _, inst := range updatingInstalls {
			if inst.Install.ID == initialInstallID {
				currentTab = "updating"
				break
			}
		}
		if currentTab == "needs-attention" {
			for _, inst := range healthyInstalls {
				if inst.Install.ID == initialInstallID {
					currentTab = "healthy"
					break
				}
			}
		}
	}

	// Select the appropriate list based on current tab
	var filteredInstalls []customerui.InstallWithApprovalStatus
	var totalCount int64

	switch currentTab {
	case "healthy":
		filteredInstalls = healthyInstalls
		totalCount = healthyCount
	case "updating":
		filteredInstalls = updatingInstalls
		totalCount = updatingCount
	default: // "needs-attention"
		filteredInstalls = needsAttentionInstalls
		totalCount = needsAttentionCount
	}

	// Apply pagination to filtered installs
	var paginatedInstalls []customerui.InstallWithApprovalStatus
	if len(filteredInstalls) > 0 {
		startIndex := offset
		endIndex := offset + installsPerPage
		if startIndex >= len(filteredInstalls) {
			startIndex = 0
			page = 1
			offset = 0
		}
		if endIndex > len(filteredInstalls) {
			endIndex = len(filteredInstalls)
		}
		paginatedInstalls = filteredInstalls[startIndex:endIndex]
	}

	// Calculate pagination metadata
	totalPages := int(math.Ceil(float64(totalCount) / float64(installsPerPage)))
	if totalPages == 0 {
		totalPages = 1 // Ensure at least 1 page for empty state
	}

	// Ensure current page is valid
	if page > totalPages {
		page = totalPages
	}

	// Calculate showing range
	showingFrom := offset + 1
	showingTo := offset + len(paginatedInstalls)
	if totalCount == 0 {
		showingFrom = 0
	}

	// Generate page numbers for pagination controls
	var pageNumbers []int
	startPage := max(1, page-2)
	endPage := min(totalPages, page+2)
	for i := startPage; i <= endPage; i++ {
		pageNumbers = append(pageNumbers, i)
	}

	pagination := customerui.InstallPaginationData{
		Installs:            paginatedInstalls,
		CurrentPage:         page,
		TotalPages:          totalPages,
		HasPrevious:         page > 1,
		HasNext:             page < totalPages,
		PreviousPage:        page - 1,
		NextPage:            page + 1,
		TotalCount:          totalCount,
		PerPage:             installsPerPage,
		ShowingFrom:         showingFrom,
		ShowingTo:           showingTo,
		PageNumbers:         pageNumbers,
		CurrentTab:          currentTab,
		NeedsAttentionCount: needsAttentionCount,
		HealthyCount:        healthyCount,
		UpdatingCount:       updatingCount,
	}

	// Always use the subdomain's org for theme — it is the authoritative org context
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)

	// Fetch platform and app name information for all installs
	// Build maps of AppID -> Platform and AppID -> AppName by fetching apps from Nuon API
	platformMap := make(map[string]string)
	appNameMap := make(map[string]string)
	for _, inst := range paginatedInstalls {
		appID := inst.Install.GetAppID()
		if _, exists := platformMap[appID]; !exists {
			// Initialize Nuon client with the org's credentials
			org := inst.Install.GetNuonOrg()
			if org == nil || org.APIToken == "" {
				platformMap[appID] = ""
				// Fallback for install-link installs when API is unavailable
				if inst.Install.InstallLinkID != nil && appNameMap[appID] == "" {
					appNameMap[appID] = inst.Install.InstallLink.AppName
				}
				continue
			}
			nuonClient, err := nuon.NewClientWithURL(
				org.APIToken,
				org.NuonOrgID,
				h.nuonAPIURL,
			)
			if err == nil {
				// Fetch app details to get platform and name
				app, err := nuonClient.GetApp(c.Request.Context(), appID)
				if err == nil && app != nil {
					appNameMap[appID] = app.Name
					if app.RunnerConfig != nil {
						platformMap[appID] = string(app.RunnerConfig.AppRunnerType)
					} else {
						platformMap[appID] = ""
					}
				} else {
					platformMap[appID] = "" // Default to empty if fetch fails
					// Fallback for install-link installs when API call fails
					if inst.Install.InstallLinkID != nil {
						appNameMap[appID] = inst.Install.InstallLink.AppName
					}
				}
			} else {
				platformMap[appID] = "" // Default to empty if client init fails
				// Fallback for install-link installs when client init fails
				if inst.Install.InstallLinkID != nil {
					appNameMap[appID] = inst.Install.InstallLink.AppName
				}
			}
		}
	}

	// Set AppName on each paginated install from the fetched map
	for i, inst := range paginatedInstalls {
		paginatedInstalls[i].AppName = appNameMap[inst.Install.GetAppID()]
	}

	// Try template override first
	installsData := make([]overrides.InstallData, 0, len(paginatedInstalls))
	for _, inst := range paginatedInstalls {
		appID := inst.Install.GetAppID()
		platform := platformMap[appID]

		installsData = append(installsData, overrides.InstallData{
			ID:                 inst.Install.ID,
			Name:               inst.Install.Name,
			Status:             string(inst.Install.Status),
			Region:             inst.Install.Region,
			Platform:           platform,
			AppName:            appNameMap[appID],
			CreatedAt:          inst.Install.CreatedAt.Format("Jan 2, 2006"),
			HasPendingApproval: inst.HasPendingApprovals,
		})
	}

	pageData := overrides.InstallsPageData{
		Installs:            installsData,
		CurrentTab:          currentTab,
		TotalCount:          totalCount,
		NeedsAttentionCount: needsAttentionCount,
		UpdatingCount:       updatingCount,
		HealthyCount:        healthyCount,
		CurrentPage:         page,
		TotalPages:          totalPages,
		HasPrevious:         page > 1,
		HasNext:             page < totalPages,
		PreviousPage:        page - 1,
		NextPage:            page + 1,
		ShowingFrom:         showingFrom,
		ShowingTo:           showingTo,
	}

	ctx := h.buildTemplateContext(orgID, "Your Installs", user, theme, pageData)
	if h.tryRenderOverride(c, orgID, "installs", ctx) {
		return
	}

	// Fall back to default Templ template
	installsLayoutProps := h.buildCustomerLayoutProps("Your Installs", user, theme, h.getOrgForLayout(c))
	installsLayoutProps.HasPublishedApps = h.orgHasPublishedApps(orgID)
	installsLayoutProps.ActiveNav = "installs"
	props := customerpages.InstallsPageProps{
		LayoutProps:      installsLayoutProps,
		Pagination:       pagination,
		InitialInstallID: initialInstallID,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallsPage(props))
}

// getString safely gets a string value from a gin.H map
func getString(m gin.H, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getBool safely gets a bool value from a gin.H map
func getBool(m gin.H, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// getInt safely gets an int value from a gin.H map
func getInt(m gin.H, key string) int {
	if v, ok := m[key].(int); ok {
		return v
	}
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

// UpdateInstall handles customer install updates
func (h *Handler) UpdateInstall(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	install := installInterface.(*models.Install)

	var req struct {
		Region string `json:"region,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update allowed fields
	if req.Region != "" {
		install.Region = req.Region
	}

	if err := h.db.Save(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update install"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Install updated successfully",
		"install": install,
	})
}

// DeleteInstall handles customer install deletion (deprovision)
func (h *Handler) DeleteInstall(c *gin.Context) {
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

	// Initialize Nuon client to deprovision the install using global API URL
	nuonClient, err := nuon.NewClientWithURL(nuonOrgDep.APIToken, nuonOrgDep.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Deprovision the install via Nuon API
	if err := nuonClient.DeprovisionInstall(c.Request.Context(), install.NuonInstallID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deprovision install"})
		return
	}

	// Update status to deprovisioning
	install.Status = models.StatusDeprovisioning
	if err := h.db.Save(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update install status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Install deprovisioning initiated successfully",
		"install": install,
	})
}

// ForgetInstall handles customer install forgetting (local database removal only)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to forget install"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Install forgotten successfully",
	})
}

// loadInstallWithOrg reloads install with both InstallLink.NuonOrg and Org preloaded,
// required for published-app installs where InstallLinkID is nil.
func (h *Handler) loadInstallWithOrg(install *models.Install) error {
	return h.db.Preload("InstallLink.NuonOrg").Preload("Org").
		Where("id = ?", install.ID).First(install).Error
}

// GetInstallInputs returns the current inputs and input config for an install
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

// checkHasPendingApprovals checks if an install has workflows waiting for approval
func (h *Handler) checkHasPendingApprovals(c *gin.Context, install *models.Install) bool {
	if err := h.loadInstallWithOrg(install); err != nil {
		return false
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		return false
	}

	// Initialize Nuon client using global API URL
	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		return false // Return false if we can't initialize the client
	}

	// Get recent workflows from Nuon API - limit to just a few for performance
	const checkLimit = 5
	workflows, _, err := nuonClient.GetInstallWorkflows(c.Request.Context(), install.NuonInstallID, 0, checkLimit)
	if err != nil {
		// If we can't fetch workflow data, return false (no indicator)
		// This ensures the page still loads even if the workflow API is unavailable
		return false
	}

	// Check if any recent workflow has pending approvals
	for _, workflow := range workflows {
		if workflow.Status != nil {
			switch string(workflow.Status.Status) {
			case "approval-awaiting":
				return true
			case "in-progress":
				// Check if workflow has approval steps for approve-all capability
				if workflow.Steps != nil {
					for _, step := range workflow.Steps {
						if step.ExecutionType == "approval" {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

// checkIsUpdating checks if an install has in-progress workflows without approval steps
// This is used to categorize installs into the "Updating" tab
func (h *Handler) checkIsUpdating(c *gin.Context, install *models.Install) bool {
	if err := h.loadInstallWithOrg(install); err != nil {
		return false
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		return false
	}

	// Initialize Nuon client using global API URL
	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		return false
	}

	// Get recent workflows from Nuon API
	const checkLimit = 5
	workflows, _, err := nuonClient.GetInstallWorkflows(c.Request.Context(), install.NuonInstallID, 0, checkLimit)
	if err != nil {
		return false
	}

	// Check if any recent workflow is in-progress WITHOUT approval steps
	for _, workflow := range workflows {
		if workflow.Status != nil && string(workflow.Status.Status) == "in-progress" {
			// Check if this workflow has any approval steps
			hasApprovalStep := false
			if workflow.Steps != nil {
				for _, step := range workflow.Steps {
					if step.ExecutionType == "approval" {
						hasApprovalStep = true
						break
					}
				}
			}
			// If in-progress and no approval steps, this install is being updated
			if !hasApprovalStep {
				return true
			}
		}
	}

	return false
}

// checkHasActiveProvision checks if an install has an active provision workflow (pending, in-progress, or approval-awaiting)
func (h *Handler) checkHasActiveProvision(c *gin.Context, install *models.Install) bool {
	// NuonOrg should already be preloaded by InstallsPage query (via InstallLink.NuonOrg or Org)
	org := install.GetNuonOrg()
	if org == nil || org.ID == "" {
		return false
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		return false
	}

	ctx := c.Request.Context()
	provisionTypes := []string{"provision", "provision_sandbox", "reprovision", "reprovision_sandbox"}
	for _, wfType := range provisionTypes {
		workflows, _, err := nuonClient.GetInstallWorkflowsByType(ctx, install.NuonInstallID, 0, 1, wfType)
		if err != nil || len(workflows) == 0 {
			continue
		}
		wf := workflows[0]
		if wf.Status == nil {
			continue
		}
		status := string(wf.Status.Status)
		if status == "pending" || status == "in-progress" || status == "approval-awaiting" {
			return true
		}
	}

	return false
}

// InstallDetailPanel renders the install detail content for the sliding panel (no layout wrapper)
func (h *Handler) InstallDetailPanel(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	// Load install with both InstallLink.NuonOrg and Org (handles published-app installs)
	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()

	// Check if install still exists in Nuon API
	var apiDeletedError bool
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		checkClient, checkErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if checkErr == nil {
			_, apiErr := checkClient.GetInstall(context.Background(), install.NuonInstallID)
			if apiErr != nil {
				// Check if error is specifically a 404 NotFound
				var notFoundErr *operations.GetInstallNotFound
				if errors.As(apiErr, &notFoundErr) {
					apiDeletedError = true
					h.logger.Warn("install deleted from API but exists locally",
						zap.String("install_id", install.ID),
						zap.String("nuon_install_id", install.NuonInstallID),
					)
				}
			}
		}
	}

	// Fetch app config version info and app name
	var appName string
	var appConfigVersion int64
	var appConfigUpdatedAt string
	var installConfigVersion int64
	var installConfigUpdatedAt string
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		appClient, err := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if err == nil {
			app, err := appClient.GetApp(context.Background(), install.GetAppID())
			if err == nil && app != nil {
				appName = app.Name
				if len(app.AppConfigs) > 0 {
					appConfigVersion = app.AppConfigs[0].Version
					appConfigUpdatedAt = app.AppConfigs[0].UpdatedAt

					// Get install's current config version by matching AppConfigID
					nuonInstall, instErr := appClient.GetInstall(context.Background(), install.NuonInstallID)
					if instErr == nil && nuonInstall.AppConfigID != "" {
						for _, cfg := range app.AppConfigs {
							if cfg.ID == nuonInstall.AppConfigID {
								installConfigVersion = cfg.Version
								installConfigUpdatedAt = cfg.UpdatedAt
								break
							}
						}
					}
				}
			}
		}
	}

	// Fetch active provision workflow (provision or provision_sandbox)
	var activeProvisionWorkflow *partials.WorkflowDataPanel
	var cloudFormationLink string
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		provClient, provErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if provErr == nil {
			ctx := c.Request.Context()
			provisionTypes := []string{"provision", "provision_sandbox"}
			for _, wfType := range provisionTypes {
				workflows, _, err := provClient.GetInstallWorkflowsByType(ctx, install.NuonInstallID, 0, 1, wfType)
				if err != nil || len(workflows) == 0 {
					continue
				}
				wf := workflows[0]
				if wf.Status == nil {
					continue
				}
				status := string(wf.Status.Status)
				if status != "in-progress" && status != "approval-awaiting" && status != "pending" {
					continue
				}
				processed := processWorkflowForCustomer(wf)
				panel := ginHToWorkflowDataPanel(processed)
				activeProvisionWorkflow = &panel

				// Check if the "await install stack" step is active (pending, in-progress, or approval-awaiting)
				if wf.Steps != nil {
					for _, step := range wf.Steps {
						if step.Name != "await install stack" {
							continue
						}
						stepStatus := ""
						if step.Status != nil {
							stepStatus = string(step.Status.Status)
						}
						if stepStatus == "" || stepStatus == "pending" || stepStatus == "in-progress" || stepStatus == "approval-awaiting" {
							// Fetch CloudFormation link
							stack, stackErr := provClient.GetInstallStack(ctx, install.NuonInstallID)
							if stackErr == nil && stack != nil && stack.Versions != nil && len(stack.Versions) > 0 {
								if stack.Versions[0].QuickLinkURL != "" {
									cloudFormationLink = stack.Versions[0].QuickLinkURL
								}
							}
							// Append customer inputs to CF URL
							if cloudFormationLink != "" {
								inputConfig, inputErr := provClient.GetAppInputConfig(ctx, install.GetAppID())
								if inputErr == nil && inputConfig != nil {
									var configMap map[string]interface{}
									jsonBytes, jerr := json.Marshal(inputConfig)
									if jerr == nil {
										if json.Unmarshal(jsonBytes, &configMap) == nil {
											inputMappings := extractCustomerInputMappings(configMap)
											if len(inputMappings) > 0 {
												currentInputs, ciErr := provClient.GetInstallCurrentInputs(ctx, install.NuonInstallID)
												if ciErr == nil && currentInputs != nil && currentInputs.Values != nil {
													cloudFormationLink = appendInputsToCloudFormationURL(
														cloudFormationLink,
														currentInputs.Values,
														inputMappings,
													)
												}
											}
										}
									}
								}
							}
							break
						}
					}
				}
				break
			}
		}
	}

	// Fetch install readme only if no active provision workflow
	var installReadme *partials.ReadmeDataPanel
	if activeProvisionWorkflow == nil {
		if nuonOrg != nil && nuonOrg.APIToken != "" {
			readmeClient, readmeErr := nuon.NewClientWithURL(
				nuonOrg.APIToken,
				nuonOrg.NuonOrgID,
				h.nuonAPIURL,
			)
			if readmeErr == nil {
				readme, err := readmeClient.GetInstallReadme(c.Request.Context(), install.NuonInstallID)
				if err != nil {
					h.logger.Warn("failed to fetch install readme",
						zap.String("install_id", install.NuonInstallID),
						zap.Error(err),
					)
					// Continue without readme - non-blocking
				} else if readme != nil {
					// Convert nuon.ServiceReadme to partials.ReadmeDataPanel
					warnings := []string{}
					if readme.Warnings != nil {
						warnings = readme.Warnings
					}

					// Convert markdown to HTML
					renderedHTML := readme.Readme
					if readme.Readme != "" {
						html, err := markdown.ToHTML(readme.Readme)
						if err != nil {
							h.logger.Warn("failed to render markdown to HTML",
								zap.String("install_id", install.NuonInstallID),
								zap.Error(err),
							)
							// Fall back to raw markdown if conversion fails
							renderedHTML = readme.Readme
						} else {
							renderedHTML = html
						}
					}
					installReadme = &partials.ReadmeDataPanel{
						Readme:   renderedHTML,
						Original: readme.Original,
						Warnings: warnings,
					}
				}
			}
		}
	}

	// Get theme colors
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)
	secondaryColor, _ := GetPrimaryColors(theme.SecondaryColor)

	props := partials.InstallDetailPanelProps{
		Install:                 install,
		AppName:                 appName,
		APIDeletedError:         apiDeletedError,
		BasePath:                h.basePath,
		PrimaryColor:            primaryColor,
		SecondaryColor:          secondaryColor,
		AppConfigVersion:        appConfigVersion,
		AppConfigUpdatedAt:      appConfigUpdatedAt,
		InstallConfigVersion:    installConfigVersion,
		InstallConfigUpdatedAt:  installConfigUpdatedAt,
		ActiveProvisionWorkflow: activeProvisionWorkflow,
		InstallReadme:           installReadme,
		CloudFormationLink:      cloudFormationLink,
	}
	h.RenderTempl(c, http.StatusOK, partials.InstallDetailPanel(props))
}

// InstallWorkflowStatus returns the active provision workflow banner for HTMX polling
// This endpoint is called by HTMX polling every 5 seconds
// It must return HTML (ActiveProvisionBanner template) for HTMX to swap
// Authentication is handled by JWT middleware which returns HX-Trigger: auth-error on failure
func (h *Handler) InstallWorkflowStatus(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	// Load install with both InstallLink.NuonOrg and Org (handles published-app installs)
	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()

	// Check if install still exists in API
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		checkClient, checkErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if checkErr == nil {
			_, apiErr := checkClient.GetInstall(context.Background(), install.NuonInstallID)
			var notFoundErr *operations.GetInstallNotFound
			if errors.As(apiErr, &notFoundErr) {
				// Install deleted from API - return empty state
				theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
				primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)
				h.RenderTempl(c, http.StatusOK, partials.ActiveProvisionBanner(nil, "", install.ID, h.basePath, primaryColor))
				return
			}
		}
	}

	// Fetch active provision workflow (provision or provision_sandbox)
	var activeProvisionWorkflow *partials.WorkflowDataPanel
	var cloudFormationLink string
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		provClient, provErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if provErr == nil {
			ctx := c.Request.Context()
			provisionTypes := []string{"provision", "provision_sandbox", "reprovision", "reprovision_sandbox"}
			for _, wfType := range provisionTypes {
				workflows, _, err := provClient.GetInstallWorkflowsByType(ctx, install.NuonInstallID, 0, 1, wfType)
				if err != nil || len(workflows) == 0 {
					continue
				}
				wf := workflows[0]
				if wf.Status == nil {
					continue
				}
				status := string(wf.Status.Status)
				if status != "in-progress" && status != "approval-awaiting" && status != "pending" {
					continue
				}
				processed := processWorkflowForCustomer(wf)
				panel := ginHToWorkflowDataPanel(processed)
				activeProvisionWorkflow = &panel

				// Check if the "await install stack" step is active (pending, in-progress, or approval-awaiting)
				if wf.Steps != nil {
					for _, step := range wf.Steps {
						if step.Name != "await install stack" {
							continue
						}
						stepStatus := ""
						if step.Status != nil {
							stepStatus = string(step.Status.Status)
						}
						if stepStatus == "" || stepStatus == "pending" || stepStatus == "in-progress" || stepStatus == "approval-awaiting" {
							// Fetch CloudFormation link
							stack, stackErr := provClient.GetInstallStack(ctx, install.NuonInstallID)
							if stackErr == nil && stack != nil && stack.Versions != nil && len(stack.Versions) > 0 {
								if stack.Versions[0].QuickLinkURL != "" {
									cloudFormationLink = stack.Versions[0].QuickLinkURL
								}
							}
							// Append customer inputs to CF URL
							if cloudFormationLink != "" {
								inputConfig, inputErr := provClient.GetAppInputConfig(ctx, install.GetAppID())
								if inputErr == nil && inputConfig != nil {
									var configMap map[string]interface{}
									jsonBytes, jerr := json.Marshal(inputConfig)
									if jerr == nil {
										if json.Unmarshal(jsonBytes, &configMap) == nil {
											inputMappings := extractCustomerInputMappings(configMap)
											if len(inputMappings) > 0 {
												currentInputs, ciErr := provClient.GetInstallCurrentInputs(ctx, install.NuonInstallID)
												if ciErr == nil && currentInputs != nil && currentInputs.Values != nil {
													cloudFormationLink = appendInputsToCloudFormationURL(
														cloudFormationLink,
														currentInputs.Values,
														inputMappings,
													)
												}
											}
										}
									}
								}
							}
							break
						}
					}
				}
				break
			}
		}
	}

	// Get theme color
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)

	// Always render banner (pass nil if no workflow, component handles it)
	h.RenderTempl(c, http.StatusOK, partials.ActiveProvisionBanner(
		activeProvisionWorkflow,
		cloudFormationLink,
		install.ID,
		h.basePath,
		primaryColor,
	))
}

// InstallReadmeStatus handles HTMX polling for readme display
func (h *Handler) InstallReadmeStatus(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	// Load install with org relationships
	if err := h.db.Preload("InstallLink.NuonOrg").Preload("Org").Where("id = ?", install.ID).First(install).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		c.String(http.StatusInternalServerError, "Organization information not found")
		return
	}

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to initialize Nuon client")
		return
	}

	// Check if there's an active provision/reprovision workflow
	ctx := c.Request.Context()
	hasActiveWorkflow := false
	provisionTypes := []string{"provision", "provision_sandbox", "reprovision", "reprovision_sandbox"}

	for _, wfType := range provisionTypes {
		workflows, _, err := nuonClient.GetInstallWorkflowsByType(ctx, install.NuonInstallID, 0, 1, wfType)
		if err != nil || len(workflows) == 0 {
			continue
		}
		wf := workflows[0]
		if wf.Status != nil {
			status := string(wf.Status.Status)
			if status == "in-progress" || status == "approval-awaiting" || status == "pending" {
				hasActiveWorkflow = true
				break
			}
		}
	}

	// If there's an active workflow, return empty (readme should be hidden)
	if hasActiveWorkflow {
		c.String(http.StatusOK, "")
		return
	}

	// Fetch install readme
	readme, err := nuonClient.GetInstallReadme(ctx, install.NuonInstallID)
	if err != nil {
		h.logger.Warn("failed to fetch install readme for polling",
			zap.String("install_id", install.NuonInstallID),
			zap.Error(err),
		)
		c.String(http.StatusOK, "")
		return
	}

	// Convert to partials.ReadmeDataPanel if readme exists
	var readmePanel *partials.ReadmeDataPanel
	if readme != nil {
		warnings := []string{}
		if readme.Warnings != nil {
			warnings = readme.Warnings
		}

		// Convert markdown to HTML
		renderedHTML := readme.Readme
		if readme.Readme != "" {
			html, err := markdown.ToHTML(readme.Readme)
			if err != nil {
				h.logger.Warn("failed to render markdown to HTML",
					zap.String("install_id", install.NuonInstallID),
					zap.Error(err),
				)
				// Fall back to raw markdown if conversion fails
				renderedHTML = readme.Readme
			} else {
				renderedHTML = html
			}
		}

		readmePanel = &partials.ReadmeDataPanel{
			Readme:   renderedHTML,
			Original: readme.Original,
			Warnings: warnings,
		}
	}

	// Render the readme card partial
	h.RenderTempl(c, http.StatusOK, partials.AppReadmeCard(readmePanel, install.ID, h.basePath))
}

// AuditLogsPanel renders the audit logs panel content (no layout wrapper)
func (h *Handler) AuditLogsPanel(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	// Load install with both InstallLink.NuonOrg and Org (handles published-app installs)
	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()

	// Parse time range parameters
	now := time.Now()
	rangeType := c.DefaultQuery("range", "past-hour")
	var startTime, endTime time.Time

	startStr := c.Query("start")
	endStr := c.Query("end")

	if startStr != "" && endStr != "" {
		parsedStart, err1 := time.Parse(time.RFC3339, startStr)
		parsedEnd, err2 := time.Parse(time.RFC3339, endStr)
		if err1 == nil && err2 == nil {
			startTime = parsedStart
			endTime = parsedEnd
			rangeType = "custom"
		} else {
			startTime = now.Add(-time.Hour)
			endTime = now
			rangeType = "past-hour"
		}
	} else {
		switch rangeType {
		case "past-day":
			startTime = now.Add(-24 * time.Hour)
			endTime = now
		case "past-week":
			startTime = now.Add(-7 * 24 * time.Hour)
			endTime = now
		default:
			startTime = now.Add(-time.Hour)
			endTime = now
			rangeType = "past-hour"
		}
	}

	// Fetch audit logs via Nuon client
	var auditEntries []partials.AuditLogEntryPanel
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		client, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
		if err == nil {
			entries, err := client.GetInstallAuditLogs(c.Request.Context(), install.NuonInstallID, startTime, endTime)
			if err == nil {
				for _, entry := range entries {
					auditEntries = append(auditEntries, partials.AuditLogEntryPanel{
						LogLine:   entry.LogLine,
						TimeStamp: entry.TimeStamp,
						Type:      entry.Type,
					})
				}
			}
		}
	}

	props := partials.AuditLogsPanelProps{
		Install:   install,
		Entries:   auditEntries,
		StartTime: startTime,
		EndTime:   endTime,
		RangeType: rangeType,
		BasePath:  h.basePath,
	}
	h.RenderTempl(c, http.StatusOK, partials.AuditLogsPanel(props))
}

// getOrgForCustomerPage looks up the NuonOrg using the subdomain in the gin context.
// Returns an error if the subdomain is not set or the org is not found.
func (h *Handler) getOrgForCustomerPage(c *gin.Context) (*models.NuonOrg, error) {
	orgID := h.getOrgIDForTheme(c)
	if orgID == "" {
		return nil, fmt.Errorf("org not found for this subdomain")
	}
	var org models.NuonOrg
	if err := h.db.Where("id = ?", orgID).First(&org).Error; err != nil {
		return nil, fmt.Errorf("org not found: %w", err)
	}
	return &org, nil
}

// GetPublishedAppConfig returns app config for a published app (unauthenticated endpoint)
func (h *Handler) GetPublishedAppConfig(c *gin.Context) {
	appID := c.Param("app_id")
	if appID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing app_id parameter"})
		return
	}

	filterParam := c.Query("filter")

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	// Verify app is published for this org
	var publishedApp models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&publishedApp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found or not published"})
		return
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize client"})
		return
	}

	app, err := nuonClient.GetApp(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch app details"})
		return
	}

	var platform string
	if app.RunnerConfig != nil {
		platform = string(app.RunnerConfig.AppRunnerType)
	}

	appName := ""
	if app.Name != "" {
		appName = app.Name
	}

	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), appID)
	if err != nil {
		inputConfig = nil
	}

	var localConfig models.AppInputConfig
	h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&localConfig)
	customerInputNames := localConfig.GetCustomerInputNames()
	groupOrder := localConfig.GetGroupOrder()
	inputOrder := localConfig.GetInputOrder()

	if filterParam == "vendor" || filterParam == "customer" {
		jsonBytes, err := json.Marshal(inputConfig)
		if err == nil {
			var configMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
				if filterParam == "vendor" {
					inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeVendor)
				} else {
					inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeCustomer)
				}
				inputConfig = applyInputOrdering(inputConfig, groupOrder, inputOrder)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"platform":         platform,
		"input_config":     inputConfig,
		"app_name":         appName,
		"collapsed_groups": localConfig.GetCollapsedGroups(),
	})
}

// CustomerAppsPage renders the customer-facing app catalog
func (h *Handler) CustomerAppsPage(c *gin.Context) {
	user := h.tryGetLoggedInUser(c)

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		c.Redirect(http.StatusFound, h.basePath+"/installs")
		return
	}

	var publishedApps []models.PublishedApp
	if err := h.db.Where("org_id = ?", org.ID).Order("sort_order ASC, created_at ASC").Find(&publishedApps).Error; err != nil {
		zap.L().Warn("failed to fetch published apps", zap.Error(err))
	}

	theme, _ := models.GetOrCreateAppTheme(h.db, org.ID)

	appDisplays := make([]customerpages.PublishedAppDisplay, 0, len(publishedApps))
	if len(publishedApps) > 0 {
		nuonClient, nuonClientErr := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
		for _, pa := range publishedApps {
			display := h.buildAppDisplay(c, pa.AppID, org.ID, nuonClient, nuonClientErr)
			display.LogoLightBase64 = pa.LogoLightBase64
			display.LogoDarkBase64 = pa.LogoDarkBase64
			appDisplays = append(appDisplays, display)
		}
	}

	layoutProps := h.buildCustomerLayoutProps("App Catalog", user, theme, h.getOrgForLayout(c))
	layoutProps.HasPublishedApps = len(publishedApps) > 0
	layoutProps.ActiveNav = "apps"

	props := customerpages.CustomerAppsPageProps{
		LayoutProps: layoutProps,
		Apps:        appDisplays,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.CustomerAppsPage(props))
}

// buildAppDisplay builds a PublishedAppDisplay for a single app, fetching details from the Nuon API.
func (h *Handler) buildAppDisplay(c *gin.Context, appID string, orgID string, nuonClient *nuon.Client, nuonClientErr error) customerpages.PublishedAppDisplay {
	display := customerpages.PublishedAppDisplay{
		AppID:    appID,
		AppName:  appID, // fallback to AppID if name can't be fetched
		Platform: "unknown",
	}
	if nuonClientErr != nil {
		return display
	}

	app, err := nuonClient.GetApp(c.Request.Context(), appID)
	if err == nil && app != nil {
		if app.Name != "" {
			display.AppName = app.Name
		}
		if app.RunnerConfig != nil {
			display.Platform = string(app.RunnerConfig.AppRunnerType)
		}
		display.Description = app.Description
	}

	// Fetch components
	if comps, err := nuonClient.GetAppComponents(c.Request.Context(), appID); err != nil {
		zap.L().Warn("failed to fetch app components", zap.String("app_id", appID), zap.Error(err))
	} else {
		for _, comp := range comps {
			display.Components = append(display.Components, customerpages.ComponentDisplay{
				Name: comp.Name,
				Type: string(comp.Type),
			})
		}
	}

	// Fetch sandbox config
	if sbCfg, err := nuonClient.GetAppSandboxLatestConfig(c.Request.Context(), appID); err != nil {
		zap.L().Warn("failed to fetch app sandbox config", zap.String("app_id", appID), zap.Error(err))
	} else if sbCfg != nil {
		display.SandboxPlatform = sbCfg.CloudPlatform
		if display.SandboxPlatform == "" {
			display.SandboxPlatform = display.Platform
		}
		display.SandboxTFVersion = sbCfg.TerraformVersion
		display.SandboxDrift = sbCfg.DriftSchedule
		if sbCfg.PublicGitVcsConfig != nil {
			display.SandboxRepoIsPublic = true
			display.SandboxRepoURL = sbCfg.PublicGitVcsConfig.Repo
			display.SandboxRepoBranch = sbCfg.PublicGitVcsConfig.Branch
			display.SandboxRepoDir = sbCfg.PublicGitVcsConfig.Directory
		}
	}

	// Fetch permissions config
	if permCfg, err := nuonClient.GetLatestAppPermissionsConfig(c.Request.Context(), appID); err != nil {
		zap.L().Warn("failed to fetch app permissions config", zap.String("app_id", appID), zap.Error(err))
	} else if permCfg != nil {
		addRole := func(role *nuonmodels.AppAppAWSIAMRoleConfig, label string) {
			if role == nil || role.Name == "" {
				return
			}
			name := role.DisplayName
			if name == "" {
				name = role.Name
			}
			display.Permissions = append(display.Permissions, customerpages.IAMRoleDisplay{
				Label:       label,
				Name:        name,
				Description: role.Description,
			})
		}
		if permCfg.ProvisionAwsIamRole.Name != "" {
			prov := permCfg.ProvisionAwsIamRole.AppAppAWSIAMRoleConfig
			addRole(&prov, "Provision")
		}
		addRole(permCfg.DeprovisionAwsIamRole, "Deprovision")
		addRole(permCfg.MaintenanceAwsIamRole, "Maintenance")
		addRole(permCfg.BreakGlassAwsIamRole, "Break Glass")
		for _, r := range permCfg.AwsIamRoles {
			label := r.DisplayName
			if label == "" {
				label = r.Name
			}
			addRole(r, label)
		}
	}

	// Fetch policies config (raw HTTP — SDK model omits the policies array)
	if policies, err := nuonClient.GetLatestAppPoliciesConfigFull(c.Request.Context(), appID); err != nil {
		zap.L().Warn("failed to fetch app policies config", zap.String("app_id", appID), zap.Error(err))
	} else {
		for _, p := range policies {
			display.Policies = append(display.Policies, customerpages.PolicyDisplay{
				Name:        p.Name,
				Type:        p.Type,
				Engine:      p.Engine,
				Description: p.Description,
			})
		}
	}

	// Fetch secrets config for Secrets tab
	if secretsCfg, err := nuonClient.GetAppSecretsConfig(c.Request.Context(), appID); err != nil {
		zap.L().Warn("failed to fetch app secrets config", zap.String("app_id", appID), zap.Error(err))
	} else if secretsCfg != nil && len(secretsCfg.Secrets) > 0 {
		for _, s := range secretsCfg.Secrets {
			display.Secrets = append(display.Secrets, customerpages.SecretDisplay{
				Name:         s.Name,
				DisplayName:  s.DisplayName,
				Description:  s.Description,
				AutoGenerate: s.AutoGenerate,
			})
		}
	}

	// Fetch input config for Inputs tab
	if inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), appID); err != nil {
		zap.L().Warn("failed to fetch app input config", zap.String("app_id", appID), zap.Error(err))
	} else if inputConfig != nil {
		// Determine which inputs are customer-facing
		var localConfig models.AppInputConfig
		h.db.Where("org_id = ? AND app_id = ?", orgID, appID).First(&localConfig)
		customerInputNames := localConfig.GetCustomerInputNames()

		customerInputSet := make(map[string]bool)
		for _, name := range customerInputNames {
			customerInputSet[name] = true
		}

		// Marshal/unmarshal to work with the untyped config
		jsonBytes, _ := json.Marshal(inputConfig)
		var configMap map[string]interface{}
		if json.Unmarshal(jsonBytes, &configMap) == nil {
			if inputGroups, ok := configMap["input_groups"].([]interface{}); ok {
				for _, group := range inputGroups {
					groupMap, ok := group.(map[string]interface{})
					if !ok {
						continue
					}
					appInputs, ok := groupMap["app_inputs"].([]interface{})
					if !ok {
						continue
					}
					gd := customerpages.InputGroupDisplay{
						Name:        strVal(groupMap, "name"),
						DisplayName: strVal(groupMap, "display_name"),
					}
					for _, input := range appInputs {
						inputMap, ok := input.(map[string]interface{})
						if !ok {
							continue
						}
						inputName := strVal(inputMap, "name")
						configuredBy := "vendor"
						if customerInputSet[inputName] {
							configuredBy = "customer"
						}
						gd.Inputs = append(gd.Inputs, customerpages.InputDisplay{
							Name:         inputName,
							DisplayName:  strVal(inputMap, "display_name"),
							Description:  strVal(inputMap, "description"),
							Type:         strVal(inputMap, "input_type"),
							Required:     boolVal(inputMap, "required"),
							Sensitive:    boolVal(inputMap, "sensitive"),
							Default:      strVal(inputMap, "default"),
							ConfiguredBy: configuredBy,
						})
					}
					if len(gd.Inputs) > 0 {
						display.InputGroups = append(display.InputGroups, gd)
					}
				}
			}
		}
	}

	return display
}

// CustomerAppDetailPage renders the detail page for a single published app.
func (h *Handler) CustomerAppDetailPage(c *gin.Context) {
	appID := c.Param("app_id")
	user := h.tryGetLoggedInUser(c)

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		c.Redirect(http.StatusFound, h.basePath+"/installs")
		return
	}

	var publishedApp models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&publishedApp).Error; err != nil {
		c.Redirect(http.StatusFound, h.basePath+"/apps")
		return
	}

	theme, _ := models.GetOrCreateAppTheme(h.db, org.ID)

	nuonClient, nuonClientErr := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	display := h.buildAppDisplay(c, appID, org.ID, nuonClient, nuonClientErr)
	display.LogoLightBase64 = publishedApp.LogoLightBase64
	display.LogoDarkBase64 = publishedApp.LogoDarkBase64

	layoutProps := h.buildCustomerLayoutProps(display.AppName, user, theme, h.getOrgForLayout(c))
	layoutProps.HasPublishedApps = true
	layoutProps.ActiveNav = "apps"

	props := customerpages.CustomerAppDetailPageProps{
		LayoutProps: layoutProps,
		App:         display,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.CustomerAppDetailPage(props))
}

// CustomerAppInstallPage renders the install form for a published app
func (h *Handler) CustomerAppInstallPage(c *gin.Context) {
	appID := c.Param("app_id")

	// Require authentication
	loggedInUser := h.tryGetLoggedInUser(c)
	if loggedInUser == nil {
		redirectURL := url.QueryEscape(h.basePath + "/apps/" + appID + "/install")
		c.Redirect(http.StatusFound, h.basePath+"/login?redirect="+redirectURL)
		return
	}

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", loggedInUser, theme, h.getOrgForLayout(c)),
			Error:       "Organization not found",
		}
		h.RenderTempl(c, http.StatusNotFound, customerpages.ErrorPage(props))
		return
	}

	// Verify app is published for this org
	var publishedApp models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&publishedApp).Error; err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db, org.ID)
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", loggedInUser, theme, h.getOrgForLayout(c)),
			Error:       "App not found or not published",
		}
		h.RenderTempl(c, http.StatusNotFound, customerpages.ErrorPage(props))
		return
	}

	theme, _ := models.GetOrCreateAppTheme(h.db, org.ID)

	appName := appID
	orgName := org.NuonOrgID // fallback
	nuonClient, clientErr := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if clientErr == nil {
		app, appErr := nuonClient.GetApp(c.Request.Context(), appID)
		if appErr == nil && app != nil && app.Name != "" {
			appName = app.Name
		}
		apiOrg, orgErr := nuonClient.GetOrg(c.Request.Context())
		if orgErr == nil && apiOrg != nil && apiOrg.Name != "" {
			orgName = apiOrg.Name
		}
	}

	layoutProps := h.buildCustomerLayoutProps("Install "+appName, loggedInUser, theme, h.getOrgForLayout(c))
	layoutProps.HasPublishedApps = true
	layoutProps.ActiveNav = "apps"

	props := customerpages.AppInstallPageProps{
		LayoutProps:  layoutProps,
		AppID:        appID,
		AppName:      appName,
		OrgName:      orgName,
		LoggedInUser: loggedInUser,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.AppInstallPage(props))
}

// CreateInstallFromApp creates an install from a published app
func (h *Handler) CreateInstallFromApp(c *gin.Context) {
	customer := h.tryGetLoggedInUser(c)
	if customer == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Please log in first"})
		return
	}

	appID := c.Param("app_id")

	var req struct {
		Name     string            `json:"name" binding:"required"`
		Region   string            `json:"region"`
		Location string            `json:"location"`
		Inputs   map[string]string `json:"inputs"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	// Verify app is published for this org
	var publishedApp models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&publishedApp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found or not published"})
		return
	}

	region := req.Region
	location := req.Location
	if region == "" && location == "" {
		region = "us-east-1"
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to initialize Nuon client: %v", err)})
		return
	}

	// Get app name for the API call
	appName := appID
	app, appErr := nuonClient.GetApp(c.Request.Context(), appID)
	if appErr == nil && app != nil && app.Name != "" {
		appName = app.Name
	}

	nuonInstall, err := nuonClient.CreateInstallWithCustomName(c.Request.Context(), appID, appName, req.Name, region, location, req.Inputs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create install via Nuon API: %v", err)})
		return
	}

	// Create local Install record with no install link
	install := &models.Install{
		OrgID:             org.ID,
		UserID:            customer.ID,
		CreatedByVendorID: nil, // no vendor for published-app installs
		InstallLinkID:     nil,
		NuonAppID:         appID,
		NuonInstallID:     nuonInstall.ID,
		Name:              req.Name,
		Status:            models.StatusPending,
		Region:            region,
	}

	if err := h.db.Create(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store install locally"})
		return
	}

	token, _, err := h.auth.TokenGenerator(customer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate authentication token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Install created successfully",
		"token":   token,
		"install": install,
	})
}
