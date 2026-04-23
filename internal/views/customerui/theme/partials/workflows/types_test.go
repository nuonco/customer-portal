package workflows

import (
	"testing"

	nuonmodels "github.com/nuonco/nuon-go/models"
)

func TestDeriveGroupType_MixedActionAndComponent(t *testing.T) {
	steps := []*nuonmodels.AppWorkflowStep{
		{Name: "ch_operator_creds", StepTargetType: "install_action_workflow_runs"},
		{Name: "sync_and_plan_ctl_api_role", StepTargetType: "install_deploys"},
	}
	got := deriveGroupType(steps)
	if got != StepGroupComponent {
		t.Errorf("deriveGroupType = %v, want %v", got, StepGroupComponent)
	}
}

func TestDeriveGroupType_MixedActionAndComponentEmptyTargetType(t *testing.T) {
	// Component step has empty StepTargetType (not yet started), but its name
	// indicates a deploy. Should still classify as Component.
	steps := []*nuonmodels.AppWorkflowStep{
		{Name: "ch_operator_creds", StepTargetType: "install_action_workflow_runs"},
		{Name: "sync_and_plan_ctl_api_role", StepTargetType: ""},
	}
	got := deriveGroupType(steps)
	if got != StepGroupComponent {
		t.Errorf("deriveGroupType = %v, want %v", got, StepGroupComponent)
	}
}

func TestDeriveGroupType_MixedActionAndDeployNameEmptyTargetType(t *testing.T) {
	// Component step name contains "deploy" (not "action_run"), empty target type.
	steps := []*nuonmodels.AppWorkflowStep{
		{Name: "ch_operator_creds", StepTargetType: "install_action_workflow_runs"},
		{Name: "deploy_s3_buckets", StepTargetType: ""},
	}
	got := deriveGroupType(steps)
	if got != StepGroupComponent {
		t.Errorf("deriveGroupType = %v, want %v", got, StepGroupComponent)
	}
}

func TestDeriveGroupType_ActionOnly(t *testing.T) {
	steps := []*nuonmodels.AppWorkflowStep{
		{Name: "ch_operator_creds", StepTargetType: "install_action_workflow_runs"},
	}
	got := deriveGroupType(steps)
	if got != StepGroupAction {
		t.Errorf("deriveGroupType = %v, want %v", got, StepGroupAction)
	}
}

func TestDeriveGroupType_ComponentOnly(t *testing.T) {
	steps := []*nuonmodels.AppWorkflowStep{
		{Name: "sync_and_plan_s3_buckets", StepTargetType: "install_deploys"},
	}
	got := deriveGroupType(steps)
	if got != StepGroupComponent {
		t.Errorf("deriveGroupType = %v, want %v", got, StepGroupComponent)
	}
}

func TestDeriveGroupName_MixedActionAndComponent(t *testing.T) {
	steps := []*nuonmodels.AppWorkflowStep{
		{Name: "ch_operator_creds", StepTargetType: "install_action_workflow_runs"},
		{Name: "sync_and_plan_ctl_api_role", StepTargetType: "install_deploys"},
	}
	got := deriveGroupName(steps)
	want := "Deploy Ctl api role"
	if got != want {
		t.Errorf("deriveGroupName = %q, want %q", got, want)
	}
}

func TestDeriveGroupName_MixedActionAndComponentEmptyTargetType(t *testing.T) {
	// Component step has empty StepTargetType but component-like name.
	steps := []*nuonmodels.AppWorkflowStep{
		{Name: "ch_operator_creds", StepTargetType: "install_action_workflow_runs"},
		{Name: "sync_and_plan_ctl_api_role", StepTargetType: ""},
	}
	got := deriveGroupName(steps)
	want := "Deploy Ctl api role"
	if got != want {
		t.Errorf("deriveGroupName = %q, want %q", got, want)
	}
}

