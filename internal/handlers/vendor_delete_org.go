package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) DeleteOrg(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error":     "Org deletion is no longer supported. Each workspace has exactly one connected org.",
		"migration": "To remove this org, delete the workspace from workspace settings.",
	})
}

// UpdateThemeSettings handles PUT request to update global theme settings
