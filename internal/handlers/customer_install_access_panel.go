package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
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

	permsCfg, err := apiClient.GetInstallAppPermissionsConfig(ctx, install.NuonInstallID)
	if err != nil {
		zap.L().Warn("failed to fetch app permissions config for access panel", zap.Error(err))
		h.RenderTempl(c, http.StatusOK, partials.AccessPanel(partials.AccessPanelProps{}))
		return
	}

	roles := extractRoles(permsCfg)

	h.RenderTempl(c, http.StatusOK, partials.AccessPanel(partials.AccessPanelProps{
		Roles:     roles,
		BasePath:  h.basePath,
		InstallID: install.ID,
	}))
}

func (h *Handler) RoleDetailPanel(c *gin.Context) {
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	roleIndexStr := c.Param("role_index")
	var roleIndex int
	if _, err := fmt.Sscanf(roleIndexStr, "%d", &roleIndex); err != nil {
		c.String(http.StatusBadRequest, "Invalid role index")
		return
	}

	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil || nuonOrg.APIToken == "" {
		c.String(http.StatusNotFound, "No API connection")
		return
	}

	apiClient, err := nuon.NewClientWithURL(
		nuonOrg.APIToken,
		nuonOrg.NuonOrgID,
		h.nuonAPIURLForOrg(nuonOrg),
	)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create API client")
		return
	}

	ctx := c.Request.Context()

	permsCfg, err := apiClient.GetInstallAppPermissionsConfig(ctx, install.NuonInstallID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch app permissions config")
		return
	}

	roles := extractRoles(permsCfg)
	if roleIndex < 0 || roleIndex >= len(roles) {
		c.String(http.StatusNotFound, "Role not found")
		return
	}

	h.RenderTempl(c, http.StatusOK, partials.RoleDetailPanel(roles[roleIndex]))
}

func extractRoles(cfg *nuon.InstallAppPermissionsConfig) []partials.AccessRole {
	if cfg == nil {
		return nil
	}

	var roles []partials.AccessRole
	seen := map[string]bool{}

	addRole := func(role *nuon.InstallIAMRole, active bool) {
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

	addRole(cfg.ProvisionRole, cfg.ProvisionRole != nil && cfg.ProvisionRole.ARN != "")
	addRole(cfg.DeprovisionRole, cfg.DeprovisionRole != nil && cfg.DeprovisionRole.ARN != "")
	addRole(cfg.MaintenanceRole, cfg.MaintenanceRole != nil && cfg.MaintenanceRole.ARN != "")
	for i := range cfg.BreakGlassRoles {
		addRole(&cfg.BreakGlassRoles[i], cfg.BreakGlassRoles[i].ARN != "")
	}
	for i := range cfg.CustomRoles {
		addRole(&cfg.CustomRoles[i], cfg.CustomRoles[i].ARN != "")
	}

	return roles
}
