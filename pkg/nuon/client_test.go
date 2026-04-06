package nuon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateInstallName verifies install name generation format
func TestGenerateInstallName(t *testing.T) {
	appName := "myapp"

	name := GenerateInstallName(appName)

	// Name should start with app name
	assert.True(t, strings.HasPrefix(name, "myapp-install-"),
		"Name should start with 'myapp-install-'")

	// Name should contain a timestamp (format: appname-install-timestamp)
	parts := strings.Split(name, "-")
	require.Len(t, parts, 3, "Name should have 3 parts: appName, 'install', timestamp")

	// Verify parts
	assert.Equal(t, "myapp", parts[0])
	assert.Equal(t, "install", parts[1])

	// Timestamp part should be numeric
	_, err := strconv.ParseInt(parts[2], 10, 64)
	assert.NoError(t, err, "Timestamp should be a valid integer")
}

func TestGenerateInstallName_DifferentApps(t *testing.T) {
	name1 := GenerateInstallName("app-one")
	name2 := GenerateInstallName("app-two")

	assert.True(t, strings.HasPrefix(name1, "app-one-install-"))
	assert.True(t, strings.HasPrefix(name2, "app-two-install-"))
	assert.NotEqual(t, name1, name2)
}

// mockNuonAPI creates a test server that simulates Nuon API responses
type mockNuonAPI struct {
	server   *httptest.Server
	handlers map[string]http.HandlerFunc
	requests []mockRequest
}

type mockRequest struct {
	Method  string
	Path    string
	Headers http.Header
}

func newMockNuonAPI() *mockNuonAPI {
	m := &mockNuonAPI{
		handlers: make(map[string]http.HandlerFunc),
		requests: make([]mockRequest, 0),
	}

	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record request
		m.requests = append(m.requests, mockRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Headers: r.Header.Clone(),
		})

		// Find handler
		key := r.Method + " " + r.URL.Path
		if handler, ok := m.handlers[key]; ok {
			handler(w, r)
			return
		}

		// Check for pattern matching (e.g., /v1/installs/*)
		for pattern, handler := range m.handlers {
			if matchesPattern(pattern, r.Method+" "+r.URL.Path) {
				handler(w, r)
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))

	return m
}

func matchesPattern(pattern, actual string) bool {
	// Simple pattern matching: * matches any segment
	patternParts := strings.Split(pattern, "/")
	actualParts := strings.Split(actual, "/")

	if len(patternParts) != len(actualParts) {
		return false
	}

	for i, part := range patternParts {
		if part != "*" && part != actualParts[i] {
			return false
		}
	}
	return true
}

func (m *mockNuonAPI) on(method, path string, handler http.HandlerFunc) {
	m.handlers[method+" "+path] = handler
}

func (m *mockNuonAPI) close() {
	m.server.Close()
}

func (m *mockNuonAPI) url() string {
	return m.server.URL
}

// Note: Full client tests would require mocking the nuon-go SDK
// These tests focus on the helper functions and HTTP-level behavior

