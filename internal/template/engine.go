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
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// Type markers for typed template functions
// These markers wrap values during template rendering and are parsed later
// to restore the correct Go types for Kubernetes API compatibility
const (
	// MarkerInt marks integer values for type restoration
	MarkerInt = "__LYNQ_TYPE_INT__"
	// MarkerFloat marks float values for type restoration
	MarkerFloat = "__LYNQ_TYPE_FLOAT__"
	// MarkerBool marks boolean values for type restoration
	MarkerBool = "__LYNQ_TYPE_BOOL__"
)

// Variables contains all template variables available for rendering
type Variables map[string]interface{}

// Engine handles template rendering with Go templates + Sprig functions
type Engine struct {
	funcMap template.FuncMap
}

// NewEngine creates a new template engine with all functions
func NewEngine() *Engine {
	engine := &Engine{
		funcMap: sprig.TxtFuncMap(),
	}

	// Add custom functions
	engine.funcMap["toHost"] = toHost
	engine.funcMap["trunc63"] = trunc63
	engine.funcMap["sha1sum"] = sha1sum
	engine.funcMap["fromJson"] = fromJson

	// Add typed template functions for Kubernetes type compatibility
	// These functions wrap values with type markers that are parsed by ParseTypedValue
	engine.funcMap["int"] = toInt
	engine.funcMap["float"] = toFloat
	engine.funcMap["bool"] = toBool

	return engine
}

// Render renders a template string with the given variables
func (e *Engine) Render(templateStr string, vars Variables) (string, error) {
	if templateStr == "" {
		return "", nil
	}

	// Create template with missingkey=error to detect missing variable references
	// This ensures typos in variable names are caught as errors rather than silently
	// producing empty values. Users should use the `default` function for optional
	// variables that may have empty values, not for truly undefined variables.
	tmpl, err := template.New("template").
		Option("missingkey=error").
		Funcs(e.funcMap).
		Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// RenderMap renders all values in a map
func (e *Engine) RenderMap(m map[string]string, vars Variables) (map[string]string, error) {
	if m == nil {
		return nil, nil
	}

	result := make(map[string]string, len(m))
	for k, v := range m {
		rendered, err := e.Render(v, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to render key %s: %w", k, err)
		}
		result[k] = rendered
	}

	return result, nil
}

// Custom Functions

// toHost extracts the hostname from a URL
// Example: toHost("https://example.com:8080/path") -> "example.com"
func toHost(rawURL string) string {
	// Try to parse as URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		// If parsing fails or no host, assume it's already a hostname
		// Remove port if present
		if idx := strings.Index(rawURL, ":"); idx != -1 {
			return rawURL[:idx]
		}
		return rawURL
	}

	// Extract hostname (without port)
	host := parsedURL.Hostname()
	return host
}

// trunc63 truncates a string to 63 characters (Kubernetes label/name limit)
func trunc63(s string) string {
	if len(s) <= 63 {
		return s
	}
	return s[:63]
}

// sha1sum computes SHA1 hash of a string and returns hex-encoded result
// Example: sha1sum("test") -> "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"
func sha1sum(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// fromJson parses a JSON string into a generic interface (map or slice)
// Example: fromJson("{\"key\":\"value\"}") -> map[string]interface{}{"key": "value"}
// Returns empty map on error to allow templates to continue
func fromJson(jsonStr string) interface{} {
	var result interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// Return empty map on error to prevent template execution failure
		return map[string]interface{}{}
	}
	return result
}

// Typed Template Functions
// These functions enable proper type conversion for Kubernetes resources
// by wrapping values with type markers that are later parsed by ParseTypedValue

// toInt converts a value to an integer and wraps it with a type marker
// The marker is parsed by ParseTypedValue to restore the actual int type
// Example: toInt("42") -> "__LYNQ_TYPE_INT__42"
// Example: toInt(3.14) -> "__LYNQ_TYPE_INT__3"
func toInt(value interface{}) string {
	var result int

	switch v := value.(type) {
	case int:
		result = v
	case int64:
		result = int(v)
	case int32:
		result = int(v)
	case float64:
		result = int(v)
	case float32:
		result = int(v)
	case string:
		// Try to parse as int first
		if i, err := strconv.Atoi(v); err == nil {
			result = i
		} else if f, err := strconv.ParseFloat(v, 64); err == nil {
			// Try to parse as float and truncate
			result = int(f)
		}
		// If both fail, result stays 0
	case bool:
		if v {
			result = 1
		}
		// false stays 0
	default:
		// For nil or unsupported types, return 0
		result = 0
	}

	return fmt.Sprintf("%s%d", MarkerInt, result)
}

