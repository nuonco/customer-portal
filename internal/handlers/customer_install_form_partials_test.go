package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseInputGroups(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := parseInputGroups(nil)
		assert.Nil(t, result)
	})

	t.Run("empty config", func(t *testing.T) {
		result := parseInputGroups(map[string]interface{}{})
		assert.Nil(t, result)
	})

	t.Run("config with groups and inputs", func(t *testing.T) {
		config := map[string]interface{}{
			"input_groups": []interface{}{
				map[string]interface{}{
					"name":         "network",
					"display_name": "Network Settings",
					"description":  "Configure networking",
					"app_inputs": []interface{}{
						map[string]interface{}{
							"name":         "vpc_cidr",
							"display_name": "VPC CIDR",
							"description":  "CIDR block for the VPC",
							"type":         "text",
							"default":      "10.0.0.0/16",
							"required":     true,
							"sensitive":    false,
							"index":        float64(0),
						},
						map[string]interface{}{
							"name":         "enable_nat",
							"display_name": "Enable NAT",
							"type":         "bool",
							"default":      "true",
							"required":     false,
							"index":        float64(1),
						},
					},
				},
				map[string]interface{}{
					"name":       "empty_group",
					"app_inputs": []interface{}{},
				},
			},
		}

		result := parseInputGroups(config)
		assert.Len(t, result, 1) // empty group should be skipped
		assert.Equal(t, "network", result[0].Name)
		assert.Equal(t, "Network Settings", result[0].DisplayName)
		assert.Len(t, result[0].AppInputs, 2)
		assert.Equal(t, "vpc_cidr", result[0].AppInputs[0].Name)
		assert.True(t, result[0].AppInputs[0].Required)
		assert.Equal(t, "10.0.0.0/16", result[0].AppInputs[0].Default)
		assert.Equal(t, "enable_nat", result[0].AppInputs[1].Name)
	})

	t.Run("inputs sorted by index", func(t *testing.T) {
		config := map[string]interface{}{
			"input_groups": []interface{}{
				map[string]interface{}{
					"name": "test",
					"app_inputs": []interface{}{
						map[string]interface{}{"name": "b", "index": float64(2)},
						map[string]interface{}{"name": "a", "index": float64(0)},
						map[string]interface{}{"name": "c", "index": float64(1)},
					},
				},
			},
		}

		result := parseInputGroups(config)
		assert.Equal(t, "a", result[0].AppInputs[0].Name)
		assert.Equal(t, "c", result[0].AppInputs[1].Name)
		assert.Equal(t, "b", result[0].AppInputs[2].Name)
	})
}

func TestInstallFormDataToTemplConfig(t *testing.T) {
	data := &installFormData{
		platform: "aws_eks",
		inputConfig: map[string]interface{}{
			"input_groups": []interface{}{
				map[string]interface{}{
					"name": "general",
					"app_inputs": []interface{}{
						map[string]interface{}{"name": "region", "type": "text"},
					},
				},
			},
		},
		collapsedGroups: []string{"advanced"},
	}

	config := data.toTemplConfig()
	assert.Equal(t, "aws_eks", config.Platform)
	assert.True(t, config.CollapsedGroups["advanced"])
	assert.False(t, config.CollapsedGroups["general"])
	assert.Len(t, config.InputGroups, 1)
}

func TestIntVal(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]interface{}
		key      string
		expected int
	}{
		{"float64 value", map[string]interface{}{"x": float64(5)}, "x", 5},
		{"int value", map[string]interface{}{"x": 3}, "x", 3},
		{"missing key", map[string]interface{}{}, "x", 0},
		{"nil value", map[string]interface{}{"x": nil}, "x", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, intVal(tt.m, tt.key))
		})
	}
}
