package handlers

import (
	"cmp"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/nuon-go/models"
	"go.uber.org/zap"

	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	localModels "github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/components"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

// WorkflowsPanel renders the workflow history content for the sliding panel (no layout wrapper)
func (h *Handler) WorkflowsPanel(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		h.logger.Error("install not found in context")
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*localModels.Install)
	h.logger.Debug("WorkflowsPanel: install loaded",
		zap.String("install_id", install.ID),
		zap.String("nuon_install_id", install.NuonInstallID),
	)

	// Get pagination parameters
	offsetParam := c.DefaultQuery("offset", "0")
	offset, err := strconv.Atoi(offsetParam)
	if err != nil {
		offset = 0
	}

	limit := 10

	// Fetch workflow data using helper function
	processedWorkflows, hasMoreFromAPI, err := h.fetchWorkflowData(c, install, offset, limit)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Group workflows by date and sort in descending order - convert to panel types
	orderedWorkflowGroups := groupAndSortWorkflowsByDatePanel(processedWorkflows)

	// Get theme
	orgID := h.getOrgIDForTheme(c)
	theme, _ := localModels.GetOrCreateAppTheme(h.db, orgID)
	primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)

	props := partials.WorkflowsPanelProps{
		Install:        install,
		WorkflowGroups: orderedWorkflowGroups,
		CurrentOffset:  offset,
		Limit:          limit,
		HasNext:        hasMoreFromAPI,
		HasPrev:        offset > 0,
		NextOffset:     offset + limit,
		PrevOffset:     max(0, offset-limit),
		BasePath:       h.basePath,
		PrimaryColor:   primaryColor,
	}
	h.RenderTempl(c, http.StatusOK, partials.WorkflowsPanel(props))
}

// groupAndSortWorkflowsByDatePanel groups workflows by date for panel display
func groupAndSortWorkflowsByDatePanel(workflows []gin.H) []partials.WorkflowGroupPanel {
	grouped := make(map[string][]partials.WorkflowDataPanel)

	for _, workflow := range workflows {
		var dateKey string
		if createdAt, ok := workflow["created_at"].(time.Time); ok && !createdAt.IsZero() {
			dateKey = createdAt.Format("2006-01-02")
		}
		if dateKey != "" {
			wfData := ginHToWorkflowDataPanel(workflow)
			grouped[dateKey] = append(grouped[dateKey], wfData)
		}
	}

	var dates []string
	for date := range grouped {
		dates = append(dates, date)
	}
	for i := 0; i < len(dates); i++ {
		for j := i + 1; j < len(dates); j++ {
			if dates[i] < dates[j] {
				dates[i], dates[j] = dates[j], dates[i]
			}
		}
	}

	var orderedGroups []partials.WorkflowGroupPanel
	for _, date := range dates {
		orderedGroups = append(orderedGroups, partials.WorkflowGroupPanel{
			Date:        date,
			DisplayDate: formatDateForDisplay(date),
			Workflows:   grouped[date],
		})
	}

	return orderedGroups
}

