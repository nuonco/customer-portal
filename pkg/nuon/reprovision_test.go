package nuon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestReprovisionInstallAcceptsWorkflowResponse is the regression test for the bug
// that prompted moving off github.com/nuonco/nuon-go.
//
// That SDK asserted the response payload equalled the literal string "ok":
//
//	if resp.Payload != "ok" { return statusErr{resp.Payload} }
//
// ctl-api returns 201 with an app.WorkflowResponse instead, so every successful
// reprovision was reported as an error — the workflow started and ran while the
// portal told the customer it had failed.
func TestReprovisionInstallAcceptsWorkflowResponse(t *testing.T) {
	var gotPath, gotMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"workflow_id": "inwe42nm0jowzw3w94hclz2svo",
			"id":          "inwe42nm0jowzw3w94hclz2svo",
		})
	}))
	defer srv.Close()

	client, err := NewClientWithURL("nuon_pat_fake", "org_fake", srv.URL)
	if err != nil {
		t.Fatalf("unable to build client: %v", err)
	}

	if err := client.ReprovisionInstall(t.Context(), "inst_abc"); err != nil {
		t.Fatalf("a 201 with a workflow body must not be an error, got: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if !strings.Contains(gotPath, "/installs/inst_abc/reprovision") {
		t.Errorf("path = %q, want it to contain /installs/inst_abc/reprovision", gotPath)
	}
}

// DeprovisionInstall had the same response-shape change, and must hit the install
// endpoint rather than the sandbox one — they create different workflows, and
// calling the sandbox variant left the stack and components running while the UI
// claimed all data had been destroyed.
func TestDeprovisionInstallTargetsTheInstallNotTheSandbox(t *testing.T) {
	var gotPath, gotMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"workflow_id": "wf_1", "id": "wf_1"})
	}))
	defer srv.Close()

	client, err := NewClientWithURL("nuon_pat_fake", "org_fake", srv.URL)
	if err != nil {
		t.Fatalf("unable to build client: %v", err)
	}

	if err := client.DeprovisionInstall(t.Context(), "inst_abc"); err != nil {
		t.Fatalf("a 201 with a workflow body must not be an error, got: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/installs/inst_abc/deprovision") {
		t.Errorf("path = %q, want it to end with /installs/inst_abc/deprovision", gotPath)
	}
	if strings.Contains(gotPath, "deprovision-sandbox") {
		t.Errorf("must not call the sandbox variant, got %q", gotPath)
	}
}

// UpdateInstallInputs used to read the workflow ID from a response header; it is
// now a field on the payload.
func TestUpdateInstallInputsReadsWorkflowIDFromPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          "iin_1",
			"workflow_id": "wf_from_payload",
			"values":      map[string]string{"k": "v"},
		})
	}))
	defer srv.Close()

	client, err := NewClientWithURL("nuon_pat_fake", "org_fake", srv.URL)
	if err != nil {
		t.Fatalf("unable to build client: %v", err)
	}

	workflowID, err := client.UpdateInstallInputs(t.Context(), "inst_abc", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if workflowID != "wf_from_payload" {
		t.Errorf("workflowID = %q, want %q", workflowID, "wf_from_payload")
	}
}
