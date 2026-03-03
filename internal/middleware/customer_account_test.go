package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/testutil"
)

func TestRequireCustomerAccount_VendorPassesThrough(t *testing.T) {
	db := testutil.RequireTestDB(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("JWT_PAYLOAD", jwt.MapClaims{
			"user_id": "vendor-123",
			"email":   "vendor@test.com",
			"role":    string(models.RoleVendor),
			"name":    "Test Vendor",
		})
		c.Next()
	})
	router.Use(RequireCustomerAccount(db))
	router.GET("/installs", func(c *gin.Context) {
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/installs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestRequireCustomerAccount_ExemptPaths(t *testing.T) {
	db := testutil.RequireTestDB(t)
	gin.SetMode(gin.TestMode)

	exemptPaths := []string{
		"/account/setup",
		"/login",
		"/logout",
		"/auth/callback",
		"/install-link",
		"/custom/css/test",
	}

	for _, path := range exemptPaths {
		t.Run(path, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("JWT_PAYLOAD", jwt.MapClaims{
					"user_id": "customer-123",
					"email":   "customer@test.com",
					"role":    string(models.RoleCustomer),
					"name":    "Test Customer",
				})
				c.Next()
			})
			router.Use(RequireCustomerAccount(db))
			router.GET(path, func(c *gin.Context) {
				c.Status(200)
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", path, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, 200, w.Code)
		})
	}
}

func TestRequireCustomerAccount_CustomerWithoutAccount_Redirects(t *testing.T) {
	testutil.RequireTestDB(t)
	gin.SetMode(gin.TestMode)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		// Create an org with a subdomain
		vendor := testutil.NewTestVendor()
		tx.Create(vendor)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID, Subdomain: "testco"})
		tx.Create(org)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("subdomain", "testco")
			c.Set("JWT_PAYLOAD", jwt.MapClaims{
				"user_id": "customer-no-account",
				"email":   "customer@test.com",
				"role":    string(models.RoleCustomer),
				"name":    "Test Customer",
			})
			c.Next()
		})
		router.Use(RequireCustomerAccount(tx))
		router.GET("/installs", func(c *gin.Context) {
			c.Status(200)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/installs", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 302, w.Code)
		assert.Equal(t, "/installs", w.Header().Get("Location"))
	})
}

func TestRequireCustomerAccount_HTMXRedirect(t *testing.T) {
	testutil.RequireTestDB(t)
	gin.SetMode(gin.TestMode)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		vendor := testutil.NewTestVendor()
		tx.Create(vendor)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID, Subdomain: "testco"})
		tx.Create(org)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("subdomain", "testco")
			c.Set("JWT_PAYLOAD", jwt.MapClaims{
				"user_id": "customer-no-account",
				"email":   "customer@test.com",
				"role":    string(models.RoleCustomer),
				"name":    "Test Customer",
			})
			c.Next()
		})
		router.Use(RequireCustomerAccount(tx))
		router.GET("/installs", func(c *gin.Context) {
			c.Status(200)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/installs", nil)
		req.Header.Set("HX-Request", "true")
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "/installs", w.Header().Get("HX-Redirect"))
	})
}

func TestRequireCustomerAccount_InviteAutoJoin_WithExistingAccount(t *testing.T) {
	testutil.RequireTestDB(t)
	gin.SetMode(gin.TestMode)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		vendor := testutil.NewTestVendor()
		tx.Create(vendor)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID, Subdomain: "invite-existing"})
		tx.Create(org)

		customer := testutil.NewTestUser(testutil.UserOptions{ID: "customer-invite-existing", Email: "invited@test.com", Role: models.RoleCustomer})
		tx.Create(customer)

		// Existing account the user already owns
		existingAccount := &models.CustomerAccount{OrgID: org.ID, Name: "My Account", CreatedByUserID: customer.ID}
		tx.Create(existingAccount)
		tx.Create(&models.CustomerAccountMember{AccountID: existingAccount.ID, UserID: customer.ID, OrgID: org.ID, Role: models.CustomerAccountRoleOwner})

		// A different account with a pending invite for this user
		invitedAccount := &models.CustomerAccount{OrgID: org.ID, Name: "Team Account", CreatedByUserID: vendor.ID}
		tx.Create(invitedAccount)
		tx.Create(&models.CustomerAccountInvite{
			AccountID: invitedAccount.ID,
			OrgID:     org.ID,
			Email:     customer.Email,
		})

		var gotAccounts []models.CustomerAccountMember

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("subdomain", "invite-existing")
			c.Set("JWT_PAYLOAD", jwt.MapClaims{
				"user_id": customer.ID,
				"email":   customer.Email,
				"role":    string(models.RoleCustomer),
				"name":    customer.Name,
			})
			c.Next()
		})
		router.Use(RequireCustomerAccount(tx))
		router.GET("/installs", func(c *gin.Context) {
			gotAccounts = GetCustomerAccounts(c)
			c.Status(200)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/installs", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.Len(t, gotAccounts, 2, "should have both existing and invited account memberships")

		// Verify invite was consumed
		var invite models.CustomerAccountInvite
		tx.Where("email = ? AND org_id = ?", customer.Email, org.ID).First(&invite)
		assert.NotNil(t, invite.UsedByUserID, "invite should be marked as used")
	})
}

