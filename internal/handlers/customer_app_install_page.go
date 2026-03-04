package handlers

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) CustomerAppInstallPage(c *gin.Context) {
	appID := c.Param("app_id")

	// Require authentication
	loggedInUser := h.tryGetLoggedInUser(c)
	if loggedInUser == nil {
		redirectURL := url.QueryEscape(h.basePath + "/apps/" + appID + "/install")
		c.Redirect(http.StatusFound, h.basePath+"/login?redirect="+redirectURL)
		return
	}

	acctActive, acctOthers := h.getCustomerAccountsFromContext(c)

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", loggedInUser, theme, h.getOrgForLayout(c), acctActive, acctOthers),
			Error:       "Organization not found",
		}
		h.RenderTempl(c, http.StatusNotFound, customerpages.ErrorPage(props))
		return
	}

	// Verify app is published for this org
	var publishedApp models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ?", org.ID, appID).First(&publishedApp).Error; err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db, org.ID)
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", loggedInUser, theme, h.getOrgForLayout(c), acctActive, acctOthers),
			Error:       "App not found or not published",
		}
		h.RenderTempl(c, http.StatusNotFound, customerpages.ErrorPage(props))
		return
	}

	theme, _ := models.GetOrCreateAppTheme(h.db, org.ID)

	appName := appID
	orgName := org.NuonOrgID // fallback
	nuonClient, clientErr := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if clientErr == nil {
		app, appErr := nuonClient.GetApp(c.Request.Context(), appID)
		if appErr == nil && app != nil && app.Name != "" {
			appName = app.Name
		}
		apiOrg, orgErr := nuonClient.GetOrg(c.Request.Context())
		if orgErr == nil && apiOrg != nil && apiOrg.Name != "" {
			orgName = apiOrg.Name
		}
	}

	layoutProps := h.buildCustomerLayoutProps("Install "+appName, loggedInUser, theme, h.getOrgForLayout(c), acctActive, acctOthers)
	layoutProps.HasPublishedApps = true
	layoutProps.ActiveNav = "apps"

	props := customerpages.AppInstallPageProps{
		LayoutProps:  layoutProps,
		AppID:        appID,
		AppName:      appName,
		OrgName:      orgName,
		LoggedInUser: loggedInUser,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.AppInstallPage(props))
}

// CreateInstallFromApp creates an install from a published app
