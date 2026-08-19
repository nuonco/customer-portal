package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/nuon/sdks/nuon-go/models"
	"go.uber.org/zap"

	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	localModels "github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials/workflows"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func autoSelectGroup(groups []workflows.StepGroup) int {
	for i, g := range groups {
		switch g.Status {
		case "in-progress", "approval-awaiting", "error":
			return i
		}
	}
	if len(groups) > 0 {
		return 0
	}
	return -1
}

// ApproveWorkflowStep handles workflow step approval
func (h *Handler) ApproveWorkflowStep(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*localModels.Install)
	workflowID := c.Param("workflow_id")

	stepID := c.PostForm("step_id")
	approvalID := c.PostForm("approval_id")
	if stepID == "" || approvalID == "" {
		c.String(http.StatusBadRequest, "step_id and approval_id are required")
		return
	}

	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		c.String(http.StatusInternalServerError, "Organization not found")
		return
	}

	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to initialize Nuon client")
		return
	}

	if err := nuonClient.ApproveWorkflowStep(c.Request.Context(), workflowID, stepID, approvalID); err != nil {
		h.logger.Error("failed to approve workflow step", zap.Error(err))
		h.redirectBackWithAlert(c, install.ID, workflowID, "error", "Failed to approve workflow step")
		return
	}

	h.logger.Info("workflow step approved",
		zap.String("email", user.Email),
		zap.String("user_id", user.ID),
		zap.String("step_id", stepID),
		zap.String("workflow_id", workflowID),
	)

	h.redirectBackWithAlert(c, install.ID, workflowID, "success", "Workflow step approved successfully")
}

// CancelWorkflow handles workflow cancellation
func (h *Handler) CancelWorkflow(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*localModels.Install)
	workflowID := c.Param("workflow_id")

	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		c.String(http.StatusInternalServerError, "Organization not found")
		return
	}

	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to initialize Nuon client")
		return
	}

	if err := nuonClient.CancelWorkflow(c.Request.Context(), workflowID); err != nil {
		h.logger.Error("failed to cancel workflow", zap.Error(err))
		h.redirectBackWithAlert(c, install.ID, workflowID, "error", "Failed to cancel workflow")
		return
	}

	h.logger.Info("workflow cancelled",
		zap.String("email", user.Email),
		zap.String("user_id", user.ID),
		zap.String("workflow_id", workflowID),
	)

	h.redirectBackWithAlert(c, install.ID, workflowID, "success", "Workflow cancelled successfully")
}

// ApproveAllWorkflowSteps approves any currently-pending approval steps and sets
// the approve-all flag so future steps are auto-approved.
func (h *Handler) ApproveAllWorkflowSteps(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*localModels.Install)
	workflowID := c.Param("workflow_id")

	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		c.String(http.StatusInternalServerError, "Organization not found")
		return
	}

	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to initialize Nuon client")
		return
	}

	ctx := c.Request.Context()

	// Approve any steps that are currently awaiting approval.
	workflow, err := nuonClient.GetWorkflow(ctx, workflowID)
	if err != nil {
		h.logger.Error("failed to get workflow for approve-all", zap.Error(err))
		h.redirectBackWithAlert(c, install.ID, workflowID, "error", "Failed to approve all workflow steps")
		return
	}

	for _, step := range workflow.Steps {
		if step.Status != nil && string(step.Status.Status) == "approval-awaiting" && step.Approval != nil {
			if err := nuonClient.ApproveWorkflowStep(ctx, workflowID, step.ID, step.Approval.ID); err != nil {
				h.logger.Error("failed to approve step during approve-all",
					zap.String("step_id", step.ID),
					zap.Error(err),
				)
			}
		}
	}

	// Set the approve-all flag so future approval steps are auto-approved.
	if err := nuonClient.ApproveAllWorkflowSteps(ctx, workflowID); err != nil {
		h.logger.Error("failed to set workflow approve-all", zap.Error(err))
		h.redirectBackWithAlert(c, install.ID, workflowID, "error", "Failed to approve all workflow steps")
		return
	}

	h.logger.Info("workflow approve-all set",
		zap.String("email", user.Email),
		zap.String("user_id", user.ID),
		zap.String("workflow_id", workflowID),
	)

	h.redirectBackWithAlert(c, install.ID, workflowID, "success", "Workflow approved successfully")
}

