package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUser_SetPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "securePassword123",
			wantErr:  false,
		},
		{
			name:     "short password",
			password: "short",
			wantErr:  false, // bcrypt accepts short passwords
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  false, // bcrypt accepts empty (not recommended)
		},
		{
			name:     "unicode password",
			password: "密码パスワード🔐",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{}
			err := user.SetPassword(tt.password)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, user.PasswordHash)
				assert.NotEqual(t, tt.password, user.PasswordHash, "password should be hashed")
			}
		})
	}
}

func TestUser_CheckPassword(t *testing.T) {
	tests := []struct {
		name           string
		storedPassword string
		checkPassword  string
		expected       bool
	}{
		{
			name:           "correct password",
			storedPassword: "correctPassword",
			checkPassword:  "correctPassword",
			expected:       true,
		},
		{
			name:           "incorrect password",
			storedPassword: "correctPassword",
			checkPassword:  "wrongPassword",
			expected:       false,
		},
		{
			name:           "empty stored password",
			storedPassword: "",
			checkPassword:  "anyPassword",
			expected:       false, // Special case: no password set
		},
		{
			name:           "case sensitive",
			storedPassword: "CaseSensitive",
			checkPassword:  "casesensitive",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{}

			if tt.storedPassword != "" {
				err := user.SetPassword(tt.storedPassword)
				require.NoError(t, err)
			}

			result := user.CheckPassword(tt.checkPassword)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUser_HasPassword(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*User)
		expected bool
	}{
		{
			name:     "no password set",
			setup:    func(u *User) {},
			expected: false,
		},
		{
			name: "password set",
			setup: func(u *User) {
				u.SetPassword("password123")
			},
			expected: true,
		},
		{
			name: "empty password hash",
			setup: func(u *User) {
				u.PasswordHash = ""
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{}
			tt.setup(user)

			result := user.HasPassword()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUser_IsVendor(t *testing.T) {
	tests := []struct {
		name     string
		role     UserRole
		expected bool
	}{
		{
			name:     "vendor role",
			role:     RoleVendor,
			expected: true,
		},
		{
			name:     "customer role",
			role:     RoleCustomer,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{Role: tt.role}
			assert.Equal(t, tt.expected, user.IsVendor())
		})
	}
}

func TestUser_IsCustomer(t *testing.T) {
	tests := []struct {
		name     string
		role     UserRole
		expected bool
	}{
		{
			name:     "customer role",
			role:     RoleCustomer,
			expected: true,
		},
		{
			name:     "vendor role",
			role:     RoleVendor,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{Role: tt.role}
			assert.Equal(t, tt.expected, user.IsCustomer())
		})
	}
}

func TestUserRole_Constants(t *testing.T) {
	// Verify role constants have expected values
	assert.Equal(t, UserRole("vendor"), RoleVendor)
	assert.Equal(t, UserRole("customer"), RoleCustomer)
}
