package handlers

import (
	"context"
	"testing"

	nuonmodels "github.com/nuonco/nuon-go/models"
)

func TestExtractRoles(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		roles := extractRoles(context.Background(), nil)
		if roles != nil {
			t.Errorf("expected nil, got %v", roles)
		}
	})

	t.Run("nil permissions returns nil", func(t *testing.T) {
		cfg := &nuonmodels.AppAppConfig{}
		roles := extractRoles(context.Background(), cfg)
		if roles != nil {
			t.Errorf("expected nil, got %v", roles)
		}
	})

	t.Run("extracts standard roles with correct active status", func(t *testing.T) {
		cfg := &nuonmodels.AppAppConfig{
			Permissions: &nuonmodels.AppAppPermissionsConfig{
				ProvisionAwsIamRole: struct {
					nuonmodels.AppAppAWSIAMRoleConfig
				}{
					AppAppAWSIAMRoleConfig: nuonmodels.AppAppAWSIAMRoleConfig{
						Name:        "provision-role",
						DisplayName: "Provision",
						Description: "Provisions infrastructure",
					},
				},
				DeprovisionAwsIamRole: &nuonmodels.AppAppAWSIAMRoleConfig{
					Name:        "deprovision-role",
					DisplayName: "Deprovision",
					Description: "Deprovisions infrastructure",
				},
				MaintenanceAwsIamRole: &nuonmodels.AppAppAWSIAMRoleConfig{
					Name:        "maintenance-role",
					DisplayName: "Maintenance",
					Description: "Maintenance operations",
				},
				BreakGlassAwsIamRole: &nuonmodels.AppAppAWSIAMRoleConfig{
					Name:        "break-glass-role",
					DisplayName: "BreakGlass",
					Description: "Emergency access",
				},
			},
		}

		roles := extractRoles(context.Background(), cfg)

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
		cfg := &nuonmodels.AppAppConfig{
			Permissions: &nuonmodels.AppAppPermissionsConfig{
				ProvisionAwsIamRole: struct {
					nuonmodels.AppAppAWSIAMRoleConfig
				}{
					AppAppAWSIAMRoleConfig: nuonmodels.AppAppAWSIAMRoleConfig{
						Name: "my-role",
					},
				},
			},
		}

		roles := extractRoles(context.Background(), cfg)
		if len(roles) != 1 {
			t.Fatalf("expected 1 role, got %d", len(roles))
		}
		if roles[0].Name != "my-role" {
			t.Errorf("expected name my-role, got %q", roles[0].Name)
		}
	})

	t.Run("deduplicates roles by name", func(t *testing.T) {
		cfg := &nuonmodels.AppAppConfig{
			Permissions: &nuonmodels.AppAppPermissionsConfig{
				ProvisionAwsIamRole: struct {
					nuonmodels.AppAppAWSIAMRoleConfig
				}{
					AppAppAWSIAMRoleConfig: nuonmodels.AppAppAWSIAMRoleConfig{
						Name:        "shared-role",
						DisplayName: "Shared",
					},
				},
				AwsIamRoles: []*nuonmodels.AppAppAWSIAMRoleConfig{
					{Name: "shared-role", DisplayName: "Shared Duplicate"},
					{Name: "custom-role", DisplayName: "Custom"},
				},
			},
		}

		roles := extractRoles(context.Background(), cfg)
		if len(roles) != 2 {
			t.Fatalf("expected 2 roles (deduplicated), got %d", len(roles))
		}
	})

	t.Run("custom roles are active", func(t *testing.T) {
		cfg := &nuonmodels.AppAppConfig{
			Permissions: &nuonmodels.AppAppPermissionsConfig{
				AwsIamRoles: []*nuonmodels.AppAppAWSIAMRoleConfig{
					{Name: "custom-1", DisplayName: "ReadOnly", Description: "Read access"},
					{Name: "custom-2", DisplayName: "Deployer", Description: "Deploy access"},
				},
			},
		}

		roles := extractRoles(context.Background(), cfg)
		if len(roles) != 2 {
			t.Fatalf("expected 2 roles, got %d", len(roles))
		}
		for _, r := range roles {
			if !r.Active {
				t.Errorf("expected custom role %q to be active", r.Name)
			}
		}
	})

}