// RetryWorkflowStep handles retrying a failed workflow step
func (h *Handler) RetryWorkflowStep(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*localModels.Install)
	workflowID := c.Param("workflow_id")
	stepID := c.Param("step_id")

	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		c.String(http.StatusInternalServerError, "Organization not found")
		return
	}

	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to initialize Nuon client")
		return
	}

	if err := nuonClient.RetryWorkflowStep(c.Request.Context(), workflowID, stepID); err != nil {
		h.logger.Error("failed to retry workflow step", zap.Error(err))
		h.redirectBackWithAlert(c, install.ID, workflowID, "error", "Failed to retry workflow step")
		return
	}

	h.logger.Info("workflow step retry initiated",
		zap.String("email", user.Email),
		zap.String("user_id", user.ID),
		zap.String("step_id", stepID),
		zap.String("workflow_id", workflowID),
	)

	h.redirectBackWithAlert(c, install.ID, workflowID, "success", "Workflow step retry initiated")
}

// formatStepName converts "await_install_stack" to "Await install stack"
func formatStepName(stepName string) string {
	if stepName == "" {
		return ""
	}
	// Replace underscores with spaces
	formatted := strings.ReplaceAll(stepName, "_", " ")
	// Capitalize only the first letter
	if len(formatted) > 0 {
		formatted = strings.ToUpper(formatted[:1]) + formatted[1:]
	}
	return formatted
}

// BuildWorkflowDataPanel builds a WorkflowDataPanel directly from a workflow, without merging steps.
func BuildWorkflowDataPanel(workflow *models.AppWorkflow) workflows.WorkflowDataPanel {
	name := getCustomerFriendlyWorkflowName(string(workflow.Type))
	if workflow.Name != "" {
		name = workflow.Name
	}

	status := "unknown"
	statusClass := "bg-gray-100 text-gray-800"
	if workflow.Status != nil {
		status = string(workflow.Status.Status)
		statusClass = getStatusClass(status)
	}

	panel := workflows.WorkflowDataPanel{
		ID:          workflow.ID,
		Name:        name,
		Status:      status,
		StatusClass: statusClass,
		CreatedAt:   parseWorkflowTime(workflow.CreatedAt),
		FinishedAt:  parseWorkflowTime(workflow.FinishedAt),
	}

	// Check for approval steps
	hasApprovalSteps := false
	hasPendingApproval := false
	if workflow.Steps != nil {
		for _, step := range workflow.Steps {
			if step.ExecutionType == "approval" {
				hasApprovalSteps = true
				if step.Status != nil && string(step.Status.Status) == "approval-awaiting" {
					hasPendingApproval = true
				}
			}
		}
	}
	panel.HasApprovalSteps = hasApprovalSteps

	// Determine actions based on status
	switch status {
	case "approval-awaiting":
		panel.CanApprove = true
		panel.CanApproveAll = hasApprovalSteps && hasPendingApproval
		panel.CanCancel = true
		if hasApprovalSteps && !hasPendingApproval {
			panel.ApproveAllDisabledReason = "All steps have been approved"
		}
	case "in-progress":
		panel.CanCancel = true
		panel.ApproveDisabledReason = "Workflow is currently running"
		if hasApprovalSteps && hasPendingApproval {
			panel.CanApproveAll = true
		} else if hasApprovalSteps {
			panel.ApproveAllDisabledReason = "All steps have been approved"
		} else {
			panel.ApproveAllDisabledReason = "No approval steps in this workflow"
		}
	case "completed", "success":
		panel.ApproveDisabledReason = "Workflow has already completed"
		panel.ApproveAllDisabledReason = "Workflow has already completed"
		panel.CancelDisabledReason = "Workflow has already completed"
	case "error":
		panel.ApproveDisabledReason = "Workflow failed and cannot be approved"
		panel.ApproveAllDisabledReason = "Workflow failed and cannot be approved"
		panel.CancelDisabledReason = "Workflow already failed"
	case "cancelled":
		panel.ApproveDisabledReason = "Workflow was cancelled"
		panel.ApproveAllDisabledReason = "Workflow was cancelled"
		panel.CancelDisabledReason = "Workflow is already cancelled"
	default:
		panel.ApproveDisabledReason = "Approval not available for this workflow status"
		if hasApprovalSteps && hasPendingApproval {
			panel.CanApproveAll = true
		} else if hasApprovalSteps {
			panel.ApproveAllDisabledReason = "All steps have been approved"
		} else {
			panel.ApproveAllDisabledReason = "No approval steps in this workflow"
		}
		panel.CancelDisabledReason = "Cannot cancel workflow in current state"
	}

	// Find approval step
	if panel.CanApprove && workflow.Steps != nil {
		for _, step := range workflow.Steps {
			if step.Status != nil && string(step.Status.Status) == "approval-awaiting" && step.Approval != nil {
				panel.ApprovalStep = &workflows.ApprovalStepDataPanel{
					StepID:     step.ID,
					ApprovalID: step.Approval.ID,
				}
				break
			}
		}
	}

	// Find current step (active > pending > initializing)
	var currentStepMetadata map[string]any
	if workflow.Steps != nil {
		panel.TotalSteps = len(workflow.Steps)
		panel.CurrentStepNumber = 1

		// First pass: active or approval-awaiting
		for i, step := range workflow.Steps {
			stepStatus := ""
			if step.Status != nil {
				stepStatus = string(step.Status.Status)
			}
			if stepStatus == "in-progress" || stepStatus == "approval-awaiting" {
				panel.CurrentStepName = formatStepName(step.Name)
				panel.CurrentStepStatus = stepStatus
				panel.CurrentStepType = stepStatus
				panel.CurrentStepNumber += i
				if step.Status != nil {
					currentStepMetadata = step.Status.Metadata
				}
				break
			}
		}

		// Second pass: first pending step
		if panel.CurrentStepName == "" && (status == "pending" || status == "in-progress") {
			for i, step := range workflow.Steps {
				stepStatus := ""
				if step.Status != nil {
					stepStatus = string(step.Status.Status)
				}
				if stepStatus == "pending" || stepStatus == "" {
					panel.CurrentStepName = formatStepName(step.Name)
					panel.CurrentStepType = "pending"
					panel.CurrentStepNumber += i
					break
				}
			}
		}
	}

	// Find failed step
	if status == "error" && workflow.Steps != nil {
		for _, step := range workflow.Steps {
			if step.Status != nil && string(step.Status.Status) == "error" {
				panel.FailedStepID = step.ID
				panel.FailedStepName = formatStepName(step.Name)
				panel.FailedStepRetryable = step.Retryable
				if currentStepMetadata == nil {
					currentStepMetadata = step.Status.Metadata
				}
				break
			}
		}
	}

	// Initializing placeholder
	if panel.CurrentStepName == "" && (status == "pending" || status == "in-progress") {
		panel.CurrentStepName = "Creating provision workflow..."
		panel.CurrentStepType = "initializing"
		panel.CurrentStepStatus = status
	}

	// Policy violations
	deny, warn := extractPolicyViolations(currentStepMetadata)
	panel.DenyViolations = deny
	panel.WarnViolations = warn
	panel.HasPolicyData = len(deny) > 0 || len(warn) > 0

	return panel
}

