package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) AppDetailRedirect(c *gin.Context) {
	orgID := c.Param("org_id")
	appID := c.Param("app_id")
	c.Redirect(http.StatusFound, fmt.Sprintf("%s/orgs/%s/apps/%s/overview", h.basePath, orgID, appID))
}

// AppInputsPage displays the input configuration for an app with vendor/customer tabs
