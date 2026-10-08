package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
)

func (h *Handler) ForgetApp(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	appID := c.Param("app_id")

	var app models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&app).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "App not found"})
		return
	}

	if err := h.db.Delete(&app).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove app"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "App removed successfully"})
}