// extractPolicyViolations extracts deny and warn violations from workflow step metadata.
func extractPolicyViolations(metadata map[string]any) ([]workflows.PolicyViolation, []workflows.PolicyViolation) {
	if metadata == nil {
		return nil, nil
	}

	extract := func(key string, severity string) []workflows.PolicyViolation {
		raw, ok := metadata[key]
		if !ok {
			return nil
		}
		items, ok := raw.([]interface{})
		if !ok {
			return nil
		}
		var violations []workflows.PolicyViolation
		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			policyID, _ := m["policy_id"].(string)
			message, _ := m["message"].(string)
			violations = append(violations, workflows.PolicyViolation{
				PolicyID: policyID,
				Message:  message,
				Severity: severity,
			})
		}
		return violations
	}

	return extract("deny_violations", "deny"), extract("warn_violations", "warn")
}

// resolveCurrentStepRole fetches the IAM role for the current active workflow step's target.
// Returns empty string if no role is available or on any error (non-fatal).
func resolveCurrentStepRole(ctx context.Context, client *nuon.Client, installID string, wf *models.AppWorkflow) string {
	if wf == nil || wf.Steps == nil {
		return ""
	}
	// Find the active step
	for _, step := range wf.Steps {
		if step.Status == nil {
			continue
		}
		status := string(step.Status.Status)
		if status == "in-progress" || status == "approval-awaiting" || status == "pending" {
			if step.StepTargetID == "" || step.StepTargetType == "" {
				return ""
			}
			role, err := client.GetStepTargetRole(ctx, step.StepTargetID, step.StepTargetType, installID)
			if err != nil {
				return ""
			}
			return role
		}
	}
	return ""
}

// parseWorkflowTime converts various time formats to time.Time for template usage
func parseWorkflowTime(timeValue interface{}) time.Time {
	if timeValue == nil {
		return time.Time{}
	}

	// Try direct time.Time
	if t, ok := timeValue.(time.Time); ok {
		return t
	}

	// Try *time.Time
	if tPtr, ok := timeValue.(*time.Time); ok && tPtr != nil {
		return *tPtr
	}

	// Try string parsing
	if timeStr, ok := timeValue.(string); ok {
		// Try RFC3339 format first
		if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
			return t
		}
		// Try alternative format
		if t, err := time.Parse("2006-01-02T15:04:05Z", timeStr); err == nil {
			return t
		}
	}

	// Return zero time if all parsing fails
	return time.Time{}
}

