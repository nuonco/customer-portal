package jsonhandlers

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

const maxProfileNameLength = 255

type updateProfileRequest struct {
	Name string `json:"name"`
}

func (h *VendorHandler) UpdateProfile(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Display name is required"})
		return
	}

	if utf8.RuneCountInString(name) > maxProfileNameLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Display name must be 255 characters or fewer"})
		return
	}

	if err := h.db.Model(&models.User{}).Where("id = ?", user.ID).Update("name", name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}
