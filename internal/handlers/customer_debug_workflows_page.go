package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/components"
	customerpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	nuonmodels "github.com/nuonco/nuon-go/models"
)

const debugWorkflowsPerPage = 20

// buildDebugWorkflowsProps fetches paginated workflows and builds the page props.
func (h *Handler) buildDebugWorkflowsProps(c *gin.Context) (*customerpages.DebugWorkflowsPageProps, error) {
	user := h.tryGetLoggedInUser(c)

	installInterface, exists := c.Get("install")
	if !exists {
		return nil, fmt.Errorf("install not found")
	}
	install := installInterface.(*models.Install)

	if err := h.loadInstallWithOrg(install); err != nil {
		return nil, err
	}

	nuonOrg := install.GetNuonOrg()

	// Fetch app name
	var appName string
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		appClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
		if err == nil {
			app, err := appClient.GetApp(c.Request.Context(), install.GetAppID())
			if err == nil && app != nil {
				appName = appDisplayName(app)
			}
		}
	}

	// Parse page number
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * debugWorkflowsPerPage

	// Fetch workflows
	var workflows []*nuonmodels.AppWorkflow
	var hasMore bool
	if nuonOrg != nil && nuonOrg.APIToken != "" {
		nuonClient, err := nuon.NewClientWithURL(nuonOrg.APIToken, nuonOrg.NuonOrgID, h.nuonAPIURLForOrg(nuonOrg))
		if err == nil {
			workflows, hasMore, err = nuonClient.GetInstallWorkflows(c.Request.Context(), install.NuonInstallID, offset, debugWorkflowsPerPage)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch workflows: %w", err)
			}
		}
	}

	// Build pagination
	totalShowing := offset + len(workflows)
	showingFrom := 0
	showingTo := 0
	if len(workflows) > 0 {
		showingFrom = offset + 1
		showingTo = totalShowing
	}

	baseURL := fmt.Sprintf("%s/installs/%s/debug/workflows?page=", h.basePath, install.ID)
	pagination := components.PaginationProps{
		CurrentPage:  page,
		HasPrevious:  page > 1,
		HasNext:      hasMore,
		PreviousPage: page - 1,
		NextPage:     page + 1,
		BaseURL:      baseURL,
		ShowingFrom:  showingFrom,
		ShowingTo:    showingTo,
	}

	// Build layout props
	orgID := h.getOrgIDForTheme(c)
	theme, _ := models.GetOrCreateAppTheme(h.db, orgID)
	acctActive, acctOthers := h.getCustomerAccountsFromContext(c)

	title := "Debug — " + install.Name
	layoutProps := h.buildCustomerLayoutProps(title, user, theme, h.getOrgForLayout(c), acctActive, acctOthers)
	layoutProps.HasPublishedApps = h.orgHasPublishedApps(orgID)
	layoutProps.ActiveNav = "installs"
	layoutProps.CurrentInstallID = install.ID

	return &customerpages.DebugWorkflowsPageProps{
		LayoutProps: layoutProps,
		Install:     install,
		AppName:     appName,
		Workflows:   workflows,
		Pagination:  pagination,
	}, nil
}

// DebugWorkflowsPage renders the debug workflows list page.
func (h *Handler) DebugWorkflowsPage(c *gin.Context) {
	if !h.requireVendorRole(c) {
		return
	}

	props, err := h.buildDebugWorkflowsProps(c)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	switch c.Query("partial") {
	case "panel":
		h.RenderTempl(c, http.StatusOK, customerpages.DebugWorkflowPanel(*props))
	default:
		h.RenderTempl(c, http.StatusOK, customerpages.DebugWorkflowsPage(*props))
	}
}