// toFloat converts a value to a float64 and wraps it with a type marker
// The marker is parsed by ParseTypedValue to restore the actual float64 type
// Example: toFloat("3.14") -> "__LYNQ_TYPE_FLOAT__3.14"
// Example: toFloat(42) -> "__LYNQ_TYPE_FLOAT__42"
func toFloat(value interface{}) string {
	var result float64

	switch v := value.(type) {
	case float64:
		result = v
	case float32:
		result = float64(v)
	case int:
		result = float64(v)
	case int64:
		result = float64(v)
	case int32:
		result = float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			result = f
		}
		// If parsing fails, result stays 0
	case bool:
		if v {
			result = 1.0
		}
	default:
		result = 0
	}

	// Use %v to avoid unnecessary decimal places for whole numbers
	// e.g., 42 -> "42" instead of "42.000000"
	return fmt.Sprintf("%s%v", MarkerFloat, result)
}

// toBool converts a value to a boolean and wraps it with a type marker
// The marker is parsed by ParseTypedValue to restore the actual bool type
// Truthy values: true, "true", "TRUE", "1", "yes", "YES", non-zero numbers
// Falsy values: false, "false", "FALSE", "0", "no", "NO", zero, empty string
// Example: toBool("true") -> "__LYNQ_TYPE_BOOL__true"
// Example: toBool("1") -> "__LYNQ_TYPE_BOOL__true"
func toBool(value interface{}) string {
	var result bool

	switch v := value.(type) {
	case bool:
		result = v
	case string:
		lower := strings.ToLower(v)
		switch lower {
		case "true", "1", "yes", "on", "t", "y":
			result = true
		default:
			result = false
		}
	case int:
		result = v != 0
	case int64:
		result = v != 0
	case int32:
		result = v != 0
	case float64:
		result = v != 0
	case float32:
		result = v != 0
	default:
		result = false
	}

	return fmt.Sprintf("%s%t", MarkerBool, result)
}

// StripTypeMarker removes type markers from a rendered string and returns the raw value as a string
// This allows YAML parsers to automatically convert string numbers to integers where needed,
// while keeping values as strings in string-only fields (e.g., ConfigMap data)
//
// Format: __LYNQ_TYPE_{TYPE}__{value}
//
// Examples:
//   - "__LYNQ_TYPE_INT__42" → "42"
//   - "__LYNQ_TYPE_FLOAT__3.14" → "3.14"
//   - "__LYNQ_TYPE_BOOL__true" → "true"
//   - "plain string" → "plain string"
func StripTypeMarker(s string) string {
	// Check for int marker
	if strings.HasPrefix(s, MarkerInt) {
		return strings.TrimPrefix(s, MarkerInt)
	}

	// Check for float marker
	if strings.HasPrefix(s, MarkerFloat) {
		return strings.TrimPrefix(s, MarkerFloat)
	}

	// Check for bool marker
	if strings.HasPrefix(s, MarkerBool) {
		return strings.TrimPrefix(s, MarkerBool)
	}

	// No marker found, return original string
	return s
}

// ParseTypedValue detects type markers and converts the value to the appropriate Go type.
// This function is called during resource rendering to restore proper types for Kubernetes API.
// If no marker is found, the original string is returned unchanged.
// Example: "__LYNQ_TYPE_INT__42" -> int64(42)
// Example: "__LYNQ_TYPE_FLOAT__3.14" -> float64(3.14)
// Example: "__LYNQ_TYPE_BOOL__true" -> bool(true)
// Example: "hello" -> "hello" (unchanged)
func ParseTypedValue(s string) interface{} {
	// Check for int marker
	if strings.HasPrefix(s, MarkerInt) {
		valueStr := strings.TrimPrefix(s, MarkerInt)
		if i, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
			return i
		}
		// If parsing fails, return original string
		return s
	}

	// Check for float marker
	if strings.HasPrefix(s, MarkerFloat) {
		valueStr := strings.TrimPrefix(s, MarkerFloat)
		if f, err := strconv.ParseFloat(valueStr, 64); err == nil {
			return f
		}
		// If parsing fails, return original string
		return s
	}

	// Check for bool marker
	if strings.HasPrefix(s, MarkerBool) {
		valueStr := strings.TrimPrefix(s, MarkerBool)
		return valueStr == "true"
	}

	// No marker found, return original string
	return s
}

// BuildVariables creates Variables from database row data
// Note: hostOrURL and host are deprecated since v1.1.11 and will be removed in v1.3.0
func BuildVariables(uid, hostOrURL, activate string, extraMappings map[string]string) Variables {
	vars := Variables{
		"uid":      uid,
		"activate": activate,
	}

	// Deprecated: hostOrUrl and host variables (since v1.1.11, removed in v1.3.0)
	// Only populate if hostOrURL is provided (for backward compatibility)
	if hostOrURL != "" {
		vars["hostOrUrl"] = hostOrURL
		// Auto-extract host from hostOrURL
		vars["host"] = toHost(hostOrURL)
	}

	// Add extra mappings
	for k, v := range extraMappings {
		vars[k] = v
	}

	return vars
}
