package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
	"go.uber.org/zap"
)

func (h *Handler) UpdateAppOrder(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	var body struct {
		AppIDs          []string          `json:"app_ids"`
		AppStatuses     map[string]string `json:"app_statuses"`
		PublishedAppIDs []string          `json:"published_app_ids"` // backward compat
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// If AppStatuses not provided, fall back to legacy PublishedAppIDs
	if len(body.AppStatuses) == 0 && len(body.PublishedAppIDs) > 0 {
		body.AppStatuses = make(map[string]string, len(body.PublishedAppIDs))
		for _, id := range body.PublishedAppIDs {
			body.AppStatuses[id] = "published"
		}
	}

	h.logger.Info("UpdateAppOrder received",
		zap.Strings("app_ids", body.AppIDs),
		zap.Any("app_statuses", body.AppStatuses),
	)

	tx := h.db.Begin()
	if tx.Error != nil {
		h.logger.Error("failed to begin transaction", zap.Error(tx.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
		return
	}

	for sortOrder, appID := range body.AppIDs {
		status := body.AppStatuses[appID]
		if status != models.AppStatusPublished && status != models.AppStatusComingSoon {
			status = models.AppStatusUnpublished
		}

		var pa models.PublishedApp
		result := tx.Unscoped().Where("org_id = ? AND app_id = ?", org.ID, appID).First(&pa)
		if result.Error != nil {
			// New record
			newPA := models.PublishedApp{OrgID: org.ID, AppID: appID, SortOrder: sortOrder, Status: status}
			if err := tx.Create(&newPA).Error; err != nil {
				h.logger.Error("failed to create app record", zap.String("app_id", appID), zap.Error(err))
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
				return
			}
			h.logger.Info("created app record", zap.String("app_id", appID), zap.String("status", status), zap.Int("sort_order", sortOrder))
		} else {
			// Existing record — undelete if soft-deleted, then update
			if pa.DeletedAt.Valid {
				if err := tx.Unscoped().Model(&pa).Update("deleted_at", nil).Error; err != nil {
					h.logger.Error("failed to undelete app record", zap.String("app_id", appID), zap.Error(err))
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
					return
				}
			}
			res := tx.Model(&pa).Updates(map[string]interface{}{"sort_order": sortOrder, "status": status})
			if res.Error != nil {
				h.logger.Error("failed to update app record", zap.String("app_id", appID), zap.Error(res.Error))
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
				return
			}
			h.logger.Info("updated app record", zap.String("app_id", appID), zap.String("status", status), zap.Int("sort_order", sortOrder), zap.Int64("rows_affected", res.RowsAffected))
		}
	}

	if err := tx.Commit().Error; err != nil {
		h.logger.Error("failed to commit app order transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "configuration updated"})
}

// AppDetailRedirect redirects app detail to inputs page
