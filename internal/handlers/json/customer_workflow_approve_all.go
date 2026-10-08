package jsonhandlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	localmodels "github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/pkg/nuon"
	"gorm.io/gorm"
)

func (h *CustomerInstallWizardHandler) getInstallForWorkflowAction(c *gin.Context, orgID string) (*localmodels.Install, error) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return nil, gorm.ErrRecordNotFound
	}

	installID := strings.TrimSpace(c.Param("install_id"))
	if installID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	selected := h.selectCustomerAccountMember(c, user.ID, orgID)
	query := h.db.Where("id = ? AND org_id = ? AND user_id = ?", installID, orgID, user.ID)
	if selected != nil {
		query = h.db.Where("id = ? AND org_id = ? AND ((user_id = ?) OR (customer_account_id = ? AND visibility = ?))",
			installID, orgID, user.ID, selected.AccountID, localmodels.VisibilityAccount,
		)
	}

	var install localmodels.Install
	if err := query.First(&install).Error; err != nil {
		return nil, err
	}

	return &install, nil
}

// ApproveWorkflowStep approves a specific pending approval step for wizard JSON clients.
func (h *CustomerInstallWizardHandler) ApproveWorkflowStep(c *gin.Context) {
	org, err := h.getOrgForCustomerPortal(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	workflowID := strings.TrimSpace(c.Param("workflow_id"))
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workflow ID is required"})
		return
	}

	stepID := strings.TrimSpace(c.PostForm("step_id"))
	approvalID := strings.TrimSpace(c.PostForm("approval_id"))
	if stepID == "" || approvalID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "step_id and approval_id are required"})
		return
	}

	install, err := h.getInstallForWorkflowAction(c, org.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found or access denied"})
		return
	}

	client, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	if err := client.ApproveWorkflowStep(c.Request.Context(), workflowID, stepID, approvalID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to approve workflow step"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"install_id":  install.ID,
		"workflow_id": workflowID,
		"step_id":     stepID,
	})
}

// ApproveAllWorkflowSteps approves currently pending workflow approval steps and
// sets approve-all mode for future steps. This endpoint is JSON-only for the
// React wizard and does not change behavior of legacy HTML endpoints.
func (h *CustomerInstallWizardHandler) ApproveAllWorkflowSteps(c *gin.Context) {
	org, err := h.getOrgForCustomerPortal(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	workflowID := strings.TrimSpace(c.Param("workflow_id"))
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workflow ID is required"})
		return
	}

	install, err := h.getInstallForWorkflowAction(c, org.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found or access denied"})
		return
	}

	client, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	ctx := c.Request.Context()
	workflow, err := client.GetWorkflow(ctx, workflowID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to load workflow"})
		return
	}

	approvedSteps := 0
	failedStepApprovals := make([]string, 0)

	for _, step := range workflow.Steps {
		if step.Status != nil && string(step.Status.Status) == "approval-awaiting" && step.Approval != nil {
			if err := client.ApproveWorkflowStep(ctx, workflowID, step.ID, step.Approval.ID); err != nil {
				failedStepApprovals = append(failedStepApprovals, step.ID)
				continue
			}
			approvedSteps++
		}
	}

	if err := client.ApproveAllWorkflowSteps(ctx, workflowID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to set approve-all mode"})
		return
	}

	response := gin.H{
		"success":        true,
		"install_id":     install.ID,
		"workflow_id":    workflowID,
		"approved_steps": approvedSteps,
	}
	if len(failedStepApprovals) > 0 {
		response["failed_step_approvals"] = failedStepApprovals
	}

	c.JSON(http.StatusOK, response)
}
