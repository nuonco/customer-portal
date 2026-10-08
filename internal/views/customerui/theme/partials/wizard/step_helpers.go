package wizard

import (
	"strings"

	"github.com/nuonco/customer-portal/pkg/nuon"
)

// MatchesWizardStep returns true if the given workflow step group belongs to
// the wizard step. Stack and sandbox each correspond to a single, well-known
// group identified by its name label. Components is matched by domain label
// since there is one group per component.
func MatchesWizardStep(step string, g nuon.WorkflowStepGroup) bool {
	switch step {
	case "stack":
		return g.Labels["name"] == "provision-install-stack"
	case "sandbox":
		return g.Labels["name"] == "provision-sandbox"
	case "components":
		if g.Labels["domain"] == "component" || g.Labels["component_name"] != "" {
			return true
		}

		if g.Labels["name"] == "provision-install-stack" || g.Labels["name"] == "provision-sandbox" {
			return false
		}

		if g.Name == "provision-install-stack" || g.Name == "provision-sandbox" {
			return false
		}

		for _, s := range g.Steps {
			if s == nil {
				continue
			}
			stepName := strings.ToLower(strings.TrimSpace(s.Name))
			if _, ok := stackStepNames[stepName]; ok {
				return false
			}
			if _, ok := sandboxStepNames[stepName]; ok {
				return false
			}
		}

		return len(g.Steps) > 0
	default:
		return false
	}
}

// LatestSteps returns the last step for each execution type in the group.
// When steps are retried, the retry is appended after the original, so the
// last step of each type is always the current one.
func LatestSteps(g nuon.WorkflowStepGroup) []*nuon.WorkflowStep {
	last := make(map[string]*nuon.WorkflowStep)
	var order []string
	for _, s := range g.Steps {
		if _, seen := last[s.ExecutionType]; !seen {
			order = append(order, s.ExecutionType)
		}
		last[s.ExecutionType] = s
	}
	result := make([]*nuon.WorkflowStep, 0, len(order))
	for _, key := range order {
		result = append(result, last[key])
	}
	return result
}

// ActiveStep returns the first non-completed step from the latest steps, or nil.
func ActiveStep(g nuon.WorkflowStepGroup) *nuon.WorkflowStep {
	for _, s := range LatestSteps(g) {
		if s.Status == nil {
			return s
		}
		switch s.Status.Status {
		case "completed", "success", "approved":
			continue
		}
		return s
	}
	return nil
}

// GroupCanApprove returns true if the active step is awaiting approval.
func GroupCanApprove(g nuon.WorkflowStepGroup) bool {
	s := ActiveStep(g)
	return s != nil && s.Status != nil && s.Status.Status == "approval-awaiting" && s.Approval != nil
}

// GroupCanRetry returns true if the group has a retryable error step.
func GroupCanRetry(g nuon.WorkflowStepGroup) bool {
	return RetryableStep(g) != nil
}

// RetryableStep returns a retryable failed step from the latest steps, or nil.
// A step whose status metadata reports retries_exhausted is excluded — those
// steps cannot be retried anymore even though Retryable may still be true.
func RetryableStep(g nuon.WorkflowStepGroup) *nuon.WorkflowStep {
	for _, s := range LatestSteps(g) {
		if s.Retryable && s.Finished && s.Status != nil && s.Status.Status != "completed" && s.Status.Status != "success" && !stepRetriesExhausted(s) {
			return s
		}
	}
	return nil
}

// stepRetriesExhausted returns true if the step's status metadata indicates
// retry attempts have been exhausted (e.g. auto-retry hit its max).
func stepRetriesExhausted(s *nuon.WorkflowStep) bool {
	if s == nil || s.Status == nil || s.Status.Metadata == nil {
		return false
	}
	if v, ok := s.Status.Metadata["retries_exhausted"].(bool); ok && v {
		return true
	}
	return false
}

// GroupRetriesExhausted reports whether any latest step in the group has
// exhausted its retries.
func GroupRetriesExhausted(g nuon.WorkflowStepGroup) bool {
	for _, s := range LatestSteps(g) {
		if stepRetriesExhausted(s) {
			return true
		}
	}
	return false
}

