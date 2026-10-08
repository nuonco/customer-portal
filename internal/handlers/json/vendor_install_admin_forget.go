package jsonhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
)

func (h *VendorHandler) AdminForgetInstall(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	installID := c.Param("install_id")

	var install models.Install
	if err := h.db.Where("id = ? AND org_id = ?", installID, org.ID).First(&install).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install not found"})
		return
	}

	if err := h.db.Delete(&install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to forget install"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Install forgotten successfully"})
}
