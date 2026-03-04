package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
)

func (h *Handler) ProfilePanelContent(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	// Load full user from DB to get current name
	var dbUser models.User
	if err := h.db.First(&dbUser, "id = ?", user.ID).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to load user profile")
		return
	}

	props := partials.ProfilePanelProps{
		User:     &dbUser,
		BasePath: h.basePath,
	}

	h.RenderTempl(c, http.StatusOK, partials.ProfilePanel(props))
}

// UpdateProfile handles PUT request to update user profile (name)
