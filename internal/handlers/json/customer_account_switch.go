package jsonhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

type switchCustomerAccountRequest struct {
	AccountID string `json:"account_id" binding:"required"`
}

// SwitchAccount switches the active customer account (group) for the current user.
func (h *CustomerPortalHandler) SwitchAccount(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req switchCustomerAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Account ID is required"})
		return
	}

	org, err := h.getOrgForCustomerPortal(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	var member models.CustomerAccountMember
	if err := h.db.Where(
		"user_id = ? AND org_id = ? AND account_id = ? AND deleted_at IS NULL",
		user.ID,
		org.ID,
		req.AccountID,
	).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this account"})
		return
	}

	c.SetCookie("active_account_id", req.AccountID, 60*60*24*365, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
