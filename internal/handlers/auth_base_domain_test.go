package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/testutil"
)

const testJWTSecret = "test-secret-key-for-unit-tests"

// newTestJWTMiddleware creates a minimal JWT middleware for testing token generation/parsing.
func newTestJWTMiddleware() *jwt.GinJWTMiddleware {
	mw, _ := jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "test",
		Key:         []byte(testJWTSecret),
		Timeout:     time.Hour,
		IdentityKey: "user_id",
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if user, ok := data.(*models.User); ok {
				return jwt.MapClaims{
					"user_id": user.ID,
					"email":   user.Email,
					"role":    string(user.Role),
				}
			}
			return jwt.MapClaims{}
		},
	})
	return mw
}

// generateTestToken creates a signed JWT for the given user using the test middleware.
func generateTestToken(mw *jwt.GinJWTMiddleware, user *models.User) string {
	token, _, _ := mw.TokenGenerator(user)
	return token
}

func TestCompleteSubdomainAuth_AutoCreatesAccountForNewCustomer(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		// Create vendor + org
		_, org, err := testutil.SeedMinimalData(tx)
		require.NoError(t, err)

		// Create a customer user with a name (no account membership)
		customer := testutil.NewTestCustomer(testutil.UserOptions{Name: "Alice Smith"})
		require.NoError(t, tx.Create(customer).Error)

		jwtMW := newTestJWTMiddleware()
		logger := zap.NewNop()
		h := NewHandler(tx, jwtMW, nil, "http://localhost:8080", "https://api.nuon.co", "", "", "localhost:8080", logger)

		token := generateTestToken(jwtMW, customer)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/auth/complete?token="+token, nil)
		c.Set("subdomain", org.Subdomain)

		h.CompleteSubdomainAuth(c)

		// Should redirect to /installs (not /account/setup)
		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/installs", w.Header().Get("Location"))

		// Verify account was auto-created
		var account models.CustomerAccount
		err = tx.Where("org_id = ? AND created_by_user_id = ?", org.ID, customer.ID).First(&account).Error
		require.NoError(t, err)
		assert.Equal(t, "Alice's Account", account.Name)

		// Verify membership was created
		var member models.CustomerAccountMember
		err = tx.Where("user_id = ? AND account_id = ?", customer.ID, account.ID).First(&member).Error
		require.NoError(t, err)
		assert.Equal(t, models.CustomerAccountRoleOwner, member.Role)
	})
}

func TestCompleteSubdomainAuth_AutoCreatesAccountFallbackName(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		_, org, err := testutil.SeedMinimalData(tx)
		require.NoError(t, err)

		// Create customer with no name
		customer := testutil.NewTestCustomer(testutil.UserOptions{Name: ""})
		require.NoError(t, tx.Create(customer).Error)

		jwtMW := newTestJWTMiddleware()
		logger := zap.NewNop()
		h := NewHandler(tx, jwtMW, nil, "http://localhost:8080", "https://api.nuon.co", "", "", "localhost:8080", logger)

		token := generateTestToken(jwtMW, customer)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/auth/complete?token="+token, nil)
		c.Set("subdomain", org.Subdomain)

		h.CompleteSubdomainAuth(c)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/installs", w.Header().Get("Location"))

		// Verify fallback name
		var account models.CustomerAccount
		err = tx.Where("org_id = ? AND created_by_user_id = ?", org.ID, customer.ID).First(&account).Error
		require.NoError(t, err)
		assert.Equal(t, "My Account", account.Name)
	})
}

func TestCompleteSubdomainAuth_RedirectsExistingCustomerToInstalls(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		_, org, err := testutil.SeedMinimalData(tx)
		require.NoError(t, err)

		customer := testutil.NewTestCustomer()
		require.NoError(t, tx.Create(customer).Error)

		// Create account + membership
		account := testutil.NewTestCustomerAccount(testutil.CustomerAccountOptions{
			OrgID:           org.ID,
			CreatedByUserID: customer.ID,
		})
		require.NoError(t, tx.Create(account).Error)

		member := testutil.NewTestCustomerAccountMember(testutil.CustomerAccountMemberOptions{
			AccountID: account.ID,
			UserID:    customer.ID,
			OrgID:     org.ID,
			Role:      models.CustomerAccountRoleOwner,
		})
		require.NoError(t, tx.Create(member).Error)

		jwtMW := newTestJWTMiddleware()
		logger := zap.NewNop()
		h := NewHandler(tx, jwtMW, nil, "http://localhost:8080", "https://api.nuon.co", "", "", "localhost:8080", logger)

		token := generateTestToken(jwtMW, customer)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/auth/complete?token="+token, nil)
		c.Set("subdomain", org.Subdomain)

		h.CompleteSubdomainAuth(c)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/installs", w.Header().Get("Location"))
	})
}

func TestCompleteSubdomainAuth_VendorSkipsAccountCheck(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		vendor, org, err := testutil.SeedMinimalData(tx)
		require.NoError(t, err)

		jwtMW := newTestJWTMiddleware()
		logger := zap.NewNop()
		h := NewHandler(tx, jwtMW, nil, "http://localhost:8080", "https://api.nuon.co", "", "", "localhost:8080", logger)

		token := generateTestToken(jwtMW, vendor)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/auth/complete?token="+token, nil)
		c.Set("subdomain", org.Subdomain)

		h.CompleteSubdomainAuth(c)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/installs", w.Header().Get("Location"))
	})
}
