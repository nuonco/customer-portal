package nuon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/nuonco/nuon-go"
	"github.com/nuonco/nuon-go/models"
)

// Client wraps the Nuon API client for the installer app
type Client struct {
	client   nuon.Client
	apiURL   string
	apiToken string
	orgID    string
}

// NewClient creates a new Nuon API client using default localhost URL
// Deprecated: Use NewClientWithURL instead to specify the API URL explicitly
func NewClient(apiToken, orgID string) (*Client, error) {
	// Default to localhost for backwards compatibility
	apiURL := "http://localhost:8081"

	return NewClientWithURL(apiToken, orgID, apiURL)
}

// NewClientWithURL creates a new Nuon API client with configurable URL
func NewClientWithURL(apiToken, orgID, apiURL string) (*Client, error) {
	// Create validator instance
	v := validator.New()

	fmt.Printf("NUON AUTH - API URL: %s, OrgID: %s\n", apiURL, orgID)

	// Create Nuon client with options - using exact CLI pattern
	client, err := nuon.New(
		nuon.WithValidator(v),
		nuon.WithAuthToken(apiToken),
		nuon.WithOrgID(orgID),
		nuon.WithURL(apiURL),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Nuon client: %w", err)
	}

	return &Client{
		client:   client,
		apiURL:   apiURL,
		apiToken: apiToken,
		orgID:    orgID,
	}, nil
}

// ValidateOrgAccess validates that the API token has access to the org
func (c *Client) ValidateOrgAccess(ctx context.Context) error {
	// Use the same pattern as auth/login.go - try to get orgs to validate access
	_, _, err := c.client.GetOrgs(ctx, &models.GetPaginatedQuery{
		Offset: 0,
		Limit:  1,
	})
	if err != nil {
		return fmt.Errorf("invalid API token or org access: %w", err)
	}

	return nil
}

// GetOrg fetches the current organization's details
func (c *Client) GetOrg(ctx context.Context) (*models.AppOrg, error) {
	org, err := c.client.GetOrg(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get org: %w", err)
	}
	return org, nil
}

