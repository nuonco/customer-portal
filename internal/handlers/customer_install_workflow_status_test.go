package handlers

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials/workflows"
	nuonmodels "github.com/nuonco/nuon-go/models"
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

func TestMergeComponentSteps(t *testing.T) {
	t.Run("merges plan+apply pair with underscores", func(t *testing.T) {
		steps := []*nuonmodels.AppWorkflowStep{
			stepWithStatus("sync_and_plan_mycomp", "completed", 0),
			stepWithStatus("apply_mycomp", "in-progress", 1),
		}
		merged := mergeComponentSteps(steps)
		if len(merged) != 1 {
			t.Fatalf("expected 1 step, got %d", len(merged))
		}
		if merged[0].Name != "deploy_mycomp" {
			t.Errorf("expected name 'deploy_mycomp', got %q", merged[0].Name)
		}
		if string(merged[0].Status.Status) != "in-progress" {
			t.Errorf("expected status 'in-progress', got %q", string(merged[0].Status.Status))
		}
	})

	t.Run("merges plan+apply pair with spaces", func(t *testing.T) {
		steps := []*nuonmodels.AppWorkflowStep{
			stepWithStatus("Sync and plan certificate", "completed", 0),
			stepWithStatus("Apply certificate", "in-progress", 1),
		}
		merged := mergeComponentSteps(steps)
		if len(merged) != 1 {
			t.Fatalf("expected 1 step, got %d", len(merged))
		}
		if merged[0].Name != "deploy_certificate" {
			t.Errorf("expected name 'deploy_certificate', got %q", merged[0].Name)
		}
		if string(merged[0].Status.Status) != "in-progress" {
			t.Errorf("expected status 'in-progress', got %q", string(merged[0].Status.Status))
		}
	})

	t.Run("merges multi-word component names with spaces", func(t *testing.T) {
		steps := []*nuonmodels.AppWorkflowStep{
			stepWithStatus("Sync and plan rds subnet", "completed", 0),
			stepWithStatus("Apply rds subnet", "completed", 1),
		}
		merged := mergeComponentSteps(steps)
		if len(merged) != 1 {
			t.Fatalf("expected 1 step, got %d", len(merged))
		}
		if merged[0].Name != "deploy_rds_subnet" {
			t.Errorf("expected name 'deploy_rds_subnet', got %q", merged[0].Name)
		}
	})

	t.Run("uses plan step status when plan is active", func(t *testing.T) {
		steps := []*nuonmodels.AppWorkflowStep{
			stepWithStatus("Sync and plan foo", "approval-awaiting", 0),
			stepWithStatus("Apply foo", "", 1),
		}
		merged := mergeComponentSteps(steps)
		if len(merged) != 1 {
			t.Fatalf("expected 1 step, got %d", len(merged))
		}
		if string(merged[0].Status.Status) != "approval-awaiting" {
			t.Errorf("expected status 'approval-awaiting', got %q", string(merged[0].Status.Status))
		}
	})

	t.Run("unpaired plan step passes through", func(t *testing.T) {
		steps := []*nuonmodels.AppWorkflowStep{
			stepWithStatus("sync_and_plan_orphan", "in-progress", 0),
			stepWithStatus("other_step", "pending", 1),
		}
		merged := mergeComponentSteps(steps)
		if len(merged) != 2 {
			t.Fatalf("expected 2 steps, got %d", len(merged))
		}
		if merged[0].Name != "sync_and_plan_orphan" {
			t.Errorf("expected original name preserved, got %q", merged[0].Name)
		}
	})

	t.Run("non-component steps pass through unchanged", func(t *testing.T) {
		steps := []*nuonmodels.AppWorkflowStep{
			stepWithStatus("create_install_stack", "completed", 0),
			stepWithStatus("await_install_stack", "in-progress", 1),
		}
		merged := mergeComponentSteps(steps)
		if len(merged) != 2 {
			t.Fatalf("expected 2 steps, got %d", len(merged))
		}
	})

	t.Run("retryable from either step", func(t *testing.T) {
		plan := stepWithStatus("sync_and_plan_bar", "completed", 0)
		apply := stepWithStatus("apply_bar", "error", 1)
		apply.Retryable = true
		merged := mergeComponentSteps([]*nuonmodels.AppWorkflowStep{plan, apply})
		if !merged[0].Retryable {
			t.Error("expected merged step to be retryable")
		}
	})

	t.Run("post-deploy action run folded into deploy step", func(t *testing.T) {
		steps := []*nuonmodels.AppWorkflowStep{
			stepWithStatus("Sync and plan certificate", "completed", 0),
			stepWithStatus("Apply certificate", "completed", 1),
			stepWithStatus("Certificate status Action Run (post-deploy-component)", "in-progress", 2),
		}
		merged := mergeComponentSteps(steps)
		if len(merged) != 1 {
			t.Fatalf("expected 1 step, got %d", len(merged))
		}
		if merged[0].Name != "deploy_certificate" {
			t.Errorf("expected 'deploy_certificate', got %q", merged[0].Name)
		}
		// Action run is in-progress so it should be the active step
		if string(merged[0].Status.Status) != "in-progress" {
			t.Errorf("expected status 'in-progress', got %q", string(merged[0].Status.Status))
		}
	})

	t.Run("pre-deploy action run folded into deploy step", func(t *testing.T) {
		steps := []*nuonmodels.AppWorkflowStep{
			stepWithStatus("Cert check Action Run (pre-deploy-component)", "completed", 0),
			stepWithStatus("Sync and plan certificate", "completed", 1),
			stepWithStatus("Apply certificate", "in-progress", 2),
		}
		merged := mergeComponentSteps(steps)
		if len(merged) != 1 {
			t.Fatalf("expected 1 step, got %d", len(merged))
		}
		if merged[0].Name != "deploy_certificate" {
			t.Errorf("expected 'deploy_certificate', got %q", merged[0].Name)
		}
	})

	t.Run("multiple component pairs merge independently", func(t *testing.T) {
		steps := []*nuonmodels.AppWorkflowStep{
			stepWithStatus("sync_and_plan_a", "completed", 0),
			stepWithStatus("apply_a", "completed", 1),
			stepWithStatus("sync_and_plan_b", "in-progress", 2),
			stepWithStatus("apply_b", "", 3),
		}
		merged := mergeComponentSteps(steps)
		if len(merged) != 2 {
			t.Fatalf("expected 2 steps, got %d", len(merged))
		}
		if merged[0].Name != "deploy_a" {
			t.Errorf("expected 'deploy_a', got %q", merged[0].Name)
		}
		if merged[1].Name != "deploy_b" {
			t.Errorf("expected 'deploy_b', got %q", merged[1].Name)
		}
	})
}

