package handlers

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) CustomerAppInstallPage(c *gin.Context) {
	appID := c.Param("app_id")

	loggedInUser := h.tryGetLoggedInUser(c)

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
	if err := h.db.Where("org_id = ? AND app_id = ? AND status IN ?", org.ID, appID, []string{models.AppStatusPublished, models.AppStatusComingSoon}).First(&publishedApp).Error; err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db, org.ID)
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", loggedInUser, theme, h.getOrgForLayout(c), acctActive, acctOthers),
			Error:       "App not found or not published",
		}
		h.RenderTempl(c, http.StatusNotFound, customerpages.ErrorPage(props))
		return
	}

	if publishedApp.Status == "coming_soon" {
		c.Redirect(http.StatusFound, h.basePath+"/apps/"+appID)
		return
	}

	theme, _ := models.GetOrCreateAppTheme(h.db, org.ID)

	appName := appID
	orgName := org.NuonOrgID // fallback
	nuonClient, clientErr := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if clientErr == nil {
		app, appErr := nuonClient.GetApp(c.Request.Context(), appID)
		if appErr == nil && app != nil {
			if dn := appDisplayName(app); dn != "" {
				appName = dn
			}
		}
		apiOrg, orgErr := nuonClient.GetOrg(c.Request.Context())
		if orgErr == nil && apiOrg != nil && apiOrg.Name != "" {
			orgName = apiOrg.Name
		}
	}

	layoutProps := h.buildCustomerLayoutProps("Install "+appName, loggedInUser, theme, h.getOrgForLayout(c), acctActive, acctOthers)
	layoutProps.HasPublishedApps = true
	layoutProps.ActiveNav = "apps"

	var authURL string
	if loggedInUser == nil {
		if subdomain, ok := c.Get("subdomain"); ok && subdomain != nil && subdomain.(string) != "" {
			redirect := url.QueryEscape(h.basePath + "/apps/" + appID + "/install")
			authURL = fmt.Sprintf("%s/auth/login?return_to=%s&redirect=%s", h.customerBaseURL, subdomain.(string), redirect)
		}
	}

	props := customerpages.CreateInstallPageProps{
		LayoutProps:     layoutProps,
		AppID:           appID,
		AppName:         appName,
		OrgName:         orgName,
		LogoLightBase64: publishedApp.LogoLightBase64,
		LogoDarkBase64:  publishedApp.LogoDarkBase64,
		LoggedInUser:    loggedInUser,
		AuthURL:         authURL,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.CreateInstallPage(props))
}

// CreateInstallFromApp creates an install from a published app
