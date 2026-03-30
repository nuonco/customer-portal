package handlers

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
)

// AccountsPage renders the vendor view of all customer accounts in an org.
func (h *Handler) AccountsPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}

	searchQuery := c.Query("q")

	type AccountResult struct {
		ID           string
		Name         string
		MemberCount  int64
		InstallCount int64
		CreatedAt    string
	}

	query := h.db.Table("customer_accounts").
		Select(`customer_accounts.id, customer_accounts.name, customer_accounts.created_at,
			(SELECT COUNT(*) FROM customer_account_members WHERE customer_account_members.account_id = customer_accounts.id AND customer_account_members.deleted_at IS NULL) as member_count,
			(SELECT COUNT(*) FROM installs WHERE installs.customer_account_id = customer_accounts.id AND installs.deleted_at IS NULL) as install_count`).
		Where("customer_accounts.org_id = ? AND customer_accounts.deleted_at IS NULL", org.ID).
		Order("customer_accounts.created_at DESC")

	if searchQuery != "" {
		query = query.Where("customer_accounts.name ILIKE ?", "%"+searchQuery+"%")
	}

	var results []AccountResult
	if err := query.Find(&results).Error; err != nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch accounts: %v", err))
		return
	}

	accounts := make([]vendorpages.AccountWithCounts, len(results))
	for i, r := range results {
		accounts[i] = vendorpages.AccountWithCounts{
			ID:           r.ID,
			Name:         r.Name,
			MemberCount:  r.MemberCount,
			InstallCount: r.InstallCount,
			CreatedAt:    r.CreatedAt,
		}
	}

	if isHTMXPartialRequest(c) {
		h.RenderTempl(c, http.StatusOK, vendorpages.AccountsTableBody(accounts, org.ID, h.basePath, searchQuery))
		return
	}

	allOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.AccountsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:            org.Name + " - Accounts",
			ActivePage:       "accounts",
			User:             user,
			CurrentOrg:       org,
			Orgs:             allOrgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Accounts", Path: fmt.Sprintf("%s/orgs/%s/accounts", h.basePath, org.ID), Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
			IsSuperuser:      h.isSuperuser(user),
		},
		Org:         *org,
		Accounts:    accounts,
		SearchQuery: searchQuery,
	}
	h.RenderTempl(c, http.StatusOK, vendorpages.AccountsPage(props))
}

// AccountDetailRedirect redirects to the members sub-page.
func (h *Handler) AccountDetailRedirect(c *gin.Context) {
	orgID := c.Param("org_id")
	accountID := c.Param("account_id")
	c.Redirect(http.StatusFound, fmt.Sprintf("%s/orgs/%s/accounts/%s/members", h.basePath, orgID, accountID))
}

func (h *Handler) accountDetailLayout(c *gin.Context) (*models.User, *models.NuonOrg, *models.CustomerAccount, vendorui.LayoutProps, bool) {
	user := h.GetFreshUser(c)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return nil, nil, nil, vendorui.LayoutProps{}, false
	}

	accountID := c.Param("account_id")

	var account models.CustomerAccount
	if err := h.db.Preload("CreatedBy").Where("id = ? AND org_id = ? AND deleted_at IS NULL", accountID, org.ID).First(&account).Error; err != nil {
		h.RenderErrorPage(c, http.StatusNotFound, "Account not found")
		return nil, nil, nil, vendorui.LayoutProps{}, false
	}

	allOrgs := h.GetUserOrgs(user.ID)

	layout := vendorui.LayoutProps{
		Title:      org.Name + " - " + account.Name,
		ActivePage: "accounts",
		User:       user,
		CurrentOrg: org,
		Orgs:       allOrgs,
		Breadcrumbs: []partials.Breadcrumb{
			{Text: "Accounts", Path: fmt.Sprintf("%s/orgs/%s/accounts", h.basePath, org.ID)},
			{Text: account.Name, Path: fmt.Sprintf("%s/orgs/%s/accounts/%s", h.basePath, org.ID, accountID), Active: true},
		},
		BasePath:         h.basePath,
		PortalScheme:     h.schemeFromBaseURL(),
		DashboardURL:     h.dashboardURL,
		PortalBaseDomain: h.subdomainBaseDomain,
		CSSPath:          assets.VendorCSSPath(),
		IsSuperuser:      h.isSuperuser(user),
	}

	return user, org, &account, layout, true
}

