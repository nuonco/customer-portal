package workflows

import (
	"fmt"
	"time"
)

// ProvisionPhase represents a logical grouping of workflow steps
type ProvisionPhase struct {
	Name           string
	Status         string // "not_started", "in_progress", "completed", "failed"
	Steps          []PhaseStep
	ActiveStepName string // name of in-progress step within this phase
	StepProgress   string // e.g. "Step 2 of 3"
}

// PhaseStep represents a single step within a provision phase
type PhaseStep struct {
	Name      string
	Status    string
	ID        string // step ID for retry endpoint
	Retryable bool
}

// WorkflowDataPanel represents workflow data for the panel (avoids import cycle)
type WorkflowDataPanel struct {
	ID                       string
	Name                     string
	Status                   string
	StatusClass              string
	CreatedAt                time.Time
	FinishedAt               time.Time
	CanApprove               bool
	CanApproveAll            bool
	CanCancel                bool
	HasApprovalSteps         bool
	ApproveDisabledReason    string
	ApproveAllDisabledReason string
	CancelDisabledReason     string
	ApprovalStep             *ApprovalStepDataPanel
	CurrentStepName          string            // Name of the currently executing step
	CurrentStepStatus        string            // Status of the current step (in-progress, approval-awaiting)
	CurrentStepType          string            // Type of current step: "pending", "in-progress", "approval-awaiting", or "initializing"
	CurrentStepNumber        int               // 1-indexed position of current step (1, 2, 3...)
	TotalSteps               int               // Total number of steps in workflow
	FailedStepID             string            // ID of the failed step (for retry)
	FailedStepName           string            // Name of the failed step (for display)
	FailedStepRetryable      bool              // Whether the failed step can be retried
	CurrentStepRole          string            // IAM role used by the current step target
	IsReprovision            bool              // Whether this is a reprovision workflow
	DenyViolations           []PolicyViolation // Policy deny violations for current step
	WarnViolations           []PolicyViolation // Policy warn violations for current step
	HasPolicyData            bool              // Whether the current step has any policy evaluation data
}

// PolicyViolation represents a single policy evaluation violation.
type PolicyViolation struct {
	PolicyID string
	Message  string
	Severity string // "deny" or "warn"
}

// ApprovalStepDataPanel for workflow approval steps
type ApprovalStepDataPanel struct {
	StepID     string
	ApprovalID string
}

// StackSetupData carries platform-specific data for the "await install stack" step.
type StackSetupData struct {
	Platform           string // "aws", "gcp", "azure", etc.
	CloudFormationLink string // AWS only
	TfvarsContent      string // GCP only
	NuonInstallID      string // GCP + Azure (for backend snippet / resource naming)
	AzureTemplateURL   string // Azure only — ARM template URL
	AzureLocation      string // Azure only — deployment location (e.g. "eastus")
}

// FormatConfigDate formats a date string for display
func FormatConfigDate(dateStr string) string {
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05Z", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t.Format("Jan 2, 2006")
		}
	}
	return dateStr
}

// GcpBackendSnippet returns the GCS backend config snippet
func GcpBackendSnippet(installID string) string {
	return fmt.Sprintf(`terraform {
  backend "gcs" {
    bucket = "<your-state-bucket>"
    prefix = "nuon/%s"
  }
}`, installID)
}

// GcpApplyCmd returns the terraform apply command
func GcpApplyCmd() string {
	return `terraform init && terraform apply -var-file=install.tfvars`
}