// getCustomerFriendlyWorkflowName converts technical workflow types to customer-friendly names
func getCustomerFriendlyWorkflowName(workflowType string) string {
	friendlyNames := map[string]string{
		"provision_sandbox":             "Initial Setup",
		"provision_install":             "Deploy Application",
		"reprovision_sandbox":           "Infrastructure Update",
		"reprovision_install":           "Application Update",
		"deprovision_sandbox":           "Infrastructure Cleanup",
		"deprovision_install":           "Application Removal",
		"update_inputs":                 "Configuration Update",
		"action_workflow_run":           "Custom Action",
		"drift_run":                     "Configuration Check",
		"drift_run_reprovision_sandbox": "Automated Infrastructure Fix",
	}

	if friendlyName, exists := friendlyNames[workflowType]; exists {
		return friendlyName
	}

	// Fallback: convert snake_case to Title Case
	parts := strings.Split(workflowType, "_")
	for i, part := range parts {
		parts[i] = strings.Title(part)
	}
	return strings.Join(parts, " ")
}

// getStatusClass returns CSS classes for workflow status
func getStatusClass(status string) string {
	statusClasses := map[string]string{
		"success":           "bg-green-100 text-green-800",
		"active":            "bg-green-100 text-green-800",
		"approved":          "bg-green-100 text-green-800",
		"in-progress":       "bg-blue-100 text-blue-800",
		"approval-awaiting": "bg-yellow-100 text-yellow-800",
		"pending":           "bg-yellow-100 text-yellow-800",
		"approval-denied":   "bg-red-100 text-red-800",
		"error":             "bg-red-100 text-red-800",
		"cancelled":         "bg-orange-100 text-orange-800",
		"discarded":         "bg-gray-100 text-gray-800",
	}

	if class, exists := statusClasses[status]; exists {
		return class
	}
	return "bg-gray-100 text-gray-800"
}

// extractCustomerInputMappings extracts input name -> CloudFormation parameter name mappings
// for inputs where source == "customer". The CF param name is in "cloudformation_stack_parameter_name".
// Returns a map of inputName -> cfParamName
func extractCustomerInputMappings(inputConfig interface{}) map[string]string {
	mappings := make(map[string]string)
	if inputConfig == nil {
		return mappings
	}

	// Input config structure: { input_groups: [ { app_inputs: [ { name: "", source: "", cloudformation_stack_parameter_name: "" } ] } ] }
	configMap, ok := inputConfig.(map[string]interface{})
	if !ok {
		return mappings
	}

	inputGroups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return mappings
	}

	for _, group := range inputGroups {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}

		appInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok {
			continue
		}

		for _, input := range appInputs {
			inputMap, ok := input.(map[string]interface{})
			if !ok {
				continue
			}

			source, _ := inputMap["source"].(string)
			name, _ := inputMap["name"].(string)
			cfParamName, _ := inputMap["cloudformation_stack_parameter_name"].(string)

			if source == "customer" && name != "" {
				// Use CF param name if available, otherwise fall back to input name
				if cfParamName != "" {
					mappings[name] = cfParamName
				} else {
					mappings[name] = name
				}
			}
		}
	}

	return mappings
}

// appendInputsToCloudFormationURL appends customer inputs as CloudFormation parameters to the URL
// inputMappings maps inputName -> CloudFormation parameter name
func appendInputsToCloudFormationURL(cfURL string, inputs map[string]string, inputMappings map[string]string) string {
	if len(inputMappings) == 0 {
		return cfURL
	}

	// CloudFormation quick create URLs have format:
	// https://console.aws.amazon.com/cloudformation/home#/stacks/quickcreate?templateUrl=...
	// Note: The # fragment means we need to handle query params after the fragment

	var params []string
	for inputName, cfParamName := range inputMappings {
		value := inputs[inputName]
		if value != "" {
			// CloudFormation parameters are prefixed with "param_"
			param := fmt.Sprintf("param_%s=%s",
				url.QueryEscape(cfParamName),
				url.QueryEscape(value))
			params = append(params, param)
		}
	}

	if len(params) == 0 {
		return cfURL
	}

	// Determine separator (? or &) based on existing URL structure
	separator := "&"
	if !strings.Contains(cfURL, "?") {
		separator = "?"
	}

	return cfURL + separator + strings.Join(params, "&")
}