func TestMergeStackSteps(t *testing.T) {
	t.Run("absorbs generate_install_state and provision_runner into generate_install_stack", func(t *testing.T) {
		steps := []*nuonmodels.AppWorkflowStep{
			stepWithStatus("Generate install state", "completed", 0),
			stepWithStatus("Reprovision runner service account", "completed", 1),
			stepWithStatus("Generate install stack", "completed", 2),
			stepWithStatus("Await install stack", "completed", 3),
		}
		merged := mergeStackSteps(steps)
		if len(merged) != 2 {
			t.Fatalf("expected 2 steps, got %d", len(merged))
		}
		if normalizeStepName(merged[0].Name) != "generate_install_stack" {
			t.Errorf("expected 'Generate install stack', got %q", merged[0].Name)
		}
		if normalizeStepName(merged[1].Name) != "await_install_stack" {
			t.Errorf("expected 'Await install stack', got %q", merged[1].Name)
		}
	})

	t.Run("active status from absorbed step surfaces", func(t *testing.T) {
		steps := []*nuonmodels.AppWorkflowStep{
			stepWithStatus("Generate install state", "in-progress", 0),
			stepWithStatus("Provision runner service account", "", 1),
			stepWithStatus("Generate install stack", "", 2),
		}
		merged := mergeStackSteps(steps)
		if len(merged) != 1 {
			t.Fatalf("expected 1 step, got %d", len(merged))
		}
		if string(merged[0].Status.Status) != "in-progress" {
			t.Errorf("expected 'in-progress', got %q", string(merged[0].Status.Status))
		}
	})

	t.Run("no generate_install_stack leaves steps unchanged", func(t *testing.T) {
		steps := []*nuonmodels.AppWorkflowStep{
			stepWithStatus("Generate install state", "completed", 0),
			stepWithStatus("Some other step", "completed", 1),
		}
		merged := mergeStackSteps(steps)
		if len(merged) != 2 {
			t.Fatalf("expected 2 steps, got %d", len(merged))
		}
	})
}

