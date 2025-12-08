/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"testing"
)

// TestToInt tests the toInt function
func TestToInt(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		// String conversions
		{"simple integer string", "42", MarkerInt + "42"},
		{"string with leading zeros", "007", MarkerInt + "7"},
		{"negative integer string", "-123", MarkerInt + "-123"},
		{"float string truncated", "3.14", MarkerInt + "3"},
		{"negative float string truncated", "-5.99", MarkerInt + "-5"},
		{"invalid string", "invalid", MarkerInt + "0"},
		{"empty string", "", MarkerInt + "0"},

		// Numeric conversions
		{"int input", 42, MarkerInt + "42"},
		{"int64 input", int64(9999999999), MarkerInt + "9999999999"},
		{"float64 input truncated", float64(3.99), MarkerInt + "3"},
		{"negative float64 input", float64(-7.5), MarkerInt + "-7"},

		// Edge cases
		{"nil input", nil, MarkerInt + "0"},
		{"boolean false", false, MarkerInt + "0"},
		{"boolean true", true, MarkerInt + "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toInt(tt.input)
			if result != tt.expected {
				t.Errorf("toInt(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestToFloat tests the toFloat function
func TestToFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"integer string", "42", MarkerFloat + "42"},
		{"float string", "3.14", MarkerFloat + "3.14"},
		{"negative float string", "-1.5", MarkerFloat + "-1.5"},
		{"invalid string", "invalid", MarkerFloat + "0"},
		{"empty string", "", MarkerFloat + "0"},
		{"int input", 42, MarkerFloat + "42"},
		{"float64 input", float64(3.14159), MarkerFloat + "3.14159"},
		{"nil input", nil, MarkerFloat + "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toFloat(tt.input)
			if result != tt.expected {
				t.Errorf("toFloat(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestToBool tests the toBool function
func TestToBool(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		// String conversions
		{"true string", "true", MarkerBool + "true"},
		{"false string", "false", MarkerBool + "false"},
		{"TRUE string", "TRUE", MarkerBool + "true"},
		{"False string", "False", MarkerBool + "false"},
		{"1 string", "1", MarkerBool + "true"},
		{"0 string", "0", MarkerBool + "false"},
		{"yes string", "yes", MarkerBool + "true"},
		{"no string", "no", MarkerBool + "false"},
		{"empty string", "", MarkerBool + "false"},
		{"invalid string", "invalid", MarkerBool + "false"},

		// Boolean conversions
		{"true boolean", true, MarkerBool + "true"},
		{"false boolean", false, MarkerBool + "false"},

		// Numeric conversions
		{"non-zero int", 42, MarkerBool + "true"},
		{"zero int", 0, MarkerBool + "false"},
		{"negative int", -1, MarkerBool + "true"},
		{"non-zero float", 0.1, MarkerBool + "true"},
		{"zero float", 0.0, MarkerBool + "false"},

		// Edge cases
		{"nil input", nil, MarkerBool + "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toBool(tt.input)
			if result != tt.expected {
				t.Errorf("toBool(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseTypedValue tests the ParseTypedValue function
func TestParseTypedValue(t *testing.T) {
	t.Run("parsing int markers", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected interface{}
		}{
			{"positive integer", MarkerInt + "42", int64(42)},
			{"negative integer", MarkerInt + "-123", int64(-123)},
			{"zero", MarkerInt + "0", int64(0)},
			{"invalid int returns original", MarkerInt + "invalid", MarkerInt + "invalid"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := ParseTypedValue(tt.input)
				if result != tt.expected {
					t.Errorf("ParseTypedValue(%q) = %v (%T), want %v (%T)", tt.input, result, result, tt.expected, tt.expected)
				}
			})
		}
	})

	t.Run("parsing float markers", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected float64
		}{
			{"simple float", MarkerFloat + "3.14", 3.14},
			{"integer as float", MarkerFloat + "42", 42.0},
			{"negative float", MarkerFloat + "-1.5", -1.5},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := ParseTypedValue(tt.input)
				f, ok := result.(float64)
				if !ok {
					t.Errorf("ParseTypedValue(%q) type = %T, want float64", tt.input, result)
					return
				}
				if f != tt.expected {
					t.Errorf("ParseTypedValue(%q) = %v, want %v", tt.input, f, tt.expected)
				}
			})
		}
	})

	t.Run("parsing bool markers", func(t *testing.T) {
		if result := ParseTypedValue(MarkerBool + "true"); result != true {
			t.Errorf("ParseTypedValue(true marker) = %v, want true", result)
		}
		if result := ParseTypedValue(MarkerBool + "false"); result != false {
			t.Errorf("ParseTypedValue(false marker) = %v, want false", result)
		}
	})

	t.Run("unmarked strings", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
		}{
			{"plain string", "hello world"},
			{"number without marker", "42"},
			{"empty string", ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := ParseTypedValue(tt.input)
				if result != tt.input {
					t.Errorf("ParseTypedValue(%q) = %v, want %q (unchanged)", tt.input, result, tt.input)
				}
			})
		}
	})
}

