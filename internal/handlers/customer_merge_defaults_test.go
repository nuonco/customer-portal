package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/internal/testutil"
	"github.com/nuonco/customer-portal/pkg/nuon"
)

func TestMergeDefaultInputs(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		// Set up vendor + org
		vendor := testutil.NewTestVendor()
		require.NoError(t, tx.Create(vendor).Error)

		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID})
		require.NoError(t, tx.Create(org).Error)

		appID := "test-app-123"

		// Create local AppInputConfig marking only "name" as customer-facing
		customerNames, _ := json.Marshal([]string{"name"})
		localConfig := &models.AppInputConfig{
			OrgID:              org.ID,
			AppID:              appID,
			CustomerInputNames: string(customerNames),
		}
		require.NoError(t, tx.Create(localConfig).Error)

		// Mock Nuon API returning full input config with defaults
		mockAPI := testutil.NewMockNuonAPIServer()
		defer mockAPI.Close()

		inputConfigResponse := map[string]interface{}{
			"input_groups": []interface{}{
				map[string]interface{}{
					"name": "Settings",
					"app_inputs": []interface{}{
						map[string]interface{}{
							"name":    "name",
							"default": "my-install",
						},
						map[string]interface{}{
							"name":    "cluster_version",
							"default": "1.28",
						},
						map[string]interface{}{
							"name":    "node_count",
							"default": "3",
						},
						map[string]interface{}{
							"name":    "optional_no_default",
							"default": "",
						},
					},
				},
			},
		}
		mockAPI.On("GET", "/v1/apps/"+appID+"/input-config", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(inputConfigResponse)
		})

		nuonClient, err := nuon.NewClientWithURL("test-token", "test-org-id", mockAPI.URL())
		require.NoError(t, err)

		handler := &Handler{db: tx}

		t.Run("fills defaults for non-customer-facing inputs", func(t *testing.T) {
			customerInputs := map[string]string{
				"name": "custom-name",
			}

			result := handler.mergeDefaultInputs(t.Context(), nuonClient, appID, org.ID, customerInputs)

			// Customer-provided value preserved
			assert.Equal(t, "custom-name", result["name"])
			// Non-customer-facing inputs get defaults
			assert.Equal(t, "1.28", result["cluster_version"])
			assert.Equal(t, "3", result["node_count"])
			// Empty default is not added
			assert.Empty(t, result["optional_no_default"])
		})

		t.Run("does not override explicitly provided inputs", func(t *testing.T) {
			inputs := map[string]string{
				"name":            "custom-name",
				"cluster_version": "1.30",
			}

			result := handler.mergeDefaultInputs(t.Context(), nuonClient, appID, org.ID, inputs)

			assert.Equal(t, "1.30", result["cluster_version"])
			assert.Equal(t, "3", result["node_count"])
		})

		t.Run("returns original inputs when API fails", func(t *testing.T) {
			badClient, err := nuon.NewClientWithURL("test-token", "test-org-id", "http://localhost:1")
			require.NoError(t, err)

			inputs := map[string]string{"name": "test"}
			result := handler.mergeDefaultInputs(t.Context(), badClient, appID, org.ID, inputs)

			assert.Equal(t, map[string]string{"name": "test"}, result)
		})

		t.Run("works with no local config", func(t *testing.T) {
			// Use a different app ID with no local config
			otherAppID := "other-app-456"
			mockAPI.On("GET", "/v1/apps/"+otherAppID+"/input-config", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(inputConfigResponse)
			})

			inputs := map[string]string{"name": "test"}
			result := handler.mergeDefaultInputs(t.Context(), nuonClient, otherAppID, org.ID, inputs)

			// With no local config, no inputs are customer-facing, so all defaults are filled
			assert.Equal(t, "test", result["name"]) // explicitly provided, not overridden
			assert.Equal(t, "1.28", result["cluster_version"])
			assert.Equal(t, "3", result["node_count"])
		})
	})
}
