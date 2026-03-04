package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RootRedirect(c *gin.Context) {
	c.Redirect(http.StatusFound, "/login")
}

// LoginPageConfig holds configuration for the login page
