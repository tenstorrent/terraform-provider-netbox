package netbox

import (
	"encoding/json"
	"testing"
)

func TestFlattenCustomFields(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected map[string]interface{}
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: nil,
		},
		{
			name: "simple string value",
			input: map[string]interface{}{
				"field1": "value1",
			},
			expected: map[string]interface{}{
				"field1": "value1",
			},
		},
		{
			name: "numeric values",
			input: map[string]interface{}{
				"int_field":   42,
				"int64_field": int64(100),
				"float_field": 3.14,
			},
			expected: map[string]interface{}{
				"int_field":   "42",
				"int64_field": "100",
				"float_field": "3.14",
			},
		},
		{
			name: "boolean value",
			input: map[string]interface{}{
				"bool_field": true,
			},
			expected: map[string]interface{}{
				"bool_field": "true",
			},
		},
		{
			name: "null value",
			input: map[string]interface{}{
				"null_field": nil,
			},
			expected: map[string]interface{}{
				"null_field": "",
			},
		},
		{
			name: "complex nested object (IP address reference)",
			input: map[string]interface{}{
				"gateway": map[string]interface{}{
					"id":      9,
					"address": "10.21.10.254/24",
					"display": "10.21.10.254/24",
					"family": map[string]interface{}{
						"value": 4,
						"label": "IPv4",
					},
					"url": "https://netbox.example.com/api/ipam/ip-addresses/9/",
				},
			},
			expected: map[string]interface{}{
				"gateway": `{"address":"10.21.10.254/24","display":"10.21.10.254/24","family":{"label":"IPv4","value":4},"id":9,"url":"https://netbox.example.com/api/ipam/ip-addresses/9/"}`,
			},
		},
		{
			name: "array value",
			input: map[string]interface{}{
				"tags": []string{"tag1", "tag2", "tag3"},
			},
			expected: map[string]interface{}{
				"tags": `["tag1","tag2","tag3"]`,
			},
		},
		{
			name: "mixed types",
			input: map[string]interface{}{
				"text_field":   "simple text",
				"number_field": 123,
				"bool_field":   false,
				"null_field":   nil,
				"object_field": map[string]interface{}{
					"nested": "value",
				},
			},
			expected: map[string]interface{}{
				"text_field":   "simple text",
				"number_field": "123",
				"bool_field":   "false",
				"null_field":   "",
				"object_field": `{"nested":"value"}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := flattenCustomFields(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("expected non-nil result, got nil")
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d fields, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				actualValue, ok := result[key]
				if !ok {
					t.Errorf("expected field %q not found in result", key)
					continue
				}

				// For JSON strings, compare the parsed objects to avoid formatting differences
				expectedStr, expectedIsStr := expectedValue.(string)
				actualStr, actualIsStr := actualValue.(string)

				if expectedIsStr && actualIsStr {
					// Try to parse as JSON
					var expectedJSON, actualJSON interface{}
					expectedIsJSON := json.Unmarshal([]byte(expectedStr), &expectedJSON) == nil
					actualIsJSON := json.Unmarshal([]byte(actualStr), &actualJSON) == nil

					if expectedIsJSON && actualIsJSON {
						// Compare as JSON objects
						expectedJSONStr, _ := json.Marshal(expectedJSON)
						actualJSONStr, _ := json.Marshal(actualJSON)
						if string(expectedJSONStr) != string(actualJSONStr) {
							t.Errorf("field %q: expected JSON %q, got %q", key, expectedStr, actualStr)
						}
						continue
					}
				}

				// Compare as strings
				if actualValue != expectedValue {
					t.Errorf("field %q: expected %q, got %q", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestFlattenCustomFields_ComplexRealWorldExample(t *testing.T) {
	// Simulate a real NetBox custom field response with an IP address object reference
	input := map[string]interface{}{
		"gateway": map[string]interface{}{
			"id":          9,
			"url":         "https://netbox.zonda.systems/api/ipam/ip-addresses/9/",
			"display":     "10.21.10.254/24",
			"address":     "10.21.10.254/24",
			"description": "",
			"family": map[string]interface{}{
				"value": float64(4),
				"label": "IPv4",
			},
		},
		"vlan_purpose": "management",
		"monitoring":   true,
		"priority":     10,
	}

	result := flattenCustomFields(input)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Check that gateway is a JSON string
	gateway, ok := result["gateway"].(string)
	if !ok {
		t.Errorf("expected gateway to be a string, got %T", result["gateway"])
	}

	// Verify we can parse the gateway JSON
	var gatewayObj map[string]interface{}
	if err := json.Unmarshal([]byte(gateway), &gatewayObj); err != nil {
		t.Errorf("failed to parse gateway JSON: %v", err)
	}

	// Verify the gateway object has expected fields
	if gatewayObj["address"] != "10.21.10.254/24" {
		t.Errorf("expected address 10.21.10.254/24, got %v", gatewayObj["address"])
	}

	// Check simple fields
	if result["vlan_purpose"] != "management" {
		t.Errorf("expected vlan_purpose=management, got %v", result["vlan_purpose"])
	}

	if result["monitoring"] != "true" {
		t.Errorf("expected monitoring=true, got %v", result["monitoring"])
	}

	if result["priority"] != "10" {
		t.Errorf("expected priority=10, got %v", result["priority"])
	}
}

func TestCoerceJSONStringValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "JSON object string is coerced to map",
			input:    `{"cores":72,"memory_gb":128}`,
			expected: map[string]interface{}{"cores": float64(72), "memory_gb": float64(128)},
		},
		{
			name:     "JSON array string is coerced to slice",
			input:    `["a","b","c"]`,
			expected: []interface{}{"a", "b", "c"},
		},
		{
			name:     "plain string stays unchanged",
			input:    "active",
			expected: "active",
		},
		{
			name:     "JSON number string stays unchanged (not object/array)",
			input:    "42",
			expected: "42",
		},
		{
			name:     "JSON bool string stays unchanged",
			input:    "true",
			expected: "true",
		},
		{
			name:     "invalid JSON string stays unchanged",
			input:    "{not valid json",
			expected: "{not valid json",
		},
		{
			name:     "non-string value passes through unchanged",
			input:    map[string]interface{}{"already": "a map"},
			expected: map[string]interface{}{"already": "a map"},
		},
		{
			name:     "nil passes through unchanged",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coerceJSONStringValue(tt.input)
			resultJSON, _ := json.Marshal(result)
			expectedJSON, _ := json.Marshal(tt.expected)
			if string(resultJSON) != string(expectedJSON) {
				t.Errorf("expected %s, got %s", expectedJSON, resultJSON)
			}
		})
	}
}

func TestWriteCustomFields(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		wantKeys []string
		wantNil  bool
	}{
		{
			name:    "nil input returns nil",
			input:   nil,
			wantNil: true,
		},
		{
			name:    "empty map returns nil",
			input:   map[string]interface{}{},
			wantNil: true,
		},
		{
			name: "nil value keys are stripped",
			input: map[string]interface{}{
				"set":   "value",
				"unset": nil,
			},
			wantKeys: []string{"set"},
		},
		{
			name: "JSON object string is coerced to map",
			input: map[string]interface{}{
				"device_specs": `{"cores":72}`,
			},
			wantKeys: []string{"device_specs"},
		},
		{
			name: "plain string value stays as string",
			input: map[string]interface{}{
				"status": "active",
			},
			wantKeys: []string{"status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := writeCustomFields(tt.input)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			for _, k := range tt.wantKeys {
				if _, ok := result[k]; !ok {
					t.Errorf("expected key %q in result", k)
				}
			}
			// Verify JSON object coercion happened
			if ds, ok := tt.input.(map[string]interface{})["device_specs"]; ok {
				if _, isStr := ds.(string); isStr {
					if _, isMap := result["device_specs"].(map[string]interface{}); !isMap {
						t.Errorf("expected device_specs to be coerced to map, got %T", result["device_specs"])
					}
				}
			}
		})
	}
}

func TestReadCustomFields(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected map[string]interface{}
	}{
		{
			name:     "nil input returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map returns nil",
			input:    map[string]interface{}{},
			expected: nil,
		},
		{
			name: "nil value keys are stripped (unset CFs)",
			input: map[string]interface{}{
				"set_field":   "value",
				"unset_field": nil,
			},
			expected: map[string]interface{}{
				"set_field": "value",
			},
		},
		{
			name: "empty string keys are stripped (unset text CFs on NetBox 4.4)",
			input: map[string]interface{}{
				"set_field":   "value",
				"unset_field": "",
			},
			expected: map[string]interface{}{
				"set_field": "value",
			},
		},
		{
			name: "map with only unset values returns nil",
			input: map[string]interface{}{
				"unset1": nil,
				"unset2": "",
			},
			expected: nil,
		},
		{
			name: "JSON object from API is stringified for state",
			input: map[string]interface{}{
				"device_specs": map[string]interface{}{"cores": float64(72)},
			},
			expected: map[string]interface{}{
				"device_specs": `{"cores":72}`,
			},
		},
		{
			name: "plain string stays as string",
			input: map[string]interface{}{
				"status": "active",
			},
			expected: map[string]interface{}{
				"status": "active",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := readCustomFields(tt.input)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d keys, got %d: %v", len(tt.expected), len(result), result)
			}
			for k, expectedVal := range tt.expected {
				actualVal, ok := result[k]
				if !ok {
					t.Errorf("expected key %q not found", k)
					continue
				}
				expectedJSON, _ := json.Marshal(expectedVal)
				actualJSON, _ := json.Marshal(actualVal)
				if string(expectedJSON) != string(actualJSON) {
					t.Errorf("key %q: expected %s, got %s", k, expectedJSON, actualJSON)
				}
			}
		})
	}
}

func TestGetCustomFields(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected map[string]interface{}
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: nil,
		},
		{
			name: "valid map",
			input: map[string]interface{}{
				"field1": "value1",
				"field2": 123,
			},
			expected: map[string]interface{}{
				"field1": "value1",
				"field2": 123,
			},
		},
		{
			name:     "invalid type",
			input:    "not a map",
			expected: nil,
		},
		{
			name: "map with nil values excluded",
			input: map[string]interface{}{
				"set_field":   "value1",
				"unset_field": nil,
			},
			expected: map[string]interface{}{
				"set_field": "value1",
			},
		},
		{
			name: "map with only nil values returns nil",
			input: map[string]interface{}{
				"unset1": nil,
				"unset2": nil,
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCustomFields(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("expected non-nil result, got nil")
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d fields, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				actualValue, ok := result[key]
				if !ok {
					t.Errorf("expected field %q not found in result", key)
					continue
				}

				if actualValue != expectedValue {
					t.Errorf("field %q: expected %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}
