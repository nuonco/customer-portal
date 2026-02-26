package nuon

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	nuonpkg "github.com/nuonco/nuon-go"
	"github.com/nuonco/nuon-go/models"
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

// Client wraps the Nuon API client for the installer app
type Client struct {
	client   nuonpkg.Client
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
	resp, err := (&http.Client{}).Do(req)
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

	zap.L().Debug("making Nuon API request", zap.String("url", url))

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
	_, workflowID, err := c.client.UpdateInstallInputs(ctx, installID, req)
	if err != nil {
		return "", fmt.Errorf("failed to update install inputs: %w", err)
	}
	return workflowID, nil
}

// AuditLogEntry represents a single audit log entry from the Nuon API
type AuditLogEntry struct {
	InstallID string
	LogLine   string
	TimeStamp time.Time
	Type      string
}

// GetInstallAuditLogs retrieves audit logs for an install within a time range
// The API returns CSV format with columns: install_id, log_line, time_stamp, type
func (c *Client) GetInstallAuditLogs(ctx context.Context, installID string, start, end time.Time) ([]AuditLogEntry, error) {
	// Build URL with required start and end parameters (RFC3339 format)
	url := fmt.Sprintf("%s/v1/installs/%s/audit_logs?start=%s&end=%s",
		c.apiURL, installID,
		start.Format(time.RFC3339),
		end.Format(time.RFC3339))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse CSV response
	// Columns: install_id, log_line, time_stamp, type
	csvReader := csv.NewReader(strings.NewReader(string(body)))

	var entries []AuditLogEntry

	// Read all records (skip header if present)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV response: %w", err)
	}

	for i, record := range records {
		// Skip header row (if first row contains column names)
		if i == 0 && len(record) > 0 && record[0] == "install_id" {
			continue
		}

		// Expect 4 columns: install_id, log_line, time_stamp, type
		if len(record) < 4 {
			continue
		}

		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, record[2])
		if err != nil {
			// Try alternate formats
			timestamp, err = time.Parse(time.RFC3339Nano, record[2])
			if err != nil {
				// Use current time as fallback
				timestamp = time.Now()
			}
		}

		entries = append(entries, AuditLogEntry{
			InstallID: record[0],
			LogLine:   record[1],
			TimeStamp: timestamp,
			Type:      record[3],
		})
	}

	// Sort by timestamp (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].TimeStamp.After(entries[j].TimeStamp)
	})

	return entries, nil
}

// ServiceReadme represents the install readme returned by the API
type ServiceReadme struct {
	Readme   string   `json:"readme"`
	Original string   `json:"original"`
	Warnings []string `json:"warnings"`
}

// GetInstallReadme fetches the install-specific readme rendered with install data
func (c *Client) GetInstallReadme(ctx context.Context, installID string) (*ServiceReadme, error) {
	// Build URL for the readme endpoint
	url := fmt.Sprintf("%s/v1/installs/%s/readme", c.apiURL, installID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Return nil for 404 - app may not have readme configured
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var readme ServiceReadme
	if err := json.Unmarshal(body, &readme); err != nil {
		return nil, fmt.Errorf("failed to parse readme: %w", err)
	}

	return &readme, nil
}

// GetAppComponents retrieves all components for an app
func (c *Client) GetAppComponents(ctx context.Context, appID string) ([]*models.AppComponent, error) {
	components, _, err := c.client.GetAppComponents(ctx, appID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get app components: %w", err)
	}
	return components, nil
}

// appPoliciesConfigFull is the raw response shape for the policies config endpoint.
// The nuon-go generated model omits the Policies array, so we decode it manually.
type appPoliciesConfigFull struct {
	Policies []AppPoliciesConfigPolicy `json:"policies"`
}

// AppPoliciesConfigPolicy is an individual policy from the app policies config.
type AppPoliciesConfigPolicy struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Engine      string `json:"engine"`
	Description string `json:"description"`
}

// GetLatestAppPermissionsConfig fetches the latest permissions config for an app.
// Uses a direct HTTP call to avoid the SDK TextConsumer deserialization error.
func (c *Client) GetLatestAppPermissionsConfig(ctx context.Context, appID string) (*models.AppAppPermissionsConfig, error) {
	url := fmt.Sprintf("%s/v1/apps/%s/latest-app-permissions-config", c.apiURL, appID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-Nuon-Org-ID", c.orgID)
	resp, err := (&http.Client{}).Do(req)
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
	var result models.AppAppPermissionsConfig
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
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
	resp, err := (&http.Client{}).Do(req)
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

// GetAppSandboxLatestConfig retrieves the latest sandbox config for an app
func (c *Client) GetAppSandboxLatestConfig(ctx context.Context, appID string) (*models.AppAppSandboxConfig, error) {
	cfg, err := c.client.GetAppSandboxLatestConfig(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sandbox config: %w", err)
	}
	return cfg, nil
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

	client := &http.Client{}
	resp, err := client.Do(req)
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
	client := &http.Client{}
	resp, err := client.Do(req)
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
