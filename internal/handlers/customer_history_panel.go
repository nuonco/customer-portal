package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon-go/models"
	"go.uber.org/zap"
)

const historyPageSize = 20

// HistoryPanel renders the workflow history panel with pagination.
func (h *Handler) HistoryPanel(c *gin.Context) {
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

	offset := 0
	if o, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil && o >= 0 {
		offset = o
	}

	var workflows []*nuonmodels.AppWorkflow
	var hasMore bool

	if nuonOrg != nil && nuonOrg.APIToken != "" {
		apiClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
		if err == nil {
			ctx := c.Request.Context()
			workflows, hasMore, err = apiClient.GetInstallWorkflows(ctx, install.NuonInstallID, offset, historyPageSize)
			if err != nil {
				h.logger.Warn("failed to fetch workflows", zap.Error(err))
			}
		}
	}

	props := partials.HistoryPanelProps{
		Install:   install,
		BasePath:  h.basePath,
		Workflows: workflows,
		Pagination: partials.HistoryPagination{
			HasNext:    hasMore,
			HasPrev:    offset > 0,
			NextOffset: offset + historyPageSize,
			PrevOffset: max(offset-historyPageSize, 0),
		},
	}

	h.RenderTempl(c, http.StatusOK, partials.HistoryPanel(props))
}