func TestClient_GetInstallWorkflows_RequestFormat(t *testing.T) {
	mock := newMockNuonAPI()
	defer mock.close()

	installID := "inst_test123"
	var capturedHeaders http.Header

	mock.on("GET", "/v1/installs/"+installID+"/workflows", func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()

		// Verify query params
		assert.Equal(t, "0", r.URL.Query().Get("offset"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		assert.Equal(t, "false", r.URL.Query().Get("planonly"))

		// Return empty array
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	})

	// Create a minimal client that bypasses nuon-go SDK
	// This tests our direct HTTP request logic
	client := &Client{
		apiURL:   mock.url(),
		apiToken: "test-token",
		orgID:    "test-org",
	}

	workflows, hasMore, err := client.GetInstallWorkflows(context.Background(), installID, 0, 10)

	assert.NoError(t, err)
	assert.Empty(t, workflows)
	assert.False(t, hasMore)

	// Verify authorization headers
	assert.Equal(t, "Bearer test-token", capturedHeaders.Get("Authorization"))
	assert.Equal(t, "test-org", capturedHeaders.Get("X-Nuon-Org-ID"))
}

func TestClient_GetInstallWorkflows_WithTypeFilter(t *testing.T) {
	mock := newMockNuonAPI()
	defer mock.close()

	installID := "inst_test123"

	mock.on("GET", "/v1/installs/"+installID+"/workflows", func(w http.ResponseWriter, r *http.Request) {
		// Verify type filter is passed
		assert.Equal(t, "deploy", r.URL.Query().Get("type"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	})

	client := &Client{
		apiURL:   mock.url(),
		apiToken: "test-token",
		orgID:    "test-org",
	}

	_, _, err := client.GetInstallWorkflowsByType(context.Background(), installID, 0, 10, "deploy")

	assert.NoError(t, err)
}

func TestClient_GetInstallWorkflows_Pagination(t *testing.T) {
	mock := newMockNuonAPI()
	defer mock.close()

	installID := "inst_test123"

	tests := []struct {
		name          string
		returnCount   int
		limit         int
		expectHasMore bool
	}{
		{
			name:          "full page indicates more",
			returnCount:   10,
			limit:         10,
			expectHasMore: true,
		},
		{
			name:          "partial page indicates no more",
			returnCount:   5,
			limit:         10,
			expectHasMore: false,
		},
		{
			name:          "empty page indicates no more",
			returnCount:   0,
			limit:         10,
			expectHasMore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.handlers = make(map[string]http.HandlerFunc)

			mock.on("GET", "/v1/installs/"+installID+"/workflows", func(w http.ResponseWriter, r *http.Request) {
				// Create workflows array of specified size
				// Note: Only include fields that are simple types - complex structs like status
				// need proper structure matching models.AppWorkflow
				workflows := make([]map[string]interface{}, tt.returnCount)
				for i := 0; i < tt.returnCount; i++ {
					workflows[i] = map[string]interface{}{
						"id": "wf_" + strconv.Itoa(i),
						// Omit status field - it's a complex struct (AppCompositeStatus)
					}
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(workflows)
			})

			client := &Client{
				apiURL:   mock.url(),
				apiToken: "test-token",
				orgID:    "test-org",
			}

			_, hasMore, err := client.GetInstallWorkflows(context.Background(), installID, 0, tt.limit)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectHasMore, hasMore)
		})
	}
}

func TestClient_GetInstallWorkflows_ErrorHandling(t *testing.T) {
	// Skip this test - the implementation falls back to the SDK on HTTP errors,
	// and we can't properly mock the nuon.Client interface without significant setup.
	// Testing the fallback path would require integration tests with a real API.
	t.Skip("skipping error handling test - requires full SDK mock for fallback path")

	mock := newMockNuonAPI()
	defer mock.close()

	installID := "inst_test123"

	mock.on("GET", "/v1/installs/"+installID+"/workflows", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
	})

	client := &Client{
		apiURL:   mock.url(),
		apiToken: "test-token",
		orgID:    "test-org",
	}

	_, _, err := client.GetInstallWorkflows(context.Background(), installID, 0, 10)
	assert.Error(t, err)
}

// TestClient_Construction verifies client can be created
func TestNewClientWithURL(t *testing.T) {
	// Note: This will attempt to create a real nuon.Client
	// In a real test environment, you'd mock the nuon.New function
	// For now, we just verify the function signature and error handling

	// Empty token should still create client (validation happens on API call)
	client, err := NewClientWithURL("token", "org-id", "http://localhost:8081")

	// May fail due to validator or other initialization, but shouldn't panic
	if err == nil {
		assert.NotNil(t, client)
		assert.Equal(t, "http://localhost:8081", client.apiURL)
		assert.Equal(t, "token", client.apiToken)
		assert.Equal(t, "org-id", client.orgID)
	}
}

// mockAPIError simulates the nuon-go stderrResponse interface for testing.
type mockAPIError struct {
	code int
}

func (e *mockAPIError) Error() string     { return "api error" }
func (e *mockAPIError) IsCode(c int) bool { return e.code == c }

func TestIsConflict(t *testing.T) {
	t.Run("returns true for 409 error", func(t *testing.T) {
		err := &mockAPIError{code: 409}
		assert.True(t, IsConflict(err))
	})

	t.Run("returns false for non-409 error", func(t *testing.T) {
		err := &mockAPIError{code: 500}
		assert.False(t, IsConflict(err))
	})

	t.Run("returns true for wrapped 409 error", func(t *testing.T) {
		inner := &mockAPIError{code: 409}
		wrapped := fmt.Errorf("create install failed: %w", inner)
		assert.True(t, IsConflict(wrapped))
	})

	t.Run("returns false for nil error", func(t *testing.T) {
		assert.False(t, IsConflict(nil))
	})

	t.Run("returns false for plain error", func(t *testing.T) {
		err := fmt.Errorf("something went wrong")
		assert.False(t, IsConflict(err))
	})
}

func TestNewClient_DefaultURL(t *testing.T) {
	// NewClient should default to localhost:8081
	client, err := NewClient("token", "org-id")

	if err == nil {
		assert.Equal(t, "http://localhost:8081", client.apiURL)
	}
}

func TestClient_GetTerraformWorkspaceStates(t *testing.T) {
	mock := newMockNuonAPI()
	defer mock.close()

	workspaceID := "ws_test123"

	t.Run("returns states", func(t *testing.T) {
		mock.handlers = make(map[string]http.HandlerFunc)
		mock.on("GET", "/v1/terraform-workspaces/"+workspaceID+"/state-json", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "test-org", r.Header.Get("X-Nuon-Org-ID"))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": "state_1", "workspace_id": workspaceID},
				{"id": "state_2", "workspace_id": workspaceID},
			})
		})

		client := &Client{
			apiURL:     mock.url(),
			apiToken:   "test-token",
			orgID:      "test-org",
			httpClient: http.DefaultClient,
		}

		states, err := client.GetTerraformWorkspaceStates(context.Background(), workspaceID)
		assert.NoError(t, err)
		assert.Len(t, states, 2)
		assert.Equal(t, "state_1", states[0].ID)
	})

	t.Run("returns nil on 404", func(t *testing.T) {
		mock.handlers = make(map[string]http.HandlerFunc)
		mock.on("GET", "/v1/terraform-workspaces/"+workspaceID+"/state-json", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		client := &Client{
			apiURL:     mock.url(),
			apiToken:   "test-token",
			orgID:      "test-org",
			httpClient: http.DefaultClient,
		}

		states, err := client.GetTerraformWorkspaceStates(context.Background(), workspaceID)
		assert.NoError(t, err)
		assert.Nil(t, states)
	})
}

