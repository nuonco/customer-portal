package workflows

import (
	"fmt"
	"strings"
	"time"

	"github.com/nuonco/customer-portal/internal/views/customerui/utils"
	"github.com/nuonco/customer-portal/pkg/nuon"
)

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
	DisablePolling           bool              // When true, disables HTMX polling (e.g. secondary panel views)
}

// PolicyViolation represents a single policy evaluation violation.
type PolicyViolation struct {
	PolicyID string
	Message  string
	Severity string // "deny" or "warn"
}

// PlanSummary holds a parsed summary of a terraform plan for display.
type PlanSummary struct {
	CreateCount  int
	UpdateCount  int
	DeleteCount  int
	ReplaceCount int
	ReadCount    int
	NoOpCount    int
	Resources    []PlanResourceChange
}

// TotalChanges returns the number of meaningful changes (excludes no-op and read).
func (p PlanSummary) TotalChanges() int {
	return p.CreateCount + p.UpdateCount + p.DeleteCount + p.ReplaceCount
}

// PlanResourceChange represents a single resource in a terraform plan.
type PlanResourceChange struct {
	Address string // e.g. "aws_iam_role.nuon_runner"
	Action  string // "create", "update", "delete", "replace", "no-op", "read"
}

// ParseTerraformPlan extracts a summary from raw terraform plan JSON (as returned by the approval contents API).
func ParseTerraformPlan(raw interface{}) *PlanSummary {
	planMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}

	rawChanges, ok := planMap["resource_changes"].([]interface{})
	if !ok {
		return nil
	}

	summary := &PlanSummary{}
	for _, rc := range rawChanges {
		rcMap, ok := rc.(map[string]interface{})
		if !ok {
			continue
		}

		address, _ := rcMap["address"].(string)
		action := resolveAction(rcMap)

		// Skip data sources with no-op
		if action == "no-op" || action == "read" {
			if action == "read" {
				summary.ReadCount++
			} else {
				summary.NoOpCount++
			}
			continue
		}

		switch action {
		case "create":
			summary.CreateCount++
		case "update":
			summary.UpdateCount++
		case "delete":
			summary.DeleteCount++
		case "replace":
			summary.ReplaceCount++
		}

		summary.Resources = append(summary.Resources, PlanResourceChange{
			Address: address,
			Action:  action,
		})
	}

	return summary
}

// resolveAction maps terraform change.actions array to a single action string.
func resolveAction(rcMap map[string]interface{}) string {
	changeMap, ok := rcMap["change"].(map[string]interface{})
	if !ok {
		return "no-op"
	}
	actions, ok := changeMap["actions"].([]interface{})
	if !ok || len(actions) == 0 {
		return "no-op"
	}
	if len(actions) == 1 {
		a, _ := actions[0].(string)
		return a
	}
	// Two-element actions like ["delete", "create"] or ["create", "delete"] = replace
	return "replace"
}

// HelmPlanSummary holds a parsed summary of a helm diff for display.
type HelmPlanSummary struct {
	AddCount     int
	ChangeCount  int
	DestroyCount int
	Changes      []HelmPlanChange
}

// TotalChanges returns the number of meaningful changes.
func (h HelmPlanSummary) TotalChanges() int {
	return h.AddCount + h.ChangeCount + h.DestroyCount
}

// HelmPlanChange represents a single resource change in a helm or kubernetes plan.
type HelmPlanChange struct {
	Kind         string // e.g. "Deployment", "Service"
	Name         string
	Namespace    string
	ResourceType string // e.g. "apps/v1"
	Action       string // "added", "changed", "destroyed"
	Before       string // rendered before content (for diff display)
	After        string // rendered after content (for diff display)
}

