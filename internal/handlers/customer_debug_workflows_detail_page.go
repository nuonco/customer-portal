package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon-go/models"
	"go.uber.org/zap"
)

// DebugWorkflowDetailPage renders the debug page with a selected workflow's detail panel.
func (h *Handler) DebugWorkflowDetailPage(c *gin.Context) {
	if !h.requireVendorRole(c) {
		return
	}

	// Build the base workflows page props (includes list + pagination)
	props, err := h.buildDebugWorkflowsProps(c)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	workflowID := c.Param("workflow_id")

	// Fetch the selected workflow
	installInterface, _ := c.Get("install")
	install := installInterface.(*models.Install)
	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil || nuonOrg.APIToken == "" {
		c.String(http.StatusInternalServerError, "No API credentials configured")
		return
	}

	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create API client: %v", err))
		return
	}

	workflow, err := nuonClient.GetWorkflow(c.Request.Context(), workflowID)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch workflow: %v", err))
		return
	}

	// Marshal to JSON for the JSON tab
	jsonBytes, err := json.MarshalIndent(workflow, "", "  ")
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to marshal workflow: %v", err))
		return
	}

	props.SelectedWorkflow = workflow
	props.WorkflowJSON = string(jsonBytes)

	// Determine which step to display in the detail card
	stepID := c.Query("step")
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
		props.SelectedStep = findActiveStep(workflow.Steps)
		props.SelectedStepIsActive = true
	}

	// Fetch step target if present
	if props.SelectedStep != nil && props.SelectedStep.StepTargetType != "" && props.SelectedStep.StepTargetID != "" {
		props.SelectedStepTarget, props.SelectedStepTargetJSON = h.fetchStepTarget(c, install, props.SelectedStep)
	}

	// Fetch stack setup data for install_stack_versions targets
	if props.SelectedStep != nil && props.SelectedStep.StepTargetType == "install_stack_versions" {
		props.SelectedStepStackSetup = h.getStackSetupData(c.Request.Context(), nuonClient, install, workflow)
		stack, stackErr := nuonClient.GetInstallStack(c.Request.Context(), install.NuonInstallID)
		if stackErr == nil && stack != nil {
			stackJSON, _ := json.MarshalIndent(stack, "", "  ")
			props.SelectedStepStackJSON = string(stackJSON)
		}
	}

	// Tab and panel state
	props.ActiveTab = c.DefaultQuery("tab", "info")
	props.ActiveStepTab = c.DefaultQuery("step_tab", "info")
	props.Expanded = c.Query("expanded") == "true"
	props.ActiveTargetTab = c.DefaultQuery("target_tab", "info")
	props.ActiveStackTab = c.DefaultQuery("stack_tab", "info")
	props.AlertType = c.Query("alert_type")
	props.AlertMsg = c.Query("alert_msg")
	props.CurrentQuery = c.Request.URL.RawQuery

	switch c.Query("partial") {
	case "panel-content":
		h.RenderTempl(c, http.StatusOK, customerpages.DebugWorkflowPanelContent(*props))
	case "panel":
		h.RenderTempl(c, http.StatusOK, customerpages.DebugWorkflowPanel(*props))
	case "panel-oob":
		h.RenderTempl(c, http.StatusOK, customerpages.DebugWorkflowPanelContentWithOOB(*props))
	case "step-card":
		h.RenderTempl(c, http.StatusOK, customerpages.DebugStepCardContent(*props))
	case "target-card":
		h.RenderTempl(c, http.StatusOK, customerpages.DebugTargetCardContent(*props))
	case "stack-card":
		h.RenderTempl(c, http.StatusOK, customerpages.DebugStackCardContent(*props))
	default:
		h.RenderTempl(c, http.StatusOK, customerpages.DebugWorkflowsPage(*props))
	}
}

// fetchStepTarget fetches the step's target resource and returns the typed object and its pretty-printed JSON.
func (h *Handler) fetchStepTarget(c *gin.Context, install *models.Install, step *nuonmodels.AppWorkflowStep) (interface{}, string) {
	if err := h.loadInstallWithOrg(install); err != nil {
		return nil, ""
	}
	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil || nuonOrg.APIToken == "" {
		return nil, ""
	}
	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
	if err != nil {
		return nil, ""
	}

	ctx := c.Request.Context()
	var target interface{}

	switch step.StepTargetType {
	case "install_deploys":
		target, err = nuonClient.GetInstallDeploy(ctx, install.NuonInstallID, step.StepTargetID)
	case "install_sandbox_runs":
		target, err = nuonClient.GetInstallSandboxRun(ctx, install.NuonInstallID, step.StepTargetID)
	case "install_action_workflow_runs":
		target, err = nuonClient.GetInstallActionWorkflowRun(ctx, install.NuonInstallID, step.StepTargetID)
	case "install_stack_versions":
		// No individual endpoint; fetch the install stack and find the matching version
		stack, stackErr := nuonClient.GetInstallStack(ctx, install.NuonInstallID)
		if stackErr != nil {
			err = stackErr
		} else if stack != nil && stack.Versions != nil {
			for _, v := range stack.Versions {
				if v != nil && v.ID == step.StepTargetID {
					target = v
					break
				}
			}
			if target == nil {
				// Version not found in stack; show the full stack as context
				target = stack
			}
		}
	default:
		// Generic fetch: try /v1/{target_type}/{target_id}
		path := fmt.Sprintf("/v1/%s/%s", strings.ReplaceAll(step.StepTargetType, "_", "-"), step.StepTargetID)
		target, err = nuonClient.GetGenericResource(ctx, path)
	}

	if err != nil {
		h.logger.Warn("failed to fetch step target",
			zap.String("target_type", step.StepTargetType),
			zap.String("target_id", step.StepTargetID),
			zap.Error(err),
		)
		return nil, ""
	}

	jsonBytes, err := json.MarshalIndent(target, "", "  ")
	if err != nil {
		return target, ""
	}
	return target, string(jsonBytes)
}
