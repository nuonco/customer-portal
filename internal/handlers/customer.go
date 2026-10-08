package handlers

import (
	"context"
	"encoding/json"
	"sort"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/assets"
	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/internal/views/customerui"
	"github.com/nuonco/customer-portal/pkg/nuon"
)

type FilterType string

const (
	FilterTypeVendor   FilterType = "vendor"
	FilterTypeCustomer FilterType = "customer"
)

// filterInputConfig filters the input config based on source field
// Vendor sees inputs where source == "vendor" (or not set for backward compatibility)
// Customer sees inputs where source == "customer"

func filterInputConfig(inputConfig interface{}, filterType FilterType) interface{} {
	if inputConfig == nil {
		return nil
	}

	// The input config is a map with input_groups
	configMap, ok := inputConfig.(map[string]interface{})
	if !ok {
		return inputConfig
	}

	inputGroups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return inputConfig
	}

	var filteredGroups []interface{}
	for _, group := range inputGroups {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}

		appInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok {
			continue
		}

		var filteredInputs []interface{}
		for _, input := range appInputs {
			inputMap, ok := input.(map[string]interface{})
			if !ok {
				continue
			}

			// Check source field (values: "vendor" or "customer")
			source, _ := inputMap["source"].(string)

			if filterType == FilterTypeCustomer {
				// Customer sees only source="customer"
				if source == "customer" {
					filteredInputs = append(filteredInputs, input)
				}
			} else {
				// Vendor sees source="vendor" or not set (backward compatibility)
				if source == "vendor" || source == "" {
					filteredInputs = append(filteredInputs, input)
				}
			}
		}

		// Only include groups that have inputs after filtering
		if len(filteredInputs) > 0 {
			filteredGroup := make(map[string]interface{})
			for k, v := range groupMap {
				filteredGroup[k] = v
			}
			filteredGroup["app_inputs"] = filteredInputs
			filteredGroups = append(filteredGroups, filteredGroup)
		}
	}

	// Return the filtered config
	result := make(map[string]interface{})
	for k, v := range configMap {
		result[k] = v
	}
	result["input_groups"] = filteredGroups
	return result
}

// filterInputConfigByLocalConfig filters the input config using the local customer input names
// For customer filtering: only include inputs that are in customerInputNames
// For vendor filtering: only include inputs that are NOT in customerInputNames

func filterInputConfigByLocalConfig(inputConfig interface{}, customerInputNames []string, filterType FilterType) interface{} {
	if inputConfig == nil {
		return nil
	}

	// When customerInputNames is nil, no config row exists — show all inputs.
	// When it's an empty slice, vendor explicitly configured zero customer inputs — show none.
	if filterType == FilterTypeCustomer && customerInputNames == nil {
		return inputConfig
	}

	// Create a set for fast lookup
	customerInputSet := make(map[string]bool)
	for _, name := range customerInputNames {
		customerInputSet[name] = true
	}

	// The input config is a map with input_groups
	configMap, ok := inputConfig.(map[string]interface{})
	if !ok {
		return inputConfig
	}

	inputGroups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return inputConfig
	}

	var filteredGroups []interface{}
	for _, group := range inputGroups {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}

		appInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok {
			continue
		}

		var filteredInputs []interface{}
		for _, input := range appInputs {
			inputMap, ok := input.(map[string]interface{})
			if !ok {
				continue
			}

			// Get input name
			inputName, _ := inputMap["name"].(string)
			isCustomerFacing := customerInputSet[inputName]

			if filterType == FilterTypeCustomer {
				// Customer sees only inputs in customerInputNames
				if isCustomerFacing {
					filteredInputs = append(filteredInputs, input)
				}
			} else {
				// Vendor sees inputs NOT in customerInputNames
				if !isCustomerFacing {
					filteredInputs = append(filteredInputs, input)
				}
			}
		}

		// Only include groups that have inputs after filtering
		if len(filteredInputs) > 0 {
			filteredGroup := make(map[string]interface{})
			for k, v := range groupMap {
				filteredGroup[k] = v
			}
			filteredGroup["app_inputs"] = filteredInputs
			filteredGroups = append(filteredGroups, filteredGroup)
		}
	}

	// Return the filtered config
	result := make(map[string]interface{})
	for k, v := range configMap {
		result[k] = v
	}
	result["input_groups"] = filteredGroups
	return result
}

