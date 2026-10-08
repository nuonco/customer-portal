package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/internal/testutil"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupTestDB creates an in-memory SQLite database for testing.
// Note: SQLite doesn't support char_length, so we disable foreign keys and constraints.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	// Create tables manually without check constraints (SQLite doesn't support char_length)
	err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		name TEXT,
		email TEXT UNIQUE,
		password_hash TEXT,
		role TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error
	require.NoError(t, err)

	err = db.Exec(`CREATE TABLE IF NOT EXISTS installs (
		id TEXT PRIMARY KEY,
		org_id TEXT,
		user_id TEXT NOT NULL,
		created_by_vendor_id TEXT NOT NULL,
		install_link_id TEXT,
		nuon_app_id TEXT DEFAULT '',
		nuon_install_id TEXT NOT NULL,
		name TEXT DEFAULT '',
		app_id TEXT DEFAULT '',
		app_name TEXT DEFAULT '',
		status TEXT DEFAULT 'pending_customer',
		visibility TEXT DEFAULT 'account',
		customer_account_id TEXT,
		region TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error
	require.NoError(t, err)

	err = db.Exec(`CREATE TABLE IF NOT EXISTS customer_account_members (
		id TEXT PRIMARY KEY,
		account_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		org_id TEXT NOT NULL,
		role TEXT DEFAULT 'member',
		joined_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error
	require.NoError(t, err)

	err = db.Exec(`CREATE TABLE IF NOT EXISTS nuon_orgs (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		org_id TEXT NOT NULL,
		api_token TEXT NOT NULL,
		name TEXT NOT NULL,
		subdomain TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error
	require.NoError(t, err)

	err = db.Exec(`CREATE TABLE IF NOT EXISTS org_members (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		invited_by TEXT,
		status TEXT DEFAULT 'active',
		invited_at DATETIME,
		joined_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error
	require.NoError(t, err)

	err = db.Exec(`CREATE TABLE IF NOT EXISTS org_invitations (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		email TEXT,
		invited_by TEXT NOT NULL,
		token TEXT NOT NULL UNIQUE,
		expires_at DATETIME,
		accepted_at DATETIME,
		used_count INTEGER DEFAULT 0,
		max_uses INTEGER DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error
	require.NoError(t, err)

	return db
}

// setJWTClaims sets JWT claims on the Gin context.
func setJWTClaims(c *gin.Context, claims map[string]interface{}) {
	c.Set("JWT_PAYLOAD", jwt.MapClaims(claims))
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name           string
		userRole       models.UserRole
		requiredRole   models.UserRole
		expectedStatus int
		shouldProceed  bool
	}{
		{
			name:           "vendor can access vendor route",
			userRole:       models.RoleVendor,
			requiredRole:   models.RoleVendor,
			expectedStatus: http.StatusOK,
			shouldProceed:  true,
		},
		{
			name:           "vendor can access customer route (super user)",
			userRole:       models.RoleVendor,
			requiredRole:   models.RoleCustomer,
			expectedStatus: http.StatusOK,
			shouldProceed:  true,
		},
		{
			name:           "customer can access customer route",
			userRole:       models.RoleCustomer,
			requiredRole:   models.RoleCustomer,
			expectedStatus: http.StatusOK,
			shouldProceed:  true,
		},
		{
			name:           "customer cannot access vendor route",
			userRole:       models.RoleCustomer,
			requiredRole:   models.RoleVendor,
			expectedStatus: http.StatusForbidden,
			shouldProceed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

			// Set up JWT claims
			setJWTClaims(c, map[string]interface{}{
				"user_id": "test-user-id",
				"email":   "test@example.com",
				"role":    string(tt.userRole),
			})

			// Track if handler was called
			handlerCalled := false
			c.Set("handler_called", &handlerCalled)

			// Create test handler chain
			middleware := RequireRole(tt.requiredRole)
			middleware(c)

			if !c.IsAborted() {
				handlerCalled = true
			}

			assert.Equal(t, tt.shouldProceed, handlerCalled, "handler called mismatch")
			if !tt.shouldProceed {
				assert.Equal(t, tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestRequireInstallOwnership(t *testing.T) {
	db := setupTestDB(t)

	// Create test user and install
	user := testutil.NewTestCustomer()
	require.NoError(t, db.Create(user).Error)

	install := testutil.NewTestInstall(testutil.InstallOptions{
		UserID: user.ID,
	})
	require.NoError(t, db.Create(install).Error)

	tests := []struct {
		name           string
		userID         string
		installID      string
		expectedStatus int
		shouldProceed  bool
	}{
		{
			name:           "valid ownership succeeds",
			userID:         user.ID,
			installID:      install.ID,
			expectedStatus: http.StatusOK,
			shouldProceed:  true,
		},
		{
			name:           "wrong user returns 404",
			userID:         "different-user-id",
			installID:      install.ID,
			expectedStatus: http.StatusNotFound,
			shouldProceed:  false,
		},
		{
			name:           "invalid ID format returns 400",
			userID:         user.ID,
			installID:      "invalid",
			expectedStatus: http.StatusBadRequest,
			shouldProceed:  false,
		},
		{
			name:           "empty ID returns 400",
			userID:         user.ID,
			installID:      "",
			expectedStatus: http.StatusBadRequest,
			shouldProceed:  false,
		},
		{
			name:           "non-existent install returns 404",
			userID:         user.ID,
			installID:      testutil.RandomInstallID(),
			expectedStatus: http.StatusNotFound,
			shouldProceed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

			// Set up JWT claims
			setJWTClaims(c, map[string]interface{}{
				"user_id": tt.userID,
				"email":   "test@example.com",
				"role":    string(models.RoleCustomer),
			})

			// Set install_id param
			c.Params = []gin.Param{{Key: "install_id", Value: tt.installID}}

			// Run middleware
			handlerCalled := false
			middleware := RequireInstallOwnership(db)
			middleware(c)

			if !c.IsAborted() {
				handlerCalled = true
			}

			assert.Equal(t, tt.shouldProceed, handlerCalled, "handler called mismatch")
			if !tt.shouldProceed {
				assert.Equal(t, tt.expectedStatus, w.Code)
			}

			// If successful, verify install is in context
			if tt.shouldProceed {
				ctxInstall, exists := c.Get("install")
				assert.True(t, exists, "install should be in context")
				assert.NotNil(t, ctxInstall)
			}
		})
	}
}

func TestRequireOrgAccessByParam(t *testing.T) {
	db := setupTestDB(t)

	// Create test user and org
	user := testutil.NewTestVendor()
	require.NoError(t, db.Create(user).Error)

	org := testutil.NewTestOrg(testutil.OrgOptions{UserID: user.ID})
	require.NoError(t, db.Create(org).Error)

	member := testutil.NewTestOrgMember(testutil.OrgMemberOptions{
		UserID: user.ID,
		OrgID:  org.ID,
		Status: models.MemberStatusActive,
	})
	require.NoError(t, db.Create(member).Error)

	tests := []struct {
		name           string
		paramOrgID     string
		contextOrgID   string // Already in context from RequireOrgContext
		expectedStatus int
		shouldProceed  bool
	}{
		{
			name:           "matching org_id succeeds",
			paramOrgID:     org.ID,
			contextOrgID:   org.ID,
			expectedStatus: http.StatusOK,
			shouldProceed:  true,
		},
		{
			name:           "member can access different org they belong to",
			paramOrgID:     org.ID,
			contextOrgID:   "", // No current context
			expectedStatus: http.StatusOK,
			shouldProceed:  true,
		},
		{
			name:           "non-member returns 404",
			paramOrgID:     testutil.RandomOrgID(),
			contextOrgID:   org.ID,
			expectedStatus: http.StatusNotFound,
			shouldProceed:  false,
		},
		{
			name:           "invalid org_id format returns 400",
			paramOrgID:     "invalid",
			contextOrgID:   org.ID,
			expectedStatus: http.StatusBadRequest,
			shouldProceed:  false,
		},
		{
			name:           "empty org_id returns 400",
			paramOrgID:     "",
			contextOrgID:   org.ID,
			expectedStatus: http.StatusBadRequest,
			shouldProceed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
			c.Request.Header.Set("Accept", "application/json")

			// Set up JWT claims
			setJWTClaims(c, map[string]interface{}{
				"user_id": user.ID,
				"email":   "test@example.com",
				"role":    string(models.RoleVendor),
			})

			// Set current org context if provided
			if tt.contextOrgID != "" {
				c.Set("org", org)
			}

			// Set org_id param
			c.Params = []gin.Param{{Key: "org_id", Value: tt.paramOrgID}}

			// Run middleware
			handlerCalled := false
			middleware := RequireOrgAccessByParam(db)
			middleware(c)

			if !c.IsAborted() {
				handlerCalled = true
			}

			assert.Equal(t, tt.shouldProceed, handlerCalled, "handler called mismatch")
			if !tt.shouldProceed {
				assert.Equal(t, tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestGetCurrentUser(t *testing.T) {
	tests := []struct {
		name     string
		claims   map[string]interface{}
		expected *models.User
	}{
		{
			name: "extracts all claims correctly",
			claims: map[string]interface{}{
				"user_id": "user-123",
				"name":    "John Doe",
				"email":   "john@example.com",
				"role":    "vendor",
			},
			expected: &models.User{
				ID:    "user-123",
				Name:  "John Doe",
				Email: "john@example.com",
				Role:  models.RoleVendor,
			},
		},
		{
			name: "handles missing name gracefully",
			claims: map[string]interface{}{
				"user_id": "user-456",
				"email":   "jane@example.com",
				"role":    "customer",
			},
			expected: &models.User{
				ID:    "user-456",
				Name:  "", // Missing name returns empty string
				Email: "jane@example.com",
				Role:  models.RoleCustomer,
			},
		},
		{
			name: "handles nil name gracefully",
			claims: map[string]interface{}{
				"user_id": "user-789",
				"name":    nil,
				"email":   "test@example.com",
				"role":    "customer",
			},
			expected: &models.User{
				ID:    "user-789",
				Name:  "",
				Email: "test@example.com",
				Role:  models.RoleCustomer,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

			setJWTClaims(c, tt.claims)

			user := GetCurrentUser(c)

			assert.Equal(t, tt.expected.ID, user.ID)
			assert.Equal(t, tt.expected.Name, user.Name)
			assert.Equal(t, tt.expected.Email, user.Email)
			assert.Equal(t, tt.expected.Role, user.Role)
		})
	}
}

func TestGetCurrentOrg(t *testing.T) {
	tests := []struct {
		name     string
		orgSet   bool
		org      *models.NuonOrg
		expected *models.NuonOrg
	}{
		{
			name:   "returns org from context",
			orgSet: true,
			org: &models.NuonOrg{
				ID:   "org-123",
				Name: "Test Org",
			},
			expected: &models.NuonOrg{
				ID:   "org-123",
				Name: "Test Org",
			},
		},
		{
			name:     "returns nil when not set",
			orgSet:   false,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			if tt.orgSet {
				c.Set("org", tt.org)
			}

			org := GetCurrentOrg(c)

			if tt.expected == nil {
				assert.Nil(t, org)
			} else {
				require.NotNil(t, org)
				assert.Equal(t, tt.expected.ID, org.ID)
				assert.Equal(t, tt.expected.Name, org.Name)
			}
		})
	}
}

func TestGetStringClaim(t *testing.T) {
	tests := []struct {
		name     string
		claims   jwt.MapClaims
		key      string
		expected string
	}{
		{
			name:     "returns string value",
			claims:   jwt.MapClaims{"name": "John"},
			key:      "name",
			expected: "John",
		},
		{
			name:     "returns empty for missing key",
			claims:   jwt.MapClaims{"other": "value"},
			key:      "name",
			expected: "",
		},
		{
			name:     "returns empty for nil value",
			claims:   jwt.MapClaims{"name": nil},
			key:      "name",
			expected: "",
		},
		{
			name:     "returns empty for non-string value",
			claims:   jwt.MapClaims{"count": 42},
			key:      "count",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStringClaim(tt.claims, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessPendingOrgInvites(t *testing.T) {
	db := setupTestDB(t)

	// Create test vendor and org
	vendor := testutil.NewTestVendor()
	require.NoError(t, db.Create(vendor).Error)

	org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID})
	require.NoError(t, db.Create(org).Error)

	// Create a new vendor user who will be invited
	invitee := testutil.NewTestVendor(testutil.UserOptions{Email: "invitee@example.com"})
	require.NoError(t, db.Create(invitee).Error)

	t.Run("auto-joins org with pending invite", func(t *testing.T) {
		// Create pending invite
		invite := models.OrgInvitation{
			OrgID:     org.ID,
			Email:     invitee.Email,
			InvitedBy: vendor.ID,
			Token:     "test-token-1",
			MaxUses:   1,
		}
		require.NoError(t, db.Create(&invite).Error)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

		setJWTClaims(c, map[string]interface{}{
			"user_id": invitee.ID,
			"email":   invitee.Email,
			"role":    string(models.RoleVendor),
			"name":    invitee.Name,
		})

		mw := ProcessPendingOrgInvites(db)
		mw(c)

		assert.False(t, c.IsAborted())

		// Verify member was created
		var member models.OrgMember
		err := db.Where("org_id = ? AND user_id = ?", org.ID, invitee.ID).First(&member).Error
		require.NoError(t, err)
		assert.Equal(t, models.MemberStatusActive, member.Status)

		// Verify invite was marked accepted
		var updatedInvite models.OrgInvitation
		require.NoError(t, db.First(&updatedInvite, "id = ?", invite.ID).Error)
		assert.NotNil(t, updatedInvite.AcceptedAt)
		assert.Equal(t, 1, updatedInvite.UsedCount)

		// Cleanup
		db.Where("org_id = ? AND user_id = ?", org.ID, invitee.ID).Delete(&models.OrgMember{})
		db.Where("id = ?", invite.ID).Unscoped().Delete(&models.OrgInvitation{})
	})

	t.Run("skips already-accepted invites", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

		setJWTClaims(c, map[string]interface{}{
			"user_id": invitee.ID,
			"email":   invitee.Email,
			"role":    string(models.RoleVendor),
			"name":    invitee.Name,
		})

		// No pending invites — should be a no-op
		mw := ProcessPendingOrgInvites(db)
		mw(c)

		assert.False(t, c.IsAborted())

		// No member should have been created
		var count int64
		db.Model(&models.OrgMember{}).Where("org_id = ? AND user_id = ?", org.ID, invitee.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("upgrades customer role to vendor on auto-join", func(t *testing.T) {
		// Create a customer user
		customer := testutil.NewTestCustomer()
		require.NoError(t, db.Create(customer).Error)

		// Create pending invite for the customer
		invite := models.OrgInvitation{
			OrgID:     org.ID,
			Email:     customer.Email,
			InvitedBy: vendor.ID,
			Token:     "test-token-upgrade",
			MaxUses:   1,
		}
		require.NoError(t, db.Create(&invite).Error)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

		setJWTClaims(c, map[string]interface{}{
			"user_id": customer.ID,
			"email":   customer.Email,
			"role":    string(models.RoleCustomer),
			"name":    customer.Name,
		})

		mw := ProcessPendingOrgInvites(db)
		mw(c)

		assert.False(t, c.IsAborted())

		// Verify member was created
		var member models.OrgMember
		err := db.Where("org_id = ? AND user_id = ?", org.ID, customer.ID).First(&member).Error
		require.NoError(t, err)
		assert.Equal(t, models.MemberStatusActive, member.Status)

		// Verify user role was upgraded in DB
		var updatedUser models.User
		require.NoError(t, db.First(&updatedUser, "id = ?", customer.ID).Error)
		assert.Equal(t, models.RoleVendor, updatedUser.Role)

		// Verify JWT claims were updated
		claims, exists := c.Get("JWT_PAYLOAD")
		require.True(t, exists)
		mc, ok := claims.(jwt.MapClaims)
		require.True(t, ok)
		assert.Equal(t, string(models.RoleVendor), mc["role"])

		// Cleanup
		db.Where("org_id = ? AND user_id = ?", org.ID, customer.ID).Delete(&models.OrgMember{})
		db.Where("id = ?", invite.ID).Unscoped().Delete(&models.OrgInvitation{})
		db.Where("id = ?", customer.ID).Unscoped().Delete(&models.User{})
	})

	t.Run("restores soft-deleted member on re-invite", func(t *testing.T) {
		// Create a user who was previously a member
		reinvitee := testutil.NewTestVendor(testutil.UserOptions{Email: "reinvitee@example.com"})
		require.NoError(t, db.Create(reinvitee).Error)

		// Create and then soft-delete a member
		now := time.Now()
		member := models.OrgMember{
			OrgID:    org.ID,
			UserID:   reinvitee.ID,
			Status:   models.MemberStatusActive,
			JoinedAt: &now,
		}
		require.NoError(t, db.Create(&member).Error)
		require.NoError(t, db.Delete(&member).Error)

		// Verify it's soft-deleted
		var count int64
		db.Model(&models.OrgMember{}).Where("org_id = ? AND user_id = ?", org.ID, reinvitee.ID).Count(&count)
		assert.Equal(t, int64(0), count)

		// Create pending invite for the removed user
		invite := models.OrgInvitation{
			OrgID:     org.ID,
			Email:     reinvitee.Email,
			InvitedBy: vendor.ID,
			Token:     "test-token-reinvite",
			MaxUses:   1,
		}
		require.NoError(t, db.Create(&invite).Error)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

		setJWTClaims(c, map[string]interface{}{
			"user_id": reinvitee.ID,
			"email":   reinvitee.Email,
			"role":    string(models.RoleVendor),
			"name":    reinvitee.Name,
		})

		mw := ProcessPendingOrgInvites(db)
		mw(c)

		assert.False(t, c.IsAborted())

		// Verify member was restored (not duplicated)
		var restored models.OrgMember
		err := db.Where("org_id = ? AND user_id = ?", org.ID, reinvitee.ID).First(&restored).Error
		require.NoError(t, err)
		assert.Equal(t, models.MemberStatusActive, restored.Status)

		// Verify only one record exists (no duplicates)
		var totalCount int64
		db.Unscoped().Model(&models.OrgMember{}).Where("org_id = ? AND user_id = ?", org.ID, reinvitee.ID).Count(&totalCount)
		assert.Equal(t, int64(1), totalCount)

		// Cleanup
		db.Where("org_id = ? AND user_id = ?", org.ID, reinvitee.ID).Unscoped().Delete(&models.OrgMember{})
		db.Where("id = ?", invite.ID).Unscoped().Delete(&models.OrgInvitation{})
		db.Where("id = ?", reinvitee.ID).Unscoped().Delete(&models.User{})
	})

	t.Run("does not block request on error", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

		setJWTClaims(c, map[string]interface{}{
			"user_id": invitee.ID,
			"email":   invitee.Email,
			"role":    string(models.RoleVendor),
			"name":    invitee.Name,
		})

		mw := ProcessPendingOrgInvites(db)
		mw(c)

		// Should always proceed
		assert.False(t, c.IsAborted())
	})
}
