package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) CustomerRootRedirect(c *gin.Context) {
	if user := h.tryGetLoggedInUser(c); user != nil {
		c.Redirect(http.StatusFound, "/installs")
	} else {
		c.Redirect(http.StatusFound, "/apps")
	}
}

// tryGetLoggedInUser attempts to extract a logged-in user from JWT cookie
// Returns nil if no valid JWT is present or if parsing fails
