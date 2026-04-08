package handlers

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon-go/models"
	"go.uber.org/zap"
)

// getStackSetupData checks for an active "await install stack" step and returns platform-specific setup data.
func (h *Handler) getStackSetupData(ctx context.Context, client *nuon.Client, install *models.Install, wf *nuonmodels.AppWorkflow) partials.StackSetupData {
	if wf.Steps == nil {
		return partials.StackSetupData{}
	}

	for _, step := range wf.Steps {
		if step.Name != "await install stack" {
			continue
		}
		// Determine platform from app runner type
		platform := h.detectPlatform(ctx, client, install)
		zap.L().Debug("getStackSetupData: detected platform", zap.String("platform", platform), zap.String("installID", install.NuonInstallID))

		stack, stackErr := client.GetInstallStack(ctx, install.NuonInstallID)
		if stackErr != nil {
			zap.L().Debug("getStackSetupData: failed to fetch install stack", zap.Error(stackErr))
		}

		switch platform {
		case "gcp":
			setup := partials.StackSetupData{
				Platform:      "gcp",
				NuonInstallID: install.NuonInstallID,
			}
			if stackErr == nil && stack != nil && stack.Versions != nil && len(stack.Versions) > 0 {
				setup.TfvarsContent = parseTfvars(stack.Versions[0].Contents)
				zap.L().Debug("getStackSetupData: parsed tfvars", zap.Bool("hasTfvars", setup.TfvarsContent != ""))
			} else {
				zap.L().Debug("getStackSetupData: no stack versions available for tfvars")
			}
			return setup

		default: // AWS and others use CloudFormation
			setup := partials.StackSetupData{Platform: platform}
			if stackErr == nil && stack != nil && stack.Versions != nil && len(stack.Versions) > 0 {
				if stack.Versions[0].QuickLinkURL != "" {
					setup.CloudFormationLink = stack.Versions[0].QuickLinkURL
				}
			}
			// Append customer inputs to CF URL
			if setup.CloudFormationLink != "" {
				inputConfig, inputErr := client.GetAppInputConfig(ctx, install.GetAppID())
				if inputErr == nil && inputConfig != nil {
					var configMap map[string]interface{}
					jsonBytes, jerr := json.Marshal(inputConfig)
					if jerr == nil {
						if json.Unmarshal(jsonBytes, &configMap) == nil {
							inputMappings := extractCustomerInputMappings(configMap)
							if len(inputMappings) > 0 {
								currentInputs, ciErr := client.GetInstallCurrentInputs(ctx, install.NuonInstallID)
								if ciErr == nil && currentInputs != nil && currentInputs.Values != nil {
									setup.CloudFormationLink = appendInputsToCloudFormationURL(
										setup.CloudFormationLink,
										currentInputs.Values,
										inputMappings,
									)
								}
							}
						}
					}
				}
			}
			return setup
		}
	}
	return partials.StackSetupData{}
}

// detectPlatform determines the cloud platform from the app's runner type.
func (h *Handler) detectPlatform(ctx context.Context, client *nuon.Client, install *models.Install) string {
	appID := install.GetAppID()
	app, err := client.GetApp(ctx, appID)
	if err != nil {
		zap.L().Debug("detectPlatform: failed to fetch app", zap.String("appID", appID), zap.Error(err))
		return "aws" // default
	}
	if app == nil || app.RunnerConfig == nil {
		zap.L().Debug("detectPlatform: app or runner config is nil", zap.String("appID", appID))
		return "aws" // default
	}
	runnerType := string(app.RunnerConfig.AppRunnerType)
	zap.L().Debug("detectPlatform: runner type", zap.String("appID", appID), zap.String("runnerType", runnerType))
	switch runnerType {
	case "gcp":
		return "gcp"
	case "azure":
		return "azure"
	default:
		return "aws"
	}
}

