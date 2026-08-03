package jsonhandlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"gorm.io/gorm"
)

type CustomerPortalHandler struct {
	db                  *gorm.DB
	customerBaseURL     string
	subdomainBaseDomain string
	nuonAPIURL          string
}

func NewCustomerPortalHandler(db *gorm.DB, customerBaseURL, subdomainBaseDomain, nuonAPIURL string) *CustomerPortalHandler {
	return &CustomerPortalHandler{
		db:                  db,
		customerBaseURL:     customerBaseURL,
		subdomainBaseDomain: subdomainBaseDomain,
		nuonAPIURL:          nuonAPIURL,
	}
}

type customerPortalThemeResponse struct {
	HeaderTitle string `json:"header_title"`
	LogoLight   string `json:"logo_light"`
	LogoDark    string `json:"logo_dark"`
	Primary     string `json:"primary"`
	PrimaryDark string `json:"primary_dark"`
	ThemeMode   string `json:"theme_mode"`
}

type customerPortalUserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type customerPortalOrgResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Subdomain    string `json:"subdomain"`
	PortalDomain string `json:"portal_domain"`
	AdminURL     string `json:"admin_url"`
}

type customerPortalAccountResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type customerPortalInstallResponse struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	AppID          string    `json:"app_id"`
	AppName        string    `json:"app_name"`
	Status         string    `json:"status"`
	Visibility     string    `json:"visibility"`
	CreatedAt      time.Time `json:"created_at"`
	AppLogoLight   string    `json:"app_logo_light"`
	AppLogoDark    string    `json:"app_logo_dark"`
	LegacyBasePath string    `json:"legacy_base_path"`
}

type customerPortalAppResponse struct {
	ID             string `json:"id"`
	AppID          string `json:"app_id"`
	DisplayName    string `json:"display_name"`
	Status         string `json:"status"`
	Summary        string `json:"summary"`
	Platform       string `json:"platform"`
	LogoLight      string `json:"logo_light"`
	LogoDark       string `json:"logo_dark"`
	LegacyBasePath string `json:"legacy_base_path"`
}

type customerPortalStateResponse struct {
	User          customerPortalUserResponse      `json:"user"`
	Org           customerPortalOrgResponse       `json:"org"`
	Theme         customerPortalThemeResponse     `json:"theme"`
	ActiveAccount *customerPortalAccountResponse  `json:"active_account"`
	OtherAccounts []customerPortalAccountResponse `json:"other_accounts"`
	Installs      []customerPortalInstallResponse `json:"installs"`
	Apps          []customerPortalAppResponse     `json:"apps"`
}

