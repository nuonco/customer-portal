package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"github.com/nuonco/nuon-go/client/operations"
	"go.uber.org/zap"
)

// buildInstallPageProps loads install data and builds shared props for install tab pages.
func (h *Handler) buildInstallPageProps(c *gin.Context, activeTab string) (*customerpages.InstallPageProps, error) {
	user := h.tryGetLoggedInUser(c)

	installInterface, exists := c.Get("install")
	if !exists {
		return nil, errors.New("install not found")
	}

	install := installInterface.(*models.Install)

	if err := h.loadInstallWithOrg(install); err != nil {
		return nil, err
	}

	nuonOrg := install.GetNuonOrg()

	// Check if install still exists in Nuon API
	var apiDeletedError bool
	var nuonClientErr error
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		checkClient, checkErr := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURLForOrg(nuonOrg),
		)
		nuonClientErr = checkErr
		if checkErr == nil {
			_, apiErr := checkClient.GetInstall(context.Background(), install.NuonInstallID)
			if apiErr != nil {
				var notFoundErr *operations.GetInstallNotFound
				if errors.As(apiErr, &notFoundErr) {
					apiDeletedError = true
					h.logger.Warn("install deleted from API but exists locally",
						zap.String("install_id", install.ID),
						zap.String("nuon_install_id", install.NuonInstallID),
					)
					if !install.APIDeleted {
						h.db.Model(install).Update("api_deleted", true)
					}
				} else {
					nuonClientErr = apiErr
				}
			} else if install.APIDeleted {
				h.db.Model(install).Update("api_deleted", false)
			}
		}
	}

	// Fetch app name
	var appName string
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		appClient, err := nuon.NewClientWithURL(
			nuonOrg.APIToken,
			nuonOrg.NuonOrgID,
			h.nuonAPIURLForOrg(nuonOrg),
		)
		if err == nil {
			app, err := appClient.GetApp(context.Background(), install.GetAppID())
			if err == nil && app != nil {
				appName = appDisplayName(app)
			}
		}
	}

	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	acctActive, acctOthers := h.getCustomerAccountsFromContext(c)

	title := install.Name
	if appName != "" {
		title = install.Name + " — " + appName
	}

	layoutProps := h.buildCustomerLayoutProps(title, user, theme, h.getOrgForLayout(c), acctActive, acctOthers)
	layoutProps.HasPublishedApps = h.orgHasPublishedApps(orgID)
	layoutProps.ActiveNav = "installs"
	if nuonClientErr != nil {
		layoutProps.NuonAPIError = "The app is experiencing network issues, and is not able to access app or install data. Please contact support for assistance."
	}
	layoutProps.CurrentInstallID = install.ID
	layoutProps.ActiveTab = activeTab

	tabTitles := map[string]string{
		"overview":   "Overview",
		"stack":      "Stack",
		"sandbox":    "Sandbox",
		"components": "Components",
		"roles":      "Roles",
		"policies":   "Policies",
		"history":    "History",
		"app-info":   "README",
		"audit":      "Audit Log",
	}

	tabIcons := map[string]string{
		"overview":   "house-simple",
		"stack":      "stack",
		"sandbox":    "shipping-container",
		"components": "cards",
		"roles":      "file-lock",
		"policies":   "shield-check",
		"audit":      "clock-counter-clockwise",
		"app-info":   "book-open",
	}

	return &customerpages.InstallPageProps{
		LayoutProps:     layoutProps,
		Install:         install,
		AppName:         appName,
		APIDeletedError: apiDeletedError,
		PageTitle:       tabTitles[activeTab],
		PageIcon:        tabIcons[activeTab],
	}, nil
}

// InstallDetailPage redirects to the overview tab.
func (h *Handler) InstallDetailPage(c *gin.Context) {
	installInterface, exists := c.Get("install")
	if !exists {
		c.String(http.StatusNotFound, "Install not found")
		return
	}
	install := installInterface.(*models.Install)
	dest := h.basePath + "/installs/" + install.ID + "/overview"
	if qs := c.Request.URL.RawQuery; qs != "" {
		dest += "?" + qs
	}
	c.Redirect(http.StatusFound, dest)
}

func (h *Handler) InstallOverviewPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "overview")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	props.AutoOpenWorkflow = c.Query("open_workflow") == "true"
	h.RenderTempl(c, http.StatusOK, customerpages.InstallOverviewPage(*props))
}

func (h *Handler) InstallStackPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "stack")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallStackPage(*props))
}

func (h *Handler) InstallSandboxPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "sandbox")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallSandboxPage(*props))
}

func (h *Handler) InstallComponentsPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "components")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallComponentsPage(*props))
}

func (h *Handler) InstallRolesPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "roles")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallRolesPage(*props))
}

func (h *Handler) InstallPoliciesPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "policies")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallPoliciesPage(*props))
}

func (h *Handler) InstallAppInfoPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "app-info")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallAppInfoPage(*props))
}

func (h *Handler) InstallAuditPage(c *gin.Context) {
	props, err := h.buildInstallPageProps(c, "audit")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	h.RenderTempl(c, http.StatusOK, customerpages.InstallAuditPage(*props))
}
