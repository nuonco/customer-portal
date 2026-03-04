package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
)

func (h *Handler) VendorRegisterPageTempl(c *gin.Context) {
	// Check for error message in query params
	errorMsg := c.Query("error")

	props := vendorpages.RegisterPageProps{
		Title:      "Create Account",
		ButtonText: "Sign Up",
		BasePath:   h.basePath,
		Error:      errorMsg,
		CSSPath:    assets.VendorCSSPath(),
	}

	h.RenderTempl(c, http.StatusOK, vendorpages.RegisterPage(props))
}

// LocalRegister handles POST /admin/register for new user registration