// parseTfvars extracts the tfvars string from stack version contents.
// Contents may be a JSON string containing a "tfvars" key.
func parseTfvars(contents interface{}) string {
	if contents == nil {
		zap.L().Debug("parseTfvars: contents is nil")
		return ""
	}

	zap.L().Debug("parseTfvars: contents type", zap.String("type", fmt.Sprintf("%T", contents)))

	var raw interface{}
	switch v := contents.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &raw); err != nil {
			// Fallback: try base64 decode, then JSON unmarshal
			decoded, b64Err := base64.StdEncoding.DecodeString(v)
			if b64Err != nil {
				decoded, b64Err = base64.RawStdEncoding.DecodeString(v)
			}
			if b64Err != nil {
				zap.L().Debug("parseTfvars: failed to unmarshal or base64-decode string contents", zap.Error(err))
				return ""
			}
			if err := json.Unmarshal(decoded, &raw); err != nil {
				zap.L().Debug("parseTfvars: base64-decoded but failed to unmarshal JSON", zap.Error(err))
				return ""
			}
		}
	case map[string]interface{}:
		raw = v
	case json.RawMessage:
		if err := json.Unmarshal(v, &raw); err != nil {
			zap.L().Debug("parseTfvars: failed to unmarshal RawMessage contents", zap.Error(err))
			return ""
		}
	default:
		// Try JSON round-trip for unknown types
		b, err := json.Marshal(v)
		if err != nil {
			zap.L().Debug("parseTfvars: unsupported contents type, marshal failed", zap.String("type", fmt.Sprintf("%T", v)))
			return ""
		}
		if err := json.Unmarshal(b, &raw); err != nil {
			zap.L().Debug("parseTfvars: unsupported contents type, unmarshal failed", zap.String("type", fmt.Sprintf("%T", v)))
			return ""
		}
	}

	if m, ok := raw.(map[string]interface{}); ok {
		if tfvars, ok := m["tfvars"]; ok {
			zap.L().Debug("parseTfvars: found tfvars key")
			return fmt.Sprintf("%v", tfvars)
		}
		zap.L().Debug("parseTfvars: no tfvars key in contents map", zap.Int("numKeys", len(m)))
	}
	return ""
}

// isActiveWorkflowStatus returns true if the workflow status indicates it's still running.
func isActiveWorkflowStatus(status string) bool {
	return status == "in-progress" || status == "approval-awaiting" || status == "pending"
}

// isTerminalWorkflowStatus returns true if the workflow is in a final state.
func isTerminalWorkflowStatus(status string) bool {
	return status == "completed" || status == "success" || status == "error" || status == "cancelled"
}

// normalizeStepName lowercases and replaces spaces with underscores for comparison.
func normalizeStepName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "_")
}

// mergeStepGroups applies all step-merging rules in sequence.
func mergeStepGroups(steps []*nuonmodels.AppWorkflowStep) []*nuonmodels.AppWorkflowStep {
	steps = mergeStackSteps(steps)
	steps = mergeSandboxSteps(steps)
	steps = mergeComponentSteps(steps)
	return steps
}

// mergeSandboxSteps merges "(re)provision_sandbox_plan" + "(re)provision_sandbox_apply_plan"
// into a single "Deploy sandbox" step.
func mergeSandboxSteps(steps []*nuonmodels.AppWorkflowStep) []*nuonmodels.AppWorkflowStep {
	planNames := map[string]bool{
		"provision_sandbox_plan":   true,
		"reprovision_sandbox_plan": true,
	}
	applyNames := map[string]bool{
		"provision_sandbox_apply_plan":   true,
		"reprovision_sandbox_apply_plan": true,
	}

	var merged []*nuonmodels.AppWorkflowStep
	i := 0
	for i < len(steps) {
		name := normalizeStepName(steps[i].Name)

		if planNames[name] && i+1 < len(steps) && applyNames[normalizeStepName(steps[i+1].Name)] {
			planStep := steps[i]
			applyStep := steps[i+1]
			groupSteps := []*nuonmodels.AppWorkflowStep{planStep, applyStep}
			active := findActiveStep(groupSteps)

			merged = append(merged, &nuonmodels.AppWorkflowStep{
				ID: active.ID, Name: "deploy_sandbox", Idx: planStep.Idx,
				Status: active.Status, Retryable: anyRetryable(groupSteps),
				ExecutionType: planStep.ExecutionType, Approval: planStep.Approval,
				StepTargetID: active.StepTargetID, StepTargetType: active.StepTargetType,
				Metadata: active.Metadata,
			})
			i += 2
			continue
		}

		merged = append(merged, steps[i])
		i++
	}
	return merged
}