// ParseHelmPlan extracts a summary from raw helm approval contents JSON.
// The format has a "plan" text field and "helm_content_diff" array.
func ParseHelmPlan(raw interface{}) *HelmPlanSummary {
	planMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}

	summary := &HelmPlanSummary{}

	// Parse the plan text for summary counts
	if planText, ok := planMap["plan"].(string); ok {
		parsePlanSummaryLine(planText, summary)
	}

	// Parse helm_content_diff entries
	diffs, ok := planMap["helm_content_diff"].([]interface{})
	if !ok {
		return summary
	}

	// Also parse plan text for per-resource changes
	planText, _ := planMap["plan"].(string)
	planChanges := parseHelmPlanLines(planText)

	for _, d := range diffs {
		dm, ok := d.(map[string]interface{})
		if !ok {
			continue
		}

		kind, _ := dm["kind"].(string)
		name, _ := dm["name"].(string)
		namespace, _ := dm["namespace"].(string)
		api, _ := dm["api"].(string)

		// Find matching action from plan text
		action := findHelmAction(planChanges, kind, name, namespace)

		before, after := buildHelmBeforeAfter(dm)

		summary.Changes = append(summary.Changes, HelmPlanChange{
			Kind:         kind,
			Name:         name,
			Namespace:    namespace,
			ResourceType: api,
			Action:       action,
			Before:       before,
			After:        after,
		})
	}

	return summary
}

// ParseKubernetesPlan extracts a summary from raw kubernetes approval contents JSON.
// The format has a "k8s_content_diff" array with op/type fields.
func ParseKubernetesPlan(raw interface{}) *HelmPlanSummary {
	planMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}

	diffs, ok := planMap["k8s_content_diff"].([]interface{})
	if !ok {
		return nil
	}

	summary := &HelmPlanSummary{}

	for _, d := range diffs {
		dm, ok := d.(map[string]interface{})
		if !ok {
			continue
		}

		// Skip items with errors
		if errStr, _ := dm["error"].(string); errStr != "" {
			continue
		}

		kind, _ := dm["kind"].(string)
		name, _ := dm["name"].(string)
		namespace, _ := dm["namespace"].(string)
		api, _ := dm["api"].(string)
		op, _ := dm["op"].(string)
		typeNum, _ := dm["type"].(float64) // JSON numbers are float64

		action := resolveK8sAction(op, int(typeNum))
		switch action {
		case "added":
			summary.AddCount++
		case "changed":
			summary.ChangeCount++
		case "destroyed":
			summary.DestroyCount++
		}

		before, after := buildK8sBeforeAfter(dm)

		summary.Changes = append(summary.Changes, HelmPlanChange{
			Kind:         kind,
			Name:         name,
			Namespace:    namespace,
			ResourceType: api,
			Action:       action,
			Before:       before,
			After:        after,
		})
	}

	return summary
}

// resolveK8sAction determines the action from the op and type fields.
func resolveK8sAction(op string, typeNum int) string {
	if op == "delete" {
		return "destroyed"
	}
	if op == "apply" {
		switch typeNum {
		case 1:
			return "destroyed"
		case 2:
			return "added"
		case 3:
			return "changed"
		}
	}
	return "changed"
}

// parsePlanSummaryLine extracts add/change/destroy counts from a helm plan text.
func parsePlanSummaryLine(planText string, summary *HelmPlanSummary) {
	for _, line := range strings.Split(planText, "\n") {
		// Match "Plan: N to add, N to change, N to destroy"
		if !strings.Contains(line, "Plan:") {
			continue
		}
		parts := strings.Fields(line)
		for i, p := range parts {
			if i+2 < len(parts) && parts[i+1] == "to" {
				n := 0
				fmt.Sscanf(p, "%d", &n)
				switch parts[i+2] {
				case "add,", "add":
					summary.AddCount = n
				case "change,", "change":
					summary.ChangeCount = n
				case "destroy,", "destroy":
					summary.DestroyCount = n
				}
			}
		}
	}
}

type helmPlanLineChange struct {
	namespace string
	name      string
	kind      string
	action    string
}

