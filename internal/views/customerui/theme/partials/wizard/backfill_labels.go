package wizard

import (
	"strings"

	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

// stackStepNames are the step names that identify the install-stack group.
var stackStepNames = map[string]struct{}{
	"generate install stack":       {},
	"await install stack":          {},
	"update install stack outputs": {},
}

// sandboxStepNames are the step names that identify a sandbox group.
var sandboxStepNames = map[string]struct{}{
	"provision sandbox plan":           {},
	"provision sandbox apply plan":     {},
	"provision sandbox dns if enabled": {},
}

// BackfillStepGroupLabels populates the labels the wizard relies on
// (`name`, `domain`, `component_name`) when the API hasn't set them yet.
// It mutates groups in place and only fills labels that are missing,
// so it becomes a no-op once the API ships the labels itself.
//
// Identification strategy (per group):
//  1. Match by group `Name` field if present.
//  2. Fall back to inspecting step names: well-known stack/sandbox steps
//     identify those groups; any step name containing an app component
//     name identifies a component group.
//
// componentNames is the set of app component names used to identify
// per-component step groups.
func BackfillStepGroupLabels(groups []nuon.WorkflowStepGroup, componentNames []string) {
	nameSet := make(map[string]struct{}, len(componentNames))
	for _, n := range componentNames {
		if n != "" {
			nameSet[n] = struct{}{}
		}
	}

	for i := range groups {
		g := &groups[i]
		if g.Labels == nil {
			g.Labels = make(map[string]string)
		}

		// 1. Match by group Name.
		switch {
		case g.Name == "provision-install-stack":
			setIfMissing(g.Labels, "name", "provision-install-stack")
			continue
		case g.Name == "provision-sandbox":
			setIfMissing(g.Labels, "name", "provision-sandbox")
			continue
		}
		if compName := matchComponentName(g.Name, nameSet); compName != "" {
			setIfMissing(g.Labels, "domain", "component")
			setIfMissing(g.Labels, "component_name", compName)
			continue
		}

		// 2. Fall back to step-name inspection.
		stack, sandbox, comp := classifyByStepNames(g.Steps, nameSet)
		switch {
		case stack:
			setIfMissing(g.Labels, "name", "provision-install-stack")
		case sandbox:
			setIfMissing(g.Labels, "name", "provision-sandbox")
		case comp != "":
			setIfMissing(g.Labels, "domain", "component")
			setIfMissing(g.Labels, "component_name", comp)
		}
	}
}

// classifyByStepNames inspects the step names in a group and returns flags for
// stack/sandbox plus a component name (empty if not a component group).
func classifyByStepNames(steps []*nuon.WorkflowStep, componentNames map[string]struct{}) (stack, sandbox bool, component string) {
	for _, s := range steps {
		if s == nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(s.Name))
		if _, ok := stackStepNames[name]; ok {
			stack = true
			return
		}
		if _, ok := sandboxStepNames[name]; ok {
			sandbox = true
			return
		}
		if c := matchComponentName(name, componentNames); c != "" {
			component = c
			return
		}
	}
	return
}

// matchComponentName returns the component name that corresponds to a name
// (group name or step name), or "" if none match. Match is exact, then a
// substring containment check.
func matchComponentName(name string, names map[string]struct{}) string {
	if name == "" {
		return ""
	}
	if _, ok := names[name]; ok {
		return name
	}
	lower := strings.ToLower(name)
	for n := range names {
		if strings.Contains(lower, strings.ToLower(n)) {
			return n
		}
	}
	return ""
}

func setIfMissing(m map[string]string, key, val string) {
	if _, ok := m[key]; !ok {
		m[key] = val
	}
}
