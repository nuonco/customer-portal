package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/nuonco/customer-portal/internal/models"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailRequired      = errors.New("email is required")
	ErrPasswordRequired   = errors.New("password is required")
	ErrEmailExists        = errors.New("email already registered")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
)

// LocalProvider implements password-based authentication
type LocalProvider struct {
	db *gorm.DB
}

// NewLocalProvider creates a new local authentication provider
func NewLocalProvider(db *gorm.DB) *LocalProvider {
	return &LocalProvider{db: db}
}

// Name returns the provider identifier
func (p *LocalProvider) Name() string {
	return "local"
}

// GetAuthorizationURL returns empty for local auth (no external redirect)
func (p *LocalProvider) GetAuthorizationURL(state string) (string, error) {
	return "", nil
}

// HandleCallback is not used for local auth
func (p *LocalProvider) HandleCallback(ctx context.Context, req CallbackRequest) (*AuthResult, error) {
	return nil, errors.New("local auth does not use callbacks")
}

// GetLogoutURL returns empty for local auth (no external logout)
func (p *LocalProvider) GetLogoutURL(sessionID string) (string, error) {
	return "", nil
}

// SupportsLogout returns false for local auth
func (p *LocalProvider) SupportsLogout() bool {
	return false
}

// Login authenticates a user with email and password
func (p *LocalProvider) Login(email, password string) (*AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, ErrEmailRequired
	}
	if password == "" {
		return nil, ErrPasswordRequired
	}

	var user models.User
	if err := p.db.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !user.CheckPassword(password) {
		return nil, ErrInvalidCredentials
	}

	return &AuthResult{
		User:      &user,
		SessionID: user.ID,
	}, nil
}

// Register creates a new user with email and password (defaults to vendor role)
func (p *LocalProvider) Register(email, password, name string) (*AuthResult, error) {
	return p.RegisterWithRole(email, password, name, models.RoleVendor)
}

// RegisterWithRole creates a new user with email, password, and specified role
func (p *LocalProvider) RegisterWithRole(email, password, name string, role models.UserRole) (*AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)

	if email == "" {
		return nil, ErrEmailRequired
	}
	if password == "" {
		return nil, ErrPasswordRequired
	}
	if len(password) < 8 {
		return nil, ErrPasswordTooShort
	}

	// Check if email already exists
	var existing models.User
	if err := p.db.Where("email = ?", email).First(&existing).Error; err == nil {
		return nil, ErrEmailExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Create new user
	user := models.User{
		Email: email,
		Name:  name,
		Role:  role,
	}

	if err := user.SetPassword(password); err != nil {
		return nil, err
	}

	if err := p.db.Create(&user).Error; err != nil {
		return nil, err
	}

	return &AuthResult{
		User:      &user,
		SessionID: user.ID,
	}, nil
}
