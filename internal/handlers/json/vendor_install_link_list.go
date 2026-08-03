package jsonhandlers

import (
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

func (h *VendorHandler) InstallLinks(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	const linksPerPage = 10
	currentTab := c.DefaultQuery("tab", "available")
	page := pageFromQuery(c)
	offset := (page - 1) * linksPerPage

	baseQuery := h.db.Where("org_id = ?", org.ID)
	if currentTab == "used" {
		baseQuery = baseQuery.Where("used = ?", true)
	} else {
		baseQuery = baseQuery.Where("used = ?", false)
	}

	var totalCount int64
	if err := baseQuery.Model(&models.InstallLink{}).Count(&totalCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count install links"})
		return
	}

	var availableCount int64
	if err := h.db.Model(&models.InstallLink{}).Where("org_id = ? AND used = ?", org.ID, false).Count(&availableCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count available install links"})
		return
	}

	var usedCount int64
	if err := h.db.Model(&models.InstallLink{}).Where("org_id = ? AND used = ?", org.ID, true).Count(&usedCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count used install links"})
		return
	}

	query := h.db.Preload("Install").Where("org_id = ?", org.ID)
	if currentTab == "used" {
		query = query.Where("used = ?", true)
	} else {
		query = query.Where("used = ?", false)
	}

	var links []models.InstallLink
	if err := query.Order("created_at DESC").Offset(offset).Limit(linksPerPage).Find(&links).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load install links"})
		return
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(linksPerPage)))
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	showingFrom := offset + 1
	showingTo := offset + len(links)
	if totalCount == 0 {
		showingFrom = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"links": links,
		"pagination": gin.H{
			"current_page":    page,
			"total_pages":     totalPages,
			"has_previous":    page > 1,
			"has_next":        page < totalPages,
			"previous_page":   page - 1,
			"next_page":       page + 1,
			"total_count":     totalCount,
			"per_page":        linksPerPage,
			"showing_from":    showingFrom,
			"showing_to":      showingTo,
			"current_tab":     currentTab,
			"available_count": availableCount,
			"used_count":      usedCount,
		},
	})
}
