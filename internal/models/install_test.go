package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstallStatus_Constants(t *testing.T) {
	// Verify status constants have expected values
	tests := []struct {
		name     string
		status   InstallStatus
		expected string
	}{
		{
			name:     "pending customer",
			status:   StatusPendingCustomer,
			expected: "pending_customer",
		},
		{
			name:     "pending",
			status:   StatusPending,
			expected: "pending",
		},
		{
			name:     "provisioning",
			status:   StatusProvisioning,
			expected: "provisioning",
		},
		{
			name:     "active",
			status:   StatusActive,
			expected: "active",
		},
		{
			name:     "failed",
			status:   StatusFailed,
			expected: "failed",
		},
		{
			name:     "deprovisioning",
			status:   StatusDeprovisioning,
			expected: "deprovisioning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, InstallStatus(tt.expected), tt.status)
		})
	}
}

func TestInstall_GetNuonOrg(t *testing.T) {
	linkID := "ilktest123456789012345"
	orgFromLink := NuonOrg{ID: "org-from-link", APIToken: "token-link", NuonOrgID: "nuon-link"}
	orgDirect := NuonOrg{ID: "org-direct", APIToken: "token-direct", NuonOrgID: "nuon-direct"}

	tests := []struct {
		name    string
		install Install
		wantID  string
	}{
		{
			name: "returns install link org when link is set",
			install: Install{
				InstallLinkID: &linkID,
				InstallLink:   InstallLink{NuonOrg: orgFromLink},
				Org:           orgDirect,
			},
			wantID: orgFromLink.ID,
		},
		{
			name: "returns direct org when install link ID is nil",
			install: Install{
				InstallLinkID: nil,
				Org:           orgDirect,
			},
			wantID: orgDirect.ID,
		},
		{
			name: "returns direct org when install link org is empty",
			install: Install{
				InstallLinkID: &linkID,
				InstallLink:   InstallLink{},
				Org:           orgDirect,
			},
			wantID: orgDirect.ID,
		},
		{
			name: "returns nil when no org available",
			install: Install{
				InstallLinkID: nil,
			},
			wantID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.install.GetNuonOrg()
			if tt.wantID == "" {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantID, result.ID)
			}
		})
	}
}

func TestInstall_GetAppID(t *testing.T) {
	linkID := "ilktest123456789012345"

	tests := []struct {
		name    string
		install Install
		want    string
	}{
		{
			name: "install-link install returns InstallLink.AppID",
			install: Install{
				InstallLinkID: &linkID,
				InstallLink:   InstallLink{AppID: "app-from-link"},
				NuonAppID:     "app-from-nuon",
				AppID:         "app-from-import",
			},
			want: "app-from-link",
		},
		{
			name: "published-app install returns NuonAppID",
			install: Install{
				InstallLinkID: nil,
				NuonAppID:     "app-from-nuon",
				AppID:         "",
			},
			want: "app-from-nuon",
		},
		{
			name: "legacy imported install falls back to AppID",
			install: Install{
				InstallLinkID: nil,
				NuonAppID:     "",
				AppID:         "app-from-import",
			},
			want: "app-from-import",
		},
		{
			name: "empty install returns empty string",
			install: Install{
				InstallLinkID: nil,
				NuonAppID:     "",
				AppID:         "",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.install.GetAppID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInstallStatus_Transitions(t *testing.T) {
	// Document the valid status transitions
	// This helps ensure status flow is understood
	validTransitions := map[InstallStatus][]InstallStatus{
		StatusPendingCustomer: {StatusPending, StatusFailed},
		StatusPending:         {StatusProvisioning, StatusFailed},
		StatusProvisioning:    {StatusActive, StatusFailed},
		StatusActive:          {StatusDeprovisioning, StatusFailed},
		StatusDeprovisioning:  {StatusFailed}, // Terminal for success (delete)
		StatusFailed:          {},             // Terminal state
	}

	for from, toStatuses := range validTransitions {
		t.Run(string(from), func(t *testing.T) {
			// This is primarily documentation, but validates the map structure
			assert.NotNil(t, toStatuses, "transitions should be defined")
		})
	}
}
