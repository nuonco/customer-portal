package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// InstallsPage redirects to the first install in the user's account,
// or to the app catalog if no installs exist.
func (h *Handler) InstallsPage(c *gin.Context) {
	user := h.tryGetLoggedInUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, h.basePath+"/login")
		return
	}

	acctActive, _ := h.getCustomerAccountsFromContext(c)
	installs := h.loadSidebarInstalls(user, acctActive)

	if len(installs) > 0 {
		c.Redirect(http.StatusFound, h.basePath+"/installs/"+installs[0].ID+"/overview")
	} else {
		c.Redirect(http.StatusFound, h.basePath+"/apps")
	}
}
