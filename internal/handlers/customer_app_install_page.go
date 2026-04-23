package handlers

import (
	"github.com/gin-gonic/gin"
)

// CustomerAppInstallPage delegates all install page requests to the wizard handler.
func (h *Handler) CustomerAppInstallPage(c *gin.Context) {
	h.InstallWizardPage(c)
}

// CreateInstallFromApp creates an install from a published app
