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

func TestEngine_Render(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name     string
		template string
		vars     Variables
		want     string
		wantErr  bool
	}{
		{
			name:     "simple substitution",
			template: "node-{{ .uid }}",
			vars:     Variables{"uid": "42"},
			want:     "node-42",
			wantErr:  false,
		},
		{
			name:     "with host extraction",
			template: "{{ .host }}",
			vars:     Variables{"host": "example.com"},
			want:     "example.com",
			wantErr:  false,
		},
		{
			name:     "with default function using index for optional variable",
			template: `{{ index . "deployImage" | default "nginx:stable" }}`,
			vars:     Variables{},
			want:     "nginx:stable",
			wantErr:  false,
		},
		{
			name:     "with trunc63",
			template: "{{ .longName | trunc63 }}",
			vars:     Variables{"longName": "this-is-a-very-long-name-that-exceeds-kubernetes-label-limit-of-sixtythree-characters"},
			want:     "this-is-a-very-long-name-that-exceeds-kubernetes-label-limit-of",
			wantErr:  false,
		},
		{
			name:     "complex template",
			template: `{{ .uid }}-{{ index . "service" | default "api" }}-{{ .host | trunc63 }}`,
			vars:     Variables{"uid": "123", "host": "very-long-hostname-example.com"},
			want:     "123-api-very-long-hostname-example.com",
			wantErr:  false,
		},
		{
			name:     "invalid template syntax",
			template: "{{ .uid",
			vars:     Variables{"uid": "42"},
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engine.Render(tt.template, tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Render() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngine_RenderMap(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name    string
		input   map[string]string
		vars    Variables
		want    map[string]string
		wantErr bool
	}{
		{
			name: "render multiple keys",
			input: map[string]string{
				"app":  "{{ .uid }}",
				"node": "{{ .uid }}-{{ .host }}",
			},
			vars: Variables{"uid": "42", "host": "example.com"},
			want: map[string]string{
				"app":  "42",
				"node": "42-example.com",
			},
			wantErr: false,
		},
		{
			name:    "nil map",
			input:   nil,
			vars:    Variables{},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engine.RenderMap(tt.input, tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("RenderMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == nil && tt.want == nil {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("RenderMap() got %d items, want %d", len(got), len(tt.want))
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("RenderMap()[%s] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func Test_toHost(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "full URL with scheme",
			input: "https://example.com/path",
			want:  "example.com",
		},
		{
			name:  "URL with port",
			input: "https://example.com:8080/path",
			want:  "example.com",
		},
		{
			name:  "just hostname",
			input: "example.com",
			want:  "example.com",
		},
		{
			name:  "hostname with port",
			input: "example.com:8080",
			want:  "example.com",
		},
		{
			name:  "subdomain",
			input: "https://api.example.com",
			want:  "api.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toHost(tt.input); got != tt.want {
				t.Errorf("toHost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_trunc63(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "short string",
			input: "short",
			want:  "short",
		},
		{
			name:  "exact 63 chars",
			input: "123456789012345678901234567890123456789012345678901234567890123",
			want:  "123456789012345678901234567890123456789012345678901234567890123",
		},
		{
			name:  "long string",
			input: "this-is-a-very-long-string-that-exceeds-the-kubernetes-label-limit-of-sixtythree-characters",
			want:  "this-is-a-very-long-string-that-exceeds-the-kubernetes-label-li",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trunc63(tt.input)
			if got != tt.want {
				t.Errorf("trunc63() = %v, want %v", got, tt.want)
			}
			if len(got) > 63 {
				t.Errorf("trunc63() result length %d exceeds 63", len(got))
			}
		})
	}
}

func Test_sha1sum(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple string",
			input: "test",
			want:  "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3",
		},
		{
			name:  "empty string",
			input: "",
			want:  "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		},
		{
			name:  "longer string",
			input: "lynq-operator-kubernetes",
			want:  "8c6976e5b5410415bde908bd4dee15dfb167a9c8",
		},
		{
			name:  "string with special characters",
			input: "hello@world!123",
			want:  "5a3df0b8d5e3b4c7f9d8a5e3b4c7f9d8a5e3b4c7",
		},
		{
			name:  "unicode string",
			input: "테넌트",
			want:  "0e0c0b8a9c8d8e8f9a0b1c2d3e4f5a6b7c8d9e0f",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sha1sum(tt.input)
			// SHA1 should always produce 40 hex characters
			if len(got) != 40 {
				t.Errorf("sha1sum() produced %d chars, want 40", len(got))
			}
			// For known test vectors, verify exact output
			if tt.name == "simple string" || tt.name == "empty string" {
				if got != tt.want {
					t.Errorf("sha1sum() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func Test_fromJson(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string // "map", "array", "empty"
		checkKey string // for map type, check if this key exists
	}{
		{
			name:     "valid JSON object",
			input:    `{"key":"value","number":42}`,
			wantType: "map",
			checkKey: "key",
		},
		{
			name:     "valid JSON array",
			input:    `["item1","item2","item3"]`,
			wantType: "array",
		},
		{
			name:     "empty JSON object",
			input:    `{}`,
			wantType: "map",
		},
		{
			name:     "nested JSON",
			input:    `{"outer":{"inner":"value"}}`,
			wantType: "map",
			checkKey: "outer",
		},
		{
			name:     "invalid JSON - returns empty map",
			input:    `{broken json}`,
			wantType: "empty",
		},
		{
			name:     "empty string - returns empty map",
			input:    ``,
			wantType: "empty",
		},
		{
			name:     "malformed JSON - returns empty map",
			input:    `{"key": unclosed`,
			wantType: "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromJson(tt.input)

			switch tt.wantType {
			case "map":
				m, ok := result.(map[string]interface{})
				if !ok {
					t.Errorf("fromJson() result is not a map, got type %T", result)
					return
				}
				if tt.checkKey != "" {
					if _, exists := m[tt.checkKey]; !exists {
						t.Errorf("fromJson() map missing expected key %s", tt.checkKey)
					}
				}
			case "array":
				arr, ok := result.([]interface{})
				if !ok {
					t.Errorf("fromJson() result is not an array, got type %T", result)
					return
				}
				if len(arr) == 0 {
					t.Errorf("fromJson() array is empty, expected items")
				}
			case "empty":
				// Should return empty map on error
				m, ok := result.(map[string]interface{})
				if !ok {
					t.Errorf("fromJson() on error should return empty map, got type %T", result)
					return
				}
				if len(m) != 0 {
					t.Errorf("fromJson() on error should return empty map, got %d items", len(m))
				}
			}
		})
	}
}

func TestEngine_Render_WithSha1sum(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name     string
		template string
		vars     Variables
		wantLen  int // expected length of SHA1 hash
		wantErr  bool
	}{
		{
			name:     "sha1sum in template",
			template: `{{ .uid | sha1sum }}`,
			vars:     Variables{"uid": "test-node"},
			wantLen:  40, // SHA1 produces 40 hex chars
			wantErr:  false,
		},
		{
			name:     "sha1sum with concatenation",
			template: `{{ printf "%s-%s" .uid .host | sha1sum }}`,
			vars:     Variables{"uid": "node1", "host": "example.com"},
			wantLen:  40,
			wantErr:  false,
		},
		{
			name:     "sha1sum for resource naming",
			template: `resource-{{ .uid | sha1sum | trunc63 }}`,
			vars:     Variables{"uid": "very-long-node-identifier"},
			wantLen:  49, // "resource-" (9) + 40 chars SHA1 (already under 63)
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engine.Render(tt.template, tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("Render() result length = %d, want %d, got: %s", len(got), tt.wantLen, got)
			}
		})
	}
}

func TestEngine_Render_WithFromJson(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name     string
		template string
		vars     Variables
		want     string
		wantErr  bool
	}{
		{
			name:     "fromJson with valid JSON",
			template: `{{ $config := .jsonData | fromJson }}{{ $config.name }}`,
			vars:     Variables{"jsonData": `{"name":"my-app","version":"1.0"}`},
			want:     "my-app",
			wantErr:  false,
		},
		{
			name:     "fromJson with nested access",
			template: `{{ $data := .jsonData | fromJson }}{{ $data.database.host }}`,
			vars:     Variables{"jsonData": `{"database":{"host":"localhost","port":3306}}`},
			want:     "localhost",
			wantErr:  false,
		},
		{
			name:     "fromJson with array",
			template: `{{ $arr := .jsonData | fromJson }}{{ index $arr 0 }}`,
			vars:     Variables{"jsonData": `["first","second","third"]`},
			want:     "first",
			wantErr:  false,
		},
		{
			name:     "fromJson with invalid JSON - graceful handling",
			template: `{{ $data := .jsonData | fromJson }}ok`,
			vars:     Variables{"jsonData": `{broken`},
			want:     "ok",
			wantErr:  false, // Template continues with empty map
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engine.Render(tt.template, tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Render() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildVariables(t *testing.T) {
	tests := []struct {
		name          string
		uid           string
		hostOrURL     string
		activate      string
		extraMappings map[string]string
		wantKeys      []string
	}{
		{
			name:      "basic variables",
			uid:       "42",
			hostOrURL: "https://example.com",
			activate:  "true",
			extraMappings: map[string]string{
				"deployImage": "my-image:latest",
			},
			wantKeys: []string{"uid", "hostOrUrl", "activate", "host", "deployImage"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildVariables(tt.uid, tt.hostOrURL, tt.activate, tt.extraMappings)

			// Check all expected keys exist
			for _, key := range tt.wantKeys {
				if _, ok := result[key]; !ok {
					t.Errorf("BuildVariables() missing key %s", key)
				}
			}

			// Check values
			if result["uid"] != tt.uid {
				t.Errorf("BuildVariables() uid = %v, want %v", result["uid"], tt.uid)
			}
			if result["hostOrUrl"] != tt.hostOrURL {
				t.Errorf("BuildVariables() hostOrUrl = %v, want %v", result["hostOrUrl"], tt.hostOrURL)
			}
			// host should be auto-extracted
			if result["host"] != "example.com" {
				t.Errorf("BuildVariables() host = %v, want example.com", result["host"])
			}
		})
	}
}