// parseHelmPlanLines parses the plan text for per-resource changes.
// Format: "namespace, name, Kind (api) to be action"
func parseHelmPlanLines(planText string) []helmPlanLineChange {
	var changes []helmPlanLineChange
	for _, line := range strings.Split(planText, "\n") {
		if !strings.Contains(line, "to be") {
			continue
		}
		// Strip ANSI escape codes
		clean := stripANSI(line)
		// Parse: "namespace, name, Kind (api) to be action"
		parts := strings.SplitN(clean, ",", 3)
		if len(parts) < 3 {
			continue
		}
		ns := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		rest := strings.TrimSpace(parts[2])
		// Extract kind before "(" and action after "to be"
		parenIdx := strings.Index(rest, "(")
		toBeIdx := strings.Index(rest, "to be")
		if parenIdx < 0 || toBeIdx < 0 {
			continue
		}
		kind := strings.TrimSpace(rest[:parenIdx])
		action := strings.TrimSpace(rest[toBeIdx+5:])
		action = normalizeHelmAction(action)
		changes = append(changes, helmPlanLineChange{namespace: ns, name: name, kind: kind, action: action})
	}
	return changes
}

func stripANSI(s string) string {
	result := strings.Builder{}
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until we find a letter
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++ // skip the letter
			}
			i = j
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

func normalizeHelmAction(action string) string {
	switch strings.ToLower(action) {
	case "added", "created":
		return "added"
	case "changed", "modified":
		return "changed"
	case "destroyed", "removed", "deleted":
		return "destroyed"
	}
	return action
}

func findHelmAction(changes []helmPlanLineChange, kind, name, namespace string) string {
	for _, c := range changes {
		if c.kind == kind && c.name == name && c.namespace == namespace {
			return c.action
		}
	}
	return "changed" // default
}

// buildHelmBeforeAfter extracts before/after content from a helm diff entry.
func buildHelmBeforeAfter(dm map[string]interface{}) (string, string) {
	// Check for direct before/after (old format)
	if before, ok := dm["before"].(string); ok {
		after, _ := dm["after"].(string)
		return before, after
	}

	// Check for entries array (new format)
	entries, ok := dm["entries"].([]interface{})
	if !ok {
		return "", ""
	}

	return buildBeforeAfterFromEntries(entries)
}

// buildK8sBeforeAfter extracts before/after content from a k8s diff entry.
func buildK8sBeforeAfter(dm map[string]interface{}) (string, string) {
	entries, ok := dm["entries"].([]interface{})
	if !ok {
		return "", ""
	}
	return buildBeforeAfterFromEntries(entries)
}

// buildBeforeAfterFromEntries builds before/after strings from diff entries.
// Entry types: 0=unchanged, 1=removal (before), 2=addition (after).
func buildBeforeAfterFromEntries(entries []interface{}) (string, string) {
	var beforeLines, afterLines []string

	for _, e := range entries {
		em, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		typeNum, _ := em["type"].(float64)
		payload, _ := em["payload"].(string)
		path, _ := em["path"].(string)

		switch int(typeNum) {
		case 0:
			// Unchanged — include in both
			if payload != "" {
				beforeLines = append(beforeLines, payload)
				afterLines = append(afterLines, payload)
			}
		case 1:
			// Removal — before only
			if path != "" {
				beforeLines = append(beforeLines, path+": "+payload)
			} else if payload != "" {
				beforeLines = append(beforeLines, payload)
			}
		case 2:
			// Addition — after only
			if path != "" {
				afterLines = append(afterLines, path+": "+payload)
			} else if payload != "" {
				afterLines = append(afterLines, payload)
			}
		}
	}

	return strings.Join(beforeLines, "\n"), strings.Join(afterLines, "\n")
}

// ApprovalStepDataPanel for workflow approval steps
type ApprovalStepDataPanel struct {
	StepID     string
	ApprovalID string
}

