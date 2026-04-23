package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWizardStepToGroupType(t *testing.T) {
	tests := []struct {
		step     string
		expected string
	}{
		{"stack", "stack"},
		{"sandbox", "sandbox"},
		{"components", "component"},
		{"inputs", "other"},
		{"", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.step, func(t *testing.T) {
			assert.Equal(t, tt.expected, wizardStepToGroupType(tt.step))
		})
	}
}

func TestNextWizardStep(t *testing.T) {
	tests := []struct {
		current  string
		expected string
	}{
		{"inputs", "stack"},
		{"stack", "sandbox"},
		{"sandbox", "components"},
		{"components", ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.current, func(t *testing.T) {
			assert.Equal(t, tt.expected, nextWizardStep(tt.current))
		})
	}
}

func TestWizardStepIndex(t *testing.T) {
	assert.Equal(t, 0, wizardStepIndex("inputs"))
	assert.Equal(t, 1, wizardStepIndex("stack"))
	assert.Equal(t, 2, wizardStepIndex("sandbox"))
	assert.Equal(t, 3, wizardStepIndex("components"))
	assert.Equal(t, 0, wizardStepIndex("unknown"))
}

func TestBuildWizardURL(t *testing.T) {
	url := buildWizardURL("/base", "app-123", "stack", "inst-456", "wf-789")
	assert.Equal(t, "/base/apps/app-123/install?step=stack&install_id=inst-456&workflow_id=wf-789", url)
}
