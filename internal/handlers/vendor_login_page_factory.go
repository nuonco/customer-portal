package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) LoginPage(config LoginPageConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, config.Template, h.MergeData(h.BaseData(), gin.H{
			"title":           config.Title + " - Installer App",
			"Title":           config.Title,
			"ButtonText":      config.ButtonText,
			"HelpText":        config.HelpText,
			"RedirectURL":     config.RedirectURL,
			"contentTemplate": "login_content",
		}))
	}
}

// CustomerLoginPageTempl renders the customer login page with vendor theming

func VendorLoginConfig(basePath string) LoginPageConfig {
	return LoginPageConfig{
		Title:       "Vendor Login",
		ButtonText:  "Login / Sign Up",
		HelpText:    "",
		RedirectURL: basePath + "/orgs",
		Template:    "vendor/login.html",
	}
}

// CustomerLoginConfig returns the login page config for customers

func CustomerLoginConfig(basePath string) LoginPageConfig {
	return LoginPageConfig{
		Title:       "Customer Login",
		ButtonText:  "Login",
		HelpText:    "Don't have an account? Accept an install link from your vendor to get started.",
		RedirectURL: basePath + "/installs",
		Template:    "customer/login.html",
	}
}

// LoginPage renders the login page with the given configuration
