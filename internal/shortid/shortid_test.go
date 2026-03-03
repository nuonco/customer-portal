package shortid

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{
			name:   "user prefix",
			prefix: "iur",
		},
		{
			name:   "org prefix",
			prefix: "ino",
		},
		{
			name:   "install prefix",
			prefix: "isi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := New(tt.prefix)

			// Should be 26 characters total
			assert.Len(t, id, 26, "ID should be 26 characters")

			// Should start with the prefix
			assert.True(t, len(id) >= len(tt.prefix), "ID should be longer than prefix")
			assert.Equal(t, tt.prefix, id[:len(tt.prefix)], "ID should start with prefix")

			// Should only contain base36 characters after prefix
			alphanumeric := regexp.MustCompile(`^[0-9a-z]+$`)
			assert.True(t, alphanumeric.MatchString(id[len(tt.prefix):]),
				"ID suffix should be base36 (lowercase alphanumeric)")
		})
	}
}

func TestNew_Uniqueness(t *testing.T) {
	// Generate multiple IDs and verify they're unique
	ids := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		id := New("tst")
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}

	assert.Len(t, ids, count, "All generated IDs should be unique")
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected bool
	}{
		{
			name:     "valid 26 char ID",
			id:       "iur12345678901234567890123",
			expected: true,
		},
		{
			name:     "generated user ID",
			id:       NewUserID(),
			expected: true,
		},
		{
			name:     "generated org ID",
			id:       NewNuonOrgID(),
			expected: true,
		},
		{
			name:     "too short",
			id:       "iur123",
			expected: false,
		},
		{
			name:     "too long",
			id:       "iur1234567890123456789012345",
			expected: false,
		},
		{
			name:     "empty string",
			id:       "",
			expected: false,
		},
		{
			name:     "exactly 25 chars",
			id:       "iur1234567890123456789012",
			expected: false,
		},
		{
			name:     "exactly 27 chars",
			id:       "iur123456789012345678901234",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValid(tt.id)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewUserID(t *testing.T) {
	id := NewUserID()

	assert.Len(t, id, 26)
	assert.Equal(t, "iur", id[:3], "User ID should have 'iur' prefix")
	assert.True(t, IsValid(id))
}

func TestNewNuonOrgID(t *testing.T) {
	id := NewNuonOrgID()

	assert.Len(t, id, 26)
	assert.Equal(t, "ino", id[:3], "Org ID should have 'ino' prefix")
	assert.True(t, IsValid(id))
}

func TestNewInstallLinkID(t *testing.T) {
	id := NewInstallLinkID()

	assert.Len(t, id, 26)
	assert.Equal(t, "ilk", id[:3], "Install link ID should have 'ilk' prefix")
	assert.True(t, IsValid(id))
}

func TestNewInstallID(t *testing.T) {
	id := NewInstallID()

	assert.Len(t, id, 26)
	assert.Equal(t, "isi", id[:3], "Install ID should have 'isi' prefix")
	assert.True(t, IsValid(id))
}

func TestNewThemeID(t *testing.T) {
	id := NewThemeID()

	assert.Len(t, id, 26)
	assert.Equal(t, "ith", id[:3], "Theme ID should have 'ith' prefix")
	assert.True(t, IsValid(id))
}

func TestNewHealthCheckConfigID(t *testing.T) {
	id := NewHealthCheckConfigID()

	assert.Len(t, id, 26)
	assert.Equal(t, "ihc", id[:3], "Health check config ID should have 'ihc' prefix")
	assert.True(t, IsValid(id))
}

func TestNewCustomerAuthConfigID(t *testing.T) {
	id := NewCustomerAuthConfigID()

	assert.Len(t, id, 26)
	assert.Equal(t, "cac", id[:3], "Customer auth config ID should have 'cac' prefix")
	assert.True(t, IsValid(id))
}

func TestNewAppInputConfigID(t *testing.T) {
	id := NewAppInputConfigID()

	assert.Len(t, id, 26)
	assert.Equal(t, "aic", id[:3], "App input config ID should have 'aic' prefix")
	assert.True(t, IsValid(id))
}

func TestNewOrgMemberID(t *testing.T) {
	id := NewOrgMemberID()

	assert.Len(t, id, 26)
	assert.Equal(t, "ogm", id[:3], "Org member ID should have 'ogm' prefix")
	assert.True(t, IsValid(id))
}

func TestNewOrgInvitationID(t *testing.T) {
	id := NewOrgInvitationID()

	assert.Len(t, id, 26)
	assert.Equal(t, "ogi", id[:3], "Org invitation ID should have 'ogi' prefix")
	assert.True(t, IsValid(id))
}

func TestNewGitHubRepoConfigID(t *testing.T) {
	id := NewGitHubRepoConfigID()

	assert.Len(t, id, 26)
	assert.Equal(t, "ghc", id[:3], "GitHub repo config ID should have 'ghc' prefix")
	assert.True(t, IsValid(id))
}

func TestNewTemplateOverrideID(t *testing.T) {
	id := NewTemplateOverrideID()

	assert.Len(t, id, 26)
	assert.Equal(t, "tov", id[:3], "Template override ID should have 'tov' prefix")
	assert.True(t, IsValid(id))
}

func TestNewAssetOverrideID(t *testing.T) {
	id := NewAssetOverrideID()

	assert.Len(t, id, 26)
	assert.Equal(t, "aov", id[:3], "Asset override ID should have 'aov' prefix")
	assert.True(t, IsValid(id))
}

func TestNewCustomerAccountID(t *testing.T) {
	id := NewCustomerAccountID()

	assert.Len(t, id, 26)
	assert.Equal(t, "cat", id[:3], "Customer account ID should have 'cat' prefix")
	assert.True(t, IsValid(id))
}

func TestNewCustomerAccountMemberID(t *testing.T) {
	id := NewCustomerAccountMemberID()

	assert.Len(t, id, 26)
	assert.Equal(t, "cam", id[:3], "Customer account member ID should have 'cam' prefix")
	assert.True(t, IsValid(id))
}

func TestNewCustomerAccountInviteID(t *testing.T) {
	id := NewCustomerAccountInviteID()

	assert.Len(t, id, 26)
	assert.Equal(t, "cai", id[:3], "Customer account invite ID should have 'cai' prefix")
	assert.True(t, IsValid(id))
}

func TestAllIDPrefixes_Unique(t *testing.T) {
	// Verify all prefixes are unique to prevent ID collisions
	prefixes := map[string]string{
		"iur": "user",
		"ino": "nuon_org",
		"ilk": "install_link",
		"isi": "install",
		"ith": "theme",
		"ihc": "health_check_config",
		"cac": "customer_auth_config",
		"aic": "app_input_config",
		"ogm": "org_member",
		"ogi": "org_invitation",
		"ghc": "github_repo_config",
		"tov": "template_override",
		"aov": "asset_override",
		"pap": "published_app",
		"cat": "customer_account",
		"cam": "customer_account_member",
		"cai": "customer_account_invite",
	}

	// Verify we have the expected number of unique prefixes
	assert.Len(t, prefixes, 17, "Should have 17 unique entity prefixes")

	// Verify each prefix is exactly 3 characters
	for prefix := range prefixes {
		assert.Len(t, prefix, 3, "All prefixes should be 3 characters")
	}
}

// BenchmarkNew measures ID generation performance
func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		New("tst")
	}
}

// BenchmarkIsValid measures validation performance
func BenchmarkIsValid(b *testing.B) {
	id := NewUserID()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsValid(id)
	}
}
