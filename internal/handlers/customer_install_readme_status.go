package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/markdown"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"go.uber.org/zap"
)

func (h *Handler) InstallReadmeStatus(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	// Load install with org relationships
	if err := h.db.Preload("InstallLink.NuonOrg").Preload("Org").Where("id = ?", install.ID).First(install).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()
	if nuonOrg == nil {
		c.String(http.StatusInternalServerError, "Organization information not found")
		return
	}

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to initialize Nuon client")
		return
	}

	// Check if there's an active provision/reprovision workflow
	ctx := c.Request.Context()
	hasActiveWorkflow := false
	provisionTypes := []string{"provision", "provision_sandbox", "reprovision", "reprovision_sandbox"}

	for _, wfType := range provisionTypes {
		workflows, _, err := nuonClient.GetInstallWorkflowsByType(ctx, install.NuonInstallID, 0, 1, wfType)
		if err != nil || len(workflows) == 0 {
			continue
		}
		wf := workflows[0]
		if wf.Status != nil {
			status := string(wf.Status.Status)
			if status == "in-progress" || status == "approval-awaiting" || status == "pending" {
				hasActiveWorkflow = true
				break
			}
		}
	}

	// If there's an active workflow, return empty (readme should be hidden)
	if hasActiveWorkflow {
		c.String(http.StatusOK, "")
		return
	}

	// Fetch install readme
	readme, err := nuonClient.GetInstallReadme(ctx, install.NuonInstallID)
	if err != nil {
		h.logger.Warn("failed to fetch install readme for polling",
			zap.String("install_id", install.NuonInstallID),
			zap.Error(err),
		)
		c.String(http.StatusOK, "")
		return
	}

	// Convert to partials.ReadmeDataPanel if readme exists
	var readmePanel *partials.ReadmeDataPanel
	if readme != nil {
		warnings := []string{}
		if readme.Warnings != nil {
			warnings = readme.Warnings
		}

		// Convert markdown to HTML
		renderedHTML := readme.Readme
		if readme.Readme != "" {
			html, err := markdown.ToHTML(readme.Readme)
			if err != nil {
				h.logger.Warn("failed to render markdown to HTML",
					zap.String("install_id", install.NuonInstallID),
					zap.Error(err),
				)
				// Fall back to raw markdown if conversion fails
				renderedHTML = readme.Readme
			} else {
				renderedHTML = html
			}
		}

		readmePanel = &partials.ReadmeDataPanel{
			Readme:   renderedHTML,
			Original: readme.Original,
			Warnings: warnings,
		}
	}

	// Render the readme card partial
	h.RenderTempl(c, http.StatusOK, partials.AppReadmeCard(readmePanel, install.ID, h.basePath))
}

// AuditLogsPanel renders the audit logs panel content (no layout wrapper)
