package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/markdown"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) CustomerAppDetailPage(c *gin.Context) {
	appID := c.Param("app_id")
	user := h.tryGetLoggedInUser(c)

	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		c.Redirect(http.StatusFound, h.basePath+"/installs")
		return
	}

	var publishedApp models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ? AND status IN ?", org.ID, appID, []string{models.AppStatusPublished, models.AppStatusComingSoon}).First(&publishedApp).Error; err != nil {
		c.Redirect(http.StatusFound, h.basePath+"/apps")
		return
	}

	theme, _ := models.GetOrCreateAppTheme(h.db, org.ID)

	nuonClient, nuonClientErr := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))

	display := h.buildAppDisplay(c, appID, org.ID, nuonClient, nuonClientErr)
	display.LogoLightBase64 = publishedApp.LogoLightBase64
	display.LogoDarkBase64 = publishedApp.LogoDarkBase64
	if publishedApp.OverviewMarkdown != "" {
		if html, err := markdown.Render([]byte(publishedApp.OverviewMarkdown)); err == nil {
			display.ReadmeHTML = html
		}
	}
	display.Status = publishedApp.Status
	if display.Status == "" {
		display.Status = "published"
	}

	acctActive, acctOthers := h.getCustomerAccountsFromContext(c)
	layoutProps := h.buildCustomerLayoutProps(display.AppName, user, theme, h.getOrgForLayout(c), acctActive, acctOthers)
	layoutProps.HasPublishedApps = true
	layoutProps.ActiveNav = "apps"

	props := customerpages.CustomerAppDetailPageProps{
		LayoutProps: layoutProps,
		App:         display,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.CustomerAppDetailPage(props))
}