func TestRequireCustomerAccount_MultipleAccounts_DefaultsToFirst(t *testing.T) {
	testutil.RequireTestDB(t)
	gin.SetMode(gin.TestMode)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		vendor := testutil.NewTestVendor()
		tx.Create(vendor)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID, Subdomain: "multi"})
		tx.Create(org)

		customer := testutil.NewTestUser(testutil.UserOptions{ID: "customer-multi", Email: "multi@test.com", Role: models.RoleCustomer})
		tx.Create(customer)

		account1 := &models.CustomerAccount{OrgID: org.ID, Name: "Account One", CreatedByUserID: customer.ID}
		tx.Create(account1)
		account2 := &models.CustomerAccount{OrgID: org.ID, Name: "Account Two", CreatedByUserID: customer.ID}
		tx.Create(account2)

		tx.Create(&models.CustomerAccountMember{AccountID: account1.ID, UserID: customer.ID, OrgID: org.ID, Role: models.CustomerAccountRoleOwner})
		tx.Create(&models.CustomerAccountMember{AccountID: account2.ID, UserID: customer.ID, OrgID: org.ID, Role: models.CustomerAccountRoleMember})

		var gotAccount *models.CustomerAccount
		var gotAccounts []models.CustomerAccountMember

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("subdomain", "multi")
			c.Set("JWT_PAYLOAD", jwt.MapClaims{
				"user_id": customer.ID,
				"email":   customer.Email,
				"role":    string(models.RoleCustomer),
				"name":    customer.Name,
			})
			c.Next()
		})
		router.Use(RequireCustomerAccount(tx))
		router.GET("/installs", func(c *gin.Context) {
			gotAccount = GetCustomerAccount(c)
			gotAccounts = GetCustomerAccounts(c)
			c.Status(200)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/installs", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.NotNil(t, gotAccount)
		assert.Equal(t, account1.ID, gotAccount.ID, "should default to first account")
		assert.Len(t, gotAccounts, 2, "should have all memberships in context")
	})
}

func TestRequireCustomerAccount_MultipleAccounts_CookieSelectsAccount(t *testing.T) {
	testutil.RequireTestDB(t)
	gin.SetMode(gin.TestMode)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		vendor := testutil.NewTestVendor()
		tx.Create(vendor)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID, Subdomain: "multi2"})
		tx.Create(org)

		customer := testutil.NewTestUser(testutil.UserOptions{ID: "customer-multi2", Email: "multi2@test.com", Role: models.RoleCustomer})
		tx.Create(customer)

		account1 := &models.CustomerAccount{OrgID: org.ID, Name: "Account A", CreatedByUserID: customer.ID}
		tx.Create(account1)
		account2 := &models.CustomerAccount{OrgID: org.ID, Name: "Account B", CreatedByUserID: customer.ID}
		tx.Create(account2)

		tx.Create(&models.CustomerAccountMember{AccountID: account1.ID, UserID: customer.ID, OrgID: org.ID, Role: models.CustomerAccountRoleOwner})
		tx.Create(&models.CustomerAccountMember{AccountID: account2.ID, UserID: customer.ID, OrgID: org.ID, Role: models.CustomerAccountRoleMember})

		var gotAccount *models.CustomerAccount

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("subdomain", "multi2")
			c.Set("JWT_PAYLOAD", jwt.MapClaims{
				"user_id": customer.ID,
				"email":   customer.Email,
				"role":    string(models.RoleCustomer),
				"name":    customer.Name,
			})
			c.Next()
		})
		router.Use(RequireCustomerAccount(tx))
		router.GET("/installs", func(c *gin.Context) {
			gotAccount = GetCustomerAccount(c)
			c.Status(200)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/installs", nil)
		// Set cookie to select account2
		req.AddCookie(&http.Cookie{Name: "active_account_id", Value: account2.ID})
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.NotNil(t, gotAccount)
		assert.Equal(t, account2.ID, gotAccount.ID, "should select account from cookie")
	})
}
