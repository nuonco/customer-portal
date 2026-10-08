package jsonhandlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/internal/testutil"
)

func TestCustomerPortalCreateAccount_CreatesAndSetsActiveCookie(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		customer := testutil.NewTestCustomer()
		require.NoError(t, tx.Create(customer).Error)

		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: customer.ID, Name: "Acme", Subdomain: "acme"})
		require.NoError(t, tx.Create(org).Error)

		h := NewCustomerPortalHandler(tx, "http://localhost:8080", "localhost:8080", "http://localhost:8081")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/portal-api/accounts?subdomain=acme", strings.NewReader(`{"name":"Platform Team"}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("JWT_PAYLOAD", jwt.MapClaims{
			"user_id": customer.ID,
			"name":    customer.Name,
			"email":   customer.Email,
			"role":    string(customer.Role),
		})

		h.CreateAccount(c)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Header().Get("Set-Cookie"), "active_account_id=")

		var response createCustomerAccountResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, "Platform Team", response.Account.Name)

		var member models.CustomerAccountMember
		require.NoError(t, tx.Where("user_id = ? AND org_id = ?", customer.ID, org.ID).First(&member).Error)
		assert.Equal(t, response.Account.ID, member.AccountID)
	})
}

func TestCustomerPortalSwitchAccount_RequiresMembership(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		customer := testutil.NewTestCustomer()
		require.NoError(t, tx.Create(customer).Error)

		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: customer.ID, Name: "Acme", Subdomain: "acme"})
		require.NoError(t, tx.Create(org).Error)

		account := testutil.NewTestCustomerAccount(testutil.CustomerAccountOptions{
			OrgID:           org.ID,
			Name:            "Other Team",
			CreatedByUserID: customer.ID,
		})
		require.NoError(t, tx.Create(account).Error)

		h := NewCustomerPortalHandler(tx, "http://localhost:8080", "localhost:8080", "http://localhost:8081")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/portal-api/accounts/switch?subdomain=acme", strings.NewReader(`{"account_id":"`+account.ID+`"}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("JWT_PAYLOAD", jwt.MapClaims{
			"user_id": customer.ID,
			"name":    customer.Name,
			"email":   customer.Email,
			"role":    string(customer.Role),
		})

		h.SwitchAccount(c)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