func TestDeriveGroupName_MixedActionAndDeployNameEmptyTargetType(t *testing.T) {
	steps := []*nuonmodels.AppWorkflowStep{
		{Name: "ch_operator_creds", StepTargetType: "install_action_workflow_runs"},
		{Name: "deploy_s3_buckets", StepTargetType: ""},
	}
	got := deriveGroupName(steps)
	want := "Deploy s3 buckets"
	if got != want {
		t.Errorf("deriveGroupName = %q, want %q", got, want)
	}
}

func TestDeriveGroupName_ComponentOnly(t *testing.T) {
	steps := []*nuonmodels.AppWorkflowStep{
		{Name: "sync_and_plan_s3_buckets", StepTargetType: "install_deploys"},
	}
	got := deriveGroupName(steps)
	want := "Deploy S3 buckets"
	if got != want {
		t.Errorf("deriveGroupName = %q, want %q", got, want)
	}
}

func TestDeriveGroupName_ActionOnly(t *testing.T) {
	steps := []*nuonmodels.AppWorkflowStep{
		{Name: "ch_operator_creds", StepTargetType: "install_action_workflow_runs"},
	}
	got := deriveGroupName(steps)
	want := "Ch operator creds"
	if got != want {
		t.Errorf("deriveGroupName = %q, want %q", got, want)
	}
}

func TestBuildStepGroups_MixedGroupClassifiedAsComponent(t *testing.T) {
	steps := []*nuonmodels.AppWorkflowStep{
		{GroupIdx: 1, Name: "ch_operator_creds", StepTargetType: "install_action_workflow_runs", ExecutionType: "user"},
		{GroupIdx: 1, Name: "sync_and_plan_ctl_api_role", StepTargetType: "install_deploys", ExecutionType: "user"},
	}
	groups := BuildStepGroups(steps)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Type != StepGroupComponent {
		t.Errorf("group type = %v, want %v", groups[0].Type, StepGroupComponent)
	}
	if groups[0].Name != "Deploy Ctl api role" {
		t.Errorf("group name = %q, want %q", groups[0].Name, "Deploy Ctl api role")
	}
}

func TestBuildStepGroups_MixedGroupEmptyTargetType(t *testing.T) {
	// Real-world case: action step has target type, component step doesn't (not yet started).
	steps := []*nuonmodels.AppWorkflowStep{
		{GroupIdx: 1, Name: "ch_operator_creds", StepTargetType: "install_action_workflow_runs", ExecutionType: "user"},
		{GroupIdx: 1, Name: "sync_and_plan_ctl_api_role", StepTargetType: "", ExecutionType: "user"},
	}
	groups := BuildStepGroups(steps)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Type != StepGroupComponent {
		t.Errorf("group type = %v, want %v", groups[0].Type, StepGroupComponent)
	}
	if groups[0].Name != "Deploy Ctl api role" {
		t.Errorf("group name = %q, want %q", groups[0].Name, "Deploy Ctl api role")
	}
}

func TestParseTerraformPlan(t *testing.T) {
	raw := map[string]interface{}{
		"resource_changes": []interface{}{
			map[string]interface{}{
				"address": "aws_iam_role.nuon_runner",
				"change":  map[string]interface{}{"actions": []interface{}{"create"}},
			},
			map[string]interface{}{
				"address": "aws_s3_bucket.data",
				"change":  map[string]interface{}{"actions": []interface{}{"update"}},
			},
			map[string]interface{}{
				"address": "aws_lambda_function.old",
				"change":  map[string]interface{}{"actions": []interface{}{"delete"}},
			},
			map[string]interface{}{
				"address": "aws_instance.web",
				"change":  map[string]interface{}{"actions": []interface{}{"delete", "create"}},
			},
			map[string]interface{}{
				"address": "data.aws_caller_identity.current",
				"change":  map[string]interface{}{"actions": []interface{}{"no-op"}},
			},
		},
	}

	plan := ParseTerraformPlan(raw)
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.CreateCount != 1 {
		t.Errorf("CreateCount = %d, want 1", plan.CreateCount)
	}
	if plan.UpdateCount != 1 {
		t.Errorf("UpdateCount = %d, want 1", plan.UpdateCount)
	}
	if plan.DeleteCount != 1 {
		t.Errorf("DeleteCount = %d, want 1", plan.DeleteCount)
	}
	if plan.ReplaceCount != 1 {
		t.Errorf("ReplaceCount = %d, want 1", plan.ReplaceCount)
	}
	if plan.NoOpCount != 1 {
		t.Errorf("NoOpCount = %d, want 1", plan.NoOpCount)
	}
	if len(plan.Resources) != 4 {
		t.Errorf("len(Resources) = %d, want 4 (no-op excluded)", len(plan.Resources))
	}
	if plan.TotalChanges() != 4 {
		t.Errorf("TotalChanges = %d, want 4", plan.TotalChanges())
	}
}

