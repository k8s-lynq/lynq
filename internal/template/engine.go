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
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
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