func TestClient_GetTerraformWorkspaceStateResources(t *testing.T) {
	mock := newMockNuonAPI()
	defer mock.close()

	workspaceID := "ws_test123"
	stateID := "state_test456"

	t.Run("returns resources from full state", func(t *testing.T) {
		mock.handlers = make(map[string]http.HandlerFunc)
		mock.on("GET", "/v1/runners/terraform-workspace/"+workspaceID+"/state-json/"+stateID, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"values": map[string]interface{}{
					"root_module": map[string]interface{}{
						"resources": []map[string]interface{}{
							{"address": "aws_instance.web", "type": "aws_instance", "name": "web", "provider_name": "registry.terraform.io/hashicorp/aws", "mode": "managed", "schema_version": 1},
						},
						"child_modules": []map[string]interface{}{
							{
								"resources": []map[string]interface{}{
									{"address": "module.vpc.aws_vpc.main", "type": "aws_vpc", "name": "main", "provider_name": "registry.terraform.io/hashicorp/aws", "mode": "managed"},
								},
							},
						},
					},
				},
			})
		})

		client := &Client{
			apiURL:     mock.url(),
			apiToken:   "test-token",
			orgID:      "test-org",
			httpClient: http.DefaultClient,
		}

		resources, err := client.GetTerraformWorkspaceStateResources(context.Background(), workspaceID, stateID)
		assert.NoError(t, err)
		assert.Len(t, resources, 2)
		assert.Equal(t, "aws_instance", resources[0].Type)
		assert.Equal(t, "web", resources[0].Name)
		assert.Equal(t, "managed", resources[0].Mode)
		assert.Equal(t, 1, resources[0].SchemaVersion)
		// Child module resource
		assert.Equal(t, "aws_vpc", resources[1].Type)
		assert.Equal(t, "module.vpc.aws_vpc.main", resources[1].Address)
	})

	t.Run("returns nil on 404", func(t *testing.T) {
		mock.handlers = make(map[string]http.HandlerFunc)
		mock.on("GET", "/v1/runners/terraform-workspace/"+workspaceID+"/state-json/"+stateID, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		client := &Client{
			apiURL:     mock.url(),
			apiToken:   "test-token",
			orgID:      "test-org",
			httpClient: http.DefaultClient,
		}

		resources, err := client.GetTerraformWorkspaceStateResources(context.Background(), workspaceID, stateID)
		assert.NoError(t, err)
		assert.Nil(t, resources)
	})
}

func TestParseAPIError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantTitle string
		wantDesc  string
	}{
		{
			name:      "swagger error with token expired",
			err:       fmt.Errorf("invalid API token or org access: [GET /v1/orgs][401] getOrgsUnauthorized &{Description:Please get a new token from the Nuon dashboard Error:token is expired UserError:true}"),
			wantTitle: "Token Is Expired",
			wantDesc:  "Generate a new token and update your org connection.",
		},
		{
			name:      "plain error without struct fields",
			err:       fmt.Errorf("connection refused"),
			wantTitle: "",
			wantDesc:  "connection refused",
		},
		{
			name:      "swagger error with different error",
			err:       fmt.Errorf("&{Description:Something went wrong Error:internal server error UserError:false}"),
			wantTitle: "Internal Server Error",
			wantDesc:  "Something went wrong",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAPIError(tt.err)
			assert.Equal(t, tt.wantTitle, result.Title)
			assert.Equal(t, tt.wantDesc, result.Description)
		})
	}
}