// StackSetupData carries platform-specific data for the "await install stack" step.
type StackSetupData struct {
	Platform           string // "aws", "gcp", "azure", etc.
	CloudFormationLink string // AWS only — quick launch URL
	TemplateURL        string // AWS only — CloudFormation template URL
	StackName          string // AWS only — CloudFormation stack name
	Region             string // AWS only — deployment region
	TfvarsContent      string // GCP only
	NuonInstallID      string // GCP + Azure (for backend snippet / resource naming)
	AzureTemplateURL   string // Azure only — ARM template URL
	AzureLocation      string // Azure only — deployment location (e.g. "eastus")
}

// IsS3Template returns true if the template URL is hosted on S3.
func (s StackSetupData) IsS3Template() bool {
	return strings.Contains(s.TemplateURL, "s3.amazonaws.com") || strings.Contains(s.TemplateURL, ".s3.")
}

// CreateStackCmd returns the AWS CLI command to create the CloudFormation stack.
func (s StackSetupData) CreateStackCmd() string {
	if s.IsS3Template() {
		return "aws cloudformation create-stack \\\n  --stack-name " + s.StackName + " \\\n  --template-url " + s.TemplateURL + " \\\n  --capabilities CAPABILITY_NAMED_IAM \\\n  --region " + s.Region
	}
	return "curl -sLo template.json \"" + s.TemplateURL + "\" \\\n  && aws cloudformation create-stack \\\n  --stack-name " + s.StackName + " \\\n  --template-body file://template.json \\\n  --capabilities CAPABILITY_NAMED_IAM \\\n  --region " + s.Region
}

// UpdateStackCmd returns the AWS CLI command to update the CloudFormation stack.
func (s StackSetupData) UpdateStackCmd() string {
	if s.IsS3Template() {
		return "aws cloudformation update-stack \\\n  --stack-name " + s.StackName + " \\\n  --template-url " + s.TemplateURL + " \\\n  --capabilities CAPABILITY_NAMED_IAM \\\n  --region " + s.Region
	}
	return "curl -sLo template.json \"" + s.TemplateURL + "\" \\\n  && aws cloudformation update-stack \\\n  --stack-name " + s.StackName + " \\\n  --template-body file://template.json \\\n  --capabilities CAPABILITY_NAMED_IAM \\\n  --region " + s.Region
}

// ConsoleURL returns the AWS CloudFormation console URL filtered by stack name.
func (s StackSetupData) ConsoleURL() string {
	return "https://console.aws.amazon.com/cloudformation/home?region=" + s.Region + "#/stacks/events?filteringText=" + s.StackName + "&filteringStatus=active&viewNested=true"
}

// WorkflowData holds processed workflow information for display in WorkflowCard.
type WorkflowData struct {
	ID                       string
	Name                     string
	Status                   string
	StatusClass              string
	CreatedAt                time.Time
	FinishedAt               time.Time
	CanApprove               bool
	CanApproveAll            bool
	CanCancel                bool
	ApprovalStep             *ApprovalStepData
	ApproveDisabledReason    string
	ApproveAllDisabledReason string
	CancelDisabledReason     string
}

// ApprovalStepData holds approval step info for workflow actions
type ApprovalStepData struct {
	StepID     string
	ApprovalID string
}

