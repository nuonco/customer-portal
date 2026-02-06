package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

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

	return db
}

func TestLocalProvider_Name(t *testing.T) {
	db := setupTestDB(t)
	provider := NewLocalProvider(db)

	assert.Equal(t, "local", provider.Name())
}

func TestLocalProvider_Login(t *testing.T) {
	db := setupTestDB(t)
	provider := NewLocalProvider(db)

	// Create a test user with password
	user := &models.User{
		Email: "test@example.com",
		Name:  "Test User",
		Role:  models.RoleVendor,
	}
	require.NoError(t, user.SetPassword("correctpassword"))
	require.NoError(t, db.Create(user).Error)

	tests := []struct {
		name       string
		email      string
		password   string
		expectErr  error
		expectUser bool
	}{
		{
			name:       "valid credentials",
			email:      "test@example.com",
			password:   "correctpassword",
			expectErr:  nil,
			expectUser: true,
		},
		{
			name:       "wrong password",
			email:      "test@example.com",
			password:   "wrongpassword",
			expectErr:  ErrInvalidCredentials,
			expectUser: false,
		},
		{
			name:       "user not found",
			email:      "nonexistent@example.com",
			password:   "anypassword",
			expectErr:  ErrInvalidCredentials,
			expectUser: false,
		},
		{
			name:       "empty email",
			email:      "",
			password:   "password",
			expectErr:  ErrEmailRequired,
			expectUser: false,
		},
		{
			name:       "empty password",
			email:      "test@example.com",
			password:   "",
			expectErr:  ErrPasswordRequired,
			expectUser: false,
		},
		{
			name:       "email with whitespace trimmed",
			email:      "  test@example.com  ",
			password:   "correctpassword",
			expectErr:  nil,
			expectUser: true,
		},
		{
			name:       "email case insensitive",
			email:      "TEST@EXAMPLE.COM",
			password:   "correctpassword",
			expectErr:  nil,
			expectUser: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Login(tt.email, tt.password)

			if tt.expectErr != nil {
				assert.ErrorIs(t, err, tt.expectErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.NotNil(t, result.User)
				assert.Equal(t, "test@example.com", result.User.Email)
				assert.NotEmpty(t, result.SessionID)
			}
		})
	}
}

func TestLocalProvider_Register(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		password  string
		userName  string
		setupUser bool // Create existing user with same email
		expectErr error
	}{
		{
			name:      "valid registration",
			email:     "newuser@example.com",
			password:  "password123",
			userName:  "New User",
			expectErr: nil,
		},
		{
			name:      "empty email",
			email:     "",
			password:  "password123",
			userName:  "User",
			expectErr: ErrEmailRequired,
		},
		{
			name:      "empty password",
			email:     "user@example.com",
			password:  "",
			userName:  "User",
			expectErr: ErrPasswordRequired,
		},
		{
			name:      "password too short",
			email:     "user@example.com",
			password:  "short",
			userName:  "User",
			expectErr: ErrPasswordTooShort,
		},
		{
			name:      "exactly 8 chars is valid",
			email:     "user8@example.com",
			password:  "12345678",
			userName:  "User",
			expectErr: nil,
		},
		{
			name:      "email already exists",
			email:     "existing@example.com",
			password:  "password123",
			userName:  "Existing User",
			setupUser: true,
			expectErr: ErrEmailExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			provider := NewLocalProvider(db)

			if tt.setupUser {
				existing := &models.User{
					Email: tt.email,
					Name:  "Existing",
					Role:  models.RoleVendor,
				}
				require.NoError(t, db.Create(existing).Error)
			}

			result, err := provider.Register(tt.email, tt.password, tt.userName)

			if tt.expectErr != nil {
				assert.ErrorIs(t, err, tt.expectErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.NotNil(t, result.User)
				assert.Equal(t, models.RoleVendor, result.User.Role, "Default role should be vendor")
				assert.NotEmpty(t, result.SessionID)

				// Verify user was saved to database
				var savedUser models.User
				err := db.Where("email = ?", result.User.Email).First(&savedUser).Error
				assert.NoError(t, err)
				assert.True(t, savedUser.HasPassword())
			}
		})
	}
}

func TestLocalProvider_RegisterWithRole(t *testing.T) {
	db := setupTestDB(t)
	provider := NewLocalProvider(db)

	// Test registering as customer
	result, err := provider.RegisterWithRole("customer@example.com", "password123", "Customer", models.RoleCustomer)

	assert.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, models.RoleCustomer, result.User.Role)

	// Test registering as vendor
	result, err = provider.RegisterWithRole("vendor@example.com", "password123", "Vendor", models.RoleVendor)

	assert.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, models.RoleVendor, result.User.Role)
}

func TestLocalProvider_GetAuthorizationURL(t *testing.T) {
	db := setupTestDB(t)
	provider := NewLocalProvider(db)

	url, err := provider.GetAuthorizationURL("state123")

	// Local auth doesn't use external authorization
	assert.NoError(t, err)
	assert.Empty(t, url)
}

func TestLocalProvider_HandleCallback(t *testing.T) {
	db := setupTestDB(t)
	provider := NewLocalProvider(db)

	result, err := provider.HandleCallback(nil, CallbackRequest{})

	// Local auth doesn't use callbacks
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestLocalProvider_GetLogoutURL(t *testing.T) {
	db := setupTestDB(t)
	provider := NewLocalProvider(db)

	url, err := provider.GetLogoutURL("session123")

	// Local auth doesn't have external logout
	assert.NoError(t, err)
	assert.Empty(t, url)
}

func TestLocalProvider_SupportsLogout(t *testing.T) {
	db := setupTestDB(t)
	provider := NewLocalProvider(db)

	assert.False(t, provider.SupportsLogout())
}

func TestLocalProvider_Login_UserWithoutPassword(t *testing.T) {
	db := setupTestDB(t)
	provider := NewLocalProvider(db)

	// Create a user without password (e.g., OIDC-only user)
	user := &models.User{
		Email: "oidcuser@example.com",
		Name:  "OIDC User",
		Role:  models.RoleVendor,
	}
	require.NoError(t, db.Create(user).Error)

	// Try to login with any password
	result, err := provider.Login("oidcuser@example.com", "anypassword")

	assert.ErrorIs(t, err, ErrInvalidCredentials)
	assert.Nil(t, result)
}
