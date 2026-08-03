package wizard

import (
	"testing"

	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func TestGroupStatus_UsesLatestStepStatusesOverStaleGroupStatus(t *testing.T) {
	group := nuon.WorkflowStepGroup{
		Status: nuon.CompositeStatus{Status: "in-progress"},
		Steps: []*nuon.WorkflowStep{
			{
				ExecutionType: "approval",
				Status:        &nuon.CompositeStatus{Status: "approved"},
			},
			{
				ExecutionType: "apply",
				Status:        &nuon.CompositeStatus{Status: "completed"},
			},
		},
	}

	if got := GroupStatus(group); got != "completed" {
		t.Fatalf("GroupStatus() = %q, want %q", got, "completed")
	}
}

func TestGroupStatus_TreatsCancelledAsTerminalCompletion(t *testing.T) {
	group := nuon.WorkflowStepGroup{
		Status: nuon.CompositeStatus{Status: "in-progress"},
		Steps: []*nuon.WorkflowStep{
			{
				ExecutionType: "apply",
				Status:        &nuon.CompositeStatus{Status: "cancelled"},
			},
		},
	}

	if got := GroupStatus(group); got != "completed" {
		t.Fatalf("GroupStatus() = %q, want %q", got, "completed")
	}
}

func TestGroupStatus_PrefersErrorOverOtherTerminalStates(t *testing.T) {
	group := nuon.WorkflowStepGroup{
		Status: nuon.CompositeStatus{Status: "in-progress"},
		Steps: []*nuon.WorkflowStep{
			{
				ExecutionType: "approval",
				Status:        &nuon.CompositeStatus{Status: "approved"},
			},
			{
				ExecutionType: "apply",
				Status:        &nuon.CompositeStatus{Status: "error"},
			},
		},
	}

	if got := GroupStatus(group); got != "error" {
		t.Fatalf("GroupStatus() = %q, want %q", got, "error")
	}
}

func TestMatchesWizardStep_ComponentsWithoutDomainLabel(t *testing.T) {
	group := nuon.WorkflowStepGroup{
		Name: "deploy-my-component",
		Steps: []*nuon.WorkflowStep{
			{Name: "deploy my component", ExecutionType: "apply"},
		},
	}

	if !MatchesWizardStep("components", group) {
		t.Fatalf("MatchesWizardStep() = false, want true")
	}
}

func TestMatchesWizardStep_ComponentsDoesNotMatchSandboxByStepName(t *testing.T) {
	group := nuon.WorkflowStepGroup{
		Steps: []*nuon.WorkflowStep{
			{Name: "provision sandbox plan", ExecutionType: "approval"},
		},
	}

	if MatchesWizardStep("components", group) {
		t.Fatalf("MatchesWizardStep() = true, want false")
	}
}