// mergeDefaultInputs fetches the full app input config from the Nuon API and
// fills in default values for any non-customer-facing inputs that are missing
// from the provided inputs map. This ensures required inputs with defaults
// (e.g. cluster_version) are sent to the API even though the customer never
// sees them in the form.
func (h *Handler) mergeDefaultInputs(ctx context.Context, nuonClient *nuon.Client, appID string, orgID string, inputs map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range inputs {
		result[k] = v
	}

	inputConfig, err := nuonClient.GetAppInputConfig(ctx, appID)
	if err != nil || inputConfig == nil {
		return result
	}

	// Determine which inputs are customer-facing via local config
	customerInputSet := make(map[string]bool)
	var localConfig models.AppInputConfig
	if err := h.db.Where("org_id = ? AND app_id = ?", orgID, appID).First(&localConfig).Error; err == nil {
		for _, name := range localConfig.GetCustomerInputNames() {
			customerInputSet[name] = true
		}
	}

	// Walk the config structure to extract defaults for non-customer-facing inputs
	jsonBytes, err := json.Marshal(inputConfig)
	if err != nil {
		return result
	}
	var configMap map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &configMap); err != nil {
		return result
	}

	inputGroups, _ := configMap["input_groups"].([]interface{})
	for _, group := range inputGroups {
		groupMap, _ := group.(map[string]interface{})
		appInputs, _ := groupMap["app_inputs"].([]interface{})
		for _, input := range appInputs {
			inputMap, _ := input.(map[string]interface{})
			name, _ := inputMap["name"].(string)
			defaultVal, _ := inputMap["default"].(string)
			if name == "" || defaultVal == "" {
				continue
			}
			// Only fill in defaults for non-customer-facing inputs not already provided
			if !customerInputSet[name] && result[name] == "" {
				result[name] = defaultVal
			}
		}
	}

	return result
}

// strVal extracts a string value from a map.

func strVal(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

// boolVal extracts a bool value from a map.

func boolVal(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}

// filterInputsByName removes inputs whose names are in the exclude set from
// input_groups[].app_inputs[]. Returns the filtered config.
func filterInputsByName(configMap map[string]interface{}, exclude map[string]bool) interface{} {
	inputGroups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return configMap
	}

	var filteredGroups []interface{}
	for _, group := range inputGroups {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}

		appInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok {
			continue
		}

		var filteredInputs []interface{}
		for _, input := range appInputs {
			inputMap, ok := input.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := inputMap["name"].(string)
			if !exclude[name] {
				filteredInputs = append(filteredInputs, input)
			}
		}

		if len(filteredInputs) > 0 {
			filteredGroup := make(map[string]interface{})
			for k, v := range groupMap {
				filteredGroup[k] = v
			}
			filteredGroup["app_inputs"] = filteredInputs
			filteredGroups = append(filteredGroups, filteredGroup)
		}
	}

	result := make(map[string]interface{})
	for k, v := range configMap {
		result[k] = v
	}
	result["input_groups"] = filteredGroups
	return result
}

// filterUserConfigurableInputs removes inputs with user_configurable=true from
// the input config. These inputs are managed by the install stack and cannot be
// updated via the API after install creation. Returns the filtered config and
// a list of removed input names (so callers can also strip them from values maps).
func filterUserConfigurableInputs(inputConfig interface{}) (interface{}, []string) {
	if inputConfig == nil {
		return nil, nil
	}

	configMap, ok := inputConfig.(map[string]interface{})
	if !ok {
		return inputConfig, nil
	}

	inputGroups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return inputConfig, nil
	}

	var removed []string
	var filteredGroups []interface{}
	for _, group := range inputGroups {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}

		appInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok {
			continue
		}

		var filteredInputs []interface{}
		for _, input := range appInputs {
			inputMap, ok := input.(map[string]interface{})
			if !ok {
				continue
			}

			userConfigurable, _ := inputMap["user_configurable"].(bool)
			if userConfigurable {
				if name, _ := inputMap["name"].(string); name != "" {
					removed = append(removed, name)
				}
				continue
			}
			filteredInputs = append(filteredInputs, input)
		}

		if len(filteredInputs) > 0 {
			filteredGroup := make(map[string]interface{})
			for k, v := range groupMap {
				filteredGroup[k] = v
			}
			filteredGroup["app_inputs"] = filteredInputs
			filteredGroups = append(filteredGroups, filteredGroup)
		}
	}

	result := make(map[string]interface{})
	for k, v := range configMap {
		result[k] = v
	}
	result["input_groups"] = filteredGroups
	return result, removed
}

