package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNuonOrg_HasMembers(t *testing.T) {
	tests := []struct {
		name     string
		members  []OrgMember
		expected bool
	}{
		{
			name:     "no members",
			members:  nil,
			expected: false,
		},
		{
			name:     "empty members slice",
			members:  []OrgMember{},
			expected: false,
		},
		{
			name: "has members",
			members: []OrgMember{
				{ID: "member-1"},
			},
			expected: true,
		},
		{
			name: "multiple members",
			members: []OrgMember{
				{ID: "member-1"},
				{ID: "member-2"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := &NuonOrg{Members: tt.members}
			assert.Equal(t, tt.expected, org.HasMembers())
		})
	}
}
