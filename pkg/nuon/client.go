package nuon

import (
	"bytes"
	"context"

	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	nuonpkg "github.com/nuonco/nuon/sdks/nuon-go"
	"github.com/nuonco/nuon/sdks/nuon-go/models"
	"go.uber.org/zap"
)

// IsUnauthorized reports whether err (or any error in its chain) is a 401 Unauthorized response.
func IsUnauthorized(err error) bool {
	for err != nil {
		if nuonpkg.IsUnauthorized(err) {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

// statusCoder matches the unexported stderrResponse interface from nuon-go.
type statusCoder interface {
	IsCode(int) bool
}

// IsConflict reports whether err (or any error in its chain) is a 409 Conflict response.
func IsConflict(err error) bool {
	for err != nil {
		if sc, ok := err.(statusCoder); ok && sc.IsCode(409) {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

// APIError holds a structured error parsed from a Nuon API response.
type APIError struct {
	Title       string // from the "Error:" field (title-cased)
	Description string // from the "Description:" field
}

// ParseAPIError extracts structured Title and Description from a Nuon API error.
// The raw go-swagger error looks like: &{Description:... Error:... UserError:true}
func ParseAPIError(err error) APIError {
	s := err.Error()
	title := extractField(s, "Error:")
	desc := extractField(s, "Description:")

	if title == "" && desc == "" {
		return APIError{Description: s}
	}

	// Title-case the error for display as a banner heading
	if title != "" {
		title = titleCase(title)
	}

	// Override the API description for token expiry since we don't support
	// getting tokens from the Nuon dashboard yet.
	if strings.EqualFold(title, "Token Is Expired") {
		desc = "Generate a new token and update your org connection."
	}

	return APIError{Title: title, Description: desc}
}

// extractField parses a "Key:value" field from a go-swagger struct string.
func extractField(s, key string) string {
	idx := strings.Index(s, key)
	if idx < 0 {
		return ""
	}
	val := s[idx+len(key):]
	for _, boundary := range []string{" Description:", " Error:", " UserError:", "}"} {
		if boundary == " "+key {
			continue
		}
		if end := strings.Index(val, boundary); end >= 0 {
			val = val[:end]
			break
		}
	}
	return strings.TrimSpace(val)
}

// titleCase capitalises the first letter of each word.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// Client wraps the Nuon API client for the installer app
type Client struct {
	client     nuonpkg.Client
	httpClient *http.Client
	apiURL     string
	apiToken   string
	orgID      string
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

	zap.L().Debug("creating Nuon client",
		zap.String("api_url", apiURL),
		zap.String("org_id", orgID),
	)

	// Create Nuon client with options - using exact CLI pattern
	client, err := nuonpkg.New(
		nuonpkg.WithValidator(v),
		nuonpkg.WithAuthToken(apiToken),
		nuonpkg.WithOrgID(orgID),
		nuonpkg.WithURL(apiURL),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Nuon client: %w", err)
	}

	return &Client{
		client: client,
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
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

// ListApps lists apps in the organization. Limited to 10 to avoid the ctl-api
// pagination limit (the SDK overfetches by one, so anything ≥100 trips the cap).
func (c *Client) ListApps(ctx context.Context) ([]*models.AppApp, error) {
	apps, _, err := c.ListAppsPaginated(ctx, 0, 10)
	return apps, err
}

// ListAppsPaginated returns one page of apps and whether more pages exist.
func (c *Client) ListAppsPaginated(ctx context.Context, offset, limit int) ([]*models.AppApp, bool, error) {
	apps, hasMore, err := c.client.GetApps(ctx, &models.GetPaginatedQuery{
		Offset: offset,
		Limit:  limit,
	})
	if err != nil {
		return nil, false, fmt.Errorf("failed to list apps: %w", err)
	}
	return apps, hasMore, nil
}

// GetApp retrieves app details including platform configuration
func (c *Client) GetApp(ctx context.Context, appID string) (*models.AppApp, error) {
	app, err := c.client.GetApp(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	return app, nil
}

// GetAppConfigFull retrieves an app config with all nested relations (permissions, roles, policies)
// by calling the SDK's GetAppConfig with recurse=true.
func (c *Client) GetAppConfigFull(ctx context.Context, appID, appConfigID string) (*models.AppAppConfig, error) {
	recurse := true
	cfg, err := c.client.GetAppConfig(ctx, appID, appConfigID, &recurse)
	if err != nil {
		return nil, fmt.Errorf("failed to get full app config: %w", err)
	}
	return cfg, nil
}

// GetAppInputConfig retrieves the input configuration for an app
func (c *Client) GetAppInputConfig(ctx context.Context, appID string) (interface{}, error) {
	inputCfg, err := c.client.GetAppInputLatestConfig(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to get app input config: %w", err)
	}

	return inputCfg, nil
}

// GetAppInputConfigRaw retrieves the input configuration as raw JSON,
// preserving all fields (including user_configurable) that the SDK struct may drop.
func (c *Client) GetAppInputConfigRaw(ctx context.Context, appID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v1/apps/%s/input-latest-config", c.apiURL, appID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

// GetAppSecretsConfig retrieves the secrets configuration for an app.
// Uses a direct HTTP call because the SDK path doesn't match the ctl-api route.
func (c *Client) GetAppSecretsConfig(ctx context.Context, appID string) (*models.AppAppSecretsConfig, error) {
	url := fmt.Sprintf("%s/v1/apps/%s/latest-secrets-config", c.apiURL, appID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var result models.AppAppSecretsConfig
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
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
	return c.CreateInstallWithCustomName(ctx, appID, appName, "", region, "", "", inputs)
}

// CreateInstallWithCustomName creates a new install with custom name and platform configuration
func (c *Client) CreateInstallWithCustomName(ctx context.Context, appID, appName, customName, region, location, platform string, inputs map[string]string) (*models.AppInstall, error) {
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
		request.AwsAccount = &models.HelpersCreateInstallAWSAccountParams{
			Region: region,
		}
	}

	// Add Azure configuration if location is provided
	if location != "" {
		request.AzureAccount = &models.HelpersCreateInstallAzureAccountParams{
			Location: location,
		}
	}

	// For GCP we make a raw HTTP call.
	// NOTE: the SDK now has HelpersCreateInstallGCPAccountParams, so this
	// workaround can be replaced with request.GcpAccount — left as-is here to
	// keep the SDK migration free of behaviour changes.
	if platform == "gcp" && request.AwsAccount == nil && request.AzureAccount == nil {
		return c.createInstallRaw(ctx, appID, installName, inputs, map[string]interface{}{
			"gcp_account": map[string]interface{}{},
		})
	}

	// Create install with platform-specific configuration
	install, err := c.client.CreateInstall(ctx, appID, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create install '%s' for app %s: %w", installName, appID, err)
	}

	return install, nil
}

// createInstallRaw creates an install via direct HTTP POST, used when the SDK
// struct is missing fields (e.g. gcp_account).
func (c *Client) createInstallRaw(ctx context.Context, appID, name string, inputs map[string]string, extraFields map[string]interface{}) (*models.AppInstall, error) {
	body := map[string]interface{}{
		"name":   name,
		"inputs": inputs,
	}
	for k, v := range extraFields {
		body[k] = v
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := strings.TrimRight(c.apiURL, "/")
	reqURL := fmt.Sprintf("%s/v1/apps/%s/installs", apiURL, url.PathEscape(appID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create install '%s' for app %s: %w", name, appID, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to create install '%s' for app %s: %s", name, appID, string(respBody))
	}

	var install models.AppInstall
	if err := json.Unmarshal(respBody, &install); err != nil {
		return nil, fmt.Errorf("failed to decode install response: %w", err)
	}

	return &install, nil
}

// GetInstall retrieves install details
func (c *Client) GetInstall(ctx context.Context, installID string) (*models.AppInstall, error) {
	install, err := c.client.GetInstall(ctx, installID)
	if err != nil {
		return nil, fmt.Errorf("failed to get install: %w", err)
	}

	return install, nil
}

// DeprovisionInstall deprovisions an install (tears down infrastructure without
// deleting the install record).
//
// This calls DeprovisionInstall, not DeprovisionInstallSandbox: they are separate
// API operations that create different workflows
// (WorkflowTypeDeprovision vs WorkflowTypeDeprovisionSandbox) and hit different
// paths (/deprovision vs /deprovision-sandbox). This wrapper previously called
// the sandbox variant, so the customer portal's "Deprovision" — which warns that
// it "will destroy all data associated with this installation" — only tore down
// the sandbox and left the stack and components running.
func (c *Client) DeprovisionInstall(ctx context.Context, installID string) error {
	_, err := c.client.DeprovisionInstall(ctx, installID, "")
	if err != nil {
		return fmt.Errorf("failed to deprovision install: %w", err)
	}

	return nil
}

// ReprovisionInstall reprovisions an install
func (c *Client) ReprovisionInstall(ctx context.Context, installID string) error {
	if _, err := c.client.ReprovisionInstall(ctx, installID, ""); err != nil {
		return fmt.Errorf("failed to reprovision install: %w", err)
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

	zap.L().Debug("making Nuon API request", zap.String("url", url))

	// Make the request
	resp, err := c.httpClient.Do(req)
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
		zap.L().Error("Nuon API returned non-OK status",
			zap.Int("status", resp.StatusCode),
		)
		// Fallback to original method if direct API call fails
		zap.L().Warn("falling back to standard GetWorkflows method")
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

	zap.L().Debug("fetched workflows from API",
		zap.Int("count", len(workflows)),
	)

	// TODO: Determine hasMore from response headers or pagination info
	hasMore := len(workflows) == limit

	return workflows, hasMore, nil
}

// ApproveWorkflowStep approves a workflow step that requires approval.
// Note: uses direct HTTP call because nuon-go's CreateWorkflowStepApprovalResponse
// omits the step_id path param (SDK bug).
func (c *Client) ApproveWorkflowStep(ctx context.Context, workflowID, stepID, approvalID string) error {
	body := `{"response_type":"approve","note":"Approved via installer app"}`
	reqURL := fmt.Sprintf("%s/v1/workflows/%s/steps/%s/approvals/%s/response", c.apiURL, workflowID, stepID, approvalID)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to approve workflow step: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to approve workflow step: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("failed to approve workflow step: status %d", resp.StatusCode)
	}

	return nil
}

// GetApprovalContents retrieves the plan contents for an approval step (e.g. terraform plan JSON).
func (c *Client) GetApprovalContents(ctx context.Context, workflowID, stepID, approvalID string) (interface{}, error) {
	return c.client.GetWorkflowStepApprovalContents(ctx, workflowID, stepID, approvalID)
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

// RetryWorkflowStep retries a failed workflow step using the workflow-level retry endpoint.
// The step_id is passed in the request body to indicate which step to retry from.
func (c *Client) RetryWorkflowStep(ctx context.Context, workflowID, stepID string) error {
	reqURL := fmt.Sprintf("%s/v1/workflows/%s/retry", c.apiURL, workflowID)
	body := fmt.Sprintf(`{"step_id":%q,"operation":"retry-step"}`, stepID)
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
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

// ---------------------------------------------------------------------------
// V2 workflow types — mirror ctl-api models, independent of nuon-go SDK.
// ---------------------------------------------------------------------------

type CompositeStatus struct {
	CreatedByID            string            `json:"created_by_id,omitempty"`
	CreatedAtTS            int64             `json:"created_at_ts,omitempty"`
	Status                 string            `json:"status,omitempty"`
	StatusHumanDescription string            `json:"status_human_description,omitempty"`
	Metadata               map[string]any    `json:"metadata,omitempty"`
	History                []CompositeStatus `json:"history,omitempty"`
}

type WorkflowStepApproval struct {
	ID          string                        `json:"id,omitempty"`
	CreatedByID string                        `json:"created_by_id,omitempty"`
	CreatedAt   string                        `json:"created_at,omitempty"`
	RunnerJobID *string                       `json:"runner_job_id,omitempty"`
	OwnerID     string                        `json:"owner_id,omitempty"`
	OwnerType   string                        `json:"owner_type,omitempty"`
	Type        string                        `json:"type,omitempty"`
	Response    *WorkflowStepApprovalResponse `json:"response,omitempty"`
}

type WorkflowStepApprovalResponse struct {
	ID          string `json:"id,omitempty"`
	CreatedByID string `json:"created_by_id,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	Type        string `json:"type,omitempty"`
	Note        string `json:"note,omitempty"`
}

type WorkflowStepPolicyValidation struct {
	ID          string          `json:"id,omitempty"`
	CreatedByID string          `json:"created_by_id,omitempty"`
	CreatedAt   string          `json:"created_at,omitempty"`
	Status      CompositeStatus `json:"status,omitempty"`
	Response    string          `json:"response,omitempty"`
}

type QueueSignal struct {
	ID             string          `json:"id,omitempty"`
	CreatedByID    string          `json:"created_by_id,omitempty"`
	CreatedAt      string          `json:"created_at,omitempty"`
	QueueID        string          `json:"queue_id,omitempty"`
	OwnerID        string          `json:"owner_id,omitempty"`
	OwnerType      string          `json:"owner_type,omitempty"`
	Status         CompositeStatus `json:"status,omitempty"`
	Type           string          `json:"type,omitempty"`
	ExecutionCount int             `json:"execution_count,omitempty"`
}

type WorkflowStep struct {
	ID                  string                        `json:"id,omitempty"`
	CreatedByID         string                        `json:"created_by_id,omitempty"`
	CreatedAt           string                        `json:"created_at,omitempty"`
	UpdatedAt           string                        `json:"updated_at,omitempty"`
	WorkflowID          string                        `json:"workflow_id,omitempty"`
	OwnerID             string                        `json:"owner_id,omitempty"`
	OwnerType           string                        `json:"owner_type,omitempty"`
	InstallWorkflowID   string                        `json:"install_workflow_id,omitempty"`
	Status              *CompositeStatus              `json:"status,omitempty"`
	Name                string                        `json:"name,omitempty"`
	Idx                 int                           `json:"idx,omitempty"`
	WorkflowStepGroupID string                        `json:"workflow_step_group_id,omitempty"`
	GroupIdx            int                           `json:"group_idx,omitempty"`
	GroupRetryIdx       int                           `json:"group_retry_idx"`
	GroupParallel       bool                          `json:"group_parallel,omitempty"`
	ExecutionType       string                        `json:"execution_type,omitempty"`
	StepTargetID        string                        `json:"step_target_id,omitempty"`
	StepTargetType      string                        `json:"step_target_type,omitempty"`
	Metadata            map[string]string             `json:"metadata,omitempty"`
	StartedAt           string                        `json:"started_at,omitempty"`
	FinishedAt          string                        `json:"finished_at,omitempty"`
	Finished            bool                          `json:"finished,omitempty"`
	Approval            *WorkflowStepApproval         `json:"approval,omitempty"`
	PolicyValidation    *WorkflowStepPolicyValidation `json:"policy_validation,omitempty"`
	ExecutionTime       int64                         `json:"execution_time,omitempty"`
	Links               map[string]any                `json:"links,omitempty"`
	Retryable           bool                          `json:"retryable,omitempty"`
	Skippable           bool                          `json:"skippable,omitempty"`
	Retried             bool                          `json:"retried,omitempty"`
	RetryIndex          int                           `json:"retry_index"`
	ResultDirective     string                        `json:"result_directive,omitempty"`
	Labels              map[string]string             `json:"labels,omitempty"`
}

type WorkflowStepGroup struct {
	ID          string            `json:"id,omitempty"`
	CreatedByID string            `json:"created_by_id,omitempty"`
	CreatedAt   string            `json:"created_at,omitempty"`
	UpdatedAt   string            `json:"updated_at,omitempty"`
	WorkflowID  string            `json:"workflow_id,omitempty"`
	GroupIdx    int               `json:"group_idx"`
	Parallel    bool              `json:"parallel,omitempty"`
	Status      CompositeStatus   `json:"status,omitempty"`
	Name        string            `json:"name,omitempty"`
	QueueSignal *QueueSignal      `json:"queue_signal,omitempty"`
	Steps       []*WorkflowStep   `json:"steps,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type Workflow struct {
	ID              string              `json:"id,omitempty"`
	CreatedByID     string              `json:"created_by_id,omitempty"`
	CreatedAt       string              `json:"created_at,omitempty"`
	UpdatedAt       string              `json:"updated_at,omitempty"`
	OwnerID         string              `json:"owner_id,omitempty"`
	OwnerType       string              `json:"owner_type,omitempty"`
	Type            string              `json:"type,omitempty"`
	Metadata        map[string]string   `json:"metadata,omitempty"`
	Status          *CompositeStatus    `json:"status,omitempty"`
	Role            string              `json:"role,omitempty"`
	ApprovalOption  string              `json:"approval_option,omitempty"`
	PlanOnly        bool                `json:"plan_only,omitempty"`
	ResultDirective string              `json:"result_directive,omitempty"`
	StartedAt       string              `json:"started_at,omitempty"`
	FinishedAt      string              `json:"finished_at,omitempty"`
	Finished        bool                `json:"finished,omitempty"`
	StepGroups      []WorkflowStepGroup `json:"step_groups,omitempty"`
	Steps           []*WorkflowStep     `json:"steps,omitempty"`
	Name            string              `json:"name,omitempty"`
	ExecutionTime   int64               `json:"execution_time,omitempty"`
	Links           map[string]any      `json:"links,omitempty"`
	Labels          map[string]string   `json:"-"`
}

// GetWorkflowV2 retrieves a single workflow by ID via direct HTTP,
// returning the full response including step_groups.
func (c *Client) GetWorkflowV2(ctx context.Context, workflowID string) (*Workflow, error) {
	reqURL := fmt.Sprintf("%s/v1/workflows/%s", c.apiURL, workflowID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get workflow returned status %d", resp.StatusCode)
	}
	var workflow Workflow
	if err := json.NewDecoder(resp.Body).Decode(&workflow); err != nil {
		return nil, fmt.Errorf("failed to decode workflow: %w", err)
	}
	return &workflow, nil
}

// GetInstallWorkflowsV2 retrieves workflow history using the V2 Workflow type.
func (c *Client) GetInstallWorkflowsV2(ctx context.Context, installID string, offset int, limit int) ([]*Workflow, bool, error) {
	reqURL := fmt.Sprintf("%s/v1/installs/%s/workflows?offset=%d&limit=%d&planonly=false",
		c.apiURL, installID, offset, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("get install workflows returned status %d", resp.StatusCode)
	}
	var workflows []*Workflow
	if err := json.NewDecoder(resp.Body).Decode(&workflows); err != nil {
		return nil, false, fmt.Errorf("failed to decode workflows: %w", err)
	}
	hasMore := len(workflows) >= limit
	return workflows, hasMore, nil
}

// GetWorkflowStepGroups retrieves step groups for a workflow.
func (c *Client) GetWorkflowStepGroups(ctx context.Context, workflowID string) ([]WorkflowStepGroup, error) {
	reqURL := fmt.Sprintf("%s/v1/workflows/%s/step-groups", c.apiURL, workflowID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get step groups returned status %d", resp.StatusCode)
	}
	var groups []WorkflowStepGroup
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		return nil, fmt.Errorf("failed to decode step groups: %w", err)
	}
	return groups, nil
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
		ActionWorkflowConfigID: &actionWorkflowConfigID,
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

// GetInstallCurrentInputs retrieves the current input values for an install
func (c *Client) GetInstallCurrentInputs(ctx context.Context, installID string) (*models.AppInstallInputs, error) {
	inputs, err := c.client.GetInstallCurrentInputs(ctx, installID)
	if err != nil {
		return nil, fmt.Errorf("failed to get install inputs: %w", err)
	}
	return inputs, nil
}

// UpdateInstallInputs updates the inputs for an install
// Returns the workflowID for the triggered workflow
func (c *Client) UpdateInstallInputs(ctx context.Context, installID string, inputs map[string]string) (string, error) {
	req := &models.ServiceUpdateInstallInputsRequest{
		Inputs: inputs,
	}
	updated, err := c.client.UpdateInstallInputs(ctx, installID, req)
	if err != nil {
		return "", fmt.Errorf("failed to update install inputs: %w", err)
	}
	if updated == nil {
		return "", nil
	}
	return updated.WorkflowID, nil
}

// StackRun represents a single stack run from the Nuon API.
// The SDK does not include a generated model for this resource.
type StackRun struct {
	ID                    string        `json:"id"`
	InstallID             string        `json:"install_id"`
	InstallStackVersionID string        `json:"install_stack_version_id"`
	StatusDescription     string        `json:"status_description"`
	CreatedAt             string        `json:"created_at"`
	UpdatedAt             string        `json:"updated_at"`
	Data                  *StackRunData `json:"data,omitempty"`
	VersionStatus         string        // populated by the handler from the parent stack version
	TemplateURL           string        // populated by the handler from the parent stack version
}

// StackRunData holds optional metadata returned by the API.
type StackRunData struct {
	RequestType string `json:"request_type"`
}

// GetInstallStackRuns retrieves stack runs for an install.
func (c *Client) GetInstallStackRuns(ctx context.Context, installID string) ([]StackRun, error) {
	reqURL := fmt.Sprintf("%s/v1/installs/%s/stack-runs", c.apiURL, installID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var runs []StackRun
	if err := json.Unmarshal(body, &runs); err != nil {
		return nil, fmt.Errorf("failed to parse stack runs: %w", err)
	}
	return runs, nil
}

// GetInstallSandboxRuns retrieves sandbox runs for an install.
func (c *Client) GetInstallSandboxRuns(ctx context.Context, installID string) ([]*models.AppInstallSandboxRun, error) {
	runs, _, err := c.client.GetInstallSandboxRuns(ctx, installID, &models.GetPaginatedQuery{Offset: 0, Limit: 100})
	return runs, err
}

// GetInstallComponents retrieves install components for an install.
func (c *Client) GetInstallComponents(ctx context.Context, installID string) ([]*models.AppInstallComponent, error) {
	components, _, err := c.client.GetInstallComponents(ctx, installID, &models.GetPaginatedQuery{Offset: 0, Limit: 100})
	return components, err
}

// GetInstallDeploys retrieves deploys for an install.
func (c *Client) GetInstallDeploys(ctx context.Context, installID string) ([]*models.AppInstallDeploy, error) {
	deploys, _, err := c.client.GetInstallDeploys(ctx, installID, &models.GetPaginatedQuery{Offset: 0, Limit: 100})
	return deploys, err
}

// GetInstallDeploy retrieves a single deploy by ID with full details including outputs.
func (c *Client) GetInstallDeploy(ctx context.Context, installID, deployID string) (*models.AppInstallDeploy, error) {
	return c.client.GetInstallDeploy(ctx, installID, deployID)
}

// GetInstallSandboxRun retrieves a single sandbox run by ID.
func (c *Client) GetInstallSandboxRun(ctx context.Context, installID, runID string) (*models.AppInstallSandboxRun, error) {
	reqURL := fmt.Sprintf("%s/v1/installs/sandbox-runs/%s", c.apiURL, runID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sandbox run: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sandbox run fetch returned status %d", resp.StatusCode)
	}
	var run models.AppInstallSandboxRun
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("failed to decode sandbox run: %w", err)
	}
	return &run, nil
}

// GetInstallActionWorkflowRun retrieves a single action workflow run by ID.
func (c *Client) GetInstallActionWorkflowRun(ctx context.Context, installID, runID string) (*models.AppInstallActionWorkflowRun, error) {
	return c.client.GetInstallActionWorkflowRun(ctx, installID, runID)
}

// GetGenericResource fetches any resource by path and returns it as a raw JSON map.
func (c *Client) GetGenericResource(ctx context.Context, path string) (map[string]interface{}, error) {
	reqURL := fmt.Sprintf("%s%s", c.apiURL, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch returned status %d", resp.StatusCode)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

// GetAppComponents retrieves all components for an app
func (c *Client) GetAppComponents(ctx context.Context, appID string) ([]*models.AppComponent, error) {
	components, _, err := c.client.GetAppComponents(ctx, appID, &models.GetPaginatedQuery{
		Offset: 0,
		Limit:  100,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get app components: %w", err)
	}
	return components, nil
}

// GetInstallComponentOutputs retrieves the outputs for a specific install component.
func (c *Client) GetInstallComponentOutputs(ctx context.Context, installID, componentID string) (map[string]string, error) {
	reqURL := fmt.Sprintf("%s/v1/installs/%s/components/%s/outputs", c.apiURL, installID, componentID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var outputs map[string]string
	if err := json.Unmarshal(body, &outputs); err != nil {
		return nil, fmt.Errorf("failed to parse outputs: %w", err)
	}
	return outputs, nil
}

// GetAppComponentLatestBuild retrieves the latest build for a component.
func (c *Client) GetAppComponentLatestBuild(ctx context.Context, appID, componentID string) (*models.AppComponentBuild, error) {
	reqURL := fmt.Sprintf("%s/v1/apps/%s/components/%s/builds/latest", c.apiURL, appID, componentID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var build models.AppComponentBuild
	if err := json.Unmarshal(body, &build); err != nil {
		return nil, fmt.Errorf("failed to parse build: %w", err)
	}
	return &build, nil
}

// appPoliciesConfigFull is the raw response shape for the policies config endpoint.
// The nuon-go generated model omits the Policies array, so we decode it manually.
type appPoliciesConfigFull struct {
	Policies []AppPoliciesConfigPolicy `json:"policies"`
}

// AppPoliciesConfigPolicy is an individual policy from the app policies config.
type AppPoliciesConfigPolicy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Engine      string `json:"engine"`
	Description string `json:"description"`
	Contents    string `json:"contents"`
}

// GetLatestAppPoliciesConfigFull fetches the latest policies config, including the
// Policies array with names and types (the SDK-generated model drops this field).
func (c *Client) GetLatestAppPoliciesConfigFull(ctx context.Context, appID string) ([]AppPoliciesConfigPolicy, error) {
	url := fmt.Sprintf("%s/v1/apps/%s/latest-policies-config", c.apiURL, appID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var result appPoliciesConfigFull
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result.Policies, nil
}

// PolicyReport represents a policy evaluation report from the Nuon API.
type PolicyReport struct {
	ID            string              `json:"id"`
	OwnerID       string              `json:"owner_id"`
	OwnerType     string              `json:"owner_type"`
	Status        *PolicyReportStatus `json:"status,omitempty"`
	DenyCount     int                 `json:"deny_count"`
	WarnCount     int                 `json:"warn_count"`
	PassCount     int                 `json:"pass_count"`
	EvaluatedAt   string              `json:"evaluated_at"`
	CreatedAt     string              `json:"created_at"`
	ComponentName string              `json:"component_name"`
	PolicyIds     []string            `json:"policy_ids"`
	Policies      []PolicyResult      `json:"policies"`
	Violations    []PolicyViolation   `json:"violations"`
	// PolicyName is not from the API — it is resolved by the handler.
	PolicyName string `json:"-"`
}

// PolicyReportStatus holds the composite status of a policy report.
type PolicyReportStatus struct {
	Status string `json:"status"`
}

// PolicyResult holds per-policy evaluation results within a report.
type PolicyResult struct {
	PolicyID   string `json:"policy_id"`
	PolicyName string `json:"policy_name"`
	Status     string `json:"status"`
	DenyCount  int    `json:"deny_count"`
	WarnCount  int    `json:"warn_count"`
	PassCount  int    `json:"pass_count"`
	InputCount int    `json:"input_count"`
}

// PolicyViolation holds a single policy violation within a report.
type PolicyViolation struct {
	PolicyID      string `json:"policy_id"`
	PolicyName    string `json:"policy_name"`
	Severity      string `json:"severity"`
	Message       string `json:"message"`
	InputIdentity string `json:"input_identity"`
	InputIndex    int    `json:"input_index"`
}

// GetInstallPolicyReports retrieves policy reports for an install, optionally filtered by owner type.
func (c *Client) GetInstallPolicyReports(ctx context.Context, installID, ownerType string) ([]PolicyReport, error) {
	reqURL := fmt.Sprintf("%s/v1/policy-reports?install_id=%s", c.apiURL, installID)
	if ownerType != "" {
		reqURL += "&owner_type=" + ownerType
	}
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var reports []PolicyReport
	if err := json.NewDecoder(resp.Body).Decode(&reports); err != nil {
		return nil, fmt.Errorf("failed to decode policy reports: %w", err)
	}
	return reports, nil
}

// GetPolicyReport retrieves a single policy report by ID.
func (c *Client) GetPolicyReport(ctx context.Context, reportID string) (*PolicyReport, error) {
	reqURL := fmt.Sprintf("%s/v1/policy-reports/%s", c.apiURL, reportID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("policy report not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var report PolicyReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return nil, fmt.Errorf("failed to decode policy report: %w", err)
	}
	return &report, nil
}

// stepTargetRoleResponse is a minimal struct to extract the role from a sandbox run or deploy.
type stepTargetRoleResponse struct {
	Role string `json:"role"`
}

// GetStepTargetRole fetches the role associated with a workflow step's target (deploy or sandbox run).
// Returns empty string if the target type doesn't support roles or on any error.
func (c *Client) GetStepTargetRole(ctx context.Context, targetID, targetType, installID string) (string, error) {
	var reqURL string
	switch targetType {
	case "install_sandbox_runs":
		reqURL = fmt.Sprintf("%s/v1/installs/sandbox-runs/%s", c.apiURL, targetID)
	case "install_deploys":
		reqURL = fmt.Sprintf("%s/v1/installs/%s/deploys/%s", c.apiURL, installID, targetID)
	default:
		return "", nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	var result stepTargetRoleResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil
	}
	return result.Role, nil
}

// GetAppSandboxLatestConfig retrieves the latest sandbox config for an app
func (c *Client) GetAppSandboxLatestConfig(ctx context.Context, appID string) (*models.AppAppSandboxConfig, error) {
	cfg, err := c.client.GetAppSandboxLatestConfig(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sandbox config: %w", err)
	}
	return cfg, nil
}

// GetTerraformWorkspaceStates retrieves the state JSON records for a terraform workspace.
// Returns the list ordered by most recent first.
func (c *Client) GetTerraformWorkspaceStates(ctx context.Context, workspaceID string) ([]*models.AppTerraformWorkspaceStateJSON, error) {
	reqURL := fmt.Sprintf("%s/v1/terraform-workspaces/%s/state-json", c.apiURL, workspaceID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var result []*models.AppTerraformWorkspaceStateJSON
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

// TerraformStateOutput represents a single output in the terraform state.
type TerraformStateOutput struct {
	Type  interface{} `json:"type"`
	Value interface{} `json:"value"`
}

// TerraformStateJSON represents the full terraform state JSON response.
type TerraformStateJSON struct {
	Values struct {
		RootModule *TerraformStateModule           `json:"root_module"`
		Outputs    map[string]TerraformStateOutput `json:"outputs"`
	} `json:"values"`
}

// TerraformStateModule represents a terraform module with resources and child modules.
type TerraformStateModule struct {
	Resources    []TerraformResource    `json:"resources"`
	ChildModules []TerraformStateModule `json:"child_modules"`
}

// TerraformResource represents a single resource in the terraform state.
type TerraformResource struct {
	Address       string                 `json:"address"`
	Mode          string                 `json:"mode"`
	Type          string                 `json:"type"`
	Name          string                 `json:"name"`
	ProviderName  string                 `json:"provider_name"`
	SchemaVersion int                    `json:"schema_version"`
	Values        map[string]interface{} `json:"values"`
}

// GetTerraformWorkspaceStateResources fetches the full terraform state and extracts all resources.
func (c *Client) GetTerraformWorkspaceStateOutputs(ctx context.Context, workspaceID, stateID string) (map[string]string, error) {
	reqURL := fmt.Sprintf("%s/v1/runners/terraform-workspace/%s/state-json/%s", c.apiURL, workspaceID, stateID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var state TerraformStateJSON
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(state.Values.Outputs) == 0 {
		return nil, nil
	}
	outputs := make(map[string]string, len(state.Values.Outputs))
	for k, o := range state.Values.Outputs {
		if s, ok := o.Value.(string); ok {
			outputs[k] = s
		} else if b, err := json.MarshalIndent(o.Value, "", "  "); err == nil {
			outputs[k] = string(b)
		}
	}
	return outputs, nil
}

func (c *Client) GetTerraformWorkspaceStateResources(ctx context.Context, workspaceID, stateID string) ([]TerraformResource, error) {
	reqURL := fmt.Sprintf("%s/v1/runners/terraform-workspace/%s/state-json/%s", c.apiURL, workspaceID, stateID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var state TerraformStateJSON
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if state.Values.RootModule == nil {
		return nil, nil
	}
	return collectResources(state.Values.RootModule), nil
}

// collectResources flattens resources from a module and all its child modules.
func collectResources(mod *TerraformStateModule) []TerraformResource {
	var resources []TerraformResource
	resources = append(resources, mod.Resources...)
	for i := range mod.ChildModules {
		resources = append(resources, collectResources(&mod.ChildModules[i])...)
	}
	return resources
}

// ListAppInstalls searches installs for a given app by query string.
// Returns up to limit results matching the query.
func (c *Client) ListAppInstalls(ctx context.Context, appID, query string) ([]*models.AppInstall, error) {
	reqURL := fmt.Sprintf("%s/v1/apps/%s/installs?q=%s&limit=20",
		c.apiURL, appID, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var installs []*models.AppInstall
	if err := json.Unmarshal(body, &installs); err != nil {
		return nil, fmt.Errorf("failed to parse installs: %w", err)
	}

	return installs, nil
}

// IsInstallNameAvailable checks if an install name is available for an app via the Nuon API.
// It queries existing installs and checks for exact name matches (case-insensitive).
// Returns true if the name is available (no existing install with that name).
func (c *Client) IsInstallNameAvailable(ctx context.Context, appID, name string) (bool, error) {
	// Use search query to find installs with matching name
	// The API returns installs that match the search query
	url := fmt.Sprintf("%s/v1/apps/%s/installs?q=%s&limit=100",
		c.apiURL, appID, name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var installs []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &installs); err != nil {
		return false, fmt.Errorf("failed to parse installs: %w", err)
	}

	// Check for exact match (case-insensitive)
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	for _, install := range installs {
		if strings.ToLower(strings.TrimSpace(install.Name)) == normalizedName {
			return false, nil // Name is taken
		}
	}

	return true, nil // Name is available
}

// InstallIAMPolicy represents a single IAM policy attached to a role.
type InstallIAMPolicy struct {
	ManagedPolicyName string `json:"managed_policy_name"`
	Name              string `json:"name"`
	Contents          string `json:"contents"` // base64-encoded JSON
}

// InstallIAMRole represents an IAM role from the app-permissions-config endpoint.
type InstallIAMRole struct {
	Name                string             `json:"name"`
	DisplayName         string             `json:"display_name"`
	Description         string             `json:"description"`
	Enabled             bool               `json:"enabled"`
	Type                string             `json:"type"`
	Policies            []InstallIAMPolicy `json:"policies"`
	PermissionsBoundary string             `json:"permissions_boundary"` // base64-encoded JSON
	ARN                 string             `json:"arn"`
}

// InstallAppPermissionsConfig is the response from GET /v1/installs/:id/app-permissions-config.
type InstallAppPermissionsConfig struct {
	ProvisionRole   *InstallIAMRole  `json:"provision_role"`
	DeprovisionRole *InstallIAMRole  `json:"deprovision_role"`
	MaintenanceRole *InstallIAMRole  `json:"maintenance_role"`
	BreakGlassRoles []InstallIAMRole `json:"break_glass_roles"`
	CustomRoles     []InstallIAMRole `json:"custom_roles"`
}

// GetInstallAppPermissionsConfig fetches the IAM roles configured for an install.
func (c *Client) GetInstallAppPermissionsConfig(ctx context.Context, installID string) (*InstallAppPermissionsConfig, error) {
	url := fmt.Sprintf("%s/v1/installs/%s/app-permissions-config", c.apiURL, installID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var result InstallAppPermissionsConfig
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}
