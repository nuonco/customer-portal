package jsonhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
)

func (h *VendorHandler) DeleteInstallLink(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	linkID := c.Param("link_id")

	if !shortid.IsValid(linkID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid link ID"})
		return
	}

	result := h.db.Where("id = ? AND org_id = ?", linkID, org.ID).Delete(&models.InstallLink{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete install"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Install link not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Install deleted successfully"})
}
