package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/models"
	customerpages "github.com/nuonco/customer-portal/internal/views/customerui/theme/pages"
	"github.com/nuonco/customer-portal/pkg/nuon"
	"go.uber.org/zap"
)

// buildDebugActionProps builds the full page props for re-rendering after an action.
func (h *Handler) buildDebugActionProps(c *gin.Context) (*customerpages.DebugWorkflowsPageProps, error) {
	props, err := h.buildDebugWorkflowsProps(c)
	if err != nil {
		return nil, err
	}

	installInterface, _ := c.Get("install")
	install := installInterface.(*models.Install)
	if err := h.loadInstallWithOrg(install); err != nil {
		return nil, err
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil || nuonOrg.APIToken == "" {
		return nil, fmt.Errorf("no API credentials configured")
	}

	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	workflowID := c.Param("workflow_id")
	workflow, err := nuonClient.GetWorkflowV2(c.Request.Context(), workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workflow: %w", err)
	}

	jsonBytes, _ := json.MarshalIndent(workflow, "", "  ")

	props.SelectedWorkflow = workflow
	props.WorkflowJSON = string(jsonBytes)

	// Determine selected step
	stepID := c.DefaultQuery("step", c.PostForm("current_step"))
	if stepID != "" {
		for _, s := range workflow.Steps {
			if s.ID == stepID {
				props.SelectedStep = s
				props.SelectedStepIsActive = false
				break
			}
		}
	}
	if props.SelectedStep == nil {
		props.SelectedStep = findActiveStepV2(workflow.Steps)
		props.SelectedStepIsActive = true
	}

	// Fetch step target
	if props.SelectedStep != nil && props.SelectedStep.StepTargetType != "" && props.SelectedStep.StepTargetID != "" {
		props.SelectedStepTarget, props.SelectedStepTargetJSON = h.fetchStepTargetV2(c, install, props.SelectedStep)
	}

	props.ActiveTab = c.DefaultQuery("tab", "info")
	props.ActiveStepTab = c.DefaultQuery("step_tab", "info")
	props.Expanded = c.Query("expanded") == "true"
	props.ActiveTargetTab = c.DefaultQuery("target_tab", "info")
	props.ActiveStackTab = c.DefaultQuery("stack_tab", "info")
	props.AlertType = c.Query("alert_type")
	props.AlertMsg = c.Query("alert_msg")
	props.CurrentQuery = c.Request.URL.RawQuery

	return props, nil
}

// renderDebugAction redirects to the workflow detail page with alert params.
// hx-boost on the body follows the redirect transparently.
func (h *Handler) renderDebugAction(c *gin.Context, props *customerpages.DebugWorkflowsPageProps) {
	redirectURL := fmt.Sprintf("%s/installs/%s/debug/workflows/%s",
		props.BasePath, props.Install.ID, props.SelectedWorkflow.ID)
	sep := "?"
	if props.ActiveTab != "" && props.ActiveTab != "info" {
		redirectURL += sep + "tab=" + props.ActiveTab
		sep = "&"
	}
	if props.SelectedStep != nil && !props.SelectedStepIsActive {
		redirectURL += sep + "step=" + props.SelectedStep.ID
		sep = "&"
	}
	if props.ActiveStepTab != "" && props.ActiveStepTab != "info" {
		redirectURL += sep + "step_tab=" + props.ActiveStepTab
		sep = "&"
	}
	if props.Expanded {
		redirectURL += sep + "expanded=true"
		sep = "&"
	}
	if props.AlertType != "" && props.AlertMsg != "" {
		redirectURL += sep + "alert_type=" + props.AlertType + "&alert_msg=" + url.QueryEscape(props.AlertMsg)
	}
	c.Redirect(http.StatusFound, redirectURL)
}

// DebugApproveStep approves a pending workflow step.
func (h *Handler) DebugApproveStep(c *gin.Context) {
	if !h.requireVendorRole(c) {
		return
	}

	stepID := c.PostForm("step_id")
	approvalID := c.PostForm("approval_id")

	props, err := h.buildDebugActionProps(c)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if stepID == "" || approvalID == "" {
		props.AlertType = "error"
		props.AlertMsg = "step_id and approval_id are required"
		h.renderDebugAction(c, props)
		return
	}

	installInterface, _ := c.Get("install")
	install := installInterface.(*models.Install)
	h.loadInstallWithOrg(install)
	nuonOrg := install.GetNuonOrg()
	nuonClient, _ := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))

	if err := nuonClient.ApproveWorkflowStep(c.Request.Context(), c.Param("workflow_id"), stepID, approvalID); err != nil {
		h.logger.Error("debug: failed to approve workflow step", zap.Error(err))
		props.AlertType = "error"
		props.AlertMsg = fmt.Sprintf("Failed to approve step: %v", err)
		h.renderDebugAction(c, props)
		return
	}

	// Re-fetch workflow to get updated state
	props, err = h.buildDebugActionProps(c)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	props.AlertType = "success"
	props.AlertMsg = "Step approved successfully"
	h.renderDebugAction(c, props)
}