// extractBracketInputs parses flat "inputs[name]" keys from a raw JSON map into
// a nested map. HTMX's json-enc extension serialises form fields with bracket
// notation (e.g. "inputs[foo]": "bar") as flat keys instead of nested objects.
// This helper merges those into the given inputs map so handlers receive the
// expected map[string]string.
func extractBracketInputs(raw map[string]interface{}, existing map[string]string) map[string]string {
	if existing == nil {
		existing = make(map[string]string)
	}
	for key, val := range raw {
		if len(key) > 7 && key[:7] == "inputs[" && key[len(key)-1] == ']' {
			name := key[7 : len(key)-1]
			switch v := val.(type) {
			case string:
				existing[name] = v
			case []interface{}:
				// json-enc produces an array for duplicate field names
				// (e.g. hidden "false" + checkbox "true"). Use the last value,
				// matching standard HTML form behavior.
				if len(v) > 0 {
					if s, ok := v[len(v)-1].(string); ok {
						existing[name] = s
					}
				}
			}
		}
	}
	return existing
}

// applyInputOrdering sorts input groups and inputs within them based on saved ordering

func applyInputOrdering(inputConfig interface{}, groupOrder []string, inputOrder map[string][]string) interface{} {
	if inputConfig == nil {
		return nil
	}

	// If no ordering is specified, return as-is
	if len(groupOrder) == 0 && len(inputOrder) == 0 {
		return inputConfig
	}

	configMap, ok := inputConfig.(map[string]interface{})
	if !ok {
		return inputConfig
	}

	inputGroups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return inputConfig
	}

	// Sort groups if ordering is specified
	if len(groupOrder) > 0 {
		groupOrderMap := make(map[string]int)
		for i, name := range groupOrder {
			groupOrderMap[name] = i
		}
		sort.SliceStable(inputGroups, func(i, j int) bool {
			groupI, _ := inputGroups[i].(map[string]interface{})
			groupJ, _ := inputGroups[j].(map[string]interface{})
			nameI, _ := groupI["name"].(string)
			nameJ, _ := groupJ["name"].(string)
			orderI, okI := groupOrderMap[nameI]
			orderJ, okJ := groupOrderMap[nameJ]
			if !okI && !okJ {
				return false
			}
			if !okI {
				return false
			}
			if !okJ {
				return true
			}
			return orderI < orderJ
		})
	}

	// Sort inputs within each group if ordering is specified
	for _, group := range inputGroups {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}
		groupName, _ := groupMap["name"].(string)
		order, hasOrder := inputOrder[groupName]
		if !hasOrder || len(order) == 0 {
			continue
		}

		appInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok {
			continue
		}

		inputOrderMap := make(map[string]int)
		for i, name := range order {
			inputOrderMap[name] = i
		}
		sort.SliceStable(appInputs, func(a, b int) bool {
			inputA, _ := appInputs[a].(map[string]interface{})
			inputB, _ := appInputs[b].(map[string]interface{})
			nameA, _ := inputA["name"].(string)
			nameB, _ := inputB["name"].(string)
			orderA, okA := inputOrderMap[nameA]
			orderB, okB := inputOrderMap[nameB]
			if !okA && !okB {
				return false
			}
			if !okA {
				return false
			}
			if !okB {
				return true
			}
			return orderA < orderB
		})
		groupMap["app_inputs"] = appInputs
	}

	// Return with sorted groups
	result := make(map[string]interface{})
	for k, v := range configMap {
		result[k] = v
	}
	result["input_groups"] = inputGroups
	return result
}

// buildCustomerLayoutProps builds the layout props for customer pages

func (h *Handler) buildCustomerLayoutProps(title string, user *models.User, theme *models.AppTheme, org *models.NuonOrg, activeAccount *models.CustomerAccount, otherAccounts []models.CustomerAccount) customerui.LayoutProps {
	primaryColor, primaryColorDark := GetPrimaryColors(theme.PrimaryColor)

	orgName := ""
	portalDomain := ""
	if org != nil {
		orgName = org.Name
		if org.Subdomain != "" {
			portalDomain = org.Subdomain + "." + h.subdomainBaseDomain
		}
	}

	adminURL := h.customerBaseURL + "/admin/orgs"
	if org != nil {
		adminURL = h.customerBaseURL + "/admin/orgs/" + org.ID
	}

	headerTitle := ""
	if !theme.HeaderTitleHidden {
		headerTitle = theme.GetHeaderTitle(orgName)
	}

	// Load installs for sidebar switcher
	sidebarInstalls := h.loadSidebarInstalls(user, activeAccount)

	return customerui.LayoutProps{
		Title:                     title,
		User:                      user,
		BasePath:                  h.basePath,
		PrimaryColor:              primaryColor,
		PrimaryColorDark:          primaryColorDark,
		PrimaryColorDarkMode:      theme.PrimaryColorDark,
		PrimaryColorDarkModeHover: DarkenColor(theme.PrimaryColorDark, 0.85),
		WhiteColor:                theme.WhiteColor,
		BlackColor:                theme.BlackColor,
		WhiteColorDark:            theme.WhiteColorDark,
		BlackColorLight:           theme.BlackColorLight,
		HeadingFont:               theme.HeadingFont,
		BodyFont:                  theme.BodyFont,
		HeadingFontBase64:         theme.HeadingFontBase64,
		BodyFontBase64:            theme.BodyFontBase64,
		LogoBase64:                theme.LogoLightBase64,
		LogoDarkBase64:            theme.LogoDarkBase64,
		FaviconBase64:             theme.FaviconBase64,
		RadiusClass:               theme.GetRadiusClass(),
		ThemeMode:                 theme.GetThemeMode(),
		HeaderTitle:               headerTitle,
		CSSPath:                   assets.CustomerCSSPath(),
		CustomCSSPath:             h.getCustomCSSPath(theme.OrgID),
		OrgName:                   orgName,
		PortalDomain:              portalDomain,
		AdminURL:                  adminURL,
		Installs:                  sidebarInstalls,
		ActiveAccount:             activeAccount,
		OtherAccounts:             otherAccounts,
	}
}

