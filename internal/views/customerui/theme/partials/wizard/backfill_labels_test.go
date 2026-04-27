package wizard

import (
	"testing"

	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func TestBackfillStepGroupLabels(t *testing.T) {
	groups := []nuon.WorkflowStepGroup{
		{Name: "provision-install-stack"},
		{Name: "provision-sandbox"},
		{Name: "api"},
		{Name: "deploy-worker"},
		{Name: "unknown"},
		{Name: "api", Labels: map[string]string{"domain": "preset", "component_name": "preset"}},
		// Step-name fallback: empty group name, identified by step names.
		{Steps: []*nuon.WorkflowStep{{Name: "generate install stack"}, {Name: "await install stack"}}},
		{Steps: []*nuon.WorkflowStep{{Name: "provision sandbox plan"}, {Name: "provision sandbox apply plan"}}},
		{Steps: []*nuon.WorkflowStep{{Name: "sync and plan api"}, {Name: "apply api"}}},
	}

	BackfillStepGroupLabels(groups, []string{"api", "worker"})

	cases := []struct {
		idx        int
		wantLabels map[string]string
	}{
		{0, map[string]string{"name": "provision-install-stack"}},
		{1, map[string]string{"name": "provision-sandbox"}},
		{2, map[string]string{"domain": "component", "component_name": "api"}},
		{3, map[string]string{"domain": "component", "component_name": "worker"}},
		{4, map[string]string{}},
		{5, map[string]string{"domain": "preset", "component_name": "preset"}},
		{6, map[string]string{"name": "provision-install-stack"}},
		{7, map[string]string{"name": "provision-sandbox"}},
		{8, map[string]string{"domain": "component", "component_name": "api"}},
	}

	for _, tc := range cases {
		got := groups[tc.idx].Labels
		for k, v := range tc.wantLabels {
			if got[k] != v {
				t.Errorf("group %d: label %q = %q, want %q", tc.idx, k, got[k], v)
			}
		}
		if tc.idx == 4 && len(got) != 0 {
			t.Errorf("group %d: expected no labels set, got %v", tc.idx, got)
		}
	}
}
