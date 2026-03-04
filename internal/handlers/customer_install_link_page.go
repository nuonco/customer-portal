package handlers

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/overrides"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
)

func (h *Handler) InstallLinkPage(c *gin.Context) {
	sha := c.Query("sha")
	if sha == "" {
		theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme, h.getOrgForLayout(c), nil, nil),
			Error:       "Missing or invalid install link",
		}
		h.RenderTempl(c, http.StatusBadRequest, customerpages.ErrorPage(props))
		return
	}

	var link models.InstallLink
	if err := h.db.Preload("NuonOrg").Where("sha = ?", sha).First(&link).Error; err != nil {
		theme, _ := models.GetOrCreateAppTheme(h.db, h.getOrgIDForTheme(c))
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme, h.getOrgForLayout(c), nil, nil),
			Error:       "Install link not found or invalid",
		}
		h.RenderTempl(c, http.StatusNotFound, customerpages.ErrorPage(props))
		return
	}

	if link.Used {
		theme, _ := models.GetOrCreateAppTheme(h.db, link.OrgID)
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme, h.getOrgForLayout(c), nil, nil),
			Error:       "This install link has already been used",
		}
		h.RenderTempl(c, http.StatusBadRequest, customerpages.ErrorPage(props))
		return
	}

	// Get global app theme for customer UI
	// Use install link's org ID (not getOrgIDForTheme which only works for vendor routes)
	orgID := link.OrgID
	theme, err := models.GetOrCreateAppTheme(h.db, orgID)
	if err != nil {
		props := customerpages.ErrorPageProps{
			LayoutProps: h.buildCustomerLayoutProps("Error", nil, theme, h.getOrgForLayout(c), nil, nil),
			Error:       "Failed to load theme settings",
		}
		h.RenderTempl(c, http.StatusInternalServerError, customerpages.ErrorPage(props))
		return
	}

	// REQUIRE authentication - redirect to login if not logged in
	loggedInUser := h.tryGetLoggedInUser(c)
	if loggedInUser == nil {
		// Build redirect URL to return to this install link after login
		redirectURL := url.QueryEscape(h.basePath + "/install-link?sha=" + sha)
		c.Redirect(http.StatusFound, h.basePath+"/login?redirect="+redirectURL)
		return
	}

	// Try template override first
	vendorInputs, _ := link.GetVendorInputs()
	vendorInputsInterface := make(map[string]interface{})
	for k, v := range vendorInputs {
		vendorInputsInterface[k] = v
	}

	pageData := overrides.InstallLinkPageData{
		SHA:          sha,
		AppName:      link.AppName,
		VendorInputs: vendorInputsInterface,
		SubmitURL:    h.basePath + "/install-link/",
	}

	ctx := h.buildTemplateContext(orgID, "Install "+link.AppName, loggedInUser, theme, pageData)
	if h.tryRenderOverride(c, orgID, "install_link", ctx) {
		return
	}

	// Fall back to default Templ template
	props := customerpages.InstallLinkPageProps{
		LayoutProps:  h.buildCustomerLayoutProps("Install "+link.AppName, nil, theme, h.getOrgForLayout(c), nil, nil),
		Link:         &link,
		LoggedInUser: loggedInUser,
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallLinkPage(props))
}

// AcceptInstallLink handles the install link acceptance
// Creates the Install via Nuon API with merged vendor + customer inputs
// Requires authentication - user must be logged in
