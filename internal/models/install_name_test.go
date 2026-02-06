package models

import (
	"testing"
)

func TestNormalizeInstallName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no change needed",
			input:    "Production",
			expected: "Production",
		},
		{
			name:     "trim leading whitespace",
			input:    "  Production",
			expected: "Production",
		},
		{
			name:     "trim trailing whitespace",
			input:    "Production  ",
			expected: "Production",
		},
		{
			name:     "trim both sides",
			input:    "  Production  ",
			expected: "Production",
		},
		{
			name:     "preserve internal spaces",
			input:    "Acme Production Server",
			expected: "Acme Production Server",
		},
		{
			name:     "preserve case",
			input:    "ACME-Production",
			expected: "ACME-Production",
		},
		{
			name:     "preserve underscores",
			input:    "my_test_install",
			expected: "my_test_install",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeInstallName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeInstallName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateInstallName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid simple name",
			input:       "Production",
			expectError: false,
		},
		{
			name:        "valid with spaces",
			input:       "Acme Production",
			expectError: false,
		},
		{
			name:        "valid with dashes",
			input:       "acme-production",
			expectError: false,
		},
		{
			name:        "valid with underscores",
			input:       "acme_production",
			expectError: false,
		},
		{
			name:        "valid with numbers",
			input:       "production-001",
			expectError: false,
		},
		{
			name:        "valid mixed case",
			input:       "ACME Production 123",
			expectError: false,
		},
		{
			name:        "valid complex name",
			input:       "Acme Corp - Production Server 1",
			expectError: false,
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
			errorMsg:    "install name cannot be empty",
		},
		{
			name:        "too long (over 255 chars)",
			input:       string(make([]byte, 256)),
			expectError: true,
			errorMsg:    "install name cannot exceed 255 characters",
		},
		{
			name:        "invalid special characters",
			input:       "Production!@#$",
			expectError: true,
			errorMsg:    "install name can only contain letters, numbers, spaces, dashes, and underscores",
		},
		{
			name:        "invalid with dots",
			input:       "production.acme.com",
			expectError: true,
			errorMsg:    "install name can only contain letters, numbers, spaces, dashes, and underscores",
		},
		{
			name:        "invalid with slashes",
			input:       "acme/production",
			expectError: true,
			errorMsg:    "install name can only contain letters, numbers, spaces, dashes, and underscores",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInstallName(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateInstallName(%q) expected error, got nil", tt.input)
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("ValidateInstallName(%q) error = %q, want %q", tt.input, err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateInstallName(%q) unexpected error: %v", tt.input, err)
				}
			}
		})
	}
}

func TestValidateInstallNameMaxLength(t *testing.T) {
	// Test exactly 255 characters - should be valid
	validName := make([]byte, 255)
	for i := range validName {
		validName[i] = 'a'
	}
	if err := ValidateInstallName(string(validName)); err != nil {
		t.Errorf("ValidateInstallName with 255 chars should be valid, got error: %v", err)
	}

	// Test 256 characters - should be invalid
	invalidName := make([]byte, 256)
	for i := range invalidName {
		invalidName[i] = 'a'
	}
	if err := ValidateInstallName(string(invalidName)); err == nil {
		t.Error("ValidateInstallName with 256 chars should be invalid, got nil")
	}
}