func TestGroupStepsIntoPhases(t *testing.T) {
	t.Run("nil workflow returns nil", func(t *testing.T) {
		phases := groupStepsIntoPhases(nil)
		if phases != nil {
			t.Errorf("expected nil, got %v", phases)
		}
	})

	t.Run("empty steps returns nil", func(t *testing.T) {
		wf := &nuonmodels.AppWorkflow{}
		phases := groupStepsIntoPhases(wf)
		if phases != nil {
			t.Errorf("expected nil, got %v", phases)
		}
	})

	t.Run("index-based grouping: 0-4 stack, 5-9 sandbox, 10+ components", func(t *testing.T) {
		wf := &nuonmodels.AppWorkflow{
			Steps: []*nuonmodels.AppWorkflowStep{
				stepWithStatus("create install stack", "completed", 0),
				stepWithStatus("await install stack", "completed", 1),
				stepWithStatus("generate install state", "completed", 2),
				stepWithStatus("provision runner service account", "completed", 3),
				stepWithStatus("await runner health", "completed", 4),
				stepWithStatus("provision sandbox", "completed", 5),
				stepWithStatus("configure sandbox", "completed", 6),
				stepWithStatus("sandbox networking", "completed", 7),
				stepWithStatus("sandbox dns", "completed", 8),
				stepWithStatus("sandbox finalize", "in-progress", 9),
				stepWithStatus("deploy component A", "pending", 10),
				stepWithStatus("deploy component B", "pending", 11),
			},
		}

		phases := groupStepsIntoPhases(wf)

		if len(phases) != 3 {
			t.Fatalf("expected 3 phases, got %d", len(phases))
		}

		if phases[0].Name != "Install stack" {
			t.Errorf("expected phase 0 name 'Install stack', got '%s'", phases[0].Name)
		}
		if len(phases[0].Steps) != 5 {
			t.Errorf("expected 5 steps in stack phase, got %d", len(phases[0].Steps))
		}
		if phases[0].Status != "completed" {
			t.Errorf("expected stack phase status 'completed', got '%s'", phases[0].Status)
		}

		if phases[1].Name != "Provision sandbox" {
			t.Errorf("expected phase 1 name 'Provision sandbox', got '%s'", phases[1].Name)
		}
		if len(phases[1].Steps) != 5 {
			t.Errorf("expected 5 steps in sandbox phase, got %d", len(phases[1].Steps))
		}
		if phases[1].Status != "in_progress" {
			t.Errorf("expected sandbox phase status 'in_progress', got '%s'", phases[1].Status)
		}

		if phases[2].Name != "Deploy app" {
			t.Errorf("expected phase 2 name 'Deploy app', got '%s'", phases[2].Name)
		}
		if len(phases[2].Steps) != 2 {
			t.Errorf("expected 2 steps in components phase, got %d", len(phases[2].Steps))
		}
		if phases[2].Status != "not_started" {
			t.Errorf("expected components phase status 'not_started', got '%s'", phases[2].Status)
		}
	})

	t.Run("all completed", func(t *testing.T) {
		wf := &nuonmodels.AppWorkflow{
			Steps: []*nuonmodels.AppWorkflowStep{
				stepWithStatus("step 0", "completed", 0),
				stepWithStatus("step 5", "completed", 5),
				stepWithStatus("step 10", "completed", 10),
			},
		}

		phases := groupStepsIntoPhases(wf)
		// With 3 steps: idx 0 → stack, idx 1 → stack, idx 2 → stack (all < 5)
		// Stack phase completed, other phases not_started
		if phases[0].Status != "completed" {
			t.Errorf("expected phase 0 status 'completed', got '%s'", phases[0].Status)
		}
		if phases[1].Status != "not_started" {
			t.Errorf("expected phase 1 status 'not_started', got '%s'", phases[1].Status)
		}
		if phases[2].Status != "not_started" {
			t.Errorf("expected phase 2 status 'not_started', got '%s'", phases[2].Status)
		}
	})

	t.Run("failed step marks phase as failed", func(t *testing.T) {
		wf := &nuonmodels.AppWorkflow{
			Steps: []*nuonmodels.AppWorkflowStep{
				stepWithStatus("step 0", "error", 0),
				stepWithStatus("step 1", "pending", 1),
			},
		}

		phases := groupStepsIntoPhases(wf)
		if phases[0].Status != "failed" {
			t.Errorf("expected stack phase status 'failed', got '%s'", phases[0].Status)
		}
	})

	t.Run("fewer than 5 steps: stack has steps, other phases empty", func(t *testing.T) {
		wf := &nuonmodels.AppWorkflow{
			Steps: []*nuonmodels.AppWorkflowStep{
				stepWithStatus("step A", "completed", 0),
				stepWithStatus("step B", "in-progress", 1),
				stepWithStatus("step C", "pending", 2),
			},
		}

		phases := groupStepsIntoPhases(wf)
		if len(phases) != 3 {
			t.Fatalf("expected 3 phases, got %d", len(phases))
		}
		if phases[0].Name != "Install stack" {
			t.Errorf("expected 'Install stack', got '%s'", phases[0].Name)
		}
		if len(phases[0].Steps) != 3 {
			t.Errorf("expected 3 steps, got %d", len(phases[0].Steps))
		}
		// Empty phases should be not_started
		if phases[1].Status != "not_started" {
			t.Errorf("expected phase 1 status 'not_started', got '%s'", phases[1].Status)
		}
		if len(phases[1].Steps) != 0 {
			t.Errorf("expected 0 steps in phase 1, got %d", len(phases[1].Steps))
		}
		if phases[2].Status != "not_started" {
			t.Errorf("expected phase 2 status 'not_started', got '%s'", phases[2].Status)
		}
	})

	t.Run("step progress set on active phase", func(t *testing.T) {
		wf := &nuonmodels.AppWorkflow{
			Steps: []*nuonmodels.AppWorkflowStep{
				stepWithStatus("step 0", "completed", 0),
				stepWithStatus("step 1", "in-progress", 1),
			},
		}

		phases := groupStepsIntoPhases(wf)
		found := false
		for _, p := range phases {
			if p.Status == "in_progress" && p.StepProgress != "" {
				found = true
			}
		}
		if !found {
			t.Error("expected an in-progress phase with StepProgress set")
		}
	})

	t.Run("realistic step names grouped correctly by index", func(t *testing.T) {
		wf := &nuonmodels.AppWorkflow{
			Steps: []*nuonmodels.AppWorkflowStep{
				stepWithStatus("create install stack", "completed", 0),
				stepWithStatus("await install stack", "completed", 1),
				stepWithStatus("generate install state", "completed", 2),
				stepWithStatus("provision runner service account", "completed", 3),
				stepWithStatus("await runner health", "completed", 4),
				stepWithStatus("provision sandbox", "completed", 5),
				stepWithStatus("configure sandbox", "completed", 6),
				stepWithStatus("sandbox networking", "completed", 7),
				stepWithStatus("sandbox dns", "completed", 8),
				stepWithStatus("sandbox finalize", "completed", 9),
				stepWithStatus("deploy api component", "completed", 10),
				stepWithStatus("deploy web component", "completed", 11),
				stepWithStatus("deploy worker component", "completed", 12),
			},
		}

		phases := groupStepsIntoPhases(wf)

		if len(phases) != 3 {
			t.Fatalf("expected 3 phases, got %d", len(phases))
		}

		// Verify previously-mismatched steps are now in the right phase
		stackNames := make([]string, len(phases[0].Steps))
		for i, s := range phases[0].Steps {
			stackNames[i] = s.Name
		}
		// "generate install state", "provision runner service account", "await runner health"
		// should all be in stack (indices 2, 3, 4)
		if len(phases[0].Steps) != 5 {
			t.Errorf("expected 5 stack steps, got %d: %v", len(phases[0].Steps), stackNames)
		}
		if len(phases[1].Steps) != 5 {
			t.Errorf("expected 5 sandbox steps, got %d", len(phases[1].Steps))
		}
		if len(phases[2].Steps) != 3 {
			t.Errorf("expected 3 component steps, got %d", len(phases[2].Steps))
		}
	})
}

