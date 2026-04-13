package handlers

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/partials"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

// GetInstallFormFields returns HTML partials for the install form region selector and input fields.
// Detects install-link vs published-app mode from query params (sha or app_id).
func (h *Handler) GetInstallFormFields(c *gin.Context) {
	sha := c.Query("sha")
	appID := c.Query("app_id")

	if sha == "" && appID == "" {
		h.RenderTempl(c, http.StatusBadRequest, partials.InstallFormError("Missing sha or app_id parameter"))
		return
	}

	var config *installFormData
	var err error

	if sha != "" {
		config, err = h.getInstallLinkFormData(c, sha)
	} else {
		config, err = h.getPublishedAppFormData(c, appID)
	}

	if err != nil {
		h.RenderTempl(c, http.StatusOK, partials.InstallFormError(err.Error()))
		return
	}

	h.RenderTempl(c, http.StatusOK, partials.InstallFormFields(config.toTemplConfig()))
}

// installFormData holds the raw config data fetched from the Nuon API
type installFormData struct {
	platform        string
	inputConfig     interface{}
	collapsedGroups []string
}

func (d *installFormData) toTemplConfig() partials.InstallFormConfig {
	config := partials.InstallFormConfig{
		Platform:        d.platform,
		CollapsedGroups: make(map[string]bool),
	}
	for _, name := range d.collapsedGroups {
		config.CollapsedGroups[name] = true
	}
	config.InputGroups = parseInputGroups(d.inputConfig)
	return config
}

// getInstallLinkFormData fetches config data for an install link
func (h *Handler) getInstallLinkFormData(c *gin.Context, sha string) (*installFormData, error) {
	var link models.InstallLink
	if err := h.db.Preload("NuonOrg").Where("sha = ?", sha).First(&link).Error; err != nil {
		return nil, errNotFound("Install link not found")
	}
	if link.Used {
		return nil, errBadRequest("Install link already used")
	}

	return h.fetchAppFormData(c, link.NuonOrg.APIToken, link.NuonOrg.NuonOrgID, link.AppID, link.OrgID, h.nuonAPIURLForOrg(&link.NuonOrg))
}

// getPublishedAppFormData fetches config data for a published app
func (h *Handler) getPublishedAppFormData(c *gin.Context, appID string) (*installFormData, error) {
	org, err := h.getOrgForCustomerPage(c)
	if err != nil {
		return nil, errNotFound("Organization not found")
	}

	var publishedApp models.PublishedApp
	if err := h.db.Where("org_id = ? AND app_id = ? AND status IN ?", org.ID, appID, []string{models.AppStatusPublished, models.AppStatusComingSoon}).First(&publishedApp).Error; err != nil {
		return nil, errNotFound("App not found or not published")
	}

	return h.fetchAppFormData(c, org.APIToken, org.NuonOrgID, appID, org.ID, h.nuonAPIURLForOrg(org))
}

// fetchAppFormData is the shared logic for fetching and filtering app config
func (h *Handler) fetchAppFormData(c *gin.Context, apiToken, nuonOrgID, appID, orgID, apiURL string) (*installFormData, error) {
	nuonClient, err := nuon.NewClientWithURL(apiToken, nuonOrgID, apiURL)
	if err != nil {
		return nil, errInternal("Failed to initialize client")
	}

	app, err := nuonClient.GetApp(c.Request.Context(), appID)
	if err != nil {
		return nil, errInternal("Failed to fetch app details")
	}

	var platform string
	if app.RunnerConfig != nil {
		platform = string(app.RunnerConfig.AppRunnerType)
	}

	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), appID)
	if err != nil {
		inputConfig = nil
	}

	// Apply customer filtering and ordering
	var localConfig models.AppInputConfig
	configExists := h.db.Where("org_id = ? AND app_id = ?", orgID, appID).First(&localConfig).Error == nil
	customerInputNames := localConfig.GetCustomerInputNames()
	// When no config row exists, show all inputs to customers
	if !configExists {
		customerInputNames = nil
	}
	groupOrder := localConfig.GetGroupOrder()
	inputOrder := localConfig.GetInputOrder()

	jsonBytes, err := json.Marshal(inputConfig)
	if err == nil {
		var configMap map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
			inputConfig = filterInputConfigByLocalConfig(configMap, customerInputNames, FilterTypeCustomer)
			inputConfig = applyInputOrdering(inputConfig, groupOrder, inputOrder)
		}
	}

	return &installFormData{
		platform:        platform,
		inputConfig:     inputConfig,
		collapsedGroups: localConfig.GetCollapsedGroups(),
	}, nil
}

// parseInputGroups converts the generic interface{} input config into typed structs
func parseInputGroups(inputConfig interface{}) []partials.InstallFormInputGroup {
	if inputConfig == nil {
		return nil
	}

	configMap, ok := inputConfig.(map[string]interface{})
	if !ok {
		return nil
	}

	rawGroups, ok := configMap["input_groups"].([]interface{})
	if !ok {
		return nil
	}

	var groups []partials.InstallFormInputGroup
	for _, rawGroup := range rawGroups {
		groupMap, ok := rawGroup.(map[string]interface{})
		if !ok {
			continue
		}

		group := partials.InstallFormInputGroup{
			Name:        strVal(groupMap, "name"),
			DisplayName: strVal(groupMap, "display_name"),
			Description: strVal(groupMap, "description"),
		}

		rawInputs, ok := groupMap["app_inputs"].([]interface{})
		if !ok || len(rawInputs) == 0 {
			continue
		}

		for _, rawInput := range rawInputs {
			inputMap, ok := rawInput.(map[string]interface{})
			if !ok {
				continue
			}

			input := partials.InstallFormInput{
				Name:        strVal(inputMap, "name"),
				DisplayName: strVal(inputMap, "display_name"),
				Description: strVal(inputMap, "description"),
				Type:        strVal(inputMap, "type"),
				Default:     strVal(inputMap, "default"),
				Required:    boolVal(inputMap, "required"),
				Sensitive:   boolVal(inputMap, "sensitive"),
				Index:       intVal(inputMap, "index"),
			}
			group.AppInputs = append(group.AppInputs, input)
		}

		// Sort inputs by index
		sort.Slice(group.AppInputs, func(i, j int) bool {
			return group.AppInputs[i].Index < group.AppInputs[j].Index
		})

		groups = append(groups, group)
	}

	return groups
}

func intVal(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// Simple error types for form data fetching
type formError struct {
	message string
}

func (e *formError) Error() string { return e.message }

func errNotFound(msg string) error   { return &formError{message: msg} }
func errBadRequest(msg string) error { return &formError{message: msg} }
func errInternal(msg string) error   { return &formError{message: msg} }