// AccountMembersPage renders the members sub-page of a customer account.
func (h *Handler) AccountMembersPage(c *gin.Context) {
	_, org, account, layout, ok := h.accountDetailLayout(c)
	if !ok {
		return
	}

	var members []models.CustomerAccountMember
	h.db.Preload("User").Where("account_id = ? AND deleted_at IS NULL", account.ID).Order("joined_at ASC").Find(&members)

	var invites []models.CustomerAccountInvite
	h.db.Preload("CreatedBy").Where("account_id = ? AND used_by_user_id IS NULL AND deleted_at IS NULL", account.ID).Order("created_at DESC").Find(&invites)

	h.RenderTempl(c, http.StatusOK, vendorpages.AccountMembersPage(vendorpages.AccountMembersPageProps{
		LayoutProps: layout,
		Org:         *org,
		Account:     *account,
		Members:     members,
		Invites:     invites,
	}))
}

// AccountInstallsPage renders the installs sub-page of a customer account.
func (h *Handler) AccountInstallsPage(c *gin.Context) {
	_, org, account, layout, ok := h.accountDetailLayout(c)
	if !ok {
		return
	}

	var installs []models.Install
	h.db.Where("customer_account_id = ? AND deleted_at IS NULL", account.ID).Order("created_at DESC").Find(&installs)

	h.RenderTempl(c, http.StatusOK, vendorpages.AccountInstallsPage(vendorpages.AccountInstallsPageProps{
		LayoutProps: layout,
		Org:         *org,
		Account:     *account,
		Installs:    installs,
	}))
}

// SearchOrgInstalls searches local DB installs in the org that can be assigned to an account.
func (h *Handler) SearchOrgInstalls(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.String(http.StatusBadRequest, "Organization context not found")
		return
	}

	accountID := c.Param("account_id")
	q := strings.TrimSpace(c.Query("q"))
	if len(q) < 3 {
		c.String(http.StatusOK, "")
		return
	}

	var installs []models.Install
	h.db.Where("org_id = ? AND name ILIKE ? AND (customer_account_id IS NULL OR customer_account_id != ?) AND deleted_at IS NULL",
		org.ID, "%"+q+"%", accountID).
		Limit(20).
		Find(&installs)

	if len(installs) == 0 {
		c.Data(http.StatusOK, "text/html", []byte(`<div class="px-4 py-3 text-sm text-cool-grey-500 dark:text-cool-grey-400">No installs found.</div>`))
		return
	}

	htmlOut := ""
	for _, inst := range installs {
		status := string(inst.Status)
		acctLabel := "unassigned"
		if inst.CustomerAccountID != nil {
			acctLabel = "other account"
		}
		htmlOut += fmt.Sprintf(`<div class="px-4 py-3 hover:bg-cool-grey-50 dark:hover:bg-dark-grey-800">
			<div class="flex items-center justify-between gap-3">
				<div>
					<div class="text-sm font-medium text-cool-grey-900 dark:text-white">%s</div>
					<div class="text-xs text-cool-grey-500 dark:text-cool-grey-400">%s &middot; %s &middot; %s</div>
				</div>
				<button
					type="button"
					data-install-id="%s"
					onclick="selectAccountInstall(this.dataset.installId, this.closest('.flex').querySelector('.text-sm.font-medium').textContent)"
					class="shrink-0 text-xs font-medium text-primary-600 dark:text-primary-400 hover:underline"
				>
					Select &rarr;
				</button>
			</div>
		</div>`,
			html.EscapeString(inst.Name), html.EscapeString(status), html.EscapeString(inst.Region), html.EscapeString(acctLabel),
			html.EscapeString(inst.ID),
		)
	}

	c.Data(http.StatusOK, "text/html", []byte(htmlOut))
}