// GroupHasApprovalStep returns true if any latest step has execution_type "approval".
func GroupHasApprovalStep(g nuon.WorkflowStepGroup) bool {
	for _, s := range LatestSteps(g) {
		if s.ExecutionType == "approval" {
			return true
		}
	}
	return false
}

// RetryableStepID returns the ID of the retryable error step, or empty string.
func RetryableStepID(g nuon.WorkflowStepGroup) string {
	s := RetryableStep(g)
	if s != nil {
		return s.ID
	}
	return ""
}

// PlanStep returns the last approval step in the group (latest retry wins).
func PlanStep(g nuon.WorkflowStepGroup) *nuon.WorkflowStep {
	var last *nuon.WorkflowStep
	for _, s := range g.Steps {
		if s.ExecutionType == "approval" {
			last = s
		}
	}
	return last
}

// ApplyStep returns the last non-approval step in the group (latest retry wins).
func ApplyStep(g nuon.WorkflowStepGroup) *nuon.WorkflowStep {
	var last *nuon.WorkflowStep
	for _, s := range g.Steps {
		if s.ExecutionType != "approval" {
			last = s
		}
	}
	return last
}

// StepStatus returns the status string for a step, or "not_started" if nil.
func StepStatus(step *nuon.WorkflowStep) string {
	if step == nil || step.Status == nil {
		return "not_started"
	}
	return step.Status.Status
}

func normalizeStepStatus(status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		return "not_started"
	}
	return status
}

func isStepSuccessTerminalStatus(status string) bool {
	switch normalizeStepStatus(status) {
	case "completed", "success", "approved", "cancelled", "skipped":
		return true
	default:
		return false
	}
}

func isStepErrorStatus(status string) bool {
	return normalizeStepStatus(status) == "error"
}

func isStepApprovalAwaitingStatus(status string) bool {
	return normalizeStepStatus(status) == "approval-awaiting"
}

func isStepActiveStatus(status string) bool {
	switch normalizeStepStatus(status) {
	case "in-progress", "active", "pending", "queued", "running", "not_started":
		return true
	default:
		return false
	}
}

// GroupStatus returns the status for a group, checking step-level status
// since the group aggregate may not reflect approval state.
func GroupStatus(g nuon.WorkflowStepGroup) string {
	latest := LatestSteps(g)
	if len(latest) > 0 {
		hasActive := false
		allSuccessTerminal := true

		for _, s := range latest {
			status := StepStatus(s)
			if isStepErrorStatus(status) {
				return "error"
			}
			if isStepApprovalAwaitingStatus(status) {
				return "approval-awaiting"
			}
			if isStepActiveStatus(status) {
				hasActive = true
			}
			if !isStepSuccessTerminalStatus(status) {
				allSuccessTerminal = false
			}
		}

		if hasActive {
			return "in-progress"
		}
		if allSuccessTerminal {
			return "completed"
		}
	}

	status := normalizeStepStatus(string(g.Status.Status))
	if status == "not_started" {
		return "in-progress"
	}
	return status
}

// FirstRetryableGroup returns the first group that is in an error state
// and has a retryable step, or nil.
func FirstRetryableGroup(groups []nuon.WorkflowStepGroup) *nuon.WorkflowStepGroup {
	for i := range groups {
		if GroupStatus(groups[i]) == "error" && GroupCanRetry(groups[i]) {
			return &groups[i]
		}
	}
	return nil
}

// FirstGroupWithApproval returns the first group that is awaiting approval,
// or the first group with an approval step if none are actively awaiting.
func FirstGroupWithApproval(groups []nuon.WorkflowStepGroup) *nuon.WorkflowStepGroup {
	for i := range groups {
		if GroupCanApprove(groups[i]) {
			return &groups[i]
		}
	}
	for i := range groups {
		if GroupHasApprovalStep(groups[i]) {
			return &groups[i]
		}
	}
	return nil
}
