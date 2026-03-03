package handlers

import (
	"fmt"
	"net/http"

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

	if c.GetHeader("HX-Request") == "true" {
		h.RenderTempl(c, http.StatusOK, vendorpages.AccountsTableBody(accounts, org.ID, h.basePath, searchQuery))
		return
	}

	allOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.AccountsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:            org.Name + " - Accounts",
			ActivePage:       "customers",
			User:             user,
			CurrentOrg:       org,
			Orgs:             allOrgs,
			Breadcrumbs:      []partials.Breadcrumb{{Text: "Accounts", Path: fmt.Sprintf("%s/orgs/%s/accounts", h.basePath, org.ID), Active: true}},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
		},
		Org:         *org,
		Accounts:    accounts,
		SearchQuery: searchQuery,
	}
	h.RenderTempl(c, http.StatusOK, vendorpages.AccountsPage(props))
}

// AccountDetailPage renders the detail view of a customer account for vendors.
func (h *Handler) AccountDetailPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}

	accountID := c.Param("account_id")

	var account models.CustomerAccount
	if err := h.db.Preload("CreatedBy").Where("id = ? AND org_id = ? AND deleted_at IS NULL", accountID, org.ID).First(&account).Error; err != nil {
		h.RenderErrorPage(c, http.StatusNotFound, "Account not found")
		return
	}

	var members []models.CustomerAccountMember
	h.db.Preload("User").Where("account_id = ? AND deleted_at IS NULL", accountID).Order("joined_at ASC").Find(&members)

	var installs []models.Install
	h.db.Where("customer_account_id = ? AND deleted_at IS NULL", accountID).Order("created_at DESC").Find(&installs)

	allOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.AccountDetailPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      org.Name + " - " + account.Name,
			ActivePage: "customers",
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
		},
		Org:      *org,
		Account:  account,
		Members:  members,
		Installs: installs,
	}
	h.RenderTempl(c, http.StatusOK, vendorpages.AccountDetailPage(props))
}
