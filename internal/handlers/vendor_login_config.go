package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/auth"
)

type VendorLoginConfigResponse struct {
	AuthURL      string `json:"authUrl"`
	ErrorMessage string `json:"errorMessage"`
	Title        string `json:"title"`
}

func (h *Handler) VendorLoginConfigJSON(c *gin.Context) {
	config := VendorLoginConfigResponse{
		Title: "Customer Portal",
	}

	if h.authProvider == nil {
		config.ErrorMessage = "Authentication not configured"
		c.JSON(http.StatusServiceUnavailable, config)
		return
	}

	if h.authProvider.Name() == "local" {
		config.ErrorMessage = "OIDC authentication required. Please configure AUTH_* environment variables."
		c.JSON(http.StatusServiceUnavailable, config)
		return
	}

	redirectTo := c.Query("redirect")
	if !strings.HasPrefix(redirectTo, "/") {
		redirectTo = ""
	}

	state, err := auth.GenerateStateWithRedirect("", redirectTo)
	if err != nil {
		config.ErrorMessage = "Failed to generate security token"
		c.JSON(http.StatusInternalServerError, config)
		return
	}

	c.SetCookie("auth_state", state, 600, "/", "", false, true)

	authURL, err := h.authProvider.GetAuthorizationURL(state)
	if err != nil {
		config.ErrorMessage = "Failed to generate login URL"
		c.JSON(http.StatusInternalServerError, config)
		return
	}

	config.AuthURL = authURL
	if errorMsg := c.Query("error"); errorMsg != "" {
		config.ErrorMessage = errorMsg
	}

	c.JSON(http.StatusOK, config)
}