// DebugApproveAllSteps sets approve-all mode on a workflow.
func (h *Handler) DebugApproveAllSteps(c *gin.Context) {
	if !h.requireVendorRole(c) {
		return
	}

	props, err := h.buildDebugActionProps(c)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	installInterface, _ := c.Get("install")
	install := installInterface.(*models.Install)
	h.loadInstallWithOrg(install)
	nuonOrg := install.GetNuonOrg()
	nuonClient, _ := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))

	if err := nuonClient.ApproveAllWorkflowSteps(c.Request.Context(), c.Param("workflow_id")); err != nil {
		h.logger.Error("debug: failed to approve all steps", zap.Error(err))
		props.AlertType = "error"
		props.AlertMsg = fmt.Sprintf("Failed to approve all steps: %v", err)
		h.renderDebugAction(c, props)
		return
	}

	props, err = h.buildDebugActionProps(c)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	props.AlertType = "success"
	props.AlertMsg = "All steps set to auto-approve"
	h.renderDebugAction(c, props)
}

// DebugCancelWorkflow cancels a running workflow.
func (h *Handler) DebugCancelWorkflow(c *gin.Context) {
	if !h.requireVendorRole(c) {
		return
	}

	props, err := h.buildDebugActionProps(c)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	installInterface, _ := c.Get("install")
	install := installInterface.(*models.Install)
	h.loadInstallWithOrg(install)
	nuonOrg := install.GetNuonOrg()
	nuonClient, _ := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))

	if err := nuonClient.CancelWorkflow(c.Request.Context(), c.Param("workflow_id")); err != nil {
		h.logger.Error("debug: failed to cancel workflow", zap.Error(err))
		props.AlertType = "error"
		props.AlertMsg = fmt.Sprintf("Failed to cancel workflow: %v", err)
		h.renderDebugAction(c, props)
		return
	}

	props, err = h.buildDebugActionProps(c)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	props.AlertType = "success"
	props.AlertMsg = "Workflow cancelled"
	h.renderDebugAction(c, props)
}

// DebugRetryStep retries a failed workflow step.
func (h *Handler) DebugRetryStep(c *gin.Context) {
	if !h.requireVendorRole(c) {
		return
	}

	stepID := c.PostForm("step_id")

	props, err := h.buildDebugActionProps(c)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if stepID == "" {
		props.AlertType = "error"
		props.AlertMsg = "step_id is required"
		h.renderDebugAction(c, props)
		return
	}

	installInterface, _ := c.Get("install")
	install := installInterface.(*models.Install)
	h.loadInstallWithOrg(install)
	nuonOrg := install.GetNuonOrg()
	nuonClient, _ := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))

	if err := nuonClient.RetryWorkflowStep(c.Request.Context(), c.Param("workflow_id"), stepID); err != nil {
		h.logger.Error("debug: failed to retry step", zap.Error(err))
		props.AlertType = "error"
		props.AlertMsg = fmt.Sprintf("Failed to retry step: %v", err)
		h.renderDebugAction(c, props)
		return
	}

	props, err = h.buildDebugActionProps(c)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	props.AlertType = "success"
	props.AlertMsg = "Step retry initiated"
	h.renderDebugAction(c, props)
}