// TestTypedFunctionsInTemplates tests the integration of typed functions with the template engine
func TestTypedFunctionsInTemplates(t *testing.T) {
	engine := NewEngine()

	t.Run("int function in template", func(t *testing.T) {
		tests := []struct {
			name     string
			template string
			vars     Variables
			expected int64
		}{
			{
				name:     "port number",
				template: `{{ .port | int }}`,
				vars:     Variables{"port": "8080"},
				expected: 8080,
			},
			{
				name:     "replicas",
				template: `{{ .replicas | int }}`,
				vars:     Variables{"replicas": "3"},
				expected: 3,
			},
			{
				name:     "with default",
				template: `{{ index . "replicas" | default "1" | int }}`,
				vars:     Variables{},
				expected: 1,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				rendered, err := engine.Render(tt.template, tt.vars)
				if err != nil {
					t.Errorf("Render() error = %v", err)
					return
				}

				parsed := ParseTypedValue(rendered)
				if parsed != tt.expected {
					t.Errorf("Parsed value = %v (%T), want %v (%T)", parsed, parsed, tt.expected, tt.expected)
				}
			})
		}
	})

	t.Run("float function in template", func(t *testing.T) {
		template := `{{ .cpuLimit | float }}`
		vars := Variables{"cpuLimit": "1.5"}

		rendered, err := engine.Render(template, vars)
		if err != nil {
			t.Errorf("Render() error = %v", err)
			return
		}

		parsed := ParseTypedValue(rendered)
		f, ok := parsed.(float64)
		if !ok {
			t.Errorf("Parsed type = %T, want float64", parsed)
			return
		}
		if f != 1.5 {
			t.Errorf("Parsed value = %v, want 1.5", f)
		}
	})

	t.Run("bool function in template", func(t *testing.T) {
		tests := []struct {
			name     string
			template string
			vars     Variables
			expected bool
		}{
			{
				name:     "true string",
				template: `{{ .enabled | bool }}`,
				vars:     Variables{"enabled": "true"},
				expected: true,
			},
			{
				name:     "database 1 as true",
				template: `{{ .isActive | bool }}`,
				vars:     Variables{"isActive": "1"},
				expected: true,
			},
			{
				name:     "false string",
				template: `{{ .disabled | bool }}`,
				vars:     Variables{"disabled": "false"},
				expected: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				rendered, err := engine.Render(tt.template, tt.vars)
				if err != nil {
					t.Errorf("Render() error = %v", err)
					return
				}

				parsed := ParseTypedValue(rendered)
				if parsed != tt.expected {
					t.Errorf("Parsed value = %v, want %v", parsed, tt.expected)
				}
			})
		}
	})
}

// TestKubernetesResourceScenarios tests real-world Kubernetes resource scenarios
func TestKubernetesResourceScenarios(t *testing.T) {
	engine := NewEngine()

	t.Run("Deployment replicas", func(t *testing.T) {
		vars := Variables{
			"uid":      "tenant-001",
			"replicas": "3",
		}

		rendered, _ := engine.Render(`{{ .replicas | int }}`, vars)
		parsed := ParseTypedValue(rendered)

		if _, ok := parsed.(int64); !ok {
			t.Errorf("replicas should be int64, got %T", parsed)
		}
		if parsed != int64(3) {
			t.Errorf("replicas = %v, want 3", parsed)
		}
	})

	t.Run("Literal integer in template", func(t *testing.T) {
		vars := Variables{} // Empty vars, using literal

		rendered, err := engine.Render(`{{ 3 | int }}`, vars)
		if err != nil {
			t.Errorf("Failed to render literal int: %v", err)
			return
		}
		parsed := ParseTypedValue(rendered)

		if _, ok := parsed.(int64); !ok {
			t.Errorf("literal 3 should be int64, got %T", parsed)
		}
		if parsed != int64(3) {
			t.Errorf("literal 3 = %v, want 3", parsed)
		}
	})

	t.Run("Literal integer for containerPort", func(t *testing.T) {
		vars := Variables{}

		rendered, err := engine.Render(`{{ 8080 | int }}`, vars)
		if err != nil {
			t.Errorf("Failed to render literal port: %v", err)
			return
		}
		parsed := ParseTypedValue(rendered)

		if _, ok := parsed.(int64); !ok {
			t.Errorf("literal 8080 should be int64, got %T", parsed)
		}
		if parsed != int64(8080) {
			t.Errorf("literal 8080 = %v, want 8080", parsed)
		}
	})

	t.Run("Container port", func(t *testing.T) {
		vars := Variables{"appPort": "8080"}

		rendered, _ := engine.Render(`{{ .appPort | int }}`, vars)
		parsed := ParseTypedValue(rendered)

		if _, ok := parsed.(int64); !ok {
			t.Errorf("containerPort should be int64, got %T", parsed)
		}
		if parsed != int64(8080) {
			t.Errorf("containerPort = %v, want 8080", parsed)
		}
	})

	t.Run("HPA minReplicas and maxReplicas", func(t *testing.T) {
		vars := Variables{
			"minReplicas": "2",
			"maxReplicas": "10",
		}

		minRendered, _ := engine.Render(`{{ .minReplicas | int }}`, vars)
		maxRendered, _ := engine.Render(`{{ .maxReplicas | int }}`, vars)

		minParsed := ParseTypedValue(minRendered)
		maxParsed := ParseTypedValue(maxRendered)

		if minParsed != int64(2) {
			t.Errorf("minReplicas = %v, want 2", minParsed)
		}
		if maxParsed != int64(10) {
			t.Errorf("maxReplicas = %v, want 10", maxParsed)
		}
	})

	t.Run("Feature flag boolean", func(t *testing.T) {
		vars := Variables{
			"enableMetrics":   "true",
			"enableDebugging": "false",
		}

		metricsRendered, _ := engine.Render(`{{ .enableMetrics | bool }}`, vars)
		debugRendered, _ := engine.Render(`{{ .enableDebugging | bool }}`, vars)

		metricsParsed := ParseTypedValue(metricsRendered)
		debugParsed := ParseTypedValue(debugRendered)

		if metricsParsed != true {
			t.Errorf("enableMetrics = %v, want true", metricsParsed)
		}
		if debugParsed != false {
			t.Errorf("enableDebugging = %v, want false", debugParsed)
		}
	})
}
