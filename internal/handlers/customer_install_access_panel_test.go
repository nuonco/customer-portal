package handlers

import (
	"testing"

	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func TestExtractRoles(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		roles := extractRoles(nil)
		if roles != nil {
			t.Errorf("expected nil, got %v", roles)
		}
	})

	t.Run("extracts standard roles with correct active status", func(t *testing.T) {
		cfg := &nuon.InstallAppPermissionsConfig{
			ProvisionRole: &nuon.InstallIAMRole{
				Name:        "provision-role",
				DisplayName: "Provision",
				Description: "Provisions infrastructure",
			},
			DeprovisionRole: &nuon.InstallIAMRole{
				Name:        "deprovision-role",
				DisplayName: "Deprovision",
				Description: "Deprovisions infrastructure",
			},
			MaintenanceRole: &nuon.InstallIAMRole{
				Name:        "maintenance-role",
				DisplayName: "Maintenance",
				Description: "Maintenance operations",
			},
			BreakGlassRoles: []nuon.InstallIAMRole{
				{
					Name:        "break-glass-role",
					DisplayName: "BreakGlass",
					Description: "Emergency access",
				},
			},
		}

		roles := extractRoles(cfg)

		if len(roles) != 4 {
			t.Fatalf("expected 4 roles, got %d", len(roles))
		}

		// Provision, Deprovision, Maintenance should be active
		for i := 0; i < 3; i++ {
			if !roles[i].Active {
				t.Errorf("expected role %q to be active", roles[i].Name)
			}
		}

		// BreakGlass should be inactive
		if roles[3].Active {
			t.Errorf("expected BreakGlass role to be inactive")
		}
		if roles[3].Name != "BreakGlass" {
			t.Errorf("expected name BreakGlass, got %q", roles[3].Name)
		}
	})

	t.Run("uses Name as fallback when DisplayName is empty", func(t *testing.T) {
		cfg := &nuon.InstallAppPermissionsConfig{
			ProvisionRole: &nuon.InstallIAMRole{
				Name: "my-role",
			},
		}

		roles := extractRoles(cfg)
		if len(roles) != 1 {
			t.Fatalf("expected 1 role, got %d", len(roles))
		}
		if roles[0].Name != "my-role" {
			t.Errorf("expected name my-role, got %q", roles[0].Name)
		}
	})

	t.Run("deduplicates roles by name", func(t *testing.T) {
		cfg := &nuon.InstallAppPermissionsConfig{
			ProvisionRole: &nuon.InstallIAMRole{
				Name:        "shared-role",
				DisplayName: "Shared",
			},
			CustomRoles: []nuon.InstallIAMRole{
				{Name: "shared-role", DisplayName: "Shared Duplicate"},
				{Name: "custom-role", DisplayName: "Custom"},
			},
		}

		roles := extractRoles(cfg)
		if len(roles) != 2 {
			t.Fatalf("expected 2 roles (deduplicated), got %d", len(roles))
		}
	})

	t.Run("custom roles are active", func(t *testing.T) {
		cfg := &nuon.InstallAppPermissionsConfig{
			CustomRoles: []nuon.InstallIAMRole{
				{Name: "custom-1", DisplayName: "ReadOnly", Description: "Read access"},
				{Name: "custom-2", DisplayName: "Deployer", Description: "Deploy access"},
			},
		}

		roles := extractRoles(cfg)
		if len(roles) != 2 {
			t.Fatalf("expected 2 roles, got %d", len(roles))
		}
		for _, r := range roles {
			if !r.Active {
				t.Errorf("expected custom role %q to be active", r.Name)
			}
		}
	})

	t.Run("break glass roles are inactive", func(t *testing.T) {
		cfg := &nuon.InstallAppPermissionsConfig{
			BreakGlassRoles: []nuon.InstallIAMRole{
				{Name: "bg-1", DisplayName: "Emergency Access"},
			},
		}

		roles := extractRoles(cfg)
		if len(roles) != 1 {
			t.Fatalf("expected 1 role, got %d", len(roles))
		}
		if roles[0].Active {
			t.Errorf("expected break glass role to be inactive")
		}
	})
}
