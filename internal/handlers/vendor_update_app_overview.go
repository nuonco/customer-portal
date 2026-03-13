package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/markdown"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

func (h *Handler) UpdateAppOverview(c *gin.Context) {
	appID := c.Param("app_id")

	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	var req struct {
		OverviewMarkdown string `json:"overview_markdown"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var pa models.PublishedApp
	result := h.db.Unscoped().Where("org_id = ? AND app_id = ?", org.ID, appID).First(&pa)
	if result.Error != nil {
		pa = models.PublishedApp{OrgID: org.ID, AppID: appID, OverviewMarkdown: req.OverviewMarkdown}
		h.db.Create(&pa)
	} else {
		h.db.Unscoped().Model(&pa).Update("overview_markdown", req.OverviewMarkdown)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) PreviewAppOverview(c *gin.Context) {
	var req struct {
		OverviewMarkdown string `json:"overview_markdown"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "Invalid request body")
		return
	}

	html, err := markdown.Render([]byte(req.OverviewMarkdown))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to render markdown")
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
