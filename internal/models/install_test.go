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
