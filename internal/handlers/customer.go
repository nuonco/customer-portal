package handlers

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nuonco/mono/services/customer-dashboard/internal/background"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/components"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

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
		LogoBase64:         theme.LogoBase64,
		SupportContact:     theme.SupportContact,
		RadiusClass:        theme.GetRadiusClass(),
		DensityClass:       theme.GetDensityClass(),
	}
}

// InstallLinkPage renders the install link acceptance page
func (h *Handler) InstallLinkPage(c *gin.Context) {
	sha := c.Query("sha")
	if sha == "" {
		theme, _ := models.GetOrCreateAppTheme(h.db)
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme),
			Error:       "Missing or invalid install link",
		}
		h.RenderTempl(c, http.StatusBadRequest, customerpages.ErrorPage(props))
		return
	}

	var link models.InstallLink
	if err := h.db.Preload("NuonOrg").Preload("Install").Where("sha = ?", sha).First(&link).Error; err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db)
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme),
			Error:       "Install link not found or invalid",
		}
		h.RenderTempl(c, http.StatusNotFound, customerpages.ErrorPage(props))
		return
	}

	if link.Used {
		theme, _ := models.GetOrCreateAppTheme(h.db)
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme),
			Error:       "This install link has already been used",
		}
		h.RenderTempl(c, http.StatusBadRequest, customerpages.ErrorPage(props))
		return
	}

	// Get global app theme for customer UI
	theme, err := models.GetOrCreateAppTheme(h.db)
	if err != nil {
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme),
			Error:       "Failed to load theme settings",
		}
		h.RenderTempl(c, http.StatusInternalServerError, customerpages.ErrorPage(props))
		return
	}

	props := customerpages.InstallLinkPageProps{
		LayoutProps: h.buildCustomerLayoutProps("Install "+link.AppName, nil, theme),
		Link:        &link,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallLinkPage(props))
}

// AcceptInstallLink handles the install link acceptance and ownership transfer
func (h *Handler) AcceptInstallLink(c *gin.Context) {
	var req struct {
		SHA   string `json:"sha" binding:"required"`
		Email string `json:"email" binding:"required"`
		Name  string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find the install link with its associated install
	var link models.InstallLink
	if err := h.db.Preload("NuonOrg").Preload("Install").Where("sha = ?", req.SHA).First(&link).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install link not found"})
		return
	}

	if link.Used {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Install link already used"})
		return
	}

	// Check that the install exists and is in pending_customer state
	if link.Install == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No install associated with this link"})
		return
	}

	if link.Install.Status != models.StatusPendingCustomer {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Install is not in pending customer state"})
		return
	}

	// Create customer account
	customer := models.User{
		Name:  req.Name,
		Email: req.Email,
		Role:  models.RoleCustomer,
	}

	// Check if customer already exists
	var existingCustomer models.User
	if err := h.db.Where("email = ?", req.Email).First(&existingCustomer).Error; err == nil {
		// Customer exists, use existing account
		customer = existingCustomer
	} else {
		// Create new customer
		if err := h.db.Create(&customer).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create customer account"})
			return
		}
	}

	// Transfer install ownership to customer
	install := link.Install
	install.UserID = customer.ID          // Transfer ownership to customer
	install.Status = models.StatusPending // Update status to normal pending

	if err := h.db.Save(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to transfer install ownership"})
		return
	}

	// Mark link as used
	link.Used = true
	if err := h.db.Save(&link).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update install link"})
		return
	}

	// Generate JWT token for the customer
	token, _, err := h.auth.TokenGenerator(&customer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate authentication token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Install ownership transferred successfully",
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

	// Get all installs based on user role
	// Vendors see installs they created, customers see installs they own
	var allInstalls []models.Install
	query := h.db.Preload("InstallLink").Preload("InstallLink.NuonOrg").Order("created_at DESC")
	if user.Role == models.RoleVendor {
		query = query.Where("created_by_vendor_id = ?", user.ID)
	} else {
		query = query.Where("user_id = ?", user.ID)
	}
	if err := query.Find(&allInstalls).Error; err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db)
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
	theme, _ := models.GetOrCreateAppTheme(h.db)

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
		theme, _ := models.GetOrCreateAppTheme(h.db)
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
		theme, _ := models.GetOrCreateAppTheme(h.db)
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
	theme, _ := models.GetOrCreateAppTheme(h.db)

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
