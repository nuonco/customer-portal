package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/background"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/overrides"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/components"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
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
func (h *Handler) buildCustomerLayoutProps(title string, user *models.User, theme *models.AppTheme) customerui.LayoutProps {
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)
	secondaryColor, secondaryColorDark := GetPrimaryColors(theme.SecondaryColor)

	return customerui.LayoutProps{
		Title:              title,
		User:               user,
		BasePath:           h.basePath,
		PrimaryColor:       primaryColor,
		PrimaryColorDark:   primaryColorDark,
		SecondaryColor:     secondaryColor,
		SecondaryColorDark: secondaryColorDark,
		HeadingFont:        theme.HeadingFont,
		BodyFont:           theme.BodyFont,
		HeadingFontBase64:  theme.HeadingFontBase64,
		BodyFontBase64:     theme.BodyFontBase64,
		LogoBase64:         theme.LogoLightBase64,
		SupportContact:     theme.SupportContact,
		RadiusClass:        theme.GetRadiusClass(),
		DensityClass:       theme.GetDensityClass(),
		CSSPath:            assets.CustomerCSSPath(),
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
		theme, _ := models.GetOrCreateAppTheme(h.db, h.getWorkspaceIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme),
			Error:       "Missing or invalid install link",
		}
		h.RenderTempl(c, http.StatusBadRequest, customerpages.ErrorPage(props))
		return
	}

	var link models.InstallLink
	if err := h.db.Preload("NuonOrg").Where("sha = ?", sha).First(&link).Error; err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db, h.getWorkspaceIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme),
			Error:       "Install link not found or invalid",
		}
		h.RenderTempl(c, http.StatusNotFound, customerpages.ErrorPage(props))
		return
	}

	if link.Used {
		theme, _ := models.GetOrCreateAppTheme(h.db, link.WorkspaceID)
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme),
			Error:       "This install link has already been used",
		}
		h.RenderTempl(c, http.StatusBadRequest, customerpages.ErrorPage(props))
		return
	}

	// Get global app theme for customer UI
	// Use install link's workspace ID (not getWorkspaceIDForTheme which only works for vendor routes)
	workspaceID := link.WorkspaceID
	theme, err := models.GetOrCreateAppTheme(h.db, workspaceID)
	if err != nil {
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme),
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

	ctx := h.buildTemplateContext(workspaceID, "Install "+link.AppName, loggedInUser, theme, pageData)
	if h.tryRenderOverride(c, workspaceID, "install_link", ctx) {
		return
	}

	// Fall back to default Templ template
	props := customerpages.InstallLinkPageProps{
		LayoutProps:  h.buildCustomerLayoutProps("Install "+link.AppName, nil, theme),
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

	// Generate install name
	installName := nuon.GenerateInstallName(link.AppName)

	// Create the install via Nuon API with merged inputs
	nuonInstall, err := nuonClient.CreateInstallWithCustomName(c.Request.Context(), link.AppID, link.AppName, installName, region, location, mergedInputs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create install via Nuon API: %v", err)})
		return
	}

	// Create local Install record with customer as owner
	install := &models.Install{
		WorkspaceID:       link.WorkspaceID, // Link to vendor's workspace
		UserID:            customer.ID,      // Customer owns the install
		CreatedByVendorID: link.UserID,      // Track original vendor
		InstallLinkID:     link.ID,
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
	user := h.GetFreshUser(c) // Load from DB for topbar display

	// Tab and pagination parameters
	const installsPerPage = 10
	currentTab := c.DefaultQuery("tab", "needs-attention")
	page := getPageFromQuery(c)
	offset := (page - 1) * installsPerPage

	// Get all installs owned by this user
	var allInstalls []models.Install
	query := h.db.Preload("InstallLink").Preload("InstallLink.NuonOrg").Order("created_at DESC")
	query = query.Where("user_id = ?", user.ID)
	if err := query.Find(&allInstalls).Error; err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db, h.getWorkspaceIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", user, theme),
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

		// Get health check status
		hasHealthChecks := false
		pending, passed, failed := 0, 0, 0
		overallHealth := ""

		healthCheckIDs := install.InstallLink.GetHealthCheckActionIDs()
		if len(healthCheckIDs) > 0 {
			hasHealthChecks = true
			// Fetch health status via Nuon API if org has credentials
			if install.InstallLink.NuonOrg.APIToken != "" {
				client, err := nuon.NewClientWithURL(
					install.InstallLink.NuonOrg.APIToken,
					install.InstallLink.NuonOrg.NuonOrgID,
					h.nuonAPIURL,
				)
				if err == nil {
					ctx := context.Background()
					healthStatuses, overall, _ := background.CheckInstallHealthStatus(ctx, client, &install)
					overallHealth = overall
					for _, s := range healthStatuses {
						switch s.Status {
						case "Passing":
							passed++
						case "Pending":
							pending++
						case "Failing":
							failed++
						}
					}
				}
			}
		}

		installWithStatus := customerui.InstallWithApprovalStatus{
			Install:             install,
			HasPendingApprovals: hasPendingApprovals,
			IsUpdating:          isUpdating,
			HasHealthChecks:     hasHealthChecks,
			HealthChecksPending: pending,
			HealthChecksPassed:  passed,
			HealthChecksFailed:  failed,
			OverallHealthStatus: overallHealth,
		}

		// Categorization logic (priority order):
		// 1. Needs Attention: pending approvals OR unhealthy health checks
		// 2. Updating: in-progress workflow without approvals
		// 3. Healthy: everything else
		needsAttention := hasPendingApprovals || (hasHealthChecks && (failed > 0 || pending > 0))

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

	// Get global app theme for customer UI
	// Get workspace ID from installs (customers view installs from a vendor's workspace)
	// Try from paginated installs first, then all installs, then context
	var workspaceID string
	if len(paginatedInstalls) > 0 && paginatedInstalls[0].Install.WorkspaceID != "" {
		workspaceID = paginatedInstalls[0].Install.WorkspaceID
	} else if len(allInstalls) > 0 && allInstalls[0].WorkspaceID != "" {
		workspaceID = allInstalls[0].WorkspaceID
	} else {
		workspaceID = h.getWorkspaceIDForTheme(c)
	}
	theme, _ := models.GetOrCreateAppTheme(h.db, workspaceID)

	// Fetch platform information for all installs
	// Build a map of AppID -> Platform by fetching apps from Nuon API
	platformMap := make(map[string]string)
	for _, inst := range paginatedInstalls {
		appID := inst.Install.InstallLink.AppID
		if _, exists := platformMap[appID]; !exists {
			// Initialize Nuon client with the org's credentials
			nuonClient, err := nuon.NewClientWithURL(
				inst.Install.InstallLink.NuonOrg.APIToken,
				inst.Install.InstallLink.NuonOrg.NuonOrgID,
				h.nuonAPIURL,
			)
			if err == nil {
				// Fetch app details to get platform
				app, err := nuonClient.GetApp(c.Request.Context(), appID)
				if err == nil && app != nil && app.RunnerConfig != nil {
					platformMap[appID] = string(app.RunnerConfig.AppRunnerType)
				} else {
					platformMap[appID] = "" // Default to empty if fetch fails
				}
			} else {
				platformMap[appID] = "" // Default to empty if client init fails
			}
		}
	}

	// Try template override first
	installsData := make([]overrides.InstallData, 0, len(paginatedInstalls))
	for _, inst := range paginatedInstalls {
		appID := inst.Install.InstallLink.AppID
		platform := platformMap[appID]

		installsData = append(installsData, overrides.InstallData{
			ID:                  inst.Install.ID,
			Name:                inst.Install.Name,
			Status:              string(inst.Install.Status),
			Region:              inst.Install.Region,
			Platform:            platform,
			AppName:             inst.Install.InstallLink.AppName,
			CreatedAt:           inst.Install.CreatedAt.Format("Jan 2, 2006"),
			HasHealthChecks:     inst.HasHealthChecks,
			HealthChecksPassed:  inst.HealthChecksPassed,
			HealthChecksPending: inst.HealthChecksPending,
			HealthChecksFailed:  inst.HealthChecksFailed,
			HasPendingApproval:  inst.HasPendingApprovals,
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

	ctx := h.buildTemplateContext(workspaceID, "Your Installs", user, theme, pageData)
	if h.tryRenderOverride(c, workspaceID, "installs", ctx) {
		return
	}

	// Fall back to default Templ template
	props := customerpages.InstallsPageProps{
		LayoutProps: h.buildCustomerLayoutProps("Your Installs", user, theme),
		Pagination:  pagination,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallsPage(props))
}

// InstallDetail renders the install detail page
func (h *Handler) InstallDetail(c *gin.Context) {
	user := h.GetFreshUser(c) // Load from DB for topbar display

	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		theme, _ := models.GetOrCreateAppTheme(h.db, h.getWorkspaceIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", user, theme),
			Error:       "Install not found",
		}
		h.RenderTempl(c, http.StatusNotFound, customerpages.ErrorPage(props))
		return
	}

	install := installInterface.(*models.Install)

	// Load the install link with NuonOrg for display and health checks
	if err := h.db.Preload("InstallLink").Preload("InstallLink.NuonOrg").Where("id = ?", install.ID).First(install).Error; err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db, install.WorkspaceID)
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", user, theme),
			Error:       "Failed to load install details",
		}
		h.RenderTempl(c, http.StatusInternalServerError, customerpages.ErrorPage(props))
		return
	}

	// Fetch recent workflows for embedded display
	var recentWorkflows []customerui.WorkflowData
	if workflows, err := h.fetchRecentWorkflows(c, install); err != nil {
		// Log the error but don't fail the page - just show no workflows
		c.Header("X-Workflow-Error", err.Error()) // For debugging
		recentWorkflows = []customerui.WorkflowData{}
	} else {
		recentWorkflows = convertWorkflowsToData(workflows)
	}

	// Fetch platform information from app
	var platform string
	if install.InstallLink.NuonOrg.APIToken != "" {
		client, err := nuon.NewClientWithURL(
			install.InstallLink.NuonOrg.APIToken,
			install.InstallLink.NuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if err == nil {
			app, err := client.GetApp(c.Request.Context(), install.InstallLink.AppID)
			if err == nil && app != nil && app.RunnerConfig != nil {
				platform = string(app.RunnerConfig.AppRunnerType)
			}
		}
	}

	// Fetch health check status if configured
	var healthCheckStatuses []customerui.HealthCheckStatusData
	var overallHealthStatus string
	healthCheckIDs := install.InstallLink.GetHealthCheckActionIDs()

	if len(healthCheckIDs) > 0 && install.InstallLink.NuonOrg.APIToken != "" {
		// Create Nuon client using global API URL
		client, err := nuon.NewClientWithURL(
			install.InstallLink.NuonOrg.APIToken,
			install.InstallLink.NuonOrg.NuonOrgID,
			h.nuonAPIURL,
		)
		if err != nil {
			log.Printf("Failed to create Nuon client for health checks: %v", err)
		} else {
			ctx := context.Background()
			bgHealthStatuses, status, err := background.CheckInstallHealthStatus(ctx, client, install)
			if err != nil {
				log.Printf("Failed to fetch health check status: %v", err)
			} else {
				overallHealthStatus = status
				healthCheckStatuses = convertHealthCheckStatuses(bgHealthStatuses)
			}
		}
	}

	// Get global app theme for customer UI
	// Use install's workspace ID (not getWorkspaceIDForTheme which only works for vendor routes)
	workspaceID := install.WorkspaceID
	theme, _ := models.GetOrCreateAppTheme(h.db, workspaceID)

	// Try template override first
	workflowsData := make([]overrides.WorkflowData, 0, len(recentWorkflows))
	for _, wf := range recentWorkflows {
		workflowsData = append(workflowsData, overrides.WorkflowData{
			ID:          wf.ID,
			Type:        wf.Name,
			Status:      wf.Status,
			CreatedAt:   wf.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
			CompletedAt: wf.FinishedAt.Format("Jan 2, 2006 3:04 PM"),
		})
	}

	healthChecksData := make([]overrides.HealthCheckData, 0, len(healthCheckStatuses))
	for _, hc := range healthCheckStatuses {
		checkedAt := ""
		if !hc.LastRunAt.IsZero() {
			checkedAt = hc.LastRunAt.Format("Jan 2, 2006 3:04 PM")
		}
		healthChecksData = append(healthChecksData, overrides.HealthCheckData{
			ID:        hc.ActionID,
			Name:      hc.ActionName,
			Status:    hc.Status,
			Message:   hc.StatusMessage,
			CheckedAt: checkedAt,
		})
	}

	var pendingApproval *overrides.WorkflowData
	for _, wf := range recentWorkflows {
		if wf.CanApprove || wf.CanApproveAll {
			pendingApproval = &overrides.WorkflowData{
				ID:        wf.ID,
				Type:      wf.Name,
				Status:    wf.Status,
				CreatedAt: wf.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
			}
			break
		}
	}

	pageData := overrides.InstallDetailPageData{
		Install: overrides.InstallData{
			ID:                  install.ID,
			Name:                install.Name,
			Status:              string(install.Status),
			Region:              install.Region,
			Platform:            platform,
			AppName:             install.InstallLink.AppName,
			CreatedAt:           install.CreatedAt.Format("Jan 2, 2006"),
			HasHealthChecks:     len(healthCheckIDs) > 0,
			HealthChecksPassed:  countHealthCheckStatus(healthCheckStatuses, "Passing"),
			HealthChecksPending: countHealthCheckStatus(healthCheckStatuses, "Pending"),
			HealthChecksFailed:  countHealthCheckStatus(healthCheckStatuses, "Failing"),
			HasPendingApproval:  pendingApproval != nil,
		},
		Workflows:       workflowsData,
		HealthChecks:    healthChecksData,
		PendingApproval: pendingApproval,
	}

	ctx := h.buildTemplateContext(workspaceID, "Install - "+install.Name, user, theme, pageData)
	if h.tryRenderOverride(c, workspaceID, "install_detail", ctx) {
		return
	}

	// Fall back to default Templ template
	props := customerpages.InstallDetailPageProps{
		LayoutProps:         h.buildCustomerLayoutProps("Install - "+install.Name, user, theme),
		Install:             install,
		RecentWorkflows:     recentWorkflows,
		HealthCheckStatuses: healthCheckStatuses,
		OverallHealthStatus: overallHealthStatus,
		HasHealthChecks:     len(healthCheckIDs) > 0,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallDetailPage(props))
}

// countHealthCheckStatus counts health checks with the given status
func countHealthCheckStatus(statuses []customerui.HealthCheckStatusData, status string) int {
	count := 0
	for _, s := range statuses {
		if s.Status == status {
			count++
		}
	}
	return count
}

// convertWorkflowsToData converts gin.H workflow data to customerui.WorkflowData
func convertWorkflowsToData(workflows []gin.H) []customerui.WorkflowData {
	result := make([]customerui.WorkflowData, 0, len(workflows))
	for _, wf := range workflows {
		data := customerui.WorkflowData{
			ID:                       getString(wf, "id"),
			Name:                     getString(wf, "name"),
			Status:                   getString(wf, "status"),
			StatusClass:              getString(wf, "status_class"),
			CanApprove:               getBool(wf, "can_approve"),
			CanApproveAll:            getBool(wf, "can_approve_all"),
			CanCancel:                getBool(wf, "can_cancel"),
			ApproveDisabledReason:    getString(wf, "approve_disabled_reason"),
			ApproveAllDisabledReason: getString(wf, "approve_all_disabled_reason"),
			CancelDisabledReason:     getString(wf, "cancel_disabled_reason"),
		}

		// Handle time fields
		if t, ok := wf["created_at"].(time.Time); ok {
			data.CreatedAt = t
		}
		if t, ok := wf["finished_at"].(time.Time); ok {
			data.FinishedAt = t
		}

		// Handle approval step
		if step, ok := wf["approval_step"].(gin.H); ok && step != nil {
			data.ApprovalStep = &customerui.ApprovalStepData{
				StepID:     getString(step, "step_id"),
				ApprovalID: getString(step, "approval_id"),
			}
		}

		result = append(result, data)
	}
	return result
}

// convertHealthCheckStatuses converts background.HealthCheckStatus to customerui.HealthCheckStatusData
func convertHealthCheckStatuses(statuses []background.HealthCheckStatus) []customerui.HealthCheckStatusData {
	result := make([]customerui.HealthCheckStatusData, 0, len(statuses))
	for _, s := range statuses {
		result = append(result, customerui.HealthCheckStatusData{
			ActionID:      s.ActionID,
			ActionName:    s.ActionName,
			Status:        s.Status,
			StatusClass:   s.StatusClass,
			StatusMessage: s.StatusMessage,
			LastRunAt:     s.LastRunAt,
		})
	}
	return result
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

	// Load install link to get org info
	if err := h.db.Preload("InstallLink.NuonOrg").Where("id = ?", install.ID).First(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	// Initialize Nuon client to deprovision the install using global API URL
	nuonClient, err := nuon.NewClientWithURL(install.InstallLink.NuonOrg.APIToken, install.InstallLink.NuonOrg.NuonOrgID, h.nuonAPIURL)
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

// GetInstallInputs returns the current inputs and input config for an install
func (h *Handler) GetInstallInputs(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	install := installInterface.(*models.Install)

	// Load install link to get org info and app ID
	if err := h.db.Preload("InstallLink.NuonOrg").Where("id = ?", install.ID).First(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	// Handle NuonOrg loading with fallback strategies (nested preload sometimes fails)
	if install.InstallLink.NuonOrg.ID == "" {
		var installLinkWithOrg models.InstallLink
		if err := h.db.Preload("NuonOrg").Where("id = ?", install.InstallLink.ID).First(&installLinkWithOrg).Error; err == nil {
			if installLinkWithOrg.NuonOrg.ID != "" {
				install.InstallLink.NuonOrg = installLinkWithOrg.NuonOrg
			}
		}

		// If still not loaded, try manual loading
		if install.InstallLink.NuonOrg.ID == "" {
			var nuonOrg models.NuonOrg
			if err := h.db.Where("id = ?", install.InstallLink.OrgID).First(&nuonOrg).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load organization"})
				return
			}
			install.InstallLink.NuonOrg = nuonOrg
		}
	}

	// Create Nuon client using org credentials
	org := install.InstallLink.NuonOrg
	if org.APIToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization credentials not configured"})
		return
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Fetch current inputs from Nuon API
	currentInputs, err := nuonClient.GetInstallCurrentInputs(c.Request.Context(), install.NuonInstallID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch install inputs: %v", err)})
		return
	}
	// Fetch app input config for field definitions
	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), install.InstallLink.AppID)
	if err != nil {
		// Input config may not exist, that's okay - return empty
		inputConfig = nil
	}

	// Fetch local customer input config for this app
	var localConfig models.AppInputConfig
	h.db.Where("org_id = ? AND app_id = ?", install.InstallLink.OrgID, install.InstallLink.AppID).First(&localConfig)
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

	// Load install link to get org info
	if err := h.db.Preload("InstallLink.NuonOrg").Where("id = ?", install.ID).First(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	// Handle NuonOrg loading with fallback strategies (nested preload sometimes fails)
	if install.InstallLink.NuonOrg.ID == "" {
		var installLinkWithOrg models.InstallLink
		if err := h.db.Preload("NuonOrg").Where("id = ?", install.InstallLink.ID).First(&installLinkWithOrg).Error; err == nil {
			if installLinkWithOrg.NuonOrg.ID != "" {
				install.InstallLink.NuonOrg = installLinkWithOrg.NuonOrg
			}
		}

		// If still not loaded, try manual loading
		if install.InstallLink.NuonOrg.ID == "" {
			var nuonOrg models.NuonOrg
			if err := h.db.Where("id = ?", install.InstallLink.OrgID).First(&nuonOrg).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load organization"})
				return
			}
			install.InstallLink.NuonOrg = nuonOrg
		}
	}

	// Create Nuon client using org credentials
	org := install.InstallLink.NuonOrg
	if org.APIToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization credentials not configured"})
		return
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
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

// TriggerHealthChecks handles POST request to manually trigger health checks
func (h *Handler) TriggerHealthChecks(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	install := installInterface.(*models.Install)

	// Load install link with NuonOrg
	if err := h.db.Preload("InstallLink").Preload("InstallLink.NuonOrg").
		Where("id = ?", install.ID).First(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	// Handle NuonOrg loading with fallback strategies (nested preload sometimes fails)
	if install.InstallLink.NuonOrg.ID == "" {
		var installLinkWithOrg models.InstallLink
		if err := h.db.Preload("NuonOrg").Where("id = ?", install.InstallLink.ID).First(&installLinkWithOrg).Error; err == nil {
			if installLinkWithOrg.NuonOrg.ID != "" {
				install.InstallLink.NuonOrg = installLinkWithOrg.NuonOrg
			}
		}

		// If still not loaded, try manual loading
		if install.InstallLink.NuonOrg.ID == "" {
			var nuonOrg models.NuonOrg
			if err := h.db.Where("id = ?", install.InstallLink.OrgID).First(&nuonOrg).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load organization"})
				return
			}
			install.InstallLink.NuonOrg = nuonOrg
		}
	}

	// Check if health checks are configured
	// Use the new GetHealthCheckIDPairs to get parsed config/workflow IDs
	healthCheckPairs := install.InstallLink.GetHealthCheckIDPairs()
	if len(healthCheckPairs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No health checks configured for this install"})
		return
	}

	// Create Nuon client using global API URL
	org := install.InstallLink.NuonOrg
	if org.APIToken == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization credentials not configured"})
		return
	}

	client, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Nuon client"})
		return
	}

	// Trigger each health check action using the ConfigID (not WorkflowID)
	triggered := 0
	var errors []string
	for _, pair := range healthCheckPairs {
		// RunInstallAction expects the ActionWorkflowConfigID
		if err := client.RunInstallAction(c.Request.Context(), install.NuonInstallID, pair.ConfigID); err != nil {
			log.Printf("Failed to trigger health check (config=%s, workflow=%s): %v", pair.ConfigID, pair.WorkflowID, err)
			errors = append(errors, fmt.Sprintf("Action %s: %v", pair.ConfigID, err))
		} else {
			triggered++
			log.Printf("Triggered health check (config=%s) for install %s", pair.ConfigID, install.ID)
		}
	}

	if triggered == 0 && len(errors) > 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to trigger health checks. The configured action(s) may no longer exist.",
			"details": errors,
		})
		return
	}

	// For HTMX requests, return the updated health status partial
	if isHTMXRequest(c) {
		// Wait a moment for the health checks to start, then fetch status
		// In practice, health checks are async, so we return current status
		ctx := context.Background()
		bgHealthStatuses, overallHealthStatus, err := background.CheckInstallHealthStatus(ctx, client, install)
		if err != nil {
			log.Printf("Failed to fetch health check status after trigger: %v", err)
		}

		// Convert to templ data type
		healthCheckStatuses := convertHealthCheckStatuses(bgHealthStatuses)

		props := components.HealthStatusProps{
			InstallID:           install.ID,
			HasHealthChecks:     true,
			HealthCheckStatuses: healthCheckStatuses,
			OverallHealthStatus: overallHealthStatus,
			BasePath:            h.basePath,
		}
		h.RenderTempl(c, http.StatusOK, components.HealthStatus(props))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("Triggered %d health check(s)", triggered),
		"triggered": triggered,
	})
}

