package wizard

import (
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials/workflows"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

// ComponentWizardData holds type-specific plan/apply data for a single component group.
type ComponentWizardData struct {
	ApprovalType   string                     // "terraform_plan", "helm_approval", "kubernetes_manifest_approval", "pulumi_plan"
	TerraformPlan  *workflows.PlanSummary     // set for terraform_plan and pulumi_plan
	HelmPlan       *workflows.HelmPlanSummary // set for helm_approval and kubernetes_manifest_approval
	PolicyReports  []nuon.PolicyReport
	ApplyResources []nuon.TerraformResource // terraform only — live workspace resources
	ApplyOutputs   map[string]string
	// Component config — type-specific fields from AppComponentConfigConnection
	ImageURL string // external_image
	ImageTag string // external_image
}
