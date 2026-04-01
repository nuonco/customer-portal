package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon-go/models"
	"go.uber.org/zap"
)

func (h *Handler) AccessPanel(c *gin.Context) {
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil || nuonOrg.APIToken == "" {
		h.RenderTempl(c, http.StatusOK, partials.AccessPanel(partials.AccessPanelProps{}))
		return
	}

	apiClient, err := nuon.NewClientWithURL(
		nuonOrg.APIToken,
		nuonOrg.NuonOrgID,
		h.nuonAPIURLForOrg(nuonOrg),
	)
	if err != nil {
		zap.L().Warn("failed to create nuon client for access panel", zap.Error(err))
		h.RenderTempl(c, http.StatusOK, partials.AccessPanel(partials.AccessPanelProps{}))
		return
	}

	ctx := c.Request.Context()
	appID := install.GetAppID()

	app, err := apiClient.GetApp(ctx, appID)
	if err != nil || app == nil || len(app.AppConfigs) == 0 {
		zap.L().Warn("failed to fetch app for access panel", zap.Error(err))
		h.RenderTempl(c, http.StatusOK, partials.AccessPanel(partials.AccessPanelProps{}))
		return
	}

	fullCfg, err := apiClient.GetAppConfigFull(ctx, appID, app.AppConfigs[0].ID)
	if err != nil {
		zap.L().Warn("failed to fetch app config for access panel", zap.Error(err))
		h.RenderTempl(c, http.StatusOK, partials.AccessPanel(partials.AccessPanelProps{}))
		return
	}

	roles := extractRoles(ctx, fullCfg)

	h.RenderTempl(c, http.StatusOK, partials.AccessPanel(partials.AccessPanelProps{
		Roles: roles,
	}))
}

func extractRoles(_ context.Context, cfg *nuonmodels.AppAppConfig) []partials.AccessRole {
	if cfg == nil || cfg.Permissions == nil {
		return nil
	}

	var roles []partials.AccessRole
	permCfg := cfg.Permissions
	seen := map[string]bool{}

	addRole := func(role *nuonmodels.AppAppAWSIAMRoleConfig, active bool) {
		if role == nil || role.Name == "" {
			return
		}
		if seen[role.Name] {
			return
		}
		seen[role.Name] = true

		name := role.DisplayName
		if name == "" {
			name = role.Name
		}

		permBoundary := role.PermissionsBoundary
		if permBoundary != "" {
			if decoded, err := base64.StdEncoding.DecodeString(permBoundary); err == nil {
				var prettyJSON bytes.Buffer
				if json.Indent(&prettyJSON, decoded, "", "  ") == nil {
					permBoundary = prettyJSON.String()
				}
			}
		}

		r := partials.AccessRole{
			Name:                name,
			Description:         role.Description,
			Active:              active,
			PermissionsBoundary: permBoundary,
		}

		for _, p := range role.Policies {
			if p == nil {
				continue
			}
			policyType := "Vendor defined"
			if p.ManagedPolicyName != "" {
				policyType = "AWS managed"
			}
			pName := p.Name
			if pName == "" {
				pName = p.ManagedPolicyName
			}
			contents := p.Contents
			if contents != "" {
				if decoded, err := base64.StdEncoding.DecodeString(contents); err == nil {
					var prettyJSON bytes.Buffer
					if json.Indent(&prettyJSON, decoded, "", "  ") == nil {
						contents = prettyJSON.String()
					} else {
						contents = string(decoded)
					}
				}
			}
			r.Policies = append(r.Policies, partials.AccessRolePolicy{
				Name:     pName,
				Type:     policyType,
				Contents: contents,
			})
		}

		roles = append(roles, r)
	}

	if permCfg.ProvisionAwsIamRole.Name != "" {
		prov := permCfg.ProvisionAwsIamRole.AppAppAWSIAMRoleConfig
		addRole(&prov, true)
	}
	addRole(permCfg.DeprovisionAwsIamRole, true)
	addRole(permCfg.MaintenanceAwsIamRole, true)
	addRole(permCfg.BreakGlassAwsIamRole, false)
	for _, r := range permCfg.AwsIamRoles {
		addRole(r, true)
	}

	return roles
}
