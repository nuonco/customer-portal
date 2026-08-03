package jsonhandlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
)

type accountWithCounts struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MemberCount  int64  `json:"member_count"`
	InstallCount int64  `json:"install_count"`
	CreatedAt    string `json:"created_at"`
}

// Accounts returns the list of customer accounts for an org as JSON.
func (h *VendorHandler) Accounts(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	searchQuery := strings.TrimSpace(c.Query("q"))

	type accountResult struct {
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

	var results []accountResult
	if err := query.Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user groups"})
		return
	}

	accounts := make([]accountWithCounts, len(results))
	for i, r := range results {
		accounts[i] = accountWithCounts{
			ID:           r.ID,
			Name:         r.Name,
			MemberCount:  r.MemberCount,
			InstallCount: r.InstallCount,
			CreatedAt:    r.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"accounts": accounts,
		"org_id":   org.ID,
	})
}
