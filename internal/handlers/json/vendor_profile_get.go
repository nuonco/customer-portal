package jsonhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

type meResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (h *VendorHandler) Me(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var dbUser models.User
	if err := h.db.First(&dbUser, "id = ?", user.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}

	c.JSON(http.StatusOK, meResponse{ID: dbUser.ID, Email: dbUser.Email, Name: dbUser.Name})
}