func TestParseTerraformPlan_Nil(t *testing.T) {
	if plan := ParseTerraformPlan(nil); plan != nil {
		t.Errorf("expected nil for nil input, got %+v", plan)
	}
}

func TestParseTerraformPlan_Empty(t *testing.T) {
	raw := map[string]interface{}{
		"resource_changes": []interface{}{},
	}
	plan := ParseTerraformPlan(raw)
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.TotalChanges() != 0 {
		t.Errorf("TotalChanges = %d, want 0", plan.TotalChanges())
	}
}

func TestParseHelmPlan(t *testing.T) {
	raw := map[string]interface{}{
		"plan": "default, my-release, Deployment (apps/v1) to be added\ndefault, my-release, Service (v1) to be changed\nPlan: 1 to add, 1 to change, 0 to destroy",
		"helm_content_diff": []interface{}{
			map[string]interface{}{
				"api":       "apps/v1",
				"kind":      "Deployment",
				"name":      "my-release",
				"namespace": "default",
				"before":    "replicas: 1",
				"after":     "replicas: 3",
			},
			map[string]interface{}{
				"api":       "v1",
				"kind":      "Service",
				"name":      "my-release",
				"namespace": "default",
				"entries": []interface{}{
					map[string]interface{}{"type": float64(1), "payload": "port: 80", "path": ""},
					map[string]interface{}{"type": float64(2), "payload": "port: 8080", "path": ""},
				},
			},
		},
	}

	plan := ParseHelmPlan(raw)
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.AddCount != 1 {
		t.Errorf("AddCount = %d, want 1", plan.AddCount)
	}
	if plan.ChangeCount != 1 {
		t.Errorf("ChangeCount = %d, want 1", plan.ChangeCount)
	}
	if plan.DestroyCount != 0 {
		t.Errorf("DestroyCount = %d, want 0", plan.DestroyCount)
	}
	if len(plan.Changes) != 2 {
		t.Fatalf("len(Changes) = %d, want 2", len(plan.Changes))
	}
	if plan.Changes[0].Action != "added" {
		t.Errorf("Changes[0].Action = %q, want 'added'", plan.Changes[0].Action)
	}
	if plan.Changes[0].Before != "replicas: 1" {
		t.Errorf("Changes[0].Before = %q, want 'replicas: 1'", plan.Changes[0].Before)
	}
	if plan.Changes[1].Action != "changed" {
		t.Errorf("Changes[1].Action = %q, want 'changed'", plan.Changes[1].Action)
	}
	if plan.Changes[1].Before != "port: 80" {
		t.Errorf("Changes[1].Before = %q, want 'port: 80'", plan.Changes[1].Before)
	}
	if plan.Changes[1].After != "port: 8080" {
		t.Errorf("Changes[1].After = %q, want 'port: 8080'", plan.Changes[1].After)
	}
	if plan.TotalChanges() != 2 {
		t.Errorf("TotalChanges = %d, want 2", plan.TotalChanges())
	}
}