// mergeStackSteps folds internal stack-management steps into their user-visible anchors:
//   - "generate_install_state" and "(re)provision_runner_service_account" → into "generate/create_install_stack"
//   - "await_runner_health(y)" → into the preceding "update_install_stack_outputs"
func mergeStackSteps(steps []*nuonmodels.AppWorkflowStep) []*nuonmodels.AppWorkflowStep {
	preStackAbsorbable := map[string]bool{
		"generate_install_state":             true,
		"provision_runner_service_account":   true,
		"reprovision_runner_service_account": true,
	}

	postOutputsAbsorbable := map[string]bool{
		"await_runner_health":  true,
		"await_runner_healthy": true,
	}

	var merged []*nuonmodels.AppWorkflowStep
	i := 0
	for i < len(steps) {
		name := normalizeStepName(steps[i].Name)

		// Rule 1: absorb preceding steps into generate/create install stack
		if name == "generate_install_stack" || name == "create_install_stack" {
			anchor := steps[i]

			var groupSteps []*nuonmodels.AppWorkflowStep
			for len(merged) > 0 && preStackAbsorbable[normalizeStepName(merged[len(merged)-1].Name)] {
				groupSteps = append(groupSteps, merged[len(merged)-1])
				merged = merged[:len(merged)-1]
			}
			for l, r := 0, len(groupSteps)-1; l < r; l, r = l+1, r-1 {
				groupSteps[l], groupSteps[r] = groupSteps[r], groupSteps[l]
			}
			groupSteps = append(groupSteps, anchor)

			active := findActiveStep(groupSteps)
			merged = append(merged, &nuonmodels.AppWorkflowStep{
				ID: active.ID, Name: anchor.Name, Idx: groupSteps[0].Idx,
				Status: active.Status, Retryable: anyRetryable(groupSteps),
				ExecutionType: anchor.ExecutionType, Approval: anchor.Approval,
				StepTargetID: active.StepTargetID, StepTargetType: active.StepTargetType,
				Metadata: active.Metadata,
			})
			i++
			continue
		}

		// Rule 2: absorb trailing await_runner_health into anchor step
		if name == "update_install_stack_outputs" || name == "reprovision_sandbox_dns_if_enabled" {
			anchor := steps[i]
			groupSteps := []*nuonmodels.AppWorkflowStep{anchor}
			j := i + 1
			for j < len(steps) && postOutputsAbsorbable[normalizeStepName(steps[j].Name)] {
				groupSteps = append(groupSteps, steps[j])
				j++
			}

			active := findActiveStep(groupSteps)
			merged = append(merged, &nuonmodels.AppWorkflowStep{
				ID: active.ID, Name: anchor.Name, Idx: anchor.Idx,
				Status: active.Status, Retryable: anyRetryable(groupSteps),
				ExecutionType: anchor.ExecutionType, Approval: anchor.Approval,
				StepTargetID: active.StepTargetID, StepTargetType: active.StepTargetType,
				Metadata: active.Metadata,
			})
			i = j
			continue
		}

		merged = append(merged, steps[i])
		i++
	}
	return merged
}

// findActiveStep returns the first non-completed step, or the last step if all completed.
func findActiveStep(steps []*nuonmodels.AppWorkflowStep) *nuonmodels.AppWorkflowStep {
	for _, s := range steps {
		status := ""
		if s.Status != nil {
			status = string(s.Status.Status)
		}
		switch status {
		case "completed", "success", "approved":
			continue
		default:
			return s
		}
	}
	return steps[len(steps)-1]
}

// anyRetryable returns true if any step in the slice is retryable.
func anyRetryable(steps []*nuonmodels.AppWorkflowStep) bool {
	for _, s := range steps {
		if s.Retryable {
			return true
		}
	}
	return false
}

// isComponentActionRun checks if a step name is a pre- or post-deploy action run
// for the given component name (already normalized).
func isComponentActionRun(normalizedName, componentName string) bool {
	return strings.HasSuffix(normalizedName, "(pre-deploy-component)") ||
		strings.HasSuffix(normalizedName, "(post-deploy-component)")
}

