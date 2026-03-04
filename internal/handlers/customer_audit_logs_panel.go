package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) AuditLogsPanel(c *gin.Context) {
	// Get install from middleware (RequireInstallOwnership sets this)
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}

	install := installInterface.(*models.Install)

	// Load install with both InstallLink.NuonOrg and Org (handles published-app installs)
	if err := h.loadInstallWithOrg(install); err != nil {
		c.String(http.StatusInternalServerError, "Failed to load install details")
		return
	}

	nuonOrg := install.GetNuonOrg()

	// Parse time range parameters
	now := time.Now()
	rangeType := c.DefaultQuery("range", "past-hour")
	var startTime, endTime time.Time

	startStr := c.Query("start")
	endStr := c.Query("end")

	if startStr != "" && endStr != "" {
		parsedStart, err1 := time.Parse(time.RFC3339, startStr)
		parsedEnd, err2 := time.Parse(time.RFC3339, endStr)
		if err1 == nil && err2 == nil {
			startTime = parsedStart
			endTime = parsedEnd
			rangeType = "custom"
		} else {
			startTime = now.Add(-time.Hour)
			endTime = now
			rangeType = "past-hour"
		}
	} else {
		switch rangeType {
		case "past-day":
			startTime = now.Add(-24 * time.Hour)
			endTime = now
		case "past-week":
			startTime = now.Add(-7 * 24 * time.Hour)
			endTime = now
		default:
			startTime = now.Add(-time.Hour)
			endTime = now
			rangeType = "past-hour"
		}
	}

	// Fetch audit logs via Nuon client
	var auditEntries []partials.AuditLogEntryPanel
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		client, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURL)
		if err == nil {
			entries, err := client.GetInstallAuditLogs(c.Request.Context(), install.NuonInstallID, startTime, endTime)
			if err == nil {
				for _, entry := range entries {
					auditEntries = append(auditEntries, partials.AuditLogEntryPanel{
						LogLine:   entry.LogLine,
						TimeStamp: entry.TimeStamp,
						Type:      entry.Type,
					})
				}
			}
		}
	}

	props := partials.AuditLogsPanelProps{
		Install:   install,
		Entries:   auditEntries,
		StartTime: startTime,
		EndTime:   endTime,
		RangeType: rangeType,
		BasePath:  h.basePath,
	}
	h.RenderTempl(c, http.StatusOK, partials.AuditLogsPanel(props))
}

// getOrgForCustomerPage looks up the NuonOrg using the subdomain in the gin context.
// Returns an error if the subdomain is not set or the org is not found.