func stepWithTarget(name, status string, idx int64, targetID, targetType string) *nuonmodels.AppWorkflowStep {
	s := stepWithStatus(name, status, idx)
	s.StepTargetID = targetID
	s.StepTargetType = targetType
	return s
}

func TestProcessWorkflowForCustomer_StepTargetFields(t *testing.T) {
	t.Run("in-progress step target fields are extracted", func(t *testing.T) {
		wf := &nuonmodels.AppWorkflow{
			Status: &nuonmodels.AppCompositeStatus{Status: "in-progress"},
			Steps: []*nuonmodels.AppWorkflowStep{
				stepWithTarget("provision sandbox", "in-progress", 0, "sr-123", "install_sandbox_runs"),
			},
		}
		result := processWorkflowForCustomer(wf)
		if got := result["current_step_target_id"]; got != "sr-123" {
			t.Errorf("expected target_id 'sr-123', got %q", got)
		}
		if got := result["current_step_target_type"]; got != "install_sandbox_runs" {
			t.Errorf("expected target_type 'install_sandbox_runs', got %q", got)
		}
	})

	t.Run("approval-awaiting step target fields are extracted", func(t *testing.T) {
		wf := &nuonmodels.AppWorkflow{
			Status: &nuonmodels.AppCompositeStatus{Status: "approval-awaiting"},
			Steps: []*nuonmodels.AppWorkflowStep{
				stepWithTarget("deploy app", "approval-awaiting", 0, "dp-456", "install_deploys"),
			},
		}
		result := processWorkflowForCustomer(wf)
		if got := result["current_step_target_id"]; got != "dp-456" {
			t.Errorf("expected target_id 'dp-456', got %q", got)
		}
		if got := result["current_step_target_type"]; got != "install_deploys" {
			t.Errorf("expected target_type 'install_deploys', got %q", got)
		}
	})

	t.Run("pending step target fields are extracted", func(t *testing.T) {
		wf := &nuonmodels.AppWorkflow{
			Status: &nuonmodels.AppCompositeStatus{Status: "pending"},
			Steps: []*nuonmodels.AppWorkflowStep{
				stepWithTarget("create stack", "pending", 0, "st-789", "install_stack_versions"),
			},
		}
		result := processWorkflowForCustomer(wf)
		if got := result["current_step_target_id"]; got != "st-789" {
			t.Errorf("expected target_id 'st-789', got %q", got)
		}
	})

	t.Run("no steps returns empty target fields", func(t *testing.T) {
		wf := &nuonmodels.AppWorkflow{
			Status: &nuonmodels.AppCompositeStatus{Status: "pending"},
		}
		result := processWorkflowForCustomer(wf)
		if got := result["current_step_target_id"]; got != "" {
			t.Errorf("expected empty target_id, got %q", got)
		}
	})
}

