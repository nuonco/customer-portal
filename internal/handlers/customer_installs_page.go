package handlers

import (
	"context"
	"math"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/overrides"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) InstallsPage(c *gin.Context) {
	user := h.tryGetLoggedInUser(c) // Returns nil if not logged in

	// For unauthenticated visitors, render empty page with login CTA
	if user == nil {
		orgID := h.getOrgIDForTheme(c)
		theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
		layoutProps := h.buildCustomerLayoutProps("Your Installs", nil, theme, h.getOrgForLayout(c), nil, nil)
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
		// Validate install exists and user has access
		var install models.Install
		detailQuery := h.db.Where("id = ?", installIDParam)
		detailMember := middleware.GetCustomerAccountMember(c)
		if detailMember == nil {
			orgID := h.getOrgIDForTheme(c)
			if orgID != "" {
				var members []models.CustomerAccountMember
				if h.db.Where("user_id = ? AND org_id = ? AND deleted_at IS NULL", user.ID, orgID).Find(&members).Error == nil && len(members) > 0 {
					detailMember = middleware.SelectActiveMember(c, members)
				}
			}
		}
		if detailMember != nil {
			detailQuery = detailQuery.Where(
				"(user_id = ? AND (customer_account_id IS NULL OR customer_account_id = ?)) OR (customer_account_id = ? AND visibility = ?)",
				user.ID, detailMember.AccountID, detailMember.AccountID, models.VisibilityAccount,
			)
		} else {
			detailQuery = detailQuery.Where("user_id = ? AND customer_account_id IS NULL", user.ID)
		}
		if err := detailQuery.First(&install).Error; err != nil {
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

	// Get all installs visible to this user (own installs + account-shared installs)
	var allInstalls []models.Install
	query := h.db.Preload("InstallLink").Preload("InstallLink.NuonOrg").Preload("Org").Order("created_at DESC")

	// Use active account from middleware context for account-based visibility
	activeMember := middleware.GetCustomerAccountMember(c)
	if activeMember == nil && user != nil {
		// Fallback: look up membership from DB (route may not use RequireCustomerAccount middleware)
		orgID := h.getOrgIDForTheme(c)
		if orgID != "" {
			var members []models.CustomerAccountMember
			if h.db.Preload("Account").Where("user_id = ? AND org_id = ? AND deleted_at IS NULL", user.ID, orgID).Find(&members).Error == nil && len(members) > 0 {
				activeMember = middleware.SelectActiveMember(c, members)
			}
		}
	}
	if activeMember != nil {
		query = query.Where(
			"(user_id = ? AND (customer_account_id IS NULL OR customer_account_id = ?)) OR (customer_account_id = ? AND visibility = ?)",
			user.ID, activeMember.AccountID, activeMember.AccountID, models.VisibilityAccount,
		)
	} else {
		query = query.Where("user_id = ? AND customer_account_id IS NULL", user.ID)
	}
	if err := query.Find(&allInstalls).Error; err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
		acctActive, acctOthers := h.getCustomerAccountsFromContext(c)
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", user, theme, h.getOrgForLayout(c), acctActive, acctOthers),
			Error:       "Failed to load installs",
		}
		h.RenderTempl(c, http.StatusInternalServerError, customerpages.ErrorPage(props))
		return
	}

	// Check status of all installs in parallel with bounded concurrency
	statuses := make([]installStatus, len(allInstalls))
	{
		var wg sync.WaitGroup
		sem := make(chan struct{}, 10)
		ctx := c.Request.Context()
		for i, install := range allInstalls {
			wg.Add(1)
			go func(i int, install models.Install) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				statuses[i] = h.checkInstallStatus(ctx, &install)
			}(i, install)
		}
		wg.Wait()
	}

	// Separate installs by status into tabs
	var needsAttentionInstalls []customerui.InstallWithApprovalStatus
	var healthyInstalls []customerui.InstallWithApprovalStatus
	var updatingInstalls []customerui.InstallWithApprovalStatus

	for i, install := range allInstalls {
		st := statuses[i]
		installWithStatus := customerui.InstallWithApprovalStatus{
			Install:             install,
			HasPendingApprovals: st.HasPendingApprovals,
			IsUpdating:          st.IsUpdating,
		}

		// Categorization logic (priority order):
		// 1. Needs Attention: pending approvals OR active provision
		// 2. Updating: in-progress workflow without approvals
		// 3. Healthy: everything else
		needsAttention := st.HasPendingApprovals || st.HasActiveProvision

		if needsAttention {
			needsAttentionInstalls = append(needsAttentionInstalls, installWithStatus)
		} else if st.IsUpdating {
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

	// Fetch platform and app name information for all paginated installs in parallel
	platformMap := make(map[string]string)
	appNameMap := make(map[string]string)

	// Collect unique app IDs and their org credentials, plus fallback names
	type appFetchInfo struct {
		appID        string
		org          *models.NuonOrg
		fallbackName string
	}
	var toFetch []appFetchInfo
	seen := make(map[string]bool)
	for _, inst := range paginatedInstalls {
		appID := inst.Install.GetAppID()
		if seen[appID] {
			continue
		}
		seen[appID] = true
		org := inst.Install.GetNuonOrg()
		var fallback string
		if inst.Install.InstallLinkID != nil {
			fallback = inst.Install.InstallLink.AppName
		}
		if org == nil || org.APIToken == "" {
			platformMap[appID] = ""
			if fallback != "" {
				appNameMap[appID] = fallback
			}
			continue
		}
		toFetch = append(toFetch, appFetchInfo{appID: appID, org: org, fallbackName: fallback})
	}

	// Fetch app details in parallel with bounded concurrency
	if len(toFetch) > 0 {
		type appResult struct {
			appID    string
			platform string
			appName  string
		}
		results := make([]appResult, len(toFetch))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 10)
		for i, info := range toFetch {
			wg.Add(1)
			go func(i int, info appFetchInfo) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				res := appResult{appID: info.appID, appName: info.fallbackName}
				nuonClient, err := nuon.NewClientWithURL(info.org.APIToken, info.org.NuonOrgID, h.nuonAPIURL)
				if err == nil {
					app, err := nuonClient.GetApp(c.Request.Context(), info.appID)
					if err == nil && app != nil {
						res.appName = app.Name
						if app.RunnerConfig != nil {
							res.platform = string(app.RunnerConfig.AppRunnerType)
						}
					}
				}
				results[i] = res
			}(i, info)
		}
		wg.Wait()
		for _, res := range results {
			platformMap[res.appID] = res.platform
			if res.appName != "" {
				appNameMap[res.appID] = res.appName
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
	acctActive, acctOthers := h.getCustomerAccountsFromContext(c)
	installsLayoutProps := h.buildCustomerLayoutProps("Your Installs", user, theme, h.getOrgForLayout(c), acctActive, acctOthers)
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

type installStatus struct {
	HasPendingApprovals bool
	IsUpdating          bool
	HasActiveProvision  bool
}

// checkInstallStatus determines all status flags for an install using minimal API calls.
// It uses the already-preloaded NuonOrg (no extra DB query) and makes one GetInstallWorkflows
// call plus up to 4 GetInstallWorkflowsByType calls (short-circuiting on first active match).

func (h *Handler) checkInstallStatus(ctx context.Context, install *models.Install) installStatus {
	org := install.GetNuonOrg()
	if org == nil || org.APIToken == "" {
		return installStatus{}
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		return installStatus{}
	}

	var st installStatus

	// Single call to get recent workflows — determines both HasPendingApprovals and IsUpdating
	const checkLimit = 5
	workflows, _, err := nuonClient.GetInstallWorkflows(ctx, install.NuonInstallID, 0, checkLimit)
	if err == nil {
		for _, workflow := range workflows {
			if workflow.Status == nil {
				continue
			}
			status := string(workflow.Status.Status)
			switch status {
			case "approval-awaiting":
				st.HasPendingApprovals = true
			case "in-progress":
				hasApprovalStep := false
				if workflow.Steps != nil {
					for _, step := range workflow.Steps {
						if step.ExecutionType == "approval" {
							hasApprovalStep = true
							break
						}
					}
				}
				if hasApprovalStep {
					st.HasPendingApprovals = true
				} else {
					st.IsUpdating = true
				}
			}
		}
	}

	// Check for active provision workflows (short-circuit on first match)
	provisionTypes := []string{"provision", "provision_sandbox", "reprovision", "reprovision_sandbox"}
	for _, wfType := range provisionTypes {
		provWorkflows, _, err := nuonClient.GetInstallWorkflowsByType(ctx, install.NuonInstallID, 0, 1, wfType)
		if err != nil || len(provWorkflows) == 0 {
			continue
		}
		wf := provWorkflows[0]
		if wf.Status == nil {
			continue
		}
		s := string(wf.Status.Status)
		if s == "pending" || s == "in-progress" || s == "approval-awaiting" {
			st.HasActiveProvision = true
			break
		}
	}

	return st
}

// InstallDetailPanel renders the install detail content for the sliding panel (no layout wrapper)
