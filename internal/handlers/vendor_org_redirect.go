package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) OrgRedirect(c *gin.Context) {
	orgID := c.Param("org_id")
	c.Redirect(http.StatusFound, fmt.Sprintf("%s/orgs/%s/accounts", h.basePath, orgID))
}

// OrgDetailPage renders the org detail page with paginated install links using Templ