// ListApps lists all apps in the organization using same pattern as apps/list.go
func (c *Client) ListApps(ctx context.Context) ([]*models.AppApp, error) {
	apps, _, err := c.client.GetApps(ctx, &models.GetPaginatedQuery{
		Offset: 0,
		Limit:  100, // Get up to 100 apps
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	return apps, nil
}

// GetApp retrieves app details including platform configuration
func (c *Client) GetApp(ctx context.Context, appID string) (*models.AppApp, error) {
	app, err := c.client.GetApp(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	return app, nil
}

// GetAppInputConfig retrieves the input configuration for an app
func (c *Client) GetAppInputConfig(ctx context.Context, appID string) (interface{}, error) {
	inputCfg, err := c.client.GetAppInputLatestConfig(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to get app input config: %w", err)
	}

	return inputCfg, nil
}

// GenerateInstallName creates a unique install name
func GenerateInstallName(appName string) string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	return fmt.Sprintf("%s-install-%s", appName, timestamp)
}

// CreateInstall creates a new install for the given app using same pattern as installs/create.go
func (c *Client) CreateInstall(ctx context.Context, appID, appName, region string) (*models.AppInstall, error) {
	return c.CreateInstallWithInputs(ctx, appID, appName, region, make(map[string]string))
}

// CreateInstallWithInputs creates a new install for the given app with specified inputs
func (c *Client) CreateInstallWithInputs(ctx context.Context, appID, appName, region string, inputs map[string]string) (*models.AppInstall, error) {
	return c.CreateInstallWithCustomName(ctx, appID, appName, "", region, "", inputs)
}

// CreateInstallWithCustomName creates a new install with custom name and platform configuration
func (c *Client) CreateInstallWithCustomName(ctx context.Context, appID, appName, customName, region, location string, inputs map[string]string) (*models.AppInstall, error) {
	// Use custom name or generate one
	var installName string
	if customName != "" {
		installName = customName
	} else {
		installName = GenerateInstallName(appName)
	}

	// Build the request with platform-specific configuration
	request := &models.ServiceCreateInstallRequest{
		Name:   &installName,
		Inputs: inputs,
	}

	// Add AWS configuration if region is provided
	if region != "" {
		request.AwsAccount = &models.ServiceCreateInstallRequestAwsAccount{
			Region: region,
		}
	}

	// Add Azure configuration if location is provided
	if location != "" {
		request.AzureAccount = &models.ServiceCreateInstallRequestAzureAccount{
			Location: location,
		}
	}

	// Create install with platform-specific configuration
	install, _, err := c.client.CreateInstall(ctx, appID, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create install '%s' for app %s: %w", installName, appID, err)
	}

	return install, nil
}

// GetInstall retrieves install details
func (c *Client) GetInstall(ctx context.Context, installID string) (*models.AppInstall, error) {
	install, err := c.client.GetInstall(ctx, installID)
	if err != nil {
		return nil, fmt.Errorf("failed to get install: %w", err)
	}

	return install, nil
}

// DeprovisionInstall deprovisions an install
func (c *Client) DeprovisionInstall(ctx context.Context, installID string) error {
	_, err := c.client.DeleteInstall(ctx, installID)
	if err != nil {
		return fmt.Errorf("failed to deprovision install: %w", err)
	}

	return nil
}

// GetInstallWorkflows retrieves workflow history for an install with pagination
func (c *Client) GetInstallWorkflows(ctx context.Context, installID string, offset int, limit int) ([]*models.AppWorkflow, bool, error) {
	return c.GetInstallWorkflowsByType(ctx, installID, offset, limit, "")
}

// GetInstallWorkflowsByType retrieves workflow history for an install with pagination and optional type filter
func (c *Client) GetInstallWorkflowsByType(ctx context.Context, installID string, offset int, limit int, workflowType string) ([]*models.AppWorkflow, bool, error) {
	// Make a direct HTTP request to the API with planonly parameter
	url := fmt.Sprintf("%s/v1/installs/%s/workflows?offset=%d&limit=%d&planonly=false",
		c.apiURL, installID, offset, limit)

	// Add type filter if specified
	if workflowType != "" {
		url += "&type=" + workflowType
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	req.Header.Set("Content-Type", "application/json")

	fmt.Printf("NUON CLIENT: Making request to %s\n", url)

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("NUON CLIENT ERROR: API returned status %d: %s\n", resp.StatusCode, string(body))
		// Fallback to original method if direct API call fails
		fmt.Printf("NUON CLIENT: Falling back to standard GetWorkflows method\n")
		return c.client.GetWorkflows(ctx, installID, &models.GetPaginatedQuery{
			Offset: offset,
			Limit:  limit,
		})
	}

	// Parse the response
	var workflows []*models.AppWorkflow
	if err := json.Unmarshal(body, &workflows); err != nil {
		return nil, false, fmt.Errorf("failed to parse workflows: %w", err)
	}

	fmt.Printf("NUON CLIENT SUCCESS: Raw API returned %d workflows with planonly=false\n", len(workflows))
	if len(workflows) > 0 {
		fmt.Printf("NUON CLIENT: First workflow ID=%s, Steps count=%d\n", workflows[0].ID, len(workflows[0].Steps))
		if len(workflows[0].Steps) > 0 {
			fmt.Printf("NUON CLIENT: First step - ID=%s, ExecutionType=%s, Approval present=%v\n",
				workflows[0].Steps[0].ID, workflows[0].Steps[0].ExecutionType, workflows[0].Steps[0].Approval != nil)
			// Check all steps for approval data and execution types
			approvalCount := 0
			approvalTypeCount := 0
			for i, step := range workflows[0].Steps {
				if step.Approval != nil {
					approvalCount++
				}
				if step.ExecutionType == "approval" {
					approvalTypeCount++
				}
				// Log first 3 steps in detail
				if i < 3 {
					fmt.Printf("NUON CLIENT: Step %d - ID=%s, ExecutionType=%s, Name=%s, Approval=%v\n",
						i, step.ID, step.ExecutionType, step.Name, step.Approval != nil)
				}
			}
			fmt.Printf("NUON CLIENT: Found %d steps with Approval object, %d steps with ExecutionType=approval out of %d total steps\n",
				approvalCount, approvalTypeCount, len(workflows[0].Steps))
		}
	}

	// TODO: Determine hasMore from response headers or pagination info
	hasMore := len(workflows) == limit

	return workflows, hasMore, nil
}

// ApproveWorkflowStep approves a workflow step that requires approval
func (c *Client) ApproveWorkflowStep(ctx context.Context, workflowID, stepID, approvalID string) error {
	request := &models.ServiceCreateWorkflowStepApprovalResponseRequest{
		ResponseType: "approve",
		Note:         "Approved via installer app",
	}

	_, err := c.client.CreateWorkflowStepApprovalResponse(ctx, workflowID, stepID, approvalID, request)
	if err != nil {
		return fmt.Errorf("failed to approve workflow step: %w", err)
	}

	return nil
}

// ApproveAllWorkflowSteps sets approve-all on a workflow to automatically approve all steps
func (c *Client) ApproveAllWorkflowSteps(ctx context.Context, workflowID string) error {
	approvalOption := models.AppInstallApprovalOptionApproveDashAll
	request := &models.ServiceUpdateWorkflowRequest{
		ApprovalOption: &approvalOption,
	}

	_, err := c.client.UpdateWorkflow(ctx, workflowID, request)
	if err != nil {
		return fmt.Errorf("failed to set workflow approve-all: %w", err)
	}

	return nil
}

// CancelWorkflow cancels a running workflow
func (c *Client) CancelWorkflow(ctx context.Context, workflowID string) error {
	_, err := c.client.CancelWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("failed to cancel workflow: %w", err)
	}

	return nil
}

// GetWorkflow retrieves a single workflow by ID
func (c *Client) GetWorkflow(ctx context.Context, workflowID string) (*models.AppWorkflow, error) {
	workflow, err := c.client.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	return workflow, nil
}

// GetInstallStack retrieves the stack information for an install
func (c *Client) GetInstallStack(ctx context.Context, installID string) (*models.AppInstallStack, error) {
	stack, err := c.client.GetInstallStack(ctx, installID)
	if err != nil {
		return nil, fmt.Errorf("failed to get install stack: %w", err)
	}

	return stack, nil
}

// GetAppActionWorkflows retrieves available actions for an app
func (c *Client) GetAppActionWorkflows(ctx context.Context, appID string) ([]*models.AppActionWorkflow, error) {
	actions, _, err := c.client.GetActionWorkflows(ctx, appID, &models.GetPaginatedQuery{
		Offset: 0,
		Limit:  100, // Get up to 100 actions
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list app actions: %w", err)
	}
	return actions, nil
}

// RunInstallAction triggers an action workflow run on an install
func (c *Client) RunInstallAction(ctx context.Context, installID, actionWorkflowConfigID string) error {
	err := c.client.CreateInstallActionWorkflowRun(ctx, installID, &models.ServiceCreateInstallActionWorkflowRunRequest{
		ActionWorkflowConfigID: actionWorkflowConfigID,
	})
	if err != nil {
		return fmt.Errorf("failed to run action on install: %w", err)
	}
	return nil
}

// GetInstallActionRuns retrieves recent runs of an action on an install
func (c *Client) GetInstallActionRuns(ctx context.Context, installID, actionWorkflowID string) (*models.AppInstallActionWorkflow, error) {
	result, _, err := c.client.GetInstallActionWorkflowRecentRuns(ctx, installID, actionWorkflowID, &models.GetPaginatedQuery{
		Offset: 0,
		Limit:  5, // Get last 5 runs
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get action runs: %w", err)
	}
	return result, nil
}