// loadSidebarInstalls loads all installs visible to the user for the sidebar switcher.
func (h *Handler) loadSidebarInstalls(user *models.User, activeAccount *models.CustomerAccount) []models.SidebarInstall {
	if user == nil || activeAccount == nil {
		return nil
	}

	var installs []models.Install
	err := h.db.
		Where(
			"(user_id = ? AND customer_account_id = ?) OR (customer_account_id = ? AND visibility = ?)",
			user.ID, activeAccount.ID, activeAccount.ID, models.VisibilityAccount,
		).
		Order("created_at DESC").
		Find(&installs).Error
	if err != nil {
		return nil
	}

	// Collect unique app IDs to fetch logos from PublishedApp
	appIDs := make(map[string]bool)
	for _, inst := range installs {
		appID := inst.GetAppID()
		if appID != "" {
			appIDs[appID] = true
		}
	}

	// Fetch app logos from PublishedApp records
	type appLogo struct {
		Light string
		Dark  string
	}
	logoMap := make(map[string]appLogo)
	if len(appIDs) > 0 {
		ids := make([]string, 0, len(appIDs))
		for id := range appIDs {
			ids = append(ids, id)
		}
		var publishedApps []models.PublishedApp
		h.db.Where("app_id IN ?", ids).Find(&publishedApps)
		for _, pa := range publishedApps {
			logoMap[pa.AppID] = appLogo{Light: pa.LogoLightBase64, Dark: pa.LogoDarkBase64}
		}
	}

	result := make([]models.SidebarInstall, 0, len(installs))
	for _, inst := range installs {
		appID := inst.GetAppID()
		logo := logoMap[appID]
		name := inst.Name
		if name == "" {
			name = inst.NuonInstallID
		}
		appName := inst.AppName
		result = append(result, models.SidebarInstall{
			ID:           inst.ID,
			Name:         name,
			AppName:      appName,
			AppLogoLight: logo.Light,
			AppLogoDark:  logo.Dark,
		})
	}

	return result
}

// GetInstallLinkAppConfig returns the app configuration for a customer install link
// This allows the customer-facing page to fetch input config without authentication
// Accepts optional ?filter=vendor|customer query parameter to filter inputs:
// - filter=vendor: returns inputs where user_configurable != true
// - filter=customer: returns inputs where user_configurable == true
// - no filter: returns all inputs (backward compatibility)

func (h *Handler) tryGetLoggedInUser(c *gin.Context) *models.User {
	// Try to parse token from the request (cookie, header, or query)
	token, err := h.auth.ParseToken(c)
	if err != nil || token == nil || !token.Valid {
		return nil
	}

	// Extract claims from the token
	claims := jwt.ExtractClaimsFromToken(token)
	if claims == nil {
		return nil
	}

	// Validate required claims exist
	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return nil
	}

	email, ok := claims["email"].(string)
	if !ok || email == "" {
		return nil
	}

	// Look up the user in the database to ensure they still exist
	var user models.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil
	}

	return &user
}

// InstallLinkPage renders the install link acceptance page

// UpdateInstall handles customer install updates

func (h *Handler) loadInstallWithOrg(install *models.Install) error {
	return h.db.Preload("InstallLink.NuonOrg").Preload("Org").
		Where("id = ?", install.ID).First(install).Error
}

// GetInstallInputs returns the current inputs and input config for an install
