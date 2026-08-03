package jsonhandlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

type inputFilterType string

const (
	inputFilterVendor   inputFilterType = "vendor"
	inputFilterCustomer inputFilterType = "customer"
)

func (h *VendorHandler) GetAppInputConfig(c *gin.Context) {
	appIDParam := c.Param("app_id")
	filterParam := c.Query("filter")

	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	app, err := nuonClient.GetApp(c.Request.Context(), appIDParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch app details: %v", err)})
		return
	}

	inputConfig, err := nuonClient.GetAppInputConfig(c.Request.Context(), appIDParam)
	if err != nil {
		inputConfig = nil
	}

	if filterParam == string(inputFilterVendor) || filterParam == string(inputFilterCustomer) {
		jsonBytes, err := json.Marshal(inputConfig)
		if err == nil {
			var configMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &configMap); err == nil {
				if filterParam == string(inputFilterVendor) {
					inputConfig = filterInputConfig(configMap, inputFilterVendor)

					var localConfig models.AppInputConfig
					h.db.Where("org_id = ? AND app_id = ?", org.ID, appIDParam).First(&localConfig)
					customerInputNames := localConfig.GetCustomerInputNames()
					if len(customerInputNames) > 0 {
						jsonBytes, err := json.Marshal(inputConfig)
						if err == nil {
							var vendorConfigMap map[string]interface{}
							if err := json.Unmarshal(jsonBytes, &vendorConfigMap); err == nil {
								inputConfig = filterInputConfigByLocalConfig(vendorConfigMap, customerInputNames, inputFilterVendor)
							}
						}
					}
				} else {
					inputConfig = filterInputConfig(configMap, inputFilterCustomer)
				}
			}
		}
	}

	platform := ""
	if app.RunnerConfig != nil {
		platform = string(app.RunnerConfig.AppRunnerType)
	}

	c.JSON(http.StatusOK, gin.H{
		"platform":     platform,
		"input_config": inputConfig,
	})
}

func filterInputConfig(inputConfig interface{}, filterType inputFilterType) interface{} {
	if inputConfig == nil {
		return nil
	}

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

			source, _ := inputMap["source"].(string)
			if filterType == inputFilterCustomer {
				if source == "customer" {
					filteredInputs = append(filteredInputs, input)
				}
			} else if source == "vendor" || source == "" {
				filteredInputs = append(filteredInputs, input)
			}
		}

		if len(filteredInputs) > 0 {
			filteredGroup := make(map[string]interface{})
			for key, value := range groupMap {
				filteredGroup[key] = value
			}
			filteredGroup["app_inputs"] = filteredInputs
			filteredGroups = append(filteredGroups, filteredGroup)
		}
	}

	result := make(map[string]interface{})
	for key, value := range configMap {
		result[key] = value
	}
	result["input_groups"] = filteredGroups
	return result
}

func filterInputConfigByLocalConfig(inputConfig interface{}, customerInputNames []string, filterType inputFilterType) interface{} {
	if inputConfig == nil {
		return nil
	}

	if filterType == inputFilterCustomer && customerInputNames == nil {
		return inputConfig
	}

	customerInputSet := make(map[string]bool)
	for _, name := range customerInputNames {
		customerInputSet[name] = true
	}

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

			inputName, _ := inputMap["name"].(string)
			isCustomerFacing := customerInputSet[inputName]
			if filterType == inputFilterCustomer {
				if isCustomerFacing {
					filteredInputs = append(filteredInputs, input)
				}
			} else if !isCustomerFacing {
				filteredInputs = append(filteredInputs, input)
			}
		}

		if len(filteredInputs) > 0 {
			filteredGroup := make(map[string]interface{})
			for key, value := range groupMap {
				filteredGroup[key] = value
			}
			filteredGroup["app_inputs"] = filteredInputs
			filteredGroups = append(filteredGroups, filteredGroup)
		}
	}

	result := make(map[string]interface{})
	for key, value := range configMap {
		result[key] = value
	}
	result["input_groups"] = filteredGroups
	return result
}