// ApproveWorkflowStep handles workflow step approval
func (h *Handler) ApproveWorkflowStep(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	// Get install from middleware
	installInterface, exists := c.Get("install")
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	install := installInterface.(*localModels.Install)
	workflowID := c.Param("workflow_id")

	// Handle both JSON (API) and form data (HTMX) submissions
	var stepID, approvalID string
	if isHTMXRequest(c) {
		// HTMX sends form data via hx-vals
		stepID = c.PostForm("step_id")
		approvalID = c.PostForm("approval_id")
	} else {
		// Traditional JSON API request
		var req struct {
			StepID     string `json:"step_id" binding:"required"`
			ApprovalID string `json:"approval_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		stepID = req.StepID
		approvalID = req.ApprovalID
	}

	if stepID == "" || approvalID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "step_id and approval_id are required"})
		return
	}

	// Load install with org info (handles both install-link and published-app installs)
	if err := h.loadInstallWithOrg(install); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization not found"})
		return
	}

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Approve the workflow step
	if err := nuonClient.ApproveWorkflowStep(c.Request.Context(), workflowID, stepID, approvalID); err != nil {
		h.logger.Error("failed to approve workflow step", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve workflow step"})
		return
	}

	h.logger.Info("workflow step approved",
		zap.String("email", user.Email),
		zap.String("user_id", user.ID),
		zap.String("step_id", stepID),
		zap.String("workflow_id", workflowID),
	)

	// For HTMX requests, return the updated workflow card partial
	if isHTMXRequest(c) {
		h.renderWorkflowCardPartial(c, install, workflowID, nuonClient)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workflow step approved successfully",
	})
}

// CancelWorkflow handles workflow cancellation
func (h *Handler) CancelWorkflow(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	// Get install from middleware
	installInterface, exists := c.Get("install")
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	install := installInterface.(*localModels.Install)
	workflowID := c.Param("workflow_id")

	// Load install with org info (handles both install-link and published-app installs)
	if err := h.loadInstallWithOrg(install); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization not found"})
		return
	}

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Cancel the workflow
	if err := nuonClient.CancelWorkflow(c.Request.Context(), workflowID); err != nil {
		h.logger.Error("failed to cancel workflow", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel workflow"})
		return
	}

	h.logger.Info("workflow cancelled",
		zap.String("email", user.Email),
		zap.String("user_id", user.ID),
		zap.String("workflow_id", workflowID),
	)

	// For HTMX requests, return the updated workflow status display
	if isHTMXRequest(c) {
		// Fetch updated workflow
		workflow, err := nuonClient.GetWorkflow(c.Request.Context(), workflowID)
		if err == nil {
			processed := processWorkflowForCustomer(workflow)
			wf := ginHToWorkflowDataPanel(processed)

			// Return just the status display HTML
			statusHTML := fmt.Sprintf(`
<div class="flex items-center justify-between mb-2">
	<h4 class="font-medium text-cool-grey-900 dark:text-white">%s</h4>
	<span class="text-xs px-2 py-1 rounded %s">%s</span>
</div>
<div class="text-sm text-cool-grey-500 dark:text-cool-grey-400 mb-3">
	%s
</div>
`, wf.Name, wf.StatusClass, wf.Status, wf.CreatedAt.Format("Jan 2, 2006 3:04 PM"))

			c.Data(http.StatusOK, "text/html", []byte(statusHTML))
			return
		}
		c.String(http.StatusOK, "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workflow cancelled successfully",
	})
}

// ApproveAllWorkflowSteps handles setting approve-all on a workflow
func (h *Handler) ApproveAllWorkflowSteps(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	// Get install from middleware
	installInterface, exists := c.Get("install")
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	install := installInterface.(*localModels.Install)
	workflowID := c.Param("workflow_id")

	h.logger.Debug("approving all workflow steps",
		zap.String("email", user.Email),
		zap.String("user_id", user.ID),
		zap.String("install_id", install.ID),
		zap.String("workflow_id", workflowID),
	)

	// Load install with org info (handles both install-link and published-app installs)
	if err := h.loadInstallWithOrg(install); err != nil {
		h.logger.Error("failed to load install details", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		h.logger.Error("organization not found for install", zap.String("install_id", install.ID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization not found"})
		return
	}

	h.logger.Debug("org info loaded",
		zap.String("org_id", nuonOrg.NuonOrgID),
		zap.Int("api_token_length", len(nuonOrg.APIToken)),
	)

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		h.logger.Error("failed to initialize Nuon client", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Set approve-all on the workflow
	if err := nuonClient.ApproveAllWorkflowSteps(c.Request.Context(), workflowID); err != nil {
		h.logger.Error("failed to set workflow approve-all", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set workflow approve-all"})
		return
	}

	h.logger.Info("workflow approve-all set",
		zap.String("email", user.Email),
		zap.String("user_id", user.ID),
		zap.String("workflow_id", workflowID),
	)

	// For HTMX requests, return the updated workflow status display
	if isHTMXRequest(c) {
		// Fetch updated workflow
		workflows, _, err := nuonClient.GetInstallWorkflowsByType(c.Request.Context(), install.NuonInstallID, 0, 1, "provision")
		if err == nil && len(workflows) > 0 {
			processed := processWorkflowForCustomer(workflows[0])
			wf := ginHToWorkflowDataPanel(processed)

			// Return just the status display HTML
			statusHTML := fmt.Sprintf(`
<div class="flex items-center justify-between mb-2">
	<h4 class="font-medium text-cool-grey-900 dark:text-white">%s</h4>
	<span class="text-xs px-2 py-1 rounded %s">%s</span>
</div>
<div class="text-sm text-cool-grey-500 dark:text-cool-grey-400 mb-3">
	%s
</div>
`, wf.Name, wf.StatusClass, wf.Status, wf.CreatedAt.Format("Jan 2, 2006 3:04 PM"))

			c.Data(http.StatusOK, "text/html", []byte(statusHTML))
			return
		}
		c.String(http.StatusOK, "")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workflow set to approve-all successfully",
	})
}

// RetryWorkflowStep handles retrying a failed workflow step
func (h *Handler) RetryWorkflowStep(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	// Get install from middleware
	installInterface, exists := c.Get("install")
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	install := installInterface.(*localModels.Install)
	workflowID := c.Param("workflow_id")
	stepID := c.Param("step_id")

	// Load install with org info
	if err := h.loadInstallWithOrg(install); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization not found"})
		return
	}

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Retry the failed step
	if err := nuonClient.RetryWorkflowStep(c.Request.Context(), workflowID, stepID); err != nil {
		h.logger.Error("failed to retry workflow step", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retry workflow step"})
		return
	}

	h.logger.Info("workflow step retry initiated",
		zap.String("email", user.Email),
		zap.String("user_id", user.ID),
		zap.String("step_id", stepID),
		zap.String("workflow_id", workflowID),
	)

	// For HTMX requests, return the updated workflow card partial
	if isHTMXRequest(c) {
		h.renderWorkflowCardPartial(c, install, workflowID, nuonClient)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workflow step retry initiated successfully",
	})
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

// processWorkflowForCustomer converts technical workflow data to customer-friendly format
func processWorkflowForCustomer(workflow *models.AppWorkflow) gin.H {
	// Convert technical workflow types to customer-friendly names
	workflowName := getCustomerFriendlyWorkflowName(string(workflow.Type))
	if workflow.Name != "" {
		workflowName = workflow.Name
	}

	// Determine status and actions
	status := "unknown"
	statusClass := "bg-gray-100 text-gray-800"
	canApprove := false
	canApproveAll := false
	canCancel := false
	hasApprovalSteps := false
	hasPendingApprovalSteps := false
	approveDisabledReason := ""
	approveAllDisabledReason := ""
	cancelDisabledReason := ""

	if workflow.Status != nil {
		status = string(workflow.Status.Status)
		statusClass = getStatusClass(string(workflow.Status.Status))

		// Check for approval steps in the workflow based on ExecutionType
		if workflow.Steps != nil {
			for _, step := range workflow.Steps {
				// Check ExecutionType for approval steps
				if step.ExecutionType == "approval" {
					hasApprovalSteps = true
					// Check if this approval step is still pending
					if step.Status != nil && string(step.Status.Status) == "approval-awaiting" {
						hasPendingApprovalSteps = true
					}
				}
			}
		}

		// Check if workflow needs approval or can be cancelled
		switch string(workflow.Status.Status) {
		case "approval-awaiting":
			canApprove = true
			canApproveAll = hasApprovalSteps && hasPendingApprovalSteps
			if hasApprovalSteps && !hasPendingApprovalSteps {
				approveAllDisabledReason = "All steps have been approved"
			}
			canCancel = true
		case "in-progress":
			canCancel = true
			approveDisabledReason = "Workflow is currently running"
			// Approve All is available if there are pending approval steps
			if hasApprovalSteps && hasPendingApprovalSteps {
				canApproveAll = true
			} else if hasApprovalSteps && !hasPendingApprovalSteps {
				approveAllDisabledReason = "All steps have been approved"
			} else {
				approveAllDisabledReason = "No approval steps in this workflow"
			}
		case "completed", "success":
			approveDisabledReason = "Workflow has already completed"
			approveAllDisabledReason = "Workflow has already completed"
			cancelDisabledReason = "Workflow has already completed"
		case "error":
			approveDisabledReason = "Workflow failed and cannot be approved"
			approveAllDisabledReason = "Workflow failed and cannot be approved"
			cancelDisabledReason = "Workflow already failed"
		case "cancelled":
			approveDisabledReason = "Workflow was cancelled"
			approveAllDisabledReason = "Workflow was cancelled"
			cancelDisabledReason = "Workflow is already cancelled"
		default:
			approveDisabledReason = "Approval not available for this workflow status"
			if hasApprovalSteps && hasPendingApprovalSteps {
				canApproveAll = true
			} else if hasApprovalSteps && !hasPendingApprovalSteps {
				approveAllDisabledReason = "All steps have been approved"
			} else {
				approveAllDisabledReason = "No approval steps in this workflow"
			}
			cancelDisabledReason = "Cannot cancel workflow in current state"
		}
	} else {
		approveDisabledReason = "Workflow status unknown"
		approveAllDisabledReason = "Workflow status unknown"
		cancelDisabledReason = "Workflow status unknown"
	}

	// Find approval step if workflow needs approval
	var approvalStep gin.H
	if canApprove && workflow.Steps != nil {
		for _, step := range workflow.Steps {
			if step.Status != nil && string(step.Status.Status) == "approval-awaiting" && step.Approval != nil {
				approvalStep = gin.H{
					"step_id":     step.ID,
					"approval_id": step.Approval.ID,
				}
				break
			}
		}
	}

	// Convert dates to proper time.Time objects for template usage
	createdAt := parseWorkflowTime(workflow.CreatedAt)
	finishedAt := parseWorkflowTime(workflow.FinishedAt)

	// Find the current or next step to display (three-pass algorithm)
	var currentStepName string
	var currentStepStatus string
	var currentStepType string
	var currentStepNumber int = 0
	var totalSteps int

	slices.SortFunc(workflow.Steps, func(a, b *models.AppWorkflowStep) int {
		return cmp.Compare(a.Idx, b.Idx)
	})

	if workflow.Steps != nil {
		totalSteps = len(workflow.Steps)
		currentStepNumber = 1

		// First pass: Look for active or approval-awaiting steps (priority)
		for i, step := range workflow.Steps {
			stepStatus := ""
			if step.Status != nil {
				stepStatus = string(step.Status.Status)
			}

			if stepStatus == "in-progress" {
				currentStepName = formatStepName(step.Name)
				currentStepStatus = stepStatus
				currentStepType = "in-progress"
				currentStepNumber += i
				break
			} else if stepStatus == "approval-awaiting" {
				currentStepName = formatStepName(step.Name)
				currentStepStatus = stepStatus
				currentStepType = "approval-awaiting"
				currentStepNumber += i
				break
			}
		}

		// Second pass: If no active step found and workflow is pending/in-progress,
		// show the first pending step as "waiting to start"
		if currentStepName == "" && (status == "pending" || status == "in-progress") {
			for i, step := range workflow.Steps {
				stepStatus := ""
				if step.Status != nil {
					stepStatus = string(step.Status.Status)
				}

				// Find first pending or uninitialized step
				if stepStatus == "pending" || stepStatus == "" {
					currentStepName = formatStepName(step.Name)
					currentStepType = "pending"
					currentStepNumber += i
					break
				}
			}
		}
	}

	// Fourth pass: Find the failed step for retry functionality
	var failedStepID string
	var failedStepName string
	var failedStepRetryable bool
	if status == "error" && workflow.Steps != nil {
		for _, step := range workflow.Steps {
			if step.Status != nil && string(step.Status.Status) == "error" {
				failedStepID = step.ID
				failedStepName = formatStepName(step.Name)
				failedStepRetryable = step.Retryable
				break
			}
		}
	}

	// Third pass: Placeholder step name while waiting for workflow steps to be created
	// This handles the case where workflow is active but Steps array is empty/nil
	if currentStepName == "" && (status == "pending" || status == "in-progress") {
		currentStepName = "Creating provision workflow..."
		currentStepType = "initializing"
		currentStepStatus = status
		// currentStepNumber remains 0 (no progress indicator)
	}

	return gin.H{
		"id":                          workflow.ID,
		"name":                        workflowName,
		"status":                      status,
		"status_class":                statusClass,
		"created_at":                  createdAt,
		"finished_at":                 finishedAt,
		"can_approve":                 canApprove,
		"can_approve_all":             canApproveAll,
		"can_cancel":                  canCancel,
		"has_approval_steps":          hasApprovalSteps,
		"approval_step":               approvalStep,
		"approve_disabled_reason":     approveDisabledReason,
		"approve_all_disabled_reason": approveAllDisabledReason,
		"cancel_disabled_reason":      cancelDisabledReason,
		"current_step_name":           currentStepName,
		"current_step_status":         currentStepStatus,
		"current_step_type":           currentStepType,
		"current_step_number":         currentStepNumber,
		"total_steps":                 totalSteps,
		"failed_step_id":              failedStepID,
		"failed_step_name":            failedStepName,
		"failed_step_retryable":       failedStepRetryable,
	}
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

// WorkflowGroup represents a group of workflows for a specific date
type WorkflowGroup struct {
	Date        string  `json:"date"`         // Raw date (2006-01-02)
	DisplayDate string  `json:"display_date"` // Formatted date for display
	Workflows   []gin.H `json:"workflows"`
}

// ginHToWorkflowData converts gin.H workflow data to customerui.WorkflowData
func ginHToWorkflowData(wf gin.H) customerui.WorkflowData {
	data := customerui.WorkflowData{
		ID:                       getString(wf, "id"),
		Name:                     getString(wf, "name"),
		Status:                   getString(wf, "status"),
		StatusClass:              getString(wf, "status_class"),
		CanApprove:               getBool(wf, "can_approve"),
		CanApproveAll:            getBool(wf, "can_approve_all"),
		CanCancel:                getBool(wf, "can_cancel"),
		ApproveDisabledReason:    getString(wf, "approve_disabled_reason"),
		ApproveAllDisabledReason: getString(wf, "approve_all_disabled_reason"),
		CancelDisabledReason:     getString(wf, "cancel_disabled_reason"),
	}

	// Handle time fields
	if t, ok := wf["created_at"].(time.Time); ok {
		data.CreatedAt = t
	}
	if t, ok := wf["finished_at"].(time.Time); ok {
		data.FinishedAt = t
	}

	// Handle approval step
	if step, ok := wf["approval_step"].(gin.H); ok && step != nil {
		data.ApprovalStep = &customerui.ApprovalStepData{
			StepID:     getString(step, "step_id"),
			ApprovalID: getString(step, "approval_id"),
		}
	}

	return data
}

// ginHToWorkflowDataPanel converts gin.H workflow data to partials.WorkflowDataPanel
func ginHToWorkflowDataPanel(wf gin.H) partials.WorkflowDataPanel {
	data := partials.WorkflowDataPanel{
		ID:                       getString(wf, "id"),
		Name:                     getString(wf, "name"),
		Status:                   getString(wf, "status"),
		StatusClass:              getString(wf, "status_class"),
		CanApprove:               getBool(wf, "can_approve"),
		CanApproveAll:            getBool(wf, "can_approve_all"),
		CanCancel:                getBool(wf, "can_cancel"),
		HasApprovalSteps:         getBool(wf, "has_approval_steps"),
		ApproveDisabledReason:    getString(wf, "approve_disabled_reason"),
		ApproveAllDisabledReason: getString(wf, "approve_all_disabled_reason"),
		CancelDisabledReason:     getString(wf, "cancel_disabled_reason"),
		CurrentStepName:          getString(wf, "current_step_name"),
		CurrentStepStatus:        getString(wf, "current_step_status"),
		CurrentStepType:          getString(wf, "current_step_type"),
		CurrentStepNumber:        getInt(wf, "current_step_number"),
		TotalSteps:               getInt(wf, "total_steps"),
		FailedStepID:             getString(wf, "failed_step_id"),
		FailedStepName:           getString(wf, "failed_step_name"),
		FailedStepRetryable:      getBool(wf, "failed_step_retryable"),
	}

	// Handle time fields
	if t, ok := wf["created_at"].(time.Time); ok {
		data.CreatedAt = t
	}
	if t, ok := wf["finished_at"].(time.Time); ok {
		data.FinishedAt = t
	}

	// Handle approval step
	if step, ok := wf["approval_step"].(gin.H); ok && step != nil {
		data.ApprovalStep = &partials.ApprovalStepDataPanel{
			StepID:     getString(step, "step_id"),
			ApprovalID: getString(step, "approval_id"),
		}
	}

	return data
}

// groupAndSortWorkflowsByDate groups workflows by date and returns them in descending order
func groupAndSortWorkflowsByDate(workflows []gin.H) []WorkflowGroup {
	// First, group workflows by date
	grouped := make(map[string][]gin.H)

	for _, workflow := range workflows {
		var dateKey string
		var success bool

		// Try direct time.Time
		if createdAt, ok := workflow["created_at"].(time.Time); ok {
			if !createdAt.IsZero() {
				dateKey = createdAt.Format("2006-01-02")
				success = true
			}
		} else if createdAtPtr, ok := workflow["created_at"].(*time.Time); ok && createdAtPtr != nil {
			// Try *time.Time
			if !createdAtPtr.IsZero() {
				dateKey = createdAtPtr.Format("2006-01-02")
				success = true
			}
		} else if createdAtStr, ok := workflow["created_at"].(string); ok {
			// Try string parsing (ISO format)
			if parsedTime, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
				dateKey = parsedTime.Format("2006-01-02")
				success = true
			} else if parsedTime, err := time.Parse("2006-01-02T15:04:05Z", createdAtStr); err == nil {
				dateKey = parsedTime.Format("2006-01-02")
				success = true
			}
		}

		if success && dateKey != "" {
			grouped[dateKey] = append(grouped[dateKey], workflow)
		}
	}

	// Extract and sort dates in descending order (newest first)
	var dates []string
	for date := range grouped {
		dates = append(dates, date)
	}

	// Sort dates in descending order (newest first)
	// Since dates are in YYYY-MM-DD format, string comparison works correctly
	for i := 0; i < len(dates); i++ {
		for j := i + 1; j < len(dates); j++ {
			if dates[i] < dates[j] {
				dates[i], dates[j] = dates[j], dates[i]
			}
		}
	}

	// Build ordered workflow groups
	var orderedGroups []WorkflowGroup
	for _, date := range dates {
		orderedGroups = append(orderedGroups, WorkflowGroup{
			Date:        date,
			DisplayDate: formatDateForDisplay(date),
			Workflows:   grouped[date],
		})
	}

	zap.L().Debug("grouped workflows by date",
		zap.Int("group_count", len(orderedGroups)),
		zap.Int("workflow_count", len(workflows)),
	)

	return orderedGroups
}

// formatDateForDisplay formats date for customer display
func formatDateForDisplay(date string) string {
	if t, err := time.Parse("2006-01-02", date); err == nil {
		now := time.Now()
		if t.Format("2006-01-02") == now.Format("2006-01-02") {
			return "Today"
		}
		if t.Format("2006-01-02") == now.AddDate(0, 0, -1).Format("2006-01-02") {
			return "Yesterday"
		}
		return t.Format("January 2, 2006")
	}
	return date
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// fetchWorkflowData is a helper function to fetch and process workflow data for an install.
// This version fetches only customer-visible workflow types using parallel API calls.
func (h *Handler) fetchWorkflowData(c *gin.Context, install *localModels.Install, offset, limit int) ([]gin.H, bool, error) {
	if err := h.loadInstallWithOrg(install); err != nil {
		return nil, false, fmt.Errorf("failed to load install details: %w", err)
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		return nil, false, fmt.Errorf("organization information not found")
	}

	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		return nil, false, fmt.Errorf("failed to initialize Nuon client: %w", err)
	}

	// Fetch 10 workflows per type in parallel, then combine and sort
	const perTypeLimit = 10

	type workflowResult struct {
		workflows []*models.AppWorkflow
		hasMore   bool
		err       error
	}

	resultChan := make(chan workflowResult, len(customerVisibleWorkflowTypes))

	for _, wfType := range customerVisibleWorkflowTypes {
		go func(workflowType string) {
			workflows, hasMore, err := nuonClient.GetInstallWorkflowsByType(c.Request.Context(), install.NuonInstallID, 0, perTypeLimit, workflowType)
			resultChan <- workflowResult{workflows, hasMore, err}
		}(wfType)
	}

	// Collect all workflows from parallel requests
	var allWorkflows []*models.AppWorkflow
	anyHasMore := false

	for i := 0; i < len(customerVisibleWorkflowTypes); i++ {
		result := <-resultChan
		if result.err != nil {
			h.logger.Error("error fetching workflow type", zap.Error(result.err))
			continue
		}
		allWorkflows = append(allWorkflows, result.workflows...)
		if result.hasMore {
			anyHasMore = true
		}
	}

	// Sort all workflows by created_at descending
	sortWorkflowsByCreatedAt(allWorkflows)

	// Apply pagination (offset and limit) to the combined, sorted results
	start := offset
	if start > len(allWorkflows) {
		start = len(allWorkflows)
	}
	end := start + limit
	if end > len(allWorkflows) {
		end = len(allWorkflows)
	}

	paginatedWorkflows := allWorkflows[start:end]

	// Determine if there are more results
	// There are more if: we have more items after this page, OR any type returned hasMore
	hasMore := end < len(allWorkflows) || anyHasMore

	// Process workflows for customer display
	processedWorkflows := make([]gin.H, 0, len(paginatedWorkflows))
	for _, workflow := range paginatedWorkflows {
		processed := processWorkflowForCustomer(workflow)
		processedWorkflows = append(processedWorkflows, processed)
	}

	return processedWorkflows, hasMore, nil
}

// sortWorkflowsByCreatedAt sorts workflows by created_at in descending order (newest first)
func sortWorkflowsByCreatedAt(workflows []*models.AppWorkflow) {
	for i := 0; i < len(workflows); i++ {
		for j := i + 1; j < len(workflows); j++ {
			timeI := parseWorkflowTime(workflows[i].CreatedAt)
			timeJ := parseWorkflowTime(workflows[j].CreatedAt)
			if timeJ.After(timeI) {
				workflows[i], workflows[j] = workflows[j], workflows[i]
			}
		}
	}
}

// customerVisibleWorkflowTypes defines which workflow types should be shown to customers
// in the "Latest Updates" section of the install detail page
var customerVisibleWorkflowTypes = []string{
	"provision",           // Initial install provisioning
	"provision_sandbox",   // Sandbox provisioning
	"reprovision",         // Install re-provisioning
	"reprovision_sandbox", // Sandbox re-provisioning
	"deprovision",         // Install deprovisioning
	"deprovision_sandbox", // Sandbox deprovisioning
	"deploy_components",   // Component deployment
	"input_update",        // Configuration/input updates
}

// renderWorkflowCardPartial fetches the updated workflow and renders just the workflow card partial.
// This is used by HTMX requests to update a single workflow card after an action (approve/cancel/approve-all).
func (h *Handler) renderWorkflowCardPartial(c *gin.Context, install *localModels.Install, workflowID string, nuonClient *nuon.Client) {
	// Fetch the updated workflow from the API
	workflow, err := nuonClient.GetWorkflow(c.Request.Context(), workflowID)
	if err != nil {
		h.logger.Error("failed to fetch workflow for partial render",
			zap.String("workflow_id", workflowID),
			zap.Error(err),
		)
		// Render error state workflow card
		errorProps := components.WorkflowCardProps{
			Workflow: customerui.WorkflowData{
				ID:                       workflowID,
				Name:                     "Error",
				Status:                   "error",
				StatusClass:              "bg-red-100 text-red-800",
				ApproveDisabledReason:    "Error loading workflow",
				ApproveAllDisabledReason: "Error loading workflow",
				CancelDisabledReason:     "Error loading workflow",
			},
			InstallID: install.ID,
			BasePath:  h.basePath,
		}
		h.RenderTempl(c, http.StatusInternalServerError, components.WorkflowCard(errorProps))
		return
	}

	// Process the workflow for customer display
	processed := processWorkflowForCustomer(workflow)

	// Get global app theme for styling
	theme, _ := localModels.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
	primaryColor, _ := GetPrimaryColors(theme.PrimaryColor)

	// Convert gin.H to WorkflowData
	wfData := ginHToWorkflowData(processed)

	props := components.WorkflowCardProps{
		Workflow:     wfData,
		InstallID:    install.ID,
		BasePath:     h.basePath,
		PrimaryColor: primaryColor,
	}
	h.RenderTempl(c, http.StatusOK, components.WorkflowCard(props))
}

// fetchRecentWorkflows fetches the most recent customer-visible workflow for embedded display.
// It makes parallel API calls for each customer-visible workflow type (fetching 1 per type),
// then returns the single most recent workflow across all types.
func (h *Handler) fetchRecentWorkflows(c *gin.Context, install *localModels.Install) ([]gin.H, error) {
	if err := h.loadInstallWithOrg(install); err != nil {
		return nil, fmt.Errorf("failed to load install details: %w", err)
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		return nil, fmt.Errorf("organization information not found")
	}

	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Nuon client: %w", err)
	}

	// Make parallel requests for each workflow type
	type workflowResult struct {
		workflow *models.AppWorkflow
		err      error
	}

	resultChan := make(chan workflowResult, len(customerVisibleWorkflowTypes))

	for _, wfType := range customerVisibleWorkflowTypes {
		go func(workflowType string) {
			workflows, _, err := nuonClient.GetInstallWorkflowsByType(c.Request.Context(), install.NuonInstallID, 0, 1, workflowType)
			if err != nil {
				resultChan <- workflowResult{nil, err}
				return
			}
			if len(workflows) > 0 {
				resultChan <- workflowResult{workflows[0], nil}
			} else {
				resultChan <- workflowResult{nil, nil}
			}
		}(wfType)
	}

	// Collect results and find the most recent workflow
	var mostRecentWorkflow *models.AppWorkflow
	var mostRecentTime time.Time

	for i := 0; i < len(customerVisibleWorkflowTypes); i++ {
		result := <-resultChan
		if result.err != nil {
			// Log but continue - we want to show whatever we can get
			h.logger.Error("error fetching workflow type", zap.Error(result.err))
			continue
		}
		if result.workflow != nil {
			workflowTime := parseWorkflowTime(result.workflow.CreatedAt)
			if mostRecentWorkflow == nil || workflowTime.After(mostRecentTime) {
				mostRecentWorkflow = result.workflow
				mostRecentTime = workflowTime
			}
		}
	}

	// Return the most recent workflow (or empty slice if none found)
	if mostRecentWorkflow == nil {
		return []gin.H{}, nil
	}

	processed := processWorkflowForCustomer(mostRecentWorkflow)
	return []gin.H{processed}, nil
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