// IsTerminalWorkflowStatus returns true for terminal workflow statuses.
func IsTerminalWorkflowStatus(status string) bool {
	return status == "completed" || status == "success" || status == "error" || status == "cancelled"
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

// StepGroup represents a logical group of workflow steps sharing the same GroupIdx.
type StepGroup struct {
	GroupIdx int64
	Type     string // domain label: "install-stack", "sandbox", "component", "action", "system"
	Name     string
	Status   string // not_started, in-progress, completed, error, approval-awaiting
	Steps    []*nuon.WorkflowStep
}

// StepGroupsFromAPI converts API step groups into local StepGroup values,
// using API-provided labels for name, type, and status.
func StepGroupsFromAPI(apiGroups []nuon.WorkflowStepGroup) []StepGroup {
	var groups []StepGroup
	for _, ag := range apiGroups {
		// Filter hidden groups
		if ag.Labels["type"] == "hidden" {
			continue
		}
		status := ""
		if ag.Status.Status != "" {
			status = string(ag.Status.Status)
		}
		sg := StepGroup{
			GroupIdx: int64(ag.GroupIdx),
			Name:     ag.Labels["display_name"],
			Type:     ag.Labels["domain"],
			Status:   status,
			Steps:    ag.Steps,
		}
		groups = append(groups, sg)
	}
	return groups
}

// LatestSteps returns the last step for each execution type in the group.
// When steps are retried, the retry is appended after the original, so the
// last step of each type is always the current one.
func (g StepGroup) LatestSteps() []*nuon.WorkflowStep {
	last := make(map[string]*nuon.WorkflowStep)
	var order []string
	for _, s := range g.Steps {
		key := s.ExecutionType
		if _, seen := last[key]; !seen {
			order = append(order, key)
		}
		last[key] = s
	}
	result := make([]*nuon.WorkflowStep, 0, len(order))
	for _, key := range order {
		result = append(result, last[key])
	}
	return result
}

// ActiveStep returns the first non-completed step from the latest steps, or nil.
func (g StepGroup) ActiveStep() *nuon.WorkflowStep {
	for _, s := range g.LatestSteps() {
		if s.Status == nil {
			return s
		}
		status := string(s.Status.Status)
		if status != "completed" && status != "success" && status != "approved" {
			return s
		}
	}
	return nil
}

// CanApprove returns true if the active step is awaiting approval.
func (g StepGroup) CanApprove() bool {
	s := g.ActiveStep()
	return s != nil && s.Status != nil && string(s.Status.Status) == "approval-awaiting" && s.Approval != nil
}

// CanRetry returns true if the group has a retryable error step.
func (g StepGroup) CanRetry() bool {
	return g.RetryableStep() != nil
}

// RetryableStep returns a retryable failed step from the latest steps, or nil.
func (g StepGroup) RetryableStep() *nuon.WorkflowStep {
	for _, s := range g.LatestSteps() {
		if s.Retryable && s.Finished && s.Status != nil && string(s.Status.Status) != "completed" && string(s.Status.Status) != "success" {
			return s
		}
	}
	return nil
}

// HasApprovalStep returns true if any latest step has execution_type "approval".
func (g StepGroup) HasApprovalStep() bool {
	for _, s := range g.LatestSteps() {
		if s.ExecutionType == "approval" {
			return true
		}
	}
	return false
}

// HasRetryableStep returns true if any latest step is marked retryable.
func (g StepGroup) HasRetryableStep() bool {
	for _, s := range g.LatestSteps() {
		if s.Retryable {
			return true
		}
	}
	return false
}

// RetryableStepID returns the ID of the retryable error step, or empty string.
func (g StepGroup) RetryableStepID() string {
	s := g.RetryableStep()
	if s != nil {
		return s.ID
	}
	return ""
}

// WorkflowOverviewProps holds all data needed to render the WorkflowOverview component.
type WorkflowOverviewProps struct {
	StepGroups       []StepGroup
	SelectedGroup    int
	InstallID        string
	BasePath         string
	WorkflowID       string
	WorkflowFinished bool
	ShowApproveAll   bool
	Platform         string // "aws", "gcp", "azure"
	StackSetup       StackSetupData
	Expanded         bool // Panel is expanded
}

// url returns a URLBuilder pre-filled with the workflow page path and current expanded state.
func (p WorkflowOverviewProps) url() *utils.URLBuilder {
	return utils.NewURL(p.BasePath, "installs", p.InstallID, "overview", "workflows", p.WorkflowID).
		SetBool("expanded", p.Expanded)
}

// GroupPageURL returns the full page URL for a step group selection (for href and hx-push-url).
func (p WorkflowOverviewProps) GroupPageURL(groupIdx int) string {
	return p.url().SetInt("group", groupIdx).Build()
}

// GroupPanelURL returns the partial=panel URL for a step group selection (swaps entire panel).
func (p WorkflowOverviewProps) GroupPanelURL(groupIdx int) string {
	return p.url().SetInt("group", groupIdx).Set("partial", "panel").Build()
}
