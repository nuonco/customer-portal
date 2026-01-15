package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/nuon-go/models"

	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	localModels "github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/components"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/overrides"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

// WorkflowsPage renders the workflows history page for a customer install
func (h *Handler) WorkflowsPage(c *gin.Context) {
	user := h.GetFreshUser(c) // Load from DB for topbar display
	installIDParam := c.Param("install_id")

	fmt.Printf("=== WorkflowsPage DEBUG START ===\n")
	fmt.Printf("User: %+v\n", user)
	fmt.Printf("Install ID param: %s\n", installIDParam)

	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		fmt.Printf("ERROR: Install not found in context\n")
		theme, _ := localModels.GetOrCreateAppTheme(h.db, h.getWorkspaceIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", user, theme),
			Error:       "Install not found",
		}
		h.RenderTempl(c, http.StatusNotFound, customerpages.ErrorPage(props))
		return
	}

	install := installInterface.(*localModels.Install)
	fmt.Printf("Install loaded: ID=%s, NuonInstallID=%s\n", install.ID, install.NuonInstallID)

	// Get pagination parameters
	offsetParam := c.DefaultQuery("offset", "0")
	offset, err := strconv.Atoi(offsetParam)
	if err != nil {
		offset = 0
	}

	limit := 10 // Fixed page size for customer view

	// Fetch workflow data using helper function
	processedWorkflows, hasMoreFromAPI, err := h.fetchWorkflowData(c, install, offset, limit)
	if err != nil {
		theme, _ := localModels.GetOrCreateAppTheme(h.db, h.getWorkspaceIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", user, theme),
			Error:       err.Error(),
		}
		h.RenderTempl(c, http.StatusInternalServerError, customerpages.ErrorPage(props))
		return
	}

	// Try to get CloudFormation link from install stack
	var cloudFormationLink string
	fmt.Printf("Attempting to fetch install stack for install ID: %s\n", install.NuonInstallID)

	nuonClient, err := nuon.NewClientWithURL(install.InstallLink.NuonOrg.APIToken, install.InstallLink.NuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		fmt.Printf("ERROR: Failed to initialize Nuon client for stack fetch: %v\n", err)
	} else {
		stack, err := nuonClient.GetInstallStack(c.Request.Context(), install.NuonInstallID)
		if err != nil {
			fmt.Printf("Failed to get install stack: %v\n", err)
		} else if stack != nil {
			fmt.Printf("Got install stack: %+v\n", stack)
			// Check if stack has versions with quick link
			if stack.Versions != nil && len(stack.Versions) > 0 {
				if stack.Versions[0].QuickLinkURL != "" {
					cloudFormationLink = stack.Versions[0].QuickLinkURL
					fmt.Printf("Found CloudFormation link from stack: %s\n", cloudFormationLink)
				}
			}
		}

		// Append customer inputs to CloudFormation URL if available
		if cloudFormationLink != "" {
			fmt.Printf("DEBUG: CloudFormation link found, attempting to append customer inputs\n")
			fmt.Printf("DEBUG: AppID=%s, InstallID=%s\n", install.InstallLink.AppID, install.NuonInstallID)

			// Fetch app input config to identify customer input names
			inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), install.InstallLink.AppID)
			if err != nil {
				fmt.Printf("DEBUG: Error fetching app input config: %v\n", err)
			} else if inputConfig == nil {
				fmt.Printf("DEBUG: App input config is nil\n")
			} else {
				fmt.Printf("DEBUG: Got app input config (type %T): %+v\n", inputConfig, inputConfig)

				// Convert typed struct to map for processing (same pattern as customer.go)
				var configMap map[string]interface{}
				jsonBytes, err := json.Marshal(inputConfig)
				if err != nil {
					fmt.Printf("DEBUG: Error marshaling input config: %v\n", err)
				} else if err := json.Unmarshal(jsonBytes, &configMap); err != nil {
					fmt.Printf("DEBUG: Error unmarshaling input config to map: %v\n", err)
				} else {
					fmt.Printf("DEBUG: Converted input config to map: %+v\n", configMap)

					// Extract customer input mappings from config (inputName -> CF param name)
					inputMappings := extractCustomerInputMappings(configMap)
					fmt.Printf("DEBUG: Customer input mappings found: %v\n", inputMappings)

					if len(inputMappings) > 0 {
						// Get current install inputs from Nuon API
						currentInputs, err := nuonClient.GetInstallCurrentInputs(c.Request.Context(), install.NuonInstallID)
						if err != nil {
							fmt.Printf("DEBUG: Error fetching install inputs: %v\n", err)
						} else if currentInputs == nil {
							fmt.Printf("DEBUG: Current inputs is nil\n")
						} else {
							fmt.Printf("DEBUG: Current inputs values: %+v\n", currentInputs.Values)
							if currentInputs.Values != nil {
								// Append customer inputs as CloudFormation parameters
								cloudFormationLink = appendInputsToCloudFormationURL(
									cloudFormationLink,
									currentInputs.Values,
									inputMappings,
								)
								fmt.Printf("DEBUG: CloudFormation link with customer inputs: %s\n", cloudFormationLink)
							} else {
								fmt.Printf("DEBUG: Current inputs Values is nil\n")
							}
						}
					} else {
						fmt.Printf("DEBUG: No customer input names found in config\n")
					}
				}
			}
		}
	}

	if cloudFormationLink == "" {
		fmt.Printf("No CloudFormation link found\n")
	}

	// Group workflows by date and sort in descending order - convert to templ types
	orderedWorkflowGroups := groupAndSortWorkflowsByDateTempl(processedWorkflows)

	// Get global app theme for customer UI
	workspaceID := h.getWorkspaceIDForTheme(c)
	theme, _ := localModels.GetOrCreateAppTheme(h.db, workspaceID)

	// Try template override first
	workflowsData := make([]overrides.WorkflowData, 0, len(processedWorkflows))
	for _, wf := range processedWorkflows {
		id, _ := wf["id"].(string)
		name, _ := wf["name"].(string)
		status, _ := wf["status"].(string)
		workflowsData = append(workflowsData, overrides.WorkflowData{
			ID:     id,
			Type:   name,
			Status: status,
		})
	}

	pageData := overrides.WorkflowsPageData{
		Install: overrides.InstallData{
			ID:      install.ID,
			Name:    install.Name,
			Status:  string(install.Status),
			Region:  install.Region,
			AppName: install.InstallLink.AppName,
		},
		Workflows: workflowsData,
	}

	ctx := h.buildTemplateContext("Workflow History - "+install.InstallLink.AppName, user, theme, pageData)
	if h.tryRenderOverride(c, workspaceID, "workflows", ctx) {
		return
	}

	// Fall back to default Templ template
	props := customerpages.WorkflowsPageProps{
		LayoutProps:        h.buildCustomerLayoutProps("Workflow History - "+install.InstallLink.AppName, user, theme),
		Install:            install,
		WorkflowGroups:     orderedWorkflowGroups,
		CloudFormationLink: cloudFormationLink,
		CurrentOffset:      offset,
		Limit:              limit,
		HasNext:            hasMoreFromAPI,
		HasPrev:            offset > 0,
		NextOffset:         offset + limit,
		PrevOffset:         max(0, offset-limit),
	}
	h.RenderTempl(c, http.StatusOK, customerpages.WorkflowsPage(props))
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

	// Load install link to get org info
	if err := h.db.Preload("InstallLink.NuonOrg").Where("id = ?", install.ID).First(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(install.InstallLink.NuonOrg.APIToken, install.InstallLink.NuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Approve the workflow step
	if err := nuonClient.ApproveWorkflowStep(c.Request.Context(), workflowID, stepID, approvalID); err != nil {
		fmt.Printf("Failed to approve workflow step: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve workflow step"})
		return
	}

	fmt.Printf("User %s (%s) approved workflow step %s in workflow %s\n", user.Email, user.ID, stepID, workflowID)

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

	// Load install link to get org info
	if err := h.db.Preload("InstallLink.NuonOrg").Where("id = ?", install.ID).First(install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(install.InstallLink.NuonOrg.APIToken, install.InstallLink.NuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Cancel the workflow
	if err := nuonClient.CancelWorkflow(c.Request.Context(), workflowID); err != nil {
		fmt.Printf("Failed to cancel workflow: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel workflow"})
		return
	}

	fmt.Printf("User %s (%s) cancelled workflow %s\n", user.Email, user.ID, workflowID)

	// For HTMX requests, return the updated workflow card partial
	if isHTMXRequest(c) {
		h.renderWorkflowCardPartial(c, install, workflowID, nuonClient)
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

	fmt.Printf("=== ApproveAllWorkflowSteps DEBUG START ===\n")
	fmt.Printf("User: %s (%s), Install ID: %s, Workflow ID: %s\n", user.Email, user.ID, install.ID, workflowID)

	// Load install link to get org info
	if err := h.db.Preload("InstallLink.NuonOrg").Where("id = ?", install.ID).First(install).Error; err != nil {
		fmt.Printf("ERROR: Failed to load install details: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install details"})
		return
	}

	// Handle NuonOrg loading with fallback strategies (same as WorkflowsPage)
	if install.InstallLink.NuonOrg.ID == "" {
		fmt.Printf("NuonOrg not loaded via preload, trying manual loading for OrgID=%s\n", install.InstallLink.OrgID)
		var nuonOrg localModels.NuonOrg
		if err := h.db.Where("id = ?", install.InstallLink.OrgID).First(&nuonOrg).Error; err != nil {
			fmt.Printf("Manual NuonOrg loading failed: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization information not found"})
			return
		}
		install.InstallLink.NuonOrg = nuonOrg
		fmt.Printf("Successfully loaded NuonOrg manually: ID=%s, NuonOrgID=%s\n", nuonOrg.ID, nuonOrg.NuonOrgID)
	}

	fmt.Printf("Org info loaded: OrgID=%s, APIToken length=%d\n", install.InstallLink.NuonOrg.NuonOrgID, len(install.InstallLink.NuonOrg.APIToken))

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(install.InstallLink.NuonOrg.APIToken, install.InstallLink.NuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		fmt.Printf("ERROR: Failed to initialize Nuon client: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	// Set approve-all on the workflow
	if err := nuonClient.ApproveAllWorkflowSteps(c.Request.Context(), workflowID); err != nil {
		fmt.Printf("Failed to set workflow approve-all: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set workflow approve-all"})
		return
	}

	fmt.Printf("User %s (%s) set approve-all on workflow %s\n", user.Email, user.ID, workflowID)
	fmt.Printf("=== ApproveAllWorkflowSteps DEBUG END ===\n")

	// For HTMX requests, return the updated workflow card partial
	if isHTMXRequest(c) {
		h.renderWorkflowCardPartial(c, install, workflowID, nuonClient)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workflow set to approve-all successfully",
	})
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
	approveDisabledReason := ""
	approveAllDisabledReason := ""
	cancelDisabledReason := ""

	if workflow.Status != nil {
		status = string(workflow.Status.Status)
		statusClass = getStatusClass(string(workflow.Status.Status))

		// Check for approval steps in the workflow based on ExecutionType
		hasApprovalSteps := false
		if workflow.Steps != nil {
			for _, step := range workflow.Steps {
				// Check ExecutionType for approval steps
				if step.ExecutionType == "approval" {
					hasApprovalSteps = true
					fmt.Printf("WORKFLOW %s: Found approval step - ID=%s, ExecutionType=%s\n",
						workflow.ID, step.ID, step.ExecutionType)
					break
				}
			}
			// Log debug info about steps
			fmt.Printf("WORKFLOW %s: Checked %d steps, hasApprovalSteps=%v\n",
				workflow.ID, len(workflow.Steps), hasApprovalSteps)
		}

		// Check if workflow needs approval or can be cancelled
		switch string(workflow.Status.Status) {
		case "approval-awaiting":
			canApprove = true
			canApproveAll = hasApprovalSteps
			canCancel = true
		case "in-progress":
			canCancel = true
			approveDisabledReason = "Workflow is currently running"
			// Approve All is available if there are approval steps, even when running
			if hasApprovalSteps {
				canApproveAll = true
				fmt.Printf("WORKFLOW %s: Has approval steps - enabling Approve All button\n", workflow.ID)
			} else {
				approveAllDisabledReason = "No approval steps in this workflow"
				fmt.Printf("WORKFLOW %s: No approval steps detected - disabling Approve All\n", workflow.ID)
			}
		case "completed":
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
			if hasApprovalSteps {
				canApproveAll = true
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
		"approval_step":               approvalStep,
		"approve_disabled_reason":     approveDisabledReason,
		"approve_all_disabled_reason": approveAllDisabledReason,
		"cancel_disabled_reason":      cancelDisabledReason,
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

// groupAndSortWorkflowsByDateTempl groups workflows by date and returns them in descending order for templ
func groupAndSortWorkflowsByDateTempl(workflows []gin.H) []customerpages.WorkflowGroup {
	// First, group workflows by date
	grouped := make(map[string][]customerui.WorkflowData)

	for _, workflow := range workflows {
		var dateKey string
		if createdAt, ok := workflow["created_at"].(time.Time); ok && !createdAt.IsZero() {
			dateKey = createdAt.Format("2006-01-02")
		}
		if dateKey != "" {
			// Convert gin.H to WorkflowData
			wfData := ginHToWorkflowData(workflow)
			grouped[dateKey] = append(grouped[dateKey], wfData)
		}
	}

	// Extract and sort dates in descending order (newest first)
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

	// Build ordered workflow groups
	var orderedGroups []customerpages.WorkflowGroup
	for _, date := range dates {
		orderedGroups = append(orderedGroups, customerpages.WorkflowGroup{
			Date:        date,
			DisplayDate: formatDateForDisplay(date),
			Workflows:   grouped[date],
		})
	}

	return orderedGroups
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

// groupAndSortWorkflowsByDate groups workflows by date and returns them in descending order
func groupAndSortWorkflowsByDate(workflows []gin.H) []WorkflowGroup {
	// First, group workflows by date
	grouped := make(map[string][]gin.H)

	fmt.Printf("GROUPING DEBUG: Processing %d workflows for date grouping\n", len(workflows))

	for i, workflow := range workflows {
		createdAtRaw := workflow["created_at"]
		fmt.Printf("GROUPING DEBUG %d: created_at raw value=%+v (Type: %T)\n", i, createdAtRaw, createdAtRaw)

		var dateKey string
		var success bool

		// Try direct time.Time
		if createdAt, ok := workflow["created_at"].(time.Time); ok {
			if !createdAt.IsZero() {
				dateKey = createdAt.Format("2006-01-02")
				success = true
				fmt.Printf("GROUPING DEBUG %d: Successfully parsed time.Time, dateKey=%s\n", i, dateKey)
			} else {
				fmt.Printf("GROUPING DEBUG %d: time.Time is zero value, skipping\n", i)
			}
		} else if createdAtPtr, ok := workflow["created_at"].(*time.Time); ok && createdAtPtr != nil {
			// Try *time.Time
			if !createdAtPtr.IsZero() {
				dateKey = createdAtPtr.Format("2006-01-02")
				success = true
				fmt.Printf("GROUPING DEBUG %d: Successfully parsed *time.Time, dateKey=%s\n", i, dateKey)
			} else {
				fmt.Printf("GROUPING DEBUG %d: *time.Time is zero value, skipping\n", i)
			}
		} else if createdAtStr, ok := workflow["created_at"].(string); ok {
			// Try string parsing (ISO format)
			if parsedTime, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
				dateKey = parsedTime.Format("2006-01-02")
				success = true
				fmt.Printf("GROUPING DEBUG %d: Successfully parsed string as RFC3339, dateKey=%s\n", i, dateKey)
			} else if parsedTime, err := time.Parse("2006-01-02T15:04:05Z", createdAtStr); err == nil {
				dateKey = parsedTime.Format("2006-01-02")
				success = true
				fmt.Printf("GROUPING DEBUG %d: Successfully parsed string as alternative format, dateKey=%s\n", i, dateKey)
			} else {
				fmt.Printf("GROUPING DEBUG %d: Failed to parse string time format: %s, error: %v\n", i, createdAtStr, err)
			}
		} else {
			fmt.Printf("GROUPING DEBUG %d: All type assertions failed, workflow dropped from grouping\n", i)
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

	fmt.Printf("GROUPING RESULT: Created %d date groups in descending order\n", len(orderedGroups))
	for i, group := range orderedGroups {
		fmt.Printf("GROUPING RESULT %d: Date %s (%s) has %d workflows\n", i, group.Date, group.DisplayDate, len(group.Workflows))
	}

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
	// Load install link to get org info with fallback strategies
	if err := h.db.Preload("InstallLink.NuonOrg").Where("id = ?", install.ID).First(install).Error; err != nil {
		return nil, false, fmt.Errorf("failed to load install details: %w", err)
	}

	// Handle NuonOrg loading with fallback strategies
	if install.InstallLink.NuonOrg.ID == "" {
		var installLinkWithOrg localModels.InstallLink
		if err := h.db.Preload("NuonOrg").Where("id = ?", install.InstallLink.ID).First(&installLinkWithOrg).Error; err == nil {
			if installLinkWithOrg.NuonOrg.ID != "" {
				install.InstallLink.NuonOrg = installLinkWithOrg.NuonOrg
			}
		}

		// If still not loaded, try manual loading
		if install.InstallLink.NuonOrg.ID == "" {
			var nuonOrg localModels.NuonOrg
			if err := h.db.Where("id = ?", install.InstallLink.OrgID).First(&nuonOrg).Error; err != nil {
				return nil, false, fmt.Errorf("organization information not found (InstallLink.OrgID=%s, no matching NuonOrg)", install.InstallLink.OrgID)
			}
			install.InstallLink.NuonOrg = nuonOrg
		}
	}

	// Initialize Nuon client with fallback for legacy organizations
	apiURL := h.nuonAPIURL
	if apiURL == "" {
		// Fallback for legacy organizations created before API URL support
		apiURL = "https://api.nuon.co"
	}
	nuonClient, err := nuon.NewClientWithURL(install.InstallLink.NuonOrg.APIToken, install.InstallLink.NuonOrg.NuonOrgID, apiURL)
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
			fmt.Printf("Error fetching workflow type: %v\n", result.err)
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
		fmt.Printf("Failed to fetch workflow %s for partial render: %v\n", workflowID, err)
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
	theme, _ := localModels.GetOrCreateAppTheme(h.db, h.getWorkspaceIDForTheme(c))
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
	// Load install link to get org info with fallback strategies
	if err := h.db.Preload("InstallLink.NuonOrg").Where("id = ?", install.ID).First(install).Error; err != nil {
		return nil, fmt.Errorf("failed to load install details: %w", err)
	}

	// Handle NuonOrg loading with fallback strategies
	if install.InstallLink.NuonOrg.ID == "" {
		var installLinkWithOrg localModels.InstallLink
		if err := h.db.Preload("NuonOrg").Where("id = ?", install.InstallLink.ID).First(&installLinkWithOrg).Error; err == nil {
			if installLinkWithOrg.NuonOrg.ID != "" {
				install.InstallLink.NuonOrg = installLinkWithOrg.NuonOrg
			}
		}

		if install.InstallLink.NuonOrg.ID == "" {
			var nuonOrg localModels.NuonOrg
			if err := h.db.Where("id = ?", install.InstallLink.OrgID).First(&nuonOrg).Error; err != nil {
				return nil, fmt.Errorf("organization information not found")
			}
			install.InstallLink.NuonOrg = nuonOrg
		}
	}

	// Initialize Nuon client
	apiURL := h.nuonAPIURL
	if apiURL == "" {
		apiURL = "https://api.nuon.co"
	}
	nuonClient, err := nuon.NewClientWithURL(install.InstallLink.NuonOrg.APIToken, install.InstallLink.NuonOrg.NuonOrgID, apiURL)
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
			fmt.Printf("Error fetching workflow type: %v\n", result.err)
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
				fmt.Printf("DEBUG: Input mapping: %s -> %s\n", name, mappings[name])
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
			fmt.Printf("DEBUG: Adding CF param: %s (from input %s = %s)\n", param, inputName, value)
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