// mergeComponentSteps merges consecutive component steps into a single "deploy_X"
// virtual step. It combines: optional pre-deploy action runs, "sync_and_plan_X",
// "apply_X", and optional post-deploy action runs.
func mergeComponentSteps(steps []*nuonmodels.AppWorkflowStep) []*nuonmodels.AppWorkflowStep {
	var merged []*nuonmodels.AppWorkflowStep
	i := 0
	for i < len(steps) {
		planName := normalizeStepName(steps[i].Name)

		// Check if this is a "sync_and_plan_X" step followed by "apply_X"
		if strings.HasPrefix(planName, "sync_and_plan_") && i+1 < len(steps) {
			componentName := strings.TrimPrefix(planName, "sync_and_plan_")
			nextName := normalizeStepName(steps[i+1].Name)

			if strings.HasPrefix(nextName, "apply_") && strings.TrimPrefix(nextName, "apply_") == componentName {
				planStep := steps[i]
				applyStep := steps[i+1]

				// Absorb any pre-deploy action runs already appended to merged
				var preSteps []*nuonmodels.AppWorkflowStep
				for len(merged) > 0 {
					lastName := normalizeStepName(merged[len(merged)-1].Name)
					if strings.HasSuffix(lastName, "(pre-deploy-component)") {
						preSteps = append(preSteps, merged[len(merged)-1])
						merged = merged[:len(merged)-1]
					} else {
						break
					}
				}
				// Reverse preSteps to maintain original order
				for left, right := 0, len(preSteps)-1; left < right; left, right = left+1, right-1 {
					preSteps[left], preSteps[right] = preSteps[right], preSteps[left]
				}

				// Collect all steps in this component group
				groupSteps := append(preSteps, planStep, applyStep)
				j := i + 2

				// Consume trailing post-deploy action runs
				for j < len(steps) && isComponentActionRun(normalizeStepName(steps[j].Name), componentName) {
					groupSteps = append(groupSteps, steps[j])
					j++
				}

				active := findActiveStep(groupSteps)

				virtualStep := &nuonmodels.AppWorkflowStep{
					ID:             active.ID,
					Name:           "deploy_" + componentName,
					Idx:            planStep.Idx,
					Status:         active.Status,
					Retryable:      anyRetryable(groupSteps),
					ExecutionType:  planStep.ExecutionType,
					Approval:       planStep.Approval,
					StepTargetID:   active.StepTargetID,
					StepTargetType: active.StepTargetType,
					Metadata:       active.Metadata,
				}

				merged = append(merged, virtualStep)
				i = j
				continue
			}
		}

		merged = append(merged, steps[i])
		i++
	}
	return merged
}