// checkHasPendingApprovals checks if an install has workflows waiting for approval
func (h *Handler) checkHasPendingApprovals(c *gin.Context, install *models.Install) bool {
	// Load install link to get org info with fallback strategies
	if err := h.db.Preload("InstallLink.NuonOrg").Where("id = ?", install.ID).First(install).Error; err != nil {
		return false // Return false if we can't load install details
	}

	// Handle NuonOrg loading with fallback strategies (same as in workflows.go)
	if install.InstallLink.NuonOrg.ID == "" {
		var installLinkWithOrg models.InstallLink
		if err := h.db.Preload("NuonOrg").Where("id = ?", install.InstallLink.ID).First(&installLinkWithOrg).Error; err == nil {
			if installLinkWithOrg.NuonOrg.ID != "" {
				install.InstallLink.NuonOrg = installLinkWithOrg.NuonOrg
			}
		}

		// If still not loaded, try manual loading
		if install.InstallLink.NuonOrg.ID == "" {
			var nuonOrg models.NuonOrg
			if err := h.db.Where("id = ?", install.InstallLink.OrgID).First(&nuonOrg).Error; err != nil {
				return false // Return false if we can't find org info
			}
			install.InstallLink.NuonOrg = nuonOrg
		}
	}

	// Initialize Nuon client using global API URL
	nuonClient, err := nuon.NewClientWithURL(install.InstallLink.NuonOrg.APIToken, install.InstallLink.NuonOrg.NuonOrgID, h.nuonAPIURL)
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
	// Load install link to get org info with fallback strategies
	if err := h.db.Preload("InstallLink.NuonOrg").Where("id = ?", install.ID).First(install).Error; err != nil {
		return false
	}

	// Handle NuonOrg loading with fallback strategies (same as in checkHasPendingApprovals)
	if install.InstallLink.NuonOrg.ID == "" {
		var installLinkWithOrg models.InstallLink
		if err := h.db.Preload("NuonOrg").Where("id = ?", install.InstallLink.ID).First(&installLinkWithOrg).Error; err == nil {
			if installLinkWithOrg.NuonOrg.ID != "" {
				install.InstallLink.NuonOrg = installLinkWithOrg.NuonOrg
			}
		}

		// If still not loaded, try manual loading
		if install.InstallLink.NuonOrg.ID == "" {
			var nuonOrg models.NuonOrg
			if err := h.db.Where("id = ?", install.InstallLink.OrgID).First(&nuonOrg).Error; err != nil {
				return false
			}
			install.InstallLink.NuonOrg = nuonOrg
		}
	}

	// Initialize Nuon client using global API URL
	nuonClient, err := nuon.NewClientWithURL(install.InstallLink.NuonOrg.APIToken, install.InstallLink.NuonOrg.NuonOrgID, h.nuonAPIURL)
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
