package jsonhandlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
)

type createCustomerAccountRequest struct {
	Name string `json:"name" binding:"required"`
}

type createCustomerAccountResponse struct {
	Account customerPortalAccountResponse `json:"account"`
}

// CreateAccount creates a new customer account (group) and sets it active.
func (h *CustomerPortalHandler) CreateAccount(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req createCustomerAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group name is required"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group name is required"})
		return
	}

	org, err := h.getOrgForCustomerPortal(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	account, err := h.createAccountForUser(user.ID, org.ID, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}

	// Keep compatibility with legacy account switching behavior.
	c.SetCookie("active_account_id", account.ID, 60*60*24*365, "/", "", false, true)

	c.JSON(http.StatusCreated, createCustomerAccountResponse{
		Account: customerPortalAccountResponse{
			ID:   account.ID,
			Name: account.Name,
		},
	})
}

func (h *CustomerPortalHandler) createAccountForUser(userID, orgID, accountName string) (*models.CustomerAccount, error) {
	tx := h.db.Begin()

	account := &models.CustomerAccount{
		OrgID:           orgID,
		Name:            accountName,
		CreatedByUserID: userID,
	}
	if err := tx.Create(account).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	member := &models.CustomerAccountMember{
		AccountID: account.ID,
		UserID:    userID,
		OrgID:     orgID,
		Role:      models.CustomerAccountRoleOwner,
	}
	if err := tx.Create(member).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Associate any orphaned installs with the newly created account.
	tx.Model(&models.Install{}).
		Where("user_id = ? AND org_id = ? AND customer_account_id IS NULL AND deleted_at IS NULL", userID, orgID).
		Updates(map[string]interface{}{
			"customer_account_id": account.ID,
			"visibility":          models.VisibilityAccount,
		})

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return account, nil
}
