// Package testutil provides testing utilities including factory functions
// for creating test model instances with valid default values.
package testutil

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// UserOptions allows customization of test user creation.
type UserOptions struct {
	ID       string
	Email    string
	Name     string
	Role     models.UserRole
	Password string
}

// NewTestUser creates a test user with sensible defaults.
// Options can be provided to override default values.
func NewTestUser(opts ...UserOptions) *models.User {
	user := &models.User{
		ID:        RandomUserID(),
		Email:     RandomEmail(),
		Name:      "Test User",
		Role:      models.RoleCustomer,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if len(opts) > 0 {
		opt := opts[0]
		if opt.ID != "" {
			user.ID = opt.ID
		}
		if opt.Email != "" {
			user.Email = opt.Email
		}
		if opt.Name != "" {
			user.Name = opt.Name
		}
		if opt.Role != "" {
			user.Role = opt.Role
		}
		if opt.Password != "" {
			if err := user.SetPassword(opt.Password); err != nil {
				panic("failed to set password: " + err.Error())
			}
		}
	}

	return user
}

// NewTestVendor creates a test user with vendor role.
func NewTestVendor(opts ...UserOptions) *models.User {
	if len(opts) == 0 {
		opts = []UserOptions{{}}
	}
	opts[0].Role = models.RoleVendor
	if opts[0].Name == "" {
		opts[0].Name = "Test Vendor"
	}
	return NewTestUser(opts...)
}

// NewTestCustomer creates a test user with customer role.
func NewTestCustomer(opts ...UserOptions) *models.User {
	if len(opts) == 0 {
		opts = []UserOptions{{}}
	}
	opts[0].Role = models.RoleCustomer
	if opts[0].Name == "" {
		opts[0].Name = "Test Customer"
	}
	return NewTestUser(opts...)
}

// OrgOptions allows customization of test org creation.
type OrgOptions struct {
	ID        string
	UserID    string
	NuonOrgID string
	APIToken  string
	Name      string
	Subdomain string
}

// NewTestOrg creates a test organization with sensible defaults.
func NewTestOrg(opts ...OrgOptions) *models.NuonOrg {
	org := &models.NuonOrg{
		ID:        RandomOrgID(),
		UserID:    RandomUserID(),
		NuonOrgID: "nuon-" + RandomString(12),
		APIToken:  "test-token-" + RandomString(16),
		Name:      "Test Organization",
		Subdomain: "test-" + RandomString(6),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if len(opts) > 0 {
		opt := opts[0]
		if opt.ID != "" {
			org.ID = opt.ID
		}
		if opt.UserID != "" {
			org.UserID = opt.UserID
		}
		if opt.NuonOrgID != "" {
			org.NuonOrgID = opt.NuonOrgID
		}
		if opt.APIToken != "" {
			org.APIToken = opt.APIToken
		}
		if opt.Name != "" {
			org.Name = opt.Name
		}
		if opt.Subdomain != "" {
			org.Subdomain = opt.Subdomain
		}
	}

	return org
}

// InstallOptions allows customization of test install creation.
type InstallOptions struct {
	ID                string
	OrgID             string
	UserID            string
	CreatedByVendorID string
	InstallLinkID     string
	NuonInstallID     string
	Name              string
	Status            models.InstallStatus
	Region            string
}

// NewTestInstall creates a test install with sensible defaults.
func NewTestInstall(opts ...InstallOptions) *models.Install {
	install := &models.Install{
		ID:                RandomInstallID(),
		OrgID:             RandomOrgID(),
		UserID:            RandomUserID(),
		CreatedByVendorID: RandomUserID(),
		InstallLinkID:     RandomInstallLinkID(),
		NuonInstallID:     "nuon-install-" + RandomString(12),
		Name:              "Test Install",
		Status:            models.StatusPending,
		Region:            "us-west-2",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if len(opts) > 0 {
		opt := opts[0]
		if opt.ID != "" {
			install.ID = opt.ID
		}
		if opt.OrgID != "" {
			install.OrgID = opt.OrgID
		}
		if opt.UserID != "" {
			install.UserID = opt.UserID
		}
		if opt.CreatedByVendorID != "" {
			install.CreatedByVendorID = opt.CreatedByVendorID
		}
		if opt.InstallLinkID != "" {
			install.InstallLinkID = opt.InstallLinkID
		}
		if opt.NuonInstallID != "" {
			install.NuonInstallID = opt.NuonInstallID
		}
		if opt.Name != "" {
			install.Name = opt.Name
		}
		if opt.Status != "" {
			install.Status = opt.Status
		}
		if opt.Region != "" {
			install.Region = opt.Region
		}
	}

	return install
}

// InstallLinkOptions allows customization of test install link creation.
type InstallLinkOptions struct {
	ID      string
	OrgID   string
	UserID  string
	AppID   string
	AppName string
	SHA     string
	Used    bool
}

// NewTestInstallLink creates a test install link with sensible defaults.
func NewTestInstallLink(opts ...InstallLinkOptions) *models.InstallLink {
	link := &models.InstallLink{
		ID:        RandomInstallLinkID(),
		OrgID:     RandomOrgID(),
		UserID:    RandomUserID(),
		AppID:     "app-" + RandomString(12),
		AppName:   "Test App",
		SHA:       RandomString(32),
		Used:      false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if len(opts) > 0 {
		opt := opts[0]
		if opt.ID != "" {
			link.ID = opt.ID
		}
		if opt.OrgID != "" {
			link.OrgID = opt.OrgID
		}
		if opt.UserID != "" {
			link.UserID = opt.UserID
		}
		if opt.AppID != "" {
			link.AppID = opt.AppID
		}
		if opt.AppName != "" {
			link.AppName = opt.AppName
		}
		if opt.SHA != "" {
			link.SHA = opt.SHA
		}
		if opt.Used {
			link.Used = true
		}
	}

	return link
}

// OrgMemberOptions allows customization of test org member creation.
type OrgMemberOptions struct {
	ID        string
	OrgID     string
	UserID    string
	InvitedBy *string
	Status    models.OrgMemberStatus
}

// NewTestOrgMember creates a test organization membership with sensible defaults.
func NewTestOrgMember(opts ...OrgMemberOptions) *models.OrgMember {
	member := &models.OrgMember{
		ID:        RandomID("ogm"),
		OrgID:     RandomOrgID(),
		UserID:    RandomUserID(),
		Status:    models.MemberStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if len(opts) > 0 {
		opt := opts[0]
		if opt.ID != "" {
			member.ID = opt.ID
		}
		if opt.OrgID != "" {
			member.OrgID = opt.OrgID
		}
		if opt.UserID != "" {
			member.UserID = opt.UserID
		}
		if opt.InvitedBy != nil {
			member.InvitedBy = opt.InvitedBy
		}
		if opt.Status != "" {
			member.Status = opt.Status
		}
	}

	return member
}

// ThemeOptions allows customization of test theme creation.
type ThemeOptions struct {
	ID             string
	OrgID          string
	PrimaryColor   string
	SecondaryColor string
	SupportContact string
}

// NewTestTheme creates a test app theme with sensible defaults.
func NewTestTheme(opts ...ThemeOptions) *models.AppTheme {
	theme := &models.AppTheme{
		ID:           RandomID("ith"),
		OrgID:        RandomOrgID(),
		PrimaryColor: "#2563EB",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if len(opts) > 0 {
		opt := opts[0]
		if opt.ID != "" {
			theme.ID = opt.ID
		}
		if opt.OrgID != "" {
			theme.OrgID = opt.OrgID
		}
		if opt.PrimaryColor != "" {
			theme.PrimaryColor = opt.PrimaryColor
		}
		if opt.SecondaryColor != "" {
			theme.SecondaryColor = opt.SecondaryColor
		}
		if opt.SupportContact != "" {
			theme.SupportContact = opt.SupportContact
		}
	}

	return theme
}

// CustomerAuthConfigOptions allows customization of test customer auth config.
type CustomerAuthConfigOptions struct {
	ID           string
	OrgID        string
	ProviderName string
	IssuerURL    string
	ClientID     string
	ClientSecret string
	Scopes       string
	Enabled      bool
}

// NewTestCustomerAuthConfig creates a test customer auth config with sensible defaults.
func NewTestCustomerAuthConfig(opts ...CustomerAuthConfigOptions) *models.CustomerAuthConfig {
	config := &models.CustomerAuthConfig{
		ID:           RandomID("cac"),
		OrgID:        RandomOrgID(),
		ProviderName: "Test Provider",
		IssuerURL:    "https://auth.example.com",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Scopes:       "openid,profile,email",
		Enabled:      false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if len(opts) > 0 {
		opt := opts[0]
		if opt.ID != "" {
			config.ID = opt.ID
		}
		if opt.OrgID != "" {
			config.OrgID = opt.OrgID
		}
		if opt.ProviderName != "" {
			config.ProviderName = opt.ProviderName
		}
		if opt.IssuerURL != "" {
			config.IssuerURL = opt.IssuerURL
		}
		if opt.ClientID != "" {
			config.ClientID = opt.ClientID
		}
		if opt.ClientSecret != "" {
			config.ClientSecret = opt.ClientSecret
		}
		if opt.Scopes != "" {
			config.Scopes = opt.Scopes
		}
		if opt.Enabled {
			config.Enabled = true
		}
	}

	return config
}

// CreateUserWithOrg creates a user and an org where the user is a member.
// Returns the user, org, and membership for testing scenarios requiring relationships.
func CreateUserWithOrg(userOpts UserOptions, orgOpts OrgOptions) (*models.User, *models.NuonOrg, *models.OrgMember) {
	user := NewTestUser(userOpts)

	// Link org to user
	if orgOpts.UserID == "" {
		orgOpts.UserID = user.ID
	}
	org := NewTestOrg(orgOpts)

	// Create membership
	member := NewTestOrgMember(OrgMemberOptions{
		UserID: user.ID,
		OrgID:  org.ID,
		Status: models.MemberStatusActive,
	})

	return user, org, member
}

// CreateInstallWithRelationships creates an install with all its related entities.
// Returns the install, install link, org, vendor user, and customer user.
func CreateInstallWithRelationships() (*models.Install, *models.InstallLink, *models.NuonOrg, *models.User, *models.User) {
	vendor := NewTestVendor()
	org := NewTestOrg(OrgOptions{UserID: vendor.ID})

	customer := NewTestCustomer()

	link := NewTestInstallLink(InstallLinkOptions{
		OrgID:  org.ID,
		UserID: vendor.ID,
	})

	install := NewTestInstall(InstallOptions{
		OrgID:             org.ID,
		UserID:            customer.ID,
		CreatedByVendorID: vendor.ID,
		InstallLinkID:     link.ID,
	})

	// Set up relationships
	install.Org = *org
	install.User = *customer
	install.CreatedByVendor = *vendor
	install.InstallLink = *link
	link.NuonOrg = *org

	return install, link, org, vendor, customer
}