// VendorInviteAccountMember invites a user by email to a customer account.
func (h *Handler) VendorInviteAccountMember(c *gin.Context) {
	user := h.GetFreshUser(c)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.String(http.StatusBadRequest, "Organization context not found")
		return
	}

	accountID := c.Param("account_id")
	var account models.CustomerAccount
	if err := h.db.Where("id = ? AND org_id = ? AND deleted_at IS NULL", accountID, org.ID).First(&account).Error; err != nil {
		c.Data(http.StatusNotFound, "text/html", []byte(`<span class="text-red-600">Account not found.</span>`))
		return
	}

	email := strings.TrimSpace(strings.ToLower(c.PostForm("email")))
	if email == "" {
		c.Data(http.StatusOK, "text/html", []byte(`<span class="text-sm text-red-600 dark:text-red-400">Please enter an email address.</span>`))
		return
	}

	// Check if already a member
	var existingMember models.CustomerAccountMember
	if err := h.db.Joins("JOIN users ON users.id = customer_account_members.user_id").
		Where("customer_account_members.account_id = ? AND users.email = ? AND customer_account_members.deleted_at IS NULL", account.ID, email).
		First(&existingMember).Error; err == nil {
		c.Data(http.StatusOK, "text/html", []byte(`<span class="text-sm text-amber-600 dark:text-amber-400">That user is already a member of this account.</span>`))
		return
	}

	// Check if there's already a pending invite
	var existingInvite models.CustomerAccountInvite
	if err := h.db.Where("account_id = ? AND email = ? AND used_by_user_id IS NULL AND deleted_at IS NULL", account.ID, email).
		First(&existingInvite).Error; err == nil {
		c.Data(http.StatusOK, "text/html", []byte(`<span class="text-sm text-amber-600 dark:text-amber-400">An invite for that email is already pending.</span>`))
		return
	}

	// Check if user exists — add as member immediately
	var existingUser models.User
	if err := h.db.Where("email = ? AND deleted_at IS NULL", email).First(&existingUser).Error; err == nil {
		member := &models.CustomerAccountMember{
			AccountID: account.ID,
			UserID:    existingUser.ID,
			OrgID:     account.OrgID,
			Role:      models.CustomerAccountRoleMember,
		}
		if err := h.db.Create(member).Error; err != nil {
			c.Data(http.StatusOK, "text/html", []byte(`<span class="text-sm text-red-600 dark:text-red-400">Failed to add member.</span>`))
			return
		}
		redirectURL := fmt.Sprintf("%s/orgs/%s/accounts/%s/members", h.basePath, org.ID, accountID)
		c.Header("HX-Redirect", redirectURL)
		c.Status(http.StatusOK)
		return
	}

	// User doesn't exist — create pending invite
	invite := &models.CustomerAccountInvite{
		AccountID:       account.ID,
		OrgID:           account.OrgID,
		Email:           email,
		CreatedByUserID: user.ID,
	}
	if err := h.db.Create(invite).Error; err != nil {
		c.Data(http.StatusOK, "text/html", []byte(`<span class="text-sm text-red-600 dark:text-red-400">Failed to create invite.</span>`))
		return
	}

	redirectURL := fmt.Sprintf("%s/orgs/%s/accounts/%s/members", h.basePath, org.ID, accountID)
	c.Header("HX-Redirect", redirectURL)
	c.Status(http.StatusOK)
}

// VendorDeleteAccountInvite revokes a pending account invite.
func (h *Handler) VendorDeleteAccountInvite(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.String(http.StatusBadRequest, "Organization context not found")
		return
	}

	accountID := c.Param("account_id")
	inviteID := c.Param("invite_id")

	result := h.db.Where("id = ? AND account_id = ? AND used_by_user_id IS NULL AND deleted_at IS NULL", inviteID, accountID).
		Delete(&models.CustomerAccountInvite{})
	if result.RowsAffected == 0 {
		c.Data(http.StatusNotFound, "text/html", []byte(`<span class="text-red-600">Invite not found.</span>`))
		return
	}

	redirectURL := fmt.Sprintf("%s/orgs/%s/accounts/%s/members", h.basePath, org.ID, accountID)
	c.Header("HX-Redirect", redirectURL)
	c.Status(http.StatusOK)
}

// AssignInstallToAccount assigns an existing install to a customer account.
func (h *Handler) AssignInstallToAccount(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.String(http.StatusBadRequest, "Organization context not found")
		return
	}

	accountID := c.Param("account_id")
	installID := c.PostForm("install_id")
	if installID == "" {
		c.Data(http.StatusBadRequest, "text/html", []byte(`<span class="text-red-600">Install ID is required.</span>`))
		return
	}

	// Verify account exists in this org
	var account models.CustomerAccount
	if err := h.db.Where("id = ? AND org_id = ? AND deleted_at IS NULL", accountID, org.ID).First(&account).Error; err != nil {
		c.Data(http.StatusNotFound, "text/html", []byte(`<span class="text-red-600">Account not found.</span>`))
		return
	}

	// Verify install exists in this org
	var install models.Install
	if err := h.db.Where("id = ? AND org_id = ? AND deleted_at IS NULL", installID, org.ID).First(&install).Error; err != nil {
		c.Data(http.StatusNotFound, "text/html", []byte(`<span class="text-red-600">Install not found.</span>`))
		return
	}

	// Update the install
	if err := h.db.Model(&install).Updates(map[string]interface{}{
		"customer_account_id": accountID,
		"visibility":          models.VisibilityAccount,
	}).Error; err != nil {
		c.Data(http.StatusInternalServerError, "text/html", []byte(`<span class="text-red-600">Failed to assign install.</span>`))
		return
	}

	redirectURL := fmt.Sprintf("%s/orgs/%s/accounts/%s/installs", h.basePath, org.ID, accountID)
	c.Header("HX-Redirect", redirectURL)
	c.Status(http.StatusOK)
}