func TestParseHelmPlan_Nil(t *testing.T) {
	if plan := ParseHelmPlan(nil); plan != nil {
		t.Errorf("expected nil for nil input, got %+v", plan)
	}
}

func TestParseHelmPlan_ANSIStripped(t *testing.T) {
	raw := map[string]interface{}{
		"plan": "\x1b[32mdefault, my-app, Deployment (apps/v1) to be added\x1b[0m\nPlan: 1 to add, 0 to change, 0 to destroy",
		"helm_content_diff": []interface{}{
			map[string]interface{}{
				"kind":      "Deployment",
				"name":      "my-app",
				"namespace": "default",
			},
		},
	}
	plan := ParseHelmPlan(raw)
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.AddCount != 1 {
		t.Errorf("AddCount = %d, want 1", plan.AddCount)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("len(Changes) = %d, want 1", len(plan.Changes))
	}
	if plan.Changes[0].Action != "added" {
		t.Errorf("Changes[0].Action = %q, want 'added'", plan.Changes[0].Action)
	}
}

func TestParseKubernetesPlan(t *testing.T) {
	raw := map[string]interface{}{
		"k8s_content_diff": []interface{}{
			map[string]interface{}{
				"kind":      "Deployment",
				"name":      "web",
				"namespace": "default",
				"api":       "apps/v1",
				"op":        "apply",
				"type":      float64(2), // added
				"entries": []interface{}{
					map[string]interface{}{"type": float64(2), "payload": "replicas: 1", "path": ""},
				},
			},
			map[string]interface{}{
				"kind":      "ConfigMap",
				"name":      "config",
				"namespace": "default",
				"api":       "v1",
				"op":        "apply",
				"type":      float64(3), // changed
			},
			map[string]interface{}{
				"kind":      "Service",
				"name":      "old-svc",
				"namespace": "default",
				"api":       "v1",
				"op":        "delete",
				"type":      float64(1),
			},
		},
	}

	plan := ParseKubernetesPlan(raw)
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.AddCount != 1 {
		t.Errorf("AddCount = %d, want 1", plan.AddCount)
	}
	if plan.ChangeCount != 1 {
		t.Errorf("ChangeCount = %d, want 1", plan.ChangeCount)
	}
	if plan.DestroyCount != 1 {
		t.Errorf("DestroyCount = %d, want 1", plan.DestroyCount)
	}
	if len(plan.Changes) != 3 {
		t.Fatalf("len(Changes) = %d, want 3", len(plan.Changes))
	}
	if plan.Changes[0].Action != "added" {
		t.Errorf("Changes[0].Action = %q, want 'added'", plan.Changes[0].Action)
	}
	if plan.Changes[1].Action != "changed" {
		t.Errorf("Changes[1].Action = %q, want 'changed'", plan.Changes[1].Action)
	}
	if plan.Changes[2].Action != "destroyed" {
		t.Errorf("Changes[2].Action = %q, want 'destroyed'", plan.Changes[2].Action)
	}
}

func TestParseKubernetesPlan_Nil(t *testing.T) {
	if plan := ParseKubernetesPlan(nil); plan != nil {
		t.Errorf("expected nil for nil input, got %+v", plan)
	}
}

func TestParseKubernetesPlan_SkipsErrors(t *testing.T) {
	raw := map[string]interface{}{
		"k8s_content_diff": []interface{}{
			map[string]interface{}{
				"kind":  "Deployment",
				"name":  "web",
				"op":    "apply",
				"type":  float64(2),
				"error": "some error",
			},
			map[string]interface{}{
				"kind": "Service",
				"name": "svc",
				"op":   "apply",
				"type": float64(2),
			},
		},
	}
	plan := ParseKubernetesPlan(raw)
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if len(plan.Changes) != 1 {
		t.Errorf("len(Changes) = %d, want 1 (error entry skipped)", len(plan.Changes))
	}
}
