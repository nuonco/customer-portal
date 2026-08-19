package handlers

import (
	"context"
	"encoding/base64"
	"testing"

	nuonmodels "github.com/nuonco/nuon/sdks/nuon-go/models"
)

func stepWithStatus(name string, status string, idx int64) *nuonmodels.AppWorkflowStep {
	s := &nuonmodels.AppWorkflowStep{
		Name: name,
		Idx:  idx,
	}
	if status != "" {
		s.Status = &nuonmodels.AppCompositeStatus{
			Status: nuonmodels.AppStatus(status),
		}
	}
	return s
}

func stepWithTarget(name, status string, idx int64, targetID, targetType string) *nuonmodels.AppWorkflowStep {
	s := stepWithStatus(name, status, idx)
	s.StepTargetID = targetID
	s.StepTargetType = targetType
	return s
}

func TestResolveCurrentStepRole_NoTarget(t *testing.T) {
	wf := &nuonmodels.AppWorkflow{
		Steps: []*nuonmodels.AppWorkflowStep{
			stepWithStatus("some step", "in-progress", 0),
		},
	}
	role := resolveCurrentStepRole(context.Background(), nil, "install-1", wf)
	if role != "" {
		t.Errorf("expected empty role, got %q", role)
	}
}

func TestParseTfvars(t *testing.T) {
	t.Run("nil returns empty", func(t *testing.T) {
		if got := parseTfvars(nil); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("JSON string with tfvars key", func(t *testing.T) {
		input := `{"tfvars":"install_id = \"abc123\"\nregion = \"us-central1\""}`
		got := parseTfvars(input)
		if got == "" {
			t.Error("expected non-empty tfvars")
		}
	})

	t.Run("JSON string without tfvars key", func(t *testing.T) {
		input := `{"something_else":"value"}`
		if got := parseTfvars(input); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("map with tfvars key", func(t *testing.T) {
		input := map[string]interface{}{"tfvars": "some content"}
		got := parseTfvars(input)
		if got != "some content" {
			t.Errorf("expected 'some content', got %q", got)
		}
	})

	t.Run("base64-encoded JSON with tfvars key", func(t *testing.T) {
		raw := `{"tfvars":"region = \"us-central1\""}`
		encoded := base64.StdEncoding.EncodeToString([]byte(raw))
		got := parseTfvars(encoded)
		if got == "" {
			t.Error("expected non-empty tfvars from base64-encoded input")
		}
		if got != `region = "us-central1"` {
			t.Errorf("expected 'region = \"us-central1\"', got %q", got)
		}
	})

	t.Run("base64-encoded JSON without padding", func(t *testing.T) {
		raw := `{"tfvars":"x = 1"}`
		encoded := base64.RawStdEncoding.EncodeToString([]byte(raw))
		got := parseTfvars(encoded)
		if got != "x = 1" {
			t.Errorf("expected 'x = 1', got %q", got)
		}
	})

	t.Run("non-JSON string returns empty", func(t *testing.T) {
		if got := parseTfvars("not json"); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestExtractPolicyViolations(t *testing.T) {
	t.Run("nil metadata returns nil", func(t *testing.T) {
		deny, warn := extractPolicyViolations(nil)
		if deny != nil || warn != nil {
			t.Errorf("expected nil, got deny=%v warn=%v", deny, warn)
		}
	})

	t.Run("empty metadata returns nil", func(t *testing.T) {
		deny, warn := extractPolicyViolations(map[string]any{})
		if deny != nil || warn != nil {
			t.Errorf("expected nil, got deny=%v warn=%v", deny, warn)
		}
	})

	t.Run("extracts deny violations", func(t *testing.T) {
		metadata := map[string]any{
			"deny_violations": []interface{}{
				map[string]interface{}{
					"policy_id": "pol-1",
					"message":   "container must not run as root",
					"severity":  "deny",
				},
				map[string]interface{}{
					"policy_id": "pol-2",
					"message":   "must set resource limits",
					"severity":  "deny",
				},
			},
		}
		deny, warn := extractPolicyViolations(metadata)
		if len(deny) != 2 {
			t.Fatalf("expected 2 deny violations, got %d", len(deny))
		}
		if deny[0].PolicyID != "pol-1" || deny[0].Message != "container must not run as root" {
			t.Errorf("unexpected deny[0]: %+v", deny[0])
		}
		if warn != nil {
			t.Errorf("expected nil warnings, got %v", warn)
		}
	})

	t.Run("extracts warn violations", func(t *testing.T) {
		metadata := map[string]any{
			"warn_violations": []interface{}{
				map[string]interface{}{
					"policy_id": "pol-3",
					"message":   "should set memory limits",
					"severity":  "warn",
				},
			},
		}
		deny, warn := extractPolicyViolations(metadata)
		if deny != nil {
			t.Errorf("expected nil denies, got %v", deny)
		}
		if len(warn) != 1 {
			t.Fatalf("expected 1 warn violation, got %d", len(warn))
		}
		if warn[0].Message != "should set memory limits" {
			t.Errorf("unexpected warn[0]: %+v", warn[0])
		}
	})

	t.Run("extracts both deny and warn", func(t *testing.T) {
		metadata := map[string]any{
			"deny_violations": []interface{}{
				map[string]interface{}{"policy_id": "p1", "message": "denied"},
			},
			"warn_violations": []interface{}{
				map[string]interface{}{"policy_id": "p2", "message": "warned"},
			},
		}
		deny, warn := extractPolicyViolations(metadata)
		if len(deny) != 1 || len(warn) != 1 {
			t.Errorf("expected 1 deny + 1 warn, got %d deny + %d warn", len(deny), len(warn))
		}
	})

	t.Run("wrong type in metadata is handled gracefully", func(t *testing.T) {
		metadata := map[string]any{
			"deny_violations": "not an array",
		}
		deny, warn := extractPolicyViolations(metadata)
		if deny != nil || warn != nil {
			t.Errorf("expected nil, got deny=%v warn=%v", deny, warn)
		}
	})
}
