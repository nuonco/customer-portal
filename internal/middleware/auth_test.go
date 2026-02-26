package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/testutil"
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
		region TEXT,
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