// groupStepsIntoPhases groups workflow steps into logical provision phases.
func groupStepsIntoPhases(wf *nuonmodels.AppWorkflow, isReprovision ...bool) []partials.ProvisionPhase {
	if wf == nil || len(wf.Steps) == 0 {
		return nil
	}

	// Sort steps by index
	steps := make([]*nuonmodels.AppWorkflowStep, len(wf.Steps))
	copy(steps, wf.Steps)
	slices.SortFunc(steps, func(a, b *nuonmodels.AppWorkflowStep) int {
		return cmp.Compare(a.Idx, b.Idx)
	})

	// Merge related steps (stack setup + component plan/apply/action-runs)
	steps = mergeStepGroups(steps)

	// Determine phase layout based on workflow type
	type phaseLayout struct {
		names      []string
		phaseCount int // 1, 2, or 3
	}

	layout := phaseLayout{}
	wfType := string(wf.Type)

	switch wfType {
	case "provision":
		layout = phaseLayout{names: []string{"Install stack", "Provision sandbox", "Deploy app"}, phaseCount: 3}
	case "reprovision":
		layout = phaseLayout{names: []string{"Update stack", "Update sandbox", "Update app"}, phaseCount: 3}
	case "deprovision":
		layout = phaseLayout{names: []string{"Teardown components", "Deprovision sandbox", "Remove stack"}, phaseCount: 3}
	case "reprovision_sandbox", "drift_run_reprovision_sandbox":
		layout = phaseLayout{names: []string{"Provision sandbox", "Deploy app"}, phaseCount: 2}
	case "input_update":
		layout = phaseLayout{names: []string{"Update inputs", "Deploy components"}, phaseCount: 2}
	case "app_branches_manual_update", "app_branches_config_repo_update", "app_branches_component_repo_update":
		layout = phaseLayout{names: []string{"Update branches", "Deploy components"}, phaseCount: 2}
	case "deprovision_sandbox":
		layout = phaseLayout{names: []string{"Deprovision sandbox"}, phaseCount: 1}
	case "manual_deploy", "deploy_components":
		layout = phaseLayout{names: []string{"Deploy components"}, phaseCount: 1}
	case "teardown_component":
		layout = phaseLayout{names: []string{"Teardown component"}, phaseCount: 1}
	case "teardown_components":
		layout = phaseLayout{names: []string{"Teardown components"}, phaseCount: 1}
	case "action_workflow_run":
		layout = phaseLayout{names: []string{"Run action"}, phaseCount: 1}
	case "sync_secrets":
		layout = phaseLayout{names: []string{"Sync secrets"}, phaseCount: 1}
	case "drift_run":
		layout = phaseLayout{names: []string{"Drift check"}, phaseCount: 1}
	default:
		// Fallback: use isReprovision param or default provision layout
		reprov := len(isReprovision) > 0 && isReprovision[0]
		if reprov {
			layout = phaseLayout{names: []string{"Update stack", "Update sandbox", "Update app"}, phaseCount: 3}
		} else {
			layout = phaseLayout{names: []string{"Install stack", "Provision sandbox", "Deploy app"}, phaseCount: 3}
		}
	}

	// Assign steps to phases based on layout
	phaseSteps := make([][]partials.PhaseStep, layout.phaseCount)

	for _, step := range steps {
		stepStatus := ""
		if step.Status != nil {
			stepStatus = string(step.Status.Status)
		}

		ps := partials.PhaseStep{
			Name:      formatStepName(step.Name),
			Status:    stepStatus,
			ID:        step.ID,
			Retryable: step.Retryable,
		}

		// Use the original step index (preserved through merges) for phase assignment
		origIdx := int(step.Idx)

		switch layout.phaseCount {
		case 1:
			phaseSteps[0] = append(phaseSteps[0], ps)
		case 2:
			if origIdx < 5 {
				phaseSteps[0] = append(phaseSteps[0], ps)
			} else {
				phaseSteps[1] = append(phaseSteps[1], ps)
			}
		default: // 3
			switch {
			case origIdx < 5:
				phaseSteps[0] = append(phaseSteps[0], ps)
			case origIdx < 10:
				phaseSteps[1] = append(phaseSteps[1], ps)
			default:
				phaseSteps[2] = append(phaseSteps[2], ps)
			}
		}
	}

	// Build phases — include all defined phases for consistent layout
	var phases []partials.ProvisionPhase
	for i, name := range layout.names {
		phase := partials.ProvisionPhase{
			Name:  name,
			Steps: phaseSteps[i],
		}

		if len(phaseSteps[i]) == 0 {
			phase.Status = "not_started"
			phases = append(phases, phase)
			continue
		}

		// Derive phase status from steps
		allCompleted := true
		activeStepIdx := -1
		for j, s := range phase.Steps {
			switch s.Status {
			case "error":
				phase.Status = "failed"
				phase.ActiveStepName = s.Name
				allCompleted = false
			case "in-progress", "approval-awaiting":
				if phase.Status != "failed" {
					phase.Status = "in_progress"
					phase.ActiveStepName = s.Name
					activeStepIdx = j
				}
				allCompleted = false
			case "completed", "success", "approved":
				// continue
			default:
				allCompleted = false
			}
		}

		if allCompleted && phase.Status == "" {
			phase.Status = "completed"
		} else if phase.Status == "" {
			phase.Status = "not_started"
		}

		if activeStepIdx >= 0 {
			phase.StepProgress = fmt.Sprintf("Step %d of %d", activeStepIdx+1, len(phase.Steps))
		}

		phases = append(phases, phase)
	}

	return phases
}

// InstallReadmeStatus handles HTMX polling for readme display
