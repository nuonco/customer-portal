package handlers

import (
	"testing"

	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
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

func TestPhaseStepStatus(t *testing.T) {
	ps := partials.PhaseStep{Name: "test", Status: "completed"}
	if ps.Status != "completed" {
		t.Errorf("expected 'completed', got '%s'", ps.Status)
	}
}
