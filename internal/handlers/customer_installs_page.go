package handlers

import (
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

	// Pagination parameters
	const installsPerPage = 10
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

	// Paginate directly from allInstalls (already sorted by created_at DESC from DB)
	totalCount := int64(len(allInstalls))
	var paginatedInstalls []customerui.InstallWithApprovalStatus
	if len(allInstalls) > 0 {
		startIndex := offset
		endIndex := offset + installsPerPage
		if startIndex >= len(allInstalls) {
			startIndex = 0
			page = 1
			offset = 0
		}
		if endIndex > len(allInstalls) {
			endIndex = len(allInstalls)
		}
		for _, install := range allInstalls[startIndex:endIndex] {
			paginatedInstalls = append(paginatedInstalls, customerui.InstallWithApprovalStatus{
				Install: install,
			})
		}
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
		Installs:     paginatedInstalls,
		CurrentPage:  page,
		TotalPages:   totalPages,
		HasPrevious:  page > 1,
		HasNext:      page < totalPages,
		PreviousPage: page - 1,
		NextPage:     page + 1,
		TotalCount:   totalCount,
		PerPage:      installsPerPage,
		ShowingFrom:  showingFrom,
		ShowingTo:    showingTo,
		PageNumbers:  pageNumbers,
	}

	// Always use the subdomain's org for theme — it is the authoritative org context
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)

	// Fetch app name information for all paginated installs in parallel
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
			appID   string
			appName string
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
					}
				}
				results[i] = res
			}(i, info)
		}
		wg.Wait()
		for _, res := range results {
			if res.appName != "" {
				appNameMap[res.appID] = res.appName
			}
		}
	}

	// Set AppName on each paginated install from the fetched map (preserve existing name if API lookup failed)
	for i, inst := range paginatedInstalls {
		if name := appNameMap[inst.Install.GetAppID()]; name != "" {
			paginatedInstalls[i].AppName = name
		}
	}

	// Try template override first
	installsData := make([]overrides.InstallData, 0, len(paginatedInstalls))
	for _, inst := range paginatedInstalls {
		appID := inst.Install.GetAppID()
		appName := appNameMap[appID]
		if appName == "" {
			appName = inst.AppName
		}

		installsData = append(installsData, overrides.InstallData{
			ID:        inst.Install.ID,
			Name:      inst.Install.Name,
			Region:    inst.Install.Region,
			Platform:  "",
			AppName:   appName,
			CreatedAt: inst.Install.CreatedAt.Format("Jan 2, 2006"),
		})
	}

	pageData := overrides.InstallsPageData{
		Installs:     installsData,
		TotalCount:   totalCount,
		CurrentPage:  page,
		TotalPages:   totalPages,
		HasPrevious:  page > 1,
		HasNext:      page < totalPages,
		PreviousPage: page - 1,
		NextPage:     page + 1,
		ShowingFrom:  showingFrom,
		ShowingTo:    showingTo,
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

// InstallDetailPanel renders the install detail content for the sliding panel (no layout wrapper)
