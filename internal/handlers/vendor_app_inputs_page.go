package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/assets"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui"
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func (h *Handler) AppInputsPage(c *gin.Context) {
	user := h.GetFreshUser(c)
	appID := c.Param("app_id")

	// Get org from context (validated by RequireOrgAccess middleware)
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		h.RenderErrorPage(c, http.StatusInternalServerError, "Organization context not found")
		return
	}
	orgID := org.ID

	// Initialize Nuon client
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		showCTA := nuon.IsUnauthorized(err)
		allOrgs := h.GetUserOrgs(user.ID)
		props := vendorpages.AppInputsPageProps{
			LayoutProps: vendorui.LayoutProps{
				Title:                "App - Inputs",
				ActivePage:           "apps",
				User:                 user,
				CurrentOrg:           org,
				Orgs:                 allOrgs,
				Breadcrumbs:          []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: false}, {Text: appID, Active: true}},
				BasePath:             h.basePath,
				PortalScheme:         h.schemeFromBaseURL(),
				DashboardURL:         h.dashboardURL,
				PortalBaseDomain:     h.subdomainBaseDomain,
				CSSPath:              assets.VendorCSSPath(),
				IsSuperuser:          h.isSuperuser(user),
				NuonAPIError:         err.Error(),
				NuonAPIShowUpdateCTA: showCTA,
			},
			Org:   *org,
			AppID: appID,
			App:   vendorpages.AppInfo{ID: appID},
		}
		h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
		h.RenderTempl(c, http.StatusOK, vendorpages.AppInputsPage(props))
		return
	}

	// Fetch app details from Nuon API
	app, err := nuonClient.GetApp(c.Request.Context(), appID)
	if err != nil {
		showCTA := nuon.IsUnauthorized(err)
		allOrgs := h.GetUserOrgs(user.ID)
		props := vendorpages.AppInputsPageProps{
			LayoutProps: vendorui.LayoutProps{
				Title:                "App - Inputs",
				ActivePage:           "apps",
				User:                 user,
				CurrentOrg:           org,
				Orgs:                 allOrgs,
				Breadcrumbs:          []partials.Breadcrumb{{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: false}, {Text: appID, Active: true}},
				BasePath:             h.basePath,
				PortalScheme:         h.schemeFromBaseURL(),
				DashboardURL:         h.dashboardURL,
				PortalBaseDomain:     h.subdomainBaseDomain,
				CSSPath:              assets.VendorCSSPath(),
				IsSuperuser:          h.isSuperuser(user),
				NuonAPIError:         err.Error(),
				NuonAPIShowUpdateCTA: showCTA,
			},
			Org:   *org,
			AppID: appID,
			App:   vendorpages.AppInfo{ID: appID},
		}
		h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
		h.RenderTempl(c, http.StatusOK, vendorpages.AppInputsPage(props))
		return
	}

	// Fetch app input config from Nuon API
	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), appID)
	if err != nil {
		// Input config may not exist, that's okay - we'll show empty tables
		inputConfig = nil
	}

	// Fetch local customer input config
	var localConfig models.AppInputConfig
	h.db.Where("org_id = ? AND app_id = ?", orgID, appID).First(&localConfig)
	customerInputNames := localConfig.GetCustomerInputNames()

	// Create a set for fast lookup
	customerInputSet := make(map[string]bool)
	for _, name := range customerInputNames {
		customerInputSet[name] = true
	}

	// Create a set for collapsed groups
	collapsedGroups := localConfig.GetCollapsedGroups()
	collapsedGroupSet := make(map[string]bool)
	for _, name := range collapsedGroups {
		collapsedGroupSet[name] = true
	}

	// Parse inputs into grouped structure
	var inputGroups []vendorpages.AppInputGroup

	if inputConfig != nil {
		// Convert the typed struct to JSON then back to map for flexible field access
		// This handles the case where user_configurable might exist in the API response
		// but not in the SDK struct
		jsonBytes, err := json.Marshal(inputConfig)
		if err == nil {
			var configMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
				if apiGroups, ok := configMap["input_groups"].([]interface{}); ok {
					for _, group := range apiGroups {
						if groupMap, ok := group.(map[string]interface{}); ok {
							groupName, _ := groupMap["name"].(string)
							groupDisplayName, _ := groupMap["display_name"].(string)
							if groupDisplayName == "" {
								groupDisplayName = groupName
							}

							inputGroup := vendorpages.AppInputGroup{
								Name:             groupName,
								DisplayName:      groupDisplayName,
								Inputs:           []vendorpages.AppInputInfo{},
								CollapsedDefault: collapsedGroupSet[groupName],
							}

							if appInputs, ok := groupMap["app_inputs"].([]interface{}); ok {
								for _, input := range appInputs {
									if inputMap, ok := input.(map[string]interface{}); ok {
										inputName := getMapString(inputMap, "name")
										inputInfo := vendorpages.AppInputInfo{
											Name:           inputName,
											DisplayName:    getMapString(inputMap, "display_name"),
											Description:    getMapString(inputMap, "description"),
											Type:           getMapString(inputMap, "type"),
											Required:       getMapBool(inputMap, "required"),
											Sensitive:      getMapBool(inputMap, "sensitive"),
											Default:        getStringFromAny(inputMap["default"]),
											Source:         getMapString(inputMap, "source"),
											CustomerFacing: customerInputSet[inputName],
										}

										if inputInfo.DisplayName == "" {
											inputInfo.DisplayName = inputInfo.Name
										}
										if inputInfo.Type == "" {
											inputInfo.Type = "string"
										}

										inputGroup.Inputs = append(inputGroup.Inputs, inputInfo)
									}
								}
							}

							inputGroups = append(inputGroups, inputGroup)
						}
					}
				}
			}
		}
	}

	// Apply saved ordering
	groupOrder := localConfig.GetGroupOrder()
	inputOrder := localConfig.GetInputOrder()

	// Sort groups if ordering is saved
	if len(groupOrder) > 0 {
		groupOrderMap := make(map[string]int)
		for i, name := range groupOrder {
			groupOrderMap[name] = i
		}
		sort.SliceStable(inputGroups, func(i, j int) bool {
			orderI, okI := groupOrderMap[inputGroups[i].Name]
			orderJ, okJ := groupOrderMap[inputGroups[j].Name]
			if !okI && !okJ {
				return false // Keep original order for unordered items
			}
			if !okI {
				return false // Unordered items go after ordered ones
			}
			if !okJ {
				return true // Ordered items go before unordered ones
			}
			return orderI < orderJ
		})
	}

	// Sort inputs within each group if ordering is saved
	for i := range inputGroups {
		groupName := inputGroups[i].Name
		if order, ok := inputOrder[groupName]; ok && len(order) > 0 {
			inputOrderMap := make(map[string]int)
			for j, name := range order {
				inputOrderMap[name] = j
			}
			sort.SliceStable(inputGroups[i].Inputs, func(a, b int) bool {
				orderA, okA := inputOrderMap[inputGroups[i].Inputs[a].Name]
				orderB, okB := inputOrderMap[inputGroups[i].Inputs[b].Name]
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
		}
	}

	// Get user's orgs for sidebar dropdown
	allOrgs := h.GetUserOrgs(user.ID)

	props := vendorpages.AppInputsPageProps{
		LayoutProps: vendorui.LayoutProps{
			Title:      app.Name + " - Inputs",
			ActivePage: "apps",
			User:       user,
			CurrentOrg: org,
			Orgs:       allOrgs,
			Breadcrumbs: []partials.Breadcrumb{
				{Text: org.Name, Path: fmt.Sprintf("%s/orgs/%s", h.basePath, org.ID), Active: false},
				{Text: "Apps", Path: fmt.Sprintf("%s/orgs/%s/apps", h.basePath, org.ID), Active: false},
				{Text: app.Name, Path: fmt.Sprintf("%s/orgs/%s/apps/%s", h.basePath, org.ID, appID), Active: false},
				{Text: "Inputs", Path: "", Active: true},
			},
			BasePath:         h.basePath,
			PortalScheme:     h.schemeFromBaseURL(),
			DashboardURL:     h.dashboardURL,
			PortalBaseDomain: h.subdomainBaseDomain,
			CSSPath:          assets.VendorCSSPath(),
			IsSuperuser:      h.isSuperuser(user),
		},
		Org:   *org,
		AppID: appID,
		App: vendorpages.AppInfo{
			ID:   app.ID,
			Name: app.Name,
		},
		InputGroups: inputGroups,
	}

	h.enrichLayoutWithOrgStatus(c.Request.Context(), &props.LayoutProps)
	h.RenderTempl(c, http.StatusOK, vendorpages.AppInputsPage(props))
}

// getMapString safely gets a string from a map[string]interface{}

func getMapString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getMapBool safely gets a bool from a map[string]interface{}

func getMapBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// getStringFromAny converts various types to string

func getStringFromAny(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// AppLogoPage displays the app logo upload page