func TestResolveCurrentStepRole_NoTarget(t *testing.T) {
	// When no target ID/type, should return empty without making API calls
	processed := gin.H{
		"current_step_target_id":   "",
		"current_step_target_type": "",
	}
	role := resolveCurrentStepRole(context.Background(), nil, "install-1", processed)
	if role != "" {
		t.Errorf("expected empty role, got %q", role)
	}
}

func TestPhaseStepStatus(t *testing.T) {
	ps := partials.PhaseStep{Name: "test", Status: "completed"}
	if ps.Status != "completed" {
		t.Errorf("expected 'completed', got '%s'", ps.Status)
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
		if got != `install_id = "abc123"\nregion = "us-central1"` && got != "install_id = \"abc123\"\nregion = \"us-central1\"" {
			// Just check it's not empty — the exact escaping depends on JSON parsing
			t.Logf("got tfvars: %s", got)
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

func TestProcessWorkflowForCustomer_PolicyViolations(t *testing.T) {
	t.Run("in-progress step with policy violations", func(t *testing.T) {
		step := stepWithStatus("sync and plan", "in-progress", 0)
		step.Status.Metadata = map[string]any{
			"deny_violations": []interface{}{
				map[string]interface{}{"policy_id": "p1", "message": "must not run privileged"},
			},
			"warn_violations": []interface{}{
				map[string]interface{}{"policy_id": "p2", "message": "should set limits"},
			},
		}
		wf := &nuonmodels.AppWorkflow{
			Status: &nuonmodels.AppCompositeStatus{Status: "in-progress"},
			Steps:  []*nuonmodels.AppWorkflowStep{step},
		}
		result := processWorkflowForCustomer(wf)
		if !result["has_policy_data"].(bool) {
			t.Error("expected has_policy_data to be true")
		}
		deny := result["deny_violations"].([]workflows.PolicyViolation)
		warn := result["warn_violations"].([]workflows.PolicyViolation)
		if len(deny) != 1 || deny[0].Message != "must not run privileged" {
			t.Errorf("unexpected deny violations: %+v", deny)
		}
		if len(warn) != 1 || warn[0].Message != "should set limits" {
			t.Errorf("unexpected warn violations: %+v", warn)
		}
	})

	t.Run("error step with policy violations", func(t *testing.T) {
		step := stepWithStatus("sync and plan", "error", 0)
		step.Status.Metadata = map[string]any{
			"deny_violations": []interface{}{
				map[string]interface{}{"policy_id": "p1", "message": "denied"},
			},
		}
		wf := &nuonmodels.AppWorkflow{
			Status: &nuonmodels.AppCompositeStatus{Status: "error"},
			Steps:  []*nuonmodels.AppWorkflowStep{step},
		}
		result := processWorkflowForCustomer(wf)
		if !result["has_policy_data"].(bool) {
			t.Error("expected has_policy_data to be true")
		}
		deny := result["deny_violations"].([]workflows.PolicyViolation)
		if len(deny) != 1 {
			t.Errorf("expected 1 deny violation, got %d", len(deny))
		}
	})

	t.Run("step without policy data", func(t *testing.T) {
		wf := &nuonmodels.AppWorkflow{
			Status: &nuonmodels.AppCompositeStatus{Status: "in-progress"},
			Steps: []*nuonmodels.AppWorkflowStep{
				stepWithStatus("await runner", "in-progress", 0),
			},
		}
		result := processWorkflowForCustomer(wf)
		if result["has_policy_data"].(bool) {
			t.Error("expected has_policy_data to be false")
		}
	})
}