func (h *CustomerPortalHandler) State(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org, err := h.getOrgForCustomerPortal(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	theme, err := models.GetOrCreateAppTheme(h.db, org.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load portal theme"})
		return
	}

	activeAccount, otherAccounts := h.getCustomerAccounts(c, user.ID, org.ID)
	installs := h.getVisibleInstalls(user.ID, activeAccount)
	apps := h.getPublishedApps(c, org)

	headerTitle := ""
	if !theme.HeaderTitleHidden {
		headerTitle = theme.GetHeaderTitle(org.Name)
	}

	portalDomain := ""
	if org.Subdomain != "" && h.subdomainBaseDomain != "" {
		portalDomain = org.Subdomain + "." + h.subdomainBaseDomain
	}

	adminURL := h.customerBaseURL + "/admin/orgs"
	if org.ID != "" {
		adminURL = h.customerBaseURL + "/admin/orgs/" + org.ID
	}

	response := customerPortalStateResponse{
		User: customerPortalUserResponse{
			ID:    user.ID,
			Email: user.Email,
			Name:  user.Name,
			Role:  string(user.Role),
		},
		Org: customerPortalOrgResponse{
			ID:           org.ID,
			Name:         org.Name,
			Subdomain:    org.Subdomain,
			PortalDomain: portalDomain,
			AdminURL:     adminURL,
		},
		Theme: customerPortalThemeResponse{
			HeaderTitle: headerTitle,
			LogoLight:   theme.LogoLightBase64,
			LogoDark:    theme.LogoDarkBase64,
			Primary:     theme.GetColorsForMode(false),
			PrimaryDark: theme.GetColorsForMode(true),
			ThemeMode:   theme.GetThemeMode(),
		},
		ActiveAccount: toPortalAccount(activeAccount),
		OtherAccounts: toPortalAccounts(otherAccounts),
		Installs:      installs,
		Apps:          apps,
	}

	c.JSON(http.StatusOK, response)
}

func (h *CustomerPortalHandler) getOrgForCustomerPortal(c *gin.Context) (*models.NuonOrg, error) {
	subdomain := strings.TrimSpace(c.Query("subdomain"))
	if subdomain == "" {
		subdomainValue, exists := c.Get("subdomain")
		if !exists {
			return nil, gorm.ErrRecordNotFound
		}

		contextSubdomain, ok := subdomainValue.(string)
		if !ok {
			return nil, gorm.ErrRecordNotFound
		}

		subdomain = strings.TrimSpace(contextSubdomain)
	}

	if subdomain == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var org models.NuonOrg
	if err := h.db.Where("subdomain = ?", subdomain).First(&org).Error; err != nil {
		return nil, err
	}

	return &org, nil
}

func (h *CustomerPortalHandler) getCustomerAccounts(c *gin.Context, userID, orgID string) (*models.CustomerAccount, []models.CustomerAccount) {
	var members []models.CustomerAccountMember
	if contextMembers := middleware.GetCustomerAccounts(c); len(contextMembers) > 0 {
		members = contextMembers
	} else {
		h.db.Preload("Account").Where(
			"user_id = ? AND org_id = ? AND deleted_at IS NULL",
			userID,
			orgID,
		).Find(&members)
	}

	if len(members) == 0 {
		return nil, nil
	}

	activeAccountID, _ := c.Cookie("active_account_id")
	var activeAccount *models.CustomerAccount
	for i := range members {
		if activeAccountID != "" && members[i].AccountID == activeAccountID {
			activeAccount = &members[i].Account
			break
		}
	}

	if activeAccount == nil {
		activeAccount = &members[0].Account
	}

	otherAccounts := make([]models.CustomerAccount, 0, len(members)-1)
	for i := range members {
		if members[i].AccountID != activeAccount.ID {
			otherAccounts = append(otherAccounts, members[i].Account)
		}
	}

	return activeAccount, otherAccounts
}

func (h *CustomerPortalHandler) getVisibleInstalls(userID string, activeAccount *models.CustomerAccount) []customerPortalInstallResponse {
	if activeAccount == nil {
		return nil
	}

	var installs []models.Install
	err := h.db.
		Where(
			"(user_id = ? AND customer_account_id = ?) OR (customer_account_id = ? AND visibility = ?)",
			userID,
			activeAccount.ID,
			activeAccount.ID,
			models.VisibilityAccount,
		).
		Order("created_at DESC").
		Find(&installs).Error
	if err != nil {
		return nil
	}

	appIDs := make(map[string]struct{})
	for _, install := range installs {
		appID := install.GetAppID()
		if appID != "" {
			appIDs[appID] = struct{}{}
		}
	}

	logoMap := make(map[string]struct {
		Light string
		Dark  string
	})
	if len(appIDs) > 0 {
		ids := make([]string, 0, len(appIDs))
		for appID := range appIDs {
			ids = append(ids, appID)
		}

		var publishedApps []models.PublishedApp
		h.db.Where("app_id IN ?", ids).Find(&publishedApps)
		for _, app := range publishedApps {
			logoMap[app.AppID] = struct {
				Light string
				Dark  string
			}{
				Light: app.LogoLightBase64,
				Dark:  app.LogoDarkBase64,
			}
		}
	}

	result := make([]customerPortalInstallResponse, 0, len(installs))
	for _, install := range installs {
		appID := install.GetAppID()
		logo := logoMap[appID]
		name := install.Name
		if name == "" {
			name = install.NuonInstallID
		}

		result = append(result, customerPortalInstallResponse{
			ID:             install.ID,
			Name:           name,
			AppID:          appID,
			AppName:        install.AppName,
			Status:         string(install.Status),
			Visibility:     string(install.Visibility),
			CreatedAt:      install.CreatedAt,
			AppLogoLight:   logo.Light,
			AppLogoDark:    logo.Dark,
			LegacyBasePath: "/bff/installs/" + install.ID,
		})
	}

	return result
}

func (h *CustomerPortalHandler) getPublishedApps(c *gin.Context, org *models.NuonOrg) []customerPortalAppResponse {
	var apps []models.PublishedApp
	err := h.db.
		Where("org_id = ? AND status IN ?", org.ID, []string{models.AppStatusPublished, models.AppStatusComingSoon}).
		Order("sort_order ASC, created_at ASC").
		Find(&apps).Error
	if err != nil {
		return nil
	}

	nuonClient, nuonErr := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))

	result := make([]customerPortalAppResponse, 0, len(apps))
	for _, app := range apps {
		displayName := app.AppID
		summary := summarizeMarkdown(app.OverviewMarkdown)
		platform := "unknown"

		if nuonErr == nil {
			if remoteApp, err := nuonClient.GetApp(c.Request.Context(), app.AppID); err == nil && remoteApp != nil {
				if remoteApp.DisplayName != "" {
					displayName = remoteApp.DisplayName
				} else if remoteApp.Name != "" {
					displayName = remoteApp.Name
				}
				if strings.TrimSpace(remoteApp.Description) != "" {
					summary = strings.TrimSpace(remoteApp.Description)
				}
				if remoteApp.RunnerConfig != nil {
					platform = string(remoteApp.RunnerConfig.AppRunnerType)
				}
			}
		}

		result = append(result, customerPortalAppResponse{
			ID:             app.ID,
			AppID:          app.AppID,
			DisplayName:    displayName,
			Status:         app.Status,
			Summary:        summary,
			Platform:       platform,
			LogoLight:      app.LogoLightBase64,
			LogoDark:       app.LogoDarkBase64,
			LegacyBasePath: "/bff/apps/" + app.AppID,
		})
	}

	return result
}

func (h *CustomerPortalHandler) nuonAPIURLForOrg(org *models.NuonOrg) string {
	if org != nil && org.APIURL != "" {
		return org.APIURL
	}
	return h.nuonAPIURL
}

func summarizeMarkdown(markdown string) string {
	trimmed := strings.TrimSpace(markdown)
	if trimmed == "" {
		return ""
	}

	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	trimmed = strings.ReplaceAll(trimmed, "#", "")
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	if len(trimmed) <= 180 {
		return trimmed
	}

	return strings.TrimSpace(trimmed[:177]) + "..."
}

func toPortalAccount(account *models.CustomerAccount) *customerPortalAccountResponse {
	if account == nil {
		return nil
	}

	return &customerPortalAccountResponse{ID: account.ID, Name: account.Name}
}

func toPortalAccounts(accounts []models.CustomerAccount) []customerPortalAccountResponse {
	if len(accounts) == 0 {
		return nil
	}

	result := make([]customerPortalAccountResponse, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, customerPortalAccountResponse{ID: account.ID, Name: account.Name})
	}

	return result
}
