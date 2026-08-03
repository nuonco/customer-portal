package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSafeSQLIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		expected   bool
	}{
		{name: "simple", identifier: "fk_app_themes_org", expected: true},
		{name: "starts with underscore", identifier: "_constraint_1", expected: true},
		{name: "contains numbers", identifier: "constraint123", expected: true},
		{name: "empty", identifier: "", expected: false},
		{name: "starts with number", identifier: "1constraint", expected: false},
		{name: "contains hyphen", identifier: "fk-app-themes-org", expected: false},
		{name: "contains quote", identifier: `bad\"name`, expected: false},
		{name: "contains whitespace", identifier: "bad name", expected: false},
		{name: "contains semicolon", identifier: "bad;drop", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isSafeSQLIdentifier(tt.identifier))
		})
	}
}
