package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"go.uber.org/zap"
)

func (h *Handler) DebugUserOrgs(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetCurrentOrg(c)

	h.logger.Debug("DebugUserOrgs",
		zap.String("user_id", user.ID),
		zap.String("email", user.Email),
		zap.String("org_id", org.ID),
	)

	var orgs []models.NuonOrg
	if err := h.db.Where("org_id = ?", org.ID).Find(&orgs).Error; err != nil {
		h.logger.Error("DebugUserOrgs: error querying orgs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	h.logger.Debug("DebugUserOrgs: found organizations",
		zap.Int("count", len(orgs)),
		zap.String("org_id", org.ID),
	)

	// Format debug response
	debugOrgs := make([]gin.H, len(orgs))
	for i, org := range orgs {
		debugOrgs[i] = gin.H{
			"id":      org.ID,
			"name":    org.Name,
			"org_id":  org.NuonOrgID,
			"user_id": org.UserID,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":             user.ID,
		"user_email":          user.Email,
		"organizations_count": len(orgs),
		"organizations":       debugOrgs,
		"database_connection": "ok",
	})
}

// AppInfo represents basic app information for the apps list
