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
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTemplateEngine(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Engine Behavior Suite")
}

var _ = Describe("Template Engine - Core Behaviors", func() {
	var engine *Engine

	BeforeEach(func() {
		engine = NewEngine()
	})

	Context("Variable Substitution", func() {
		Describe("Basic node variables", func() {
			It("Should substitute single variable in template", func() {
				By("Given a template with .uid variable")
				template := "node-{{ .uid }}"
				vars := Variables{"uid": "acme-corp"}

				By("When rendering the template")
				result, err := engine.Render(template, vars)

				By("Then the variable should be substituted correctly")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("node-acme-corp"))
			})

			It("Should substitute multiple variables in template", func() {
				By("Given a template with multiple variables")
				template := "{{ .uid }}-{{ .service }}-{{ .region }}"
				vars := Variables{
					"uid":     "acme",
					"service": "api",
					"region":  "us-east-1",
				}

				By("When rendering the template")
				result, err := engine.Render(template, vars)

				By("Then all variables should be substituted")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("acme-api-us-east-1"))
			})
		})

		Describe("Missing variables", func() {
			It("Should handle missing variable gracefully", func() {
				By("Given a template referencing non-existent variable")
				template := "{{ .uid }}-{{ .missing }}"
				vars := Variables{"uid": "test"}

				By("When rendering the template")
				result, err := engine.Render(template, vars)

				By("Then missing variable should render as '<no value>'")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("test-<no value>"))
			})
		})
	})

	Context("Built-in Functions", func() {
		Describe("default function", func() {
			It("Should use default value when variable is missing", func() {
				By("Given a template with default function")
				template := "{{ default \"nginx:stable\" .deployImage }}"
				vars := Variables{} // No deployImage provided

				By("When rendering without providing the variable")
				result, err := engine.Render(template, vars)

				By("Then the default value should be used")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("nginx:stable"))
			})

			It("Should use provided value when variable exists", func() {
				By("Given a template with default function")
				template := "{{ default \"nginx:stable\" .deployImage }}"
				vars := Variables{"deployImage": "nginx:1.21"}

				By("When rendering with the variable provided")
				result, err := engine.Render(template, vars)

				By("Then the provided value should be used")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("nginx:1.21"))
			})
		})

		Describe("trunc63 function for Kubernetes labels", func() {
			It("Should truncate long names to 63 characters", func() {
				By("Given a very long name exceeding K8s label limit")
				longName := "this-is-a-very-long-name-that-exceeds-kubernetes-label-limit-of-sixtythree-characters"
				template := "{{ .longName | trunc63 }}"
				vars := Variables{"longName": longName}

				By("When rendering with trunc63 function")
				result, err := engine.Render(template, vars)

				By("Then the name should be truncated to 63 characters")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(63))
				Expect(result).To(Equal(longName[:63]))
			})

			It("Should not modify names under 63 characters", func() {
				By("Given a short name")
				shortName := "short-name"
				template := "{{ .name | trunc63 }}" //nolint:goconst
				vars := Variables{"name": shortName}

				By("When rendering with trunc63 function")
				result, err := engine.Render(template, vars)

				By("Then the name should remain unchanged")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(shortName))
			})
		})

		Describe("toHost function for URL extraction", func() {
			It("Should extract host from full URL", func() {
				By("Given a full URL with protocol and path")
				template := "{{ .url | toHost }}" //nolint:goconst
				vars := Variables{"url": "https://acme.example.com/path/to/resource"}

				By("When extracting the host")
				result, err := engine.Render(template, vars)

				By("Then only the hostname should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("acme.example.com"))
			})

			It("Should handle URLs without path", func() {
				By("Given a URL without path")
				template := "{{ .url | toHost }}" //nolint:goconst
				vars := Variables{"url": "https://api.service.com"}

				By("When extracting the host")
				result, err := engine.Render(template, vars)

				By("Then the hostname should be extracted")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("api.service.com"))
			})
		})

		Describe("sha1sum function for stable identifiers", func() {
			It("Should generate stable hash from string", func() {
				By("Given a string to hash")
				template := "{{ .uid | sha1sum }}"
				vars := Variables{"uid": "acme-corp"}

				By("When generating SHA1 hash")
				result1, err1 := engine.Render(template, vars)
				result2, err2 := engine.Render(template, vars)

				By("Then the hash should be consistent")
				Expect(err1).ToNot(HaveOccurred())
				Expect(err2).ToNot(HaveOccurred())
				Expect(result1).To(Equal(result2))
				Expect(result1).To(HaveLen(40)) // SHA1 produces 40-char hex string
			})
		})

		Describe("fromJson function for complex data", func() {
			It("Should parse JSON string into object", func() {
				By("Given a JSON string")
				template := `{{ $config := fromJson .jsonData }}{{ $config.replicas }}`
				vars := Variables{
					"jsonData": `{"replicas": 3, "image": "nginx"}`,
				}

				By("When parsing and accessing the data")
				result, err := engine.Render(template, vars)

				By("Then the JSON should be parsed correctly")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("3"))
			})
		})
	})

	Context("Complex Template Scenarios", func() {
		Describe("Multi-tier application naming", func() {
			It("Should generate consistent names across components", func() {
				By("Given a multi-tier application template")
				vars := Variables{
					"uid":     "acme",
					"service": "api",
					"env":     "production",
				}

				By("When rendering deployment name")
				deployTemplate := "{{ .uid }}-{{ .service }}-{{ .env }}-deployment"
				deployName, err := engine.Render(deployTemplate, vars)
				Expect(err).ToNot(HaveOccurred())

				By("And rendering service name with same pattern")
				svcTemplate := "{{ .uid }}-{{ .service }}-{{ .env }}-service"
				svcName, err := engine.Render(svcTemplate, vars)
				Expect(err).ToNot(HaveOccurred())

				By("Then both names should follow consistent pattern")
				Expect(deployName).To(Equal("acme-api-production-deployment"))
				Expect(svcName).To(Equal("acme-api-production-service"))
			})
		})

		Describe("Dynamic image selection with fallback", func() {
			It("Should use environment-specific image or default", func() {
				By("Given a template with conditional image selection")
				template := `{{ default "nginx:stable" .image }}-{{ .uid }}`

				By("When node provides custom image")
				customVars := Variables{
					"uid":   "node1",
					"image": "nginx:1.21-alpine",
				}
				customResult, err := engine.Render(template, customVars)
				Expect(err).ToNot(HaveOccurred())
				Expect(customResult).To(Equal("nginx:1.21-alpine-node1"))

				By("And when node uses default image")
				defaultVars := Variables{"uid": "node2"}
				defaultResult, err := engine.Render(template, defaultVars)
				Expect(err).ToNot(HaveOccurred())
				Expect(defaultResult).To(Equal("nginx:stable-node2"))
			})
		})

		Describe("Resource name sanitization", func() {
			It("Should ensure names comply with K8s naming rules", func() {
				By("Given a node with very long identifier")
				longUID := "customer-acme-corp-production-us-east-1-web-application-frontend-deployment"
				template := "{{ .uid | trunc63 }}"
				vars := Variables{"uid": longUID}

				By("When rendering the name")
				result, err := engine.Render(template, vars)

				By("Then the name should be valid for Kubernetes")
				Expect(err).ToNot(HaveOccurred())
				Expect(len(result)).To(BeNumerically("<=", 63))
			})
		})
	})

	Context("Template Error Handling", func() {
		Describe("Invalid template syntax", func() {
			It("Should return error for unclosed template tag", func() {
				By("Given a template with syntax error")
				template := "{{ .uid"
				vars := Variables{"uid": "test"}

				By("When rendering the invalid template")
				_, err := engine.Render(template, vars)

				By("Then an error should be returned")
				Expect(err).To(HaveOccurred())
			})

			It("Should return error for invalid function call", func() {
				By("Given a template calling non-existent function")
				template := "{{ .uid | nonExistentFunc }}"
				vars := Variables{"uid": "test"}

				By("When rendering the template")
				_, err := engine.Render(template, vars)

				By("Then an error should be returned")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("Map Rendering for Labels and Annotations", func() {
		Describe("Label template rendering", func() {
			It("Should render all label values with variables", func() {
				By("Given a map of label templates")
				labelTemplates := map[string]string{
					"app":         "{{ .uid }}-app",
					"environment": "{{ .env }}",
					"version":     "{{ default \"v1\" .version }}",
				}
				vars := Variables{
					"uid": "acme",
					"env": "prod",
				}

				By("When rendering the label map")
				result, err := engine.RenderMap(labelTemplates, vars)

				By("Then all labels should be rendered correctly")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveKeyWithValue("app", "acme-app"))
				Expect(result).To(HaveKeyWithValue("environment", "prod"))
				Expect(result).To(HaveKeyWithValue("version", "v1")) // default used
			})
		})

		Describe("Annotation template rendering", func() {
			It("Should render complex annotation values", func() {
				By("Given annotation templates with URLs and JSON")
				annotationTemplates := map[string]string{
					"external-url":    "https://{{ .uid }}.example.com",
					"config-hash":     "{{ .uid | sha1sum }}",
					"deployment-time": "{{ .timestamp }}",
				}
				vars := Variables{
					"uid":       "acme",
					"timestamp": "2025-01-15T10:30:00Z",
				}

				By("When rendering the annotation map")
				result, err := engine.RenderMap(annotationTemplates, vars)

				By("Then all annotations should be rendered")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveKey("external-url"))
				Expect(result["external-url"]).To(Equal("https://acme.example.com"))
				Expect(result).To(HaveKey("config-hash"))
				Expect(result).To(HaveKey("deployment-time"))
			})
		})
	})

	Context("Sprig String Functions", func() {
		Describe("Case manipulation", func() {
			It("Should convert strings to uppercase", func() {
				By("Given a template with upper function")
				template := "{{ .name | upper }}"
				vars := Variables{"name": "acme-corp"}

				By("When rendering the template")
				result, err := engine.Render(template, vars)

				By("Then the string should be uppercase")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("ACME-CORP"))
			})

			It("Should convert strings to lowercase", func() {
				By("Given a template with lower function")
				template := "{{ .name | lower }}"
				vars := Variables{"name": "ACME-CORP"}

				By("When rendering the template")
				result, err := engine.Render(template, vars)

				By("Then the string should be lowercase")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("acme-corp"))
			})

			It("Should convert strings to title case", func() {
				By("Given a template with title function")
				template := "{{ .name | title }}"
				vars := Variables{"name": "acme corp api"}

				By("When rendering the template")
				result, err := engine.Render(template, vars)

				By("Then each word should be capitalized")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Acme Corp Api"))
			})
		})

		Describe("String trimming and replacement", func() {
			It("Should trim whitespace from strings", func() {
				By("Given a template with trim function")
				template := "{{ .value | trim }}"
				vars := Variables{"value": "  spaces around  "}

				By("When rendering the template")
				result, err := engine.Render(template, vars)

				By("Then whitespace should be removed")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("spaces around"))
			})

			It("Should replace characters in strings", func() {
				By("Given a template with replace function")
				template := `{{ .domain | replace "." "-" }}`
				vars := Variables{"domain": "acme.example.com"}

				By("When replacing dots with dashes")
				result, err := engine.Render(template, vars)

				By("Then all dots should be replaced")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("acme-example-com"))
			})

			It("Should quote strings properly", func() {
				By("Given a template with quote function")
				template := `{{ .value | quote }}`
				vars := Variables{"value": "needs quotes"}

				By("When quoting the string")
				result, err := engine.Render(template, vars)

				By("Then string should be wrapped in double quotes")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(`"needs quotes"`))
			})
		})
	})

	Context("Sprig Encoding Functions", func() {
		Describe("Base64 encoding and decoding", func() {
			It("Should encode strings to base64", func() {
				By("Given a template with b64enc function")
				template := "{{ .secret | b64enc }}"
				vars := Variables{"secret": "my-secret-value"}

				By("When encoding to base64")
				result, err := engine.Render(template, vars)

				By("Then the value should be base64 encoded")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("bXktc2VjcmV0LXZhbHVl"))
			})

			It("Should decode base64 strings", func() {
				By("Given a template with b64dec function")
				template := "{{ .encoded | b64dec }}"
				vars := Variables{"encoded": "bXktc2VjcmV0LXZhbHVl"}

				By("When decoding from base64")
				result, err := engine.Render(template, vars)

				By("Then the original value should be restored")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("my-secret-value"))
			})
		})

		Describe("Hashing functions", func() {
			It("Should compute SHA256 hash", func() {
				By("Given a template with sha256sum function")
				template := "{{ .data | sha256sum }}"
				vars := Variables{"data": "test-data"}

				By("When computing SHA256 hash")
				result, err := engine.Render(template, vars)

				By("Then a valid 64-character hex hash should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(64))
				// SHA256 of "test-data"
				Expect(result).To(MatchRegexp("^[a-f0-9]{64}$"))
			})
		})

		Describe("JSON serialization", func() {
			It("Should serialize data to JSON", func() {
				By("Given a template with toJson function")
				template := `{{ list "a" "b" "c" | toJson }}`
				vars := Variables{}

				By("When serializing to JSON")
				result, err := engine.Render(template, vars)

				By("Then valid JSON array should be produced")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(`["a","b","c"]`))
			})
		})
	})

	Context("Template Control Flow", func() {
		Describe("Conditional statements", func() {
			It("Should execute if branch when condition is true", func() {
				By("Given a template with if/else condition")
				template := `{{ if eq .plan "enterprise" }}premium{{ else }}standard{{ end }}`

				By("When condition is true")
				vars := Variables{"plan": "enterprise"}
				result, err := engine.Render(template, vars)

				By("Then if branch should execute")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("premium"))
			})

			It("Should execute else branch when condition is false", func() {
				By("Given a template with if/else condition")
				template := `{{ if eq .plan "enterprise" }}premium{{ else }}standard{{ end }}`

				By("When condition is false")
				vars := Variables{"plan": "basic"}
				result, err := engine.Render(template, vars)

				By("Then else branch should execute")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("standard"))
			})

			It("Should use ternary operator for inline conditionals", func() {
				By("Given a template with ternary operator")
				template := `{{ ternary "5" "2" (eq .plan "enterprise") }}`

				By("When condition is true")
				vars := Variables{"plan": "enterprise"}
				result, err := engine.Render(template, vars)

				By("Then first value should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("5"))
			})
		})

		Describe("Variable assignment", func() {
			It("Should assign and reuse variables in templates", func() {
				By("Given a template with variable assignment")
				template := `{{ $name := printf "%s-app" .uid }}{{ $name }}-deployment`
				vars := Variables{"uid": "acme"}

				By("When rendering with assigned variable")
				result, err := engine.Render(template, vars)

				By("Then variable should be reused")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("acme-app-deployment"))
			})
		})

		Describe("Range iteration", func() {
			It("Should iterate over list items", func() {
				By("Given a template with range loop")
				template := `{{ range $i, $region := list "us" "eu" "ap" }}{{ if $i }},{{ end }}{{ $region }}{{ end }}`
				vars := Variables{}

				By("When iterating over regions")
				result, err := engine.Render(template, vars)

				By("Then all items should be concatenated")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("us,eu,ap"))
			})
		})
	})

	Context("Sprig Math and Comparison Functions", func() {
		Describe("Arithmetic operations", func() {
			It("Should add numbers", func() {
				By("Given a template with add function")
				template := `{{ add 100 .offset }}`
				vars := Variables{"offset": "50"}

				By("When adding values")
				result, err := engine.Render(template, vars)

				By("Then sum should be calculated")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("150"))
			})

			It("Should find maximum value", func() {
				By("Given a template with max function")
				template := `{{ max 3 .minReplicas }}`
				vars := Variables{"minReplicas": "1"}

				By("When finding maximum")
				result, err := engine.Render(template, vars)

				By("Then larger value should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("3"))
			})
		})

		Describe("Comparison operators", func() {
			It("Should compare values with eq", func() {
				By("Given a template with equality check")
				template := `{{ eq .status "active" }}`
				vars := Variables{"status": "active"}

				By("When values are equal")
				result, err := engine.Render(template, vars)

				By("Then true should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("true"))
			})

			It("Should compare values with ne", func() {
				By("Given a template with not-equal check")
				template := `{{ ne .status "inactive" }}`
				vars := Variables{"status": "active"}

				By("When values are not equal")
				result, err := engine.Render(template, vars)

				By("Then true should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("true"))
			})
		})
	})

	Context("Sprig List Functions", func() {
		Describe("List operations", func() {
			It("Should create and join lists", func() {
				By("Given a template creating a list and joining")
				template := `{{ list "app" .uid .region | join "-" }}`
				vars := Variables{
					"uid":    "acme",
					"region": "us-east",
				}

				By("When creating and joining list")
				result, err := engine.Render(template, vars)

				By("Then elements should be joined with separator")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("app-acme-us-east"))
			})
		})
	})

	Context("Real-world Production Scenarios", func() {
		Describe("Plan-based resource configuration", func() {
			It("Should configure resources based on plan tier", func() {
				By("Given a template with plan-based replicas")
				template := `{{ if eq .plan "enterprise" }}5{{ else if eq .plan "premium" }}3{{ else }}1{{ end }}`

				By("When rendering for enterprise plan")
				enterpriseVars := Variables{"plan": "enterprise"}
				result, err := engine.Render(template, enterpriseVars)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("5"))

				By("And when rendering for premium plan")
				premiumVars := Variables{"plan": "premium"}
				result, err = engine.Render(template, premiumVars)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("3"))

				By("And when rendering for basic plan")
				basicVars := Variables{"plan": "basic"}
				result, err = engine.Render(template, basicVars)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("1"))
			})
		})

		Describe("Multi-region resource naming", func() {
			It("Should create region-aware resource names", func() {
				By("Given a template with region prefix")
				template := `{{ if .region }}{{ .region }}-{{ end }}{{ .uid }}-app`

				By("When region is provided")
				withRegion := Variables{"uid": "acme", "region": "us-east-1"}
				result, err := engine.Render(template, withRegion)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("us-east-1-acme-app"))

				By("And when region is not provided")
				withoutRegion := Variables{"uid": "acme"}
				result, err = engine.Render(template, withoutRegion)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("acme-app"))
			})
		})

		Describe("Complex JSON configuration parsing", func() {
			It("Should extract nested values from JSON", func() {
				By("Given a template parsing nested JSON")
				template := `{{ $cfg := fromJson .config }}{{ $cfg.db.host }}:{{ $cfg.db.port }}`
				vars := Variables{
					"config": `{"db":{"host":"localhost","port":5432}}`,
				}

				By("When parsing and accessing nested fields")
				result, err := engine.Render(template, vars)

				By("Then nested values should be extracted")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("localhost:5432"))
			})

			It("Should handle invalid JSON gracefully", func() {
				By("Given a template with invalid JSON")
				template := `{{ $cfg := fromJson .config }}{{ $cfg.key }}`
				vars := Variables{"config": `{invalid json`}

				By("When parsing invalid JSON")
				result, err := engine.Render(template, vars)

				By("Then empty map should be returned and template succeeds")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("<no value>"))
			})
		})

		Describe("Secure resource name generation", func() {
			It("Should create short stable names using hash", func() {
				By("Given a template using hash for name generation")
				template := `{{ .uid | sha1sum | trunc 8 }}-app`
				vars := Variables{"uid": "very-long-customer-identifier"}

				By("When generating hashed name")
				result, err := engine.Render(template, vars)

				By("Then short stable name should be created")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(12)) // 8 chars + "-app"
				Expect(result).To(MatchRegexp(`^[a-f0-9]{8}-app$`))
			})
		})

		Describe("Chained function calls", func() {
			It("Should support complex function pipelines", func() {
				By("Given a template with multiple chained functions")
				template := `{{ .uid | upper | replace "-" "_" | trunc63 }}`
				vars := Variables{"uid": "acme-corp-api-service"}

				By("When executing function pipeline")
				result, err := engine.Render(template, vars)

				By("Then all transformations should be applied")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("ACME_CORP_API_SERVICE"))
			})
		})
	})

	Context("Custom Functions - Detailed Testing", func() {
		Describe("toHost function edge cases", func() {
			It("Should handle hostname with port (no protocol)", func() {
				By("Given a hostname:port without protocol")
				template := `{{ .host | toHost }}`
				vars := Variables{"host": "example.com:8080"}

				By("When extracting host")
				result, err := engine.Render(template, vars)

				By("Then hostname without port should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("example.com"))
			})

			It("Should handle IPv4 addresses", func() {
				By("Given an IPv4 address")
				template := `{{ .url | toHost }}` //nolint:goconst
				vars := Variables{"url": "http://192.168.1.1:8080/path"}

				By("When extracting host")
				result, err := engine.Render(template, vars)

				By("Then IPv4 address should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("192.168.1.1"))
			})

			It("Should handle localhost", func() {
				By("Given localhost URL")
				template := `{{ .url | toHost }}` //nolint:goconst
				vars := Variables{"url": "http://localhost:3000/api"}

				By("When extracting host")
				result, err := engine.Render(template, vars)

				By("Then localhost should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("localhost"))
			})

			It("Should handle URLs with subdomains", func() {
				By("Given a URL with multiple subdomains")
				template := `{{ .url | toHost }}` //nolint:goconst
				vars := Variables{"url": "https://api.staging.example.com/v1/users"}

				By("When extracting host")
				result, err := engine.Render(template, vars)

				By("Then full subdomain should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("api.staging.example.com"))
			})

			It("Should handle malformed URLs gracefully", func() {
				By("Given a malformed URL")
				template := `{{ .url | toHost }}` //nolint:goconst
				vars := Variables{"url": "not-a-valid-url"}

				By("When extracting host")
				result, err := engine.Render(template, vars)

				By("Then original string should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("not-a-valid-url"))
			})

			It("Should handle empty string", func() {
				By("Given an empty URL")
				template := `{{ .url | toHost }}` //nolint:goconst
				vars := Variables{"url": ""}

				By("When extracting host")
				result, err := engine.Render(template, vars)

				By("Then empty string should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(""))
			})
		})

		Describe("trunc63 function edge cases", func() {
			It("Should handle exactly 63 characters", func() {
				By("Given a string with exactly 63 characters")
				exactLength := "a123456789b123456789c123456789d123456789e123456789f123456789012"
				template := `{{ .name | trunc63 }}` //nolint:goconst
				vars := Variables{"name": exactLength}

				By("When truncating")
				result, err := engine.Render(template, vars)

				By("Then string should remain unchanged")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(exactLength))
				Expect(result).To(HaveLen(63))
			})

			It("Should handle exactly 64 characters", func() {
				By("Given a string with exactly 64 characters")
				overLength := "a123456789b123456789c123456789d123456789e123456789f1234567890123"
				template := `{{ .name | trunc63 }}` //nolint:goconst
				vars := Variables{"name": overLength}

				By("When truncating")
				result, err := engine.Render(template, vars)

				By("Then string should be truncated to 63")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(63))
				Expect(result).To(Equal(overLength[:63]))
			})

			It("Should handle empty string", func() {
				By("Given an empty string")
				template := `{{ .name | trunc63 }}` //nolint:goconst
				vars := Variables{"name": ""}

				By("When truncating")
				result, err := engine.Render(template, vars)

				By("Then empty string should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(""))
			})

			It("Should handle unicode characters", func() {
				By("Given a string with unicode characters")
				unicodeStr := "한글-테스트-very-long-identifier-with-unicode-characters-exceeding-63-char-limit"
				template := `{{ .name | trunc63 }}` //nolint:goconst
				vars := Variables{"name": unicodeStr}

				By("When truncating")
				result, err := engine.Render(template, vars)

				By("Then string should be truncated to 63 characters")
				Expect(err).ToNot(HaveOccurred())
				Expect(len(result)).To(BeNumerically("<=", 63))
			})

			It("Should handle Kubernetes resource name format", func() {
				By("Given a typical K8s resource name pattern")
				longName := "customer-acme-corp-production-us-east-1-web-frontend-deployment-v2"
				template := `{{ .name | trunc63 }}` //nolint:goconst
				vars := Variables{"name": longName}

				By("When truncating for K8s compliance")
				result, err := engine.Render(template, vars)

				By("Then name should fit K8s 63-char limit")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(63))
				Expect(result).To(Equal(longName[:63]))
			})
		})

		Describe("sha1sum function edge cases", func() {
			It("Should produce consistent hashes", func() {
				By("Given the same input multiple times")
				template := `{{ .data | sha1sum }}` //nolint:goconst
				vars := Variables{"data": "consistent-data"}

				By("When computing hash multiple times")
				result1, err1 := engine.Render(template, vars)
				result2, err2 := engine.Render(template, vars)
				result3, err3 := engine.Render(template, vars)

				By("Then all hashes should be identical")
				Expect(err1).ToNot(HaveOccurred())
				Expect(err2).ToNot(HaveOccurred())
				Expect(err3).ToNot(HaveOccurred())
				Expect(result1).To(Equal(result2))
				Expect(result2).To(Equal(result3))
			})

			It("Should handle empty string", func() {
				By("Given an empty string")
				template := `{{ .data | sha1sum }}` //nolint:goconst
				vars := Variables{"data": ""}

				By("When computing hash")
				result, err := engine.Render(template, vars)

				By("Then valid SHA1 hash should be produced")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(40))
				// SHA1 of empty string
				Expect(result).To(Equal("da39a3ee5e6b4b0d3255bfef95601890afd80709"))
			})

			It("Should handle special characters", func() {
				By("Given a string with special characters")
				template := `{{ .data | sha1sum }}` //nolint:goconst
				vars := Variables{"data": "!@#$%^&*()_+-=[]{}|;:',.<>?/"}

				By("When computing hash")
				result, err := engine.Render(template, vars)

				By("Then valid SHA1 hash should be produced")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(40))
				Expect(result).To(MatchRegexp("^[a-f0-9]{40}$"))
			})

			It("Should handle unicode characters", func() {
				By("Given a string with unicode")
				template := `{{ .data | sha1sum }}` //nolint:goconst
				vars := Variables{"data": "안녕하세요-世界-мир"}

				By("When computing hash")
				result, err := engine.Render(template, vars)

				By("Then valid SHA1 hash should be produced")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(40))
				Expect(result).To(MatchRegexp("^[a-f0-9]{40}$"))
			})

			It("Should handle very long strings", func() {
				By("Given a very long string")
				longData := strings.Repeat("abcdefghij", 1000) // 10,000 chars
				template := `{{ .data | sha1sum }}`            //nolint:goconst
				vars := Variables{"data": longData}

				By("When computing hash")
				result, err := engine.Render(template, vars)

				By("Then valid SHA1 hash should be produced")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(40))
			})

			It("Should produce different hashes for different inputs", func() {
				By("Given two different inputs")
				template := `{{ .data | sha1sum }}` //nolint:goconst

				By("When computing hashes")
				vars1 := Variables{"data": "input1"}
				result1, err1 := engine.Render(template, vars1)

				vars2 := Variables{"data": "input2"}
				result2, err2 := engine.Render(template, vars2)

				By("Then hashes should be different")
				Expect(err1).ToNot(HaveOccurred())
				Expect(err2).ToNot(HaveOccurred())
				Expect(result1).ToNot(Equal(result2))
			})
		})

		Describe("fromJson function edge cases", func() {
			It("Should parse JSON objects", func() {
				By("Given a valid JSON object")
				template := `{{ $obj := fromJson .json }}{{ $obj.name }}`
				vars := Variables{"json": `{"name":"test","value":123}`}

				By("When parsing JSON")
				result, err := engine.Render(template, vars)

				By("Then object fields should be accessible")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("test"))
			})

			It("Should parse JSON arrays", func() {
				By("Given a valid JSON array")
				template := `{{ $arr := fromJson .json }}{{ index $arr 0 }}`
				vars := Variables{"json": `["first","second","third"]`}

				By("When parsing JSON array")
				result, err := engine.Render(template, vars)

				By("Then array elements should be accessible")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("first"))
			})

			It("Should handle empty JSON object", func() {
				By("Given an empty JSON object")
				template := `{{ $obj := fromJson .json }}{{ $obj.missing }}`
				vars := Variables{"json": `{}`}

				By("When parsing empty object")
				result, err := engine.Render(template, vars)

				By("Then missing keys should return no value")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("<no value>"))
			})

			It("Should handle empty JSON array", func() {
				By("Given an empty JSON array")
				template := `{{ $arr := fromJson .json }}{{ len $arr }}`
				vars := Variables{"json": `[]`}

				By("When parsing empty array")
				result, err := engine.Render(template, vars)

				By("Then length should be zero")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("0"))
			})

			It("Should handle deeply nested JSON", func() {
				By("Given a deeply nested JSON structure")
				template := `{{ $obj := fromJson .json }}{{ $obj.level1.level2.level3.value }}`
				vars := Variables{"json": `{"level1":{"level2":{"level3":{"value":"deep"}}}}`}

				By("When parsing nested structure")
				result, err := engine.Render(template, vars)

				By("Then deeply nested values should be accessible")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("deep"))
			})

			It("Should handle JSON with null values", func() {
				By("Given JSON with null values")
				template := `{{ $obj := fromJson .json }}{{ $obj.nullable }}`
				vars := Variables{"json": `{"nullable":null}`}

				By("When parsing JSON with nulls")
				result, err := engine.Render(template, vars)

				By("Then null should be handled")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("<no value>"))
			})

			It("Should handle JSON with boolean values", func() {
				By("Given JSON with booleans")
				template := `{{ $obj := fromJson .json }}{{ $obj.enabled }}`
				vars := Variables{"json": `{"enabled":true,"disabled":false}`}

				By("When parsing booleans")
				result, err := engine.Render(template, vars)

				By("Then boolean values should be accessible")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("true"))
			})

			It("Should handle JSON with numeric values", func() {
				By("Given JSON with numbers")
				template := `{{ $obj := fromJson .json }}{{ $obj.count }}`
				vars := Variables{"json": `{"count":42,"price":99.99}`}

				By("When parsing numbers")
				result, err := engine.Render(template, vars)

				By("Then numeric values should be accessible")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("42"))
			})

			It("Should return empty map for completely invalid JSON", func() {
				By("Given completely invalid JSON")
				template := `{{ $obj := fromJson .json }}{{ $obj.key }}`
				vars := Variables{"json": `this is not json at all`}

				By("When parsing invalid JSON")
				result, err := engine.Render(template, vars)

				By("Then empty map should be returned allowing template to continue")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("<no value>"))
			})

			It("Should handle empty string as invalid JSON", func() {
				By("Given an empty string")
				template := `{{ $obj := fromJson .json }}{{ $obj.key }}`
				vars := Variables{"json": ``}

				By("When parsing empty string")
				result, err := engine.Render(template, vars)

				By("Then empty map should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("<no value>"))
			})

			It("Should handle JSON with escaped characters", func() {
				By("Given JSON with escaped characters")
				template := `{{ $obj := fromJson .json }}{{ $obj.message }}`
				vars := Variables{"json": `{"message":"Line 1\nLine 2\tTabbed"}`}

				By("When parsing JSON with escapes")
				result, err := engine.Render(template, vars)

				By("Then escaped characters should be parsed")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("Line 1"))
			})
		})

		Describe("Custom functions in combination", func() {
			It("Should chain toHost with trunc63", func() {
				By("Given a very long URL")
				template := `{{ .url | toHost | printf "%s-service" | trunc63 }}`
				vars := Variables{"url": "https://very-long-subdomain-name-that-exceeds-limits.example.com"}

				By("When extracting host and truncating")
				result, err := engine.Render(template, vars)

				By("Then result should be K8s compliant")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(63))
			})

			It("Should use sha1sum for stable short identifiers", func() {
				By("Given a long identifier")
				template := `{{ .uid | sha1sum | trunc 8 }}`
				vars := Variables{"uid": "very-long-customer-identifier-that-needs-shortening"}

				By("When creating short stable identifier")
				result, err := engine.Render(template, vars)

				By("Then 8-char stable hash should be produced")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(8))
				Expect(result).To(MatchRegexp("^[a-f0-9]{8}$"))

				By("And it should be consistent")
				result2, err2 := engine.Render(template, vars)
				Expect(err2).ToNot(HaveOccurred())
				Expect(result2).To(Equal(result))
			})

			It("Should parse JSON and hash result", func() {
				By("Given JSON configuration")
				template := `{{ $cfg := fromJson .config }}{{ $cfg.apiKey | sha1sum | trunc 8 }}`
				vars := Variables{"config": `{"apiKey":"secret-key-12345"}`}

				By("When parsing and hashing")
				result, err := engine.Render(template, vars)

				By("Then hashed value should be extracted")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(8))
				Expect(result).To(MatchRegexp("^[a-f0-9]{8}$"))
			})
		})
	})

	Context("Conditional Template Logic for Resource Specs", func() {
		Describe("Conditional field inclusion", func() {
			It("Should include fields based on condition", func() {
				By("Given a template with conditional field")
				template := `apiVersion: v1
kind: Service
metadata:
  name: {{ .uid }}-svc
spec:
  type: ClusterIP
  {{- if eq .planId "enterprise" }}
  sessionAffinity: ClientIP
  {{- end }}
  ports:
  - port: 80`
				vars := Variables{"uid": "acme", "planId": "enterprise"}

				By("When rendering with enterprise plan")
				result, err := engine.Render(template, vars)

				By("Then conditional field should be included")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("sessionAffinity: ClientIP"))
			})

			It("Should exclude fields when condition is false", func() {
				By("Given a template with conditional field")
				template := `apiVersion: v1
kind: Service
metadata:
  name: {{ .uid }}-svc
spec:
  type: ClusterIP
  {{- if eq .planId "enterprise" }}
  sessionAffinity: ClientIP
  {{- end }}
  ports:
  - port: 80`
				vars := Variables{"uid": "acme", "planId": "basic"}

				By("When rendering with basic plan")
				result, err := engine.Render(template, vars)

				By("Then conditional field should be excluded")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(ContainSubstring("sessionAffinity"))
			})
		})

		Describe("Conditional array items", func() {
			It("Should add container based on plan tier", func() {
				By("Given a template with conditional container")
				template := `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx:latest
      {{- if eq .planId "enterprise" }}
      - name: redis
        image: redis:7-alpine
      {{- end }}`
				vars := Variables{"planId": "enterprise"}

				By("When rendering for enterprise plan")
				result, err := engine.Render(template, vars)

				By("Then redis container should be included")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("name: redis"))
				Expect(result).To(ContainSubstring("image: redis:7-alpine"))
			})

			It("Should exclude container when condition is false", func() {
				By("Given a template with conditional container")
				template := `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx:latest
      {{- if eq .planId "enterprise" }}
      - name: redis
        image: redis:7-alpine
      {{- end }}`
				vars := Variables{"planId": "basic"}

				By("When rendering for basic plan")
				result, err := engine.Render(template, vars)

				By("Then redis container should be excluded")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("name: app"))
				Expect(result).ToNot(ContainSubstring("redis"))
			})

			It("Should add multiple volumes conditionally", func() {
				By("Given a template with conditional volumes")
				template := `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      volumes:
      - name: config
        configMap:
          name: app-config
      {{- if .enableCache }}
      - name: cache
        emptyDir: {}
      {{- end }}
      {{- if .enableLogs }}
      - name: logs
        persistentVolumeClaim:
          claimName: logs-pvc
      {{- end }}`

				By("When both features are enabled")
				varsEnabled := Variables{"enableCache": true, "enableLogs": true}
				result, err := engine.Render(template, varsEnabled)

				By("Then all volumes should be present")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("name: config"))
				Expect(result).To(ContainSubstring("name: cache"))
				Expect(result).To(ContainSubstring("name: logs"))

				By("When only cache is enabled")
				varsCacheOnly := Variables{"enableCache": true, "enableLogs": false}
				result, err = engine.Render(template, varsCacheOnly)

				By("Then only config and cache volumes should be present")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("name: config"))
				Expect(result).To(ContainSubstring("name: cache"))
				Expect(result).ToNot(ContainSubstring("name: logs"))
			})
		})

		Describe("Conditional blocks with else", func() {
			It("Should use if-else for replicas based on plan", func() {
				By("Given a template with if-else for replicas")
				template := `apiVersion: apps/v1
kind: Deployment
spec:
  {{- if eq .planId "enterprise" }}
  replicas: 5
  {{- else if eq .planId "premium" }}
  replicas: 3
  {{- else }}
  replicas: 1
  {{- end }}
  selector:
    matchLabels:
      app: {{ .uid }}`

				By("When rendering for enterprise")
				varsEnterprise := Variables{"uid": "acme", "planId": "enterprise"}
				result, err := engine.Render(template, varsEnterprise)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("replicas: 5"))

				By("When rendering for premium")
				varsPremium := Variables{"uid": "acme", "planId": "premium"}
				result, err = engine.Render(template, varsPremium)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("replicas: 3"))

				By("When rendering for basic")
				varsBasic := Variables{"uid": "acme", "planId": "basic"}
				result, err = engine.Render(template, varsBasic)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("replicas: 1"))
			})
		})

		Describe("Conditional environment variables", func() {
			It("Should add environment variables conditionally", func() {
				By("Given a template with conditional env vars")
				template := `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        env:
        - name: TENANT_ID
          value: "{{ .uid }}"
        {{- if .dbHost }}
        - name: DATABASE_HOST
          value: "{{ .dbHost }}"
        {{- end }}
        {{- if .redisUrl }}
        - name: REDIS_URL
          value: "{{ .redisUrl }}"
        {{- end }}
        {{- if eq .planId "enterprise" }}
        - name: ENABLE_PREMIUM_FEATURES
          value: "true"
        {{- end }}`

				By("When all variables are provided")
				varsAll := Variables{
					"uid":      "acme",
					"dbHost":   "postgres.example.com",
					"redisUrl": "redis://redis.example.com",
					"planId":   "enterprise",
				}
				result, err := engine.Render(template, varsAll)

				By("Then all env vars should be present")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("name: TENANT_ID"))
				Expect(result).To(ContainSubstring("name: DATABASE_HOST"))
				Expect(result).To(ContainSubstring("name: REDIS_URL"))
				Expect(result).To(ContainSubstring("name: ENABLE_PREMIUM_FEATURES"))

				By("When only basic variables are provided")
				varsBasic := Variables{"uid": "acme", "planId": "basic"}
				result, err = engine.Render(template, varsBasic)

				By("Then only basic env vars should be present")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("name: TENANT_ID"))
				Expect(result).ToNot(ContainSubstring("name: DATABASE_HOST"))
				Expect(result).ToNot(ContainSubstring("name: REDIS_URL"))
				Expect(result).ToNot(ContainSubstring("ENABLE_PREMIUM_FEATURES"))
			})
		})

		Describe("Conditional annotations and labels", func() {
			It("Should add annotations based on conditions", func() {
				By("Given a template with conditional annotations")
				template := `apiVersion: v1
kind: Service
metadata:
  name: {{ .uid }}-svc
  annotations:
    lynq.sh/managed: "true"
    {{- if .enableMonitoring }}
    prometheus.io/scrape: "true"
    prometheus.io/port: "9090"
    {{- end }}
    {{- if eq .planId "enterprise" }}
    enterprise.lynq.sh/sla: "99.99"
    {{- end }}`

				By("When monitoring and enterprise plan enabled")
				vars := Variables{
					"uid":              "acme",
					"enableMonitoring": true,
					"planId":           "enterprise",
				}
				result, err := engine.Render(template, vars)

				By("Then all annotations should be present")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("lynq.sh/managed"))
				Expect(result).To(ContainSubstring("prometheus.io/scrape"))
				Expect(result).To(ContainSubstring("enterprise.lynq.sh/sla"))
			})
		})

		Describe("Complex nested conditionals", func() {
			It("Should handle nested if statements", func() {
				By("Given a template with nested conditionals")
				template := `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        resources:
          {{- if eq .planId "enterprise" }}
          limits:
            {{- if .gpuEnabled }}
            nvidia.com/gpu: "1"
            {{- end }}
            cpu: "4000m"
            memory: "8Gi"
          {{- else if eq .planId "premium" }}
          limits:
            cpu: "2000m"
            memory: "4Gi"
          {{- else }}
          limits:
            cpu: "500m"
            memory: "1Gi"
          {{- end }}`

				By("When enterprise plan with GPU")
				varsGPU := Variables{"planId": "enterprise", "gpuEnabled": true}
				result, err := engine.Render(template, varsGPU)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("nvidia.com/gpu"))
				Expect(result).To(ContainSubstring("cpu: \"4000m\""))

				By("When enterprise plan without GPU")
				varsNoGPU := Variables{"planId": "enterprise", "gpuEnabled": false}
				result, err = engine.Render(template, varsNoGPU)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(ContainSubstring("nvidia.com/gpu"))
				Expect(result).To(ContainSubstring("cpu: \"4000m\""))

				By("When premium plan")
				varsPremium := Variables{"planId": "premium"}
				result, err = engine.Render(template, varsPremium)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("cpu: \"2000m\""))
			})
		})

		Describe("Conditional with logical operators", func() {
			It("Should use OR condition for multiple plans", func() {
				By("Given a template with OR condition")
				template := `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .uid }}-config
data:
  {{- if or (eq .planId "enterprise") (eq .planId "premium") }}
  enable_advanced_features: "true"
  {{- else }}
  enable_advanced_features: "false"
  {{- end }}`

				By("When plan is enterprise")
				result, err := engine.Render(template, Variables{"uid": "acme", "planId": "enterprise"})
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("enable_advanced_features: \"true\""))

				By("When plan is premium")
				result, err = engine.Render(template, Variables{"uid": "acme", "planId": "premium"})
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("enable_advanced_features: \"true\""))

				By("When plan is basic")
				result, err = engine.Render(template, Variables{"uid": "acme", "planId": "basic"})
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("enable_advanced_features: \"false\""))
			})

			It("Should use AND condition for multiple requirements", func() {
				By("Given a template with AND condition")
				template := `apiVersion: v1
kind: Service
metadata:
  name: {{ .uid }}-svc
  {{- if and (eq .region "us-east-1") (eq .environment "production") }}
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
  {{- end }}`

				By("When both conditions are met")
				result, err := engine.Render(template, Variables{
					"uid":         "acme",
					"region":      "us-east-1",
					"environment": "production",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("aws-load-balancer-type"))

				By("When only one condition is met")
				result, err = engine.Render(template, Variables{
					"uid":         "acme",
					"region":      "us-east-1",
					"environment": "staging",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(ContainSubstring("aws-load-balancer-type"))
			})
		})

		Describe("Conditional entire resource sections", func() {
			It("Should conditionally include initContainers", func() {
				By("Given a template with conditional initContainers section")
				template := `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      {{- if .requiresMigration }}
      initContainers:
      - name: db-migration
        image: {{ .uid }}-migrations:latest
        command: ["migrate", "up"]
      {{- end }}
      containers:
      - name: app
        image: {{ .uid }}:latest`

				By("When migration is required")
				varsMigration := Variables{"uid": "acme", "requiresMigration": true}
				result, err := engine.Render(template, varsMigration)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("initContainers:"))
				Expect(result).To(ContainSubstring("db-migration"))

				By("When migration is not required")
				varsNoMigration := Variables{"uid": "acme", "requiresMigration": false}
				result, err = engine.Render(template, varsNoMigration)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(ContainSubstring("initContainers"))
				Expect(result).ToNot(ContainSubstring("db-migration"))
			})

			It("Should conditionally include security context", func() {
				By("Given a template with conditional securityContext")
				template := `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx
        {{- if .requiresPrivileged }}
        securityContext:
          privileged: true
          capabilities:
            add:
            - NET_ADMIN
        {{- else }}
        securityContext:
          runAsNonRoot: true
          runAsUser: 1000
        {{- end }}`

				By("When privileged access is required")
				varsPrivileged := Variables{"requiresPrivileged": true}
				result, err := engine.Render(template, varsPrivileged)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("privileged: true"))
				Expect(result).To(ContainSubstring("NET_ADMIN"))

				By("When privileged access is not required")
				varsUnprivileged := Variables{"requiresPrivileged": false}
				result, err = engine.Render(template, varsUnprivileged)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("runAsNonRoot: true"))
				Expect(result).ToNot(ContainSubstring("privileged"))
			})
		})
	})

	Context("LynqForm Resource Array Conditional Rendering", func() {
		Describe("Conditional entire resource arrays", func() {
			It("Should conditionally include deployments array for premium plans", func() {
				By("Given a LynqForm YAML template with conditional deployments")
				template := `apiVersion: lynq.sh/v1
kind: LynqForm
metadata:
  name: {{ .uid }}-template
spec:
  hubId: {{ .hubId }}
  {{- if or (eq .planId "premium") (eq .planId "enterprise") }}
  deployments:
  - id: redis
    nameTemplate: "{{ .uid }}-redis"
    spec:
      apiVersion: apps/v1
      kind: Deployment
      spec:
        replicas: 1
        template:
          spec:
            containers:
            - name: redis
              image: redis:7-alpine
  {{- end }}
  services:
  - id: app-service
    nameTemplate: "{{ .uid }}-svc"
    spec:
      apiVersion: v1
      kind: Service
      spec:
        ports:
        - port: 80`

				By("When rendering for enterprise plan")
				varsEnterprise := Variables{"uid": "acme", "hubId": "main-hub", "planId": "enterprise"}
				result, err := engine.Render(template, varsEnterprise)

				By("Then deployments array should be included")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("deployments:"))
				Expect(result).To(ContainSubstring("id: redis"))
				Expect(result).To(ContainSubstring("acme-redis"))
				Expect(result).To(ContainSubstring("redis:7-alpine"))
			})

			It("Should exclude deployments array for basic plan", func() {
				By("Given a LynqForm YAML template with conditional deployments")
				template := `apiVersion: lynq.sh/v1
kind: LynqForm
metadata:
  name: {{ .uid }}-template
spec:
  hubId: {{ .hubId }}
  {{- if or (eq .planId "premium") (eq .planId "enterprise") }}
  deployments:
  - id: redis
    nameTemplate: "{{ .uid }}-redis"
    spec:
      apiVersion: apps/v1
      kind: Deployment
      spec:
        replicas: 1
  {{- end }}
  services:
  - id: app-service
    nameTemplate: "{{ .uid }}-svc"
    spec:
      apiVersion: v1
      kind: Service`

				By("When rendering for basic plan")
				varsBasic := Variables{"uid": "acme", "hubId": "main-hub", "planId": "basic"}
				result, err := engine.Render(template, varsBasic)

				By("Then deployments array should be excluded")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(ContainSubstring("deployments:"))
				Expect(result).ToNot(ContainSubstring("redis"))
				Expect(result).To(ContainSubstring("services:"))
			})
		})

		Describe("Multiple conditional resource arrays", func() {
			It("Should conditionally include multiple resource types", func() {
				By("Given a template with multiple conditional arrays")
				template := `apiVersion: lynq.sh/v1
kind: LynqForm
metadata:
  name: {{ .uid }}-template
spec:
  hubId: {{ .hubId }}
  services:
  - id: main-service
    nameTemplate: "{{ .uid }}-main"
  {{- if .enableCache }}
  deployments:
  - id: redis
    nameTemplate: "{{ .uid }}-redis"
  {{- end }}
  {{- if .enableDatabase }}
  statefulSets:
  - id: postgres
    nameTemplate: "{{ .uid }}-db"
  {{- end }}
  {{- if .enableMonitoring }}
  configMaps:
  - id: prometheus-config
    nameTemplate: "{{ .uid }}-prometheus"
  {{- end }}`

				By("When all features are enabled")
				varsAll := Variables{
					"uid":              "acme",
					"hubId":            "main",
					"enableCache":      true,
					"enableDatabase":   true,
					"enableMonitoring": true,
				}
				result, err := engine.Render(template, varsAll)

				By("Then all resource arrays should be present")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("services:"))
				Expect(result).To(ContainSubstring("deployments:"))
				Expect(result).To(ContainSubstring("statefulSets:"))
				Expect(result).To(ContainSubstring("configMaps:"))

				By("When only cache is enabled")
				varsCache := Variables{
					"uid":              "acme",
					"hubId":            "main",
					"enableCache":      true,
					"enableDatabase":   false,
					"enableMonitoring": false,
				}
				result, err = engine.Render(template, varsCache)

				By("Then only services and deployments should be present")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("services:"))
				Expect(result).To(ContainSubstring("deployments:"))
				Expect(result).ToNot(ContainSubstring("statefulSets:"))
				Expect(result).ToNot(ContainSubstring("configMaps:"))
			})
		})

		Describe("Conditional resource array items", func() {
			It("Should add items to deployments array conditionally", func() {
				By("Given a template with base deployment and conditional additions")
				template := `apiVersion: lynq.sh/v1
kind: LynqForm
spec:
  deployments:
  - id: app
    nameTemplate: "{{ .uid }}-app"
    spec:
      apiVersion: apps/v1
      kind: Deployment
  {{- if eq .planId "enterprise" }}
  - id: redis
    nameTemplate: "{{ .uid }}-redis"
    spec:
      apiVersion: apps/v1
      kind: Deployment
  {{- end }}
  {{- if .enableQueue }}
  - id: rabbitmq
    nameTemplate: "{{ .uid }}-queue"
    spec:
      apiVersion: apps/v1
      kind: Deployment
  {{- end }}`

				By("When enterprise plan with queue enabled")
				varsFull := Variables{"uid": "acme", "planId": "enterprise", "enableQueue": true}
				result, err := engine.Render(template, varsFull)

				By("Then all three deployments should be present")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("id: app"))
				Expect(result).To(ContainSubstring("id: redis"))
				Expect(result).To(ContainSubstring("id: rabbitmq"))

				By("When basic plan without queue")
				varsBasic := Variables{"uid": "acme", "planId": "basic", "enableQueue": false}
				result, err = engine.Render(template, varsBasic)

				By("Then only base deployment should be present")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("id: app"))
				Expect(result).ToNot(ContainSubstring("id: redis"))
				Expect(result).ToNot(ContainSubstring("id: rabbitmq"))
			})
		})

		Describe("Conditional with plan-based architecture", func() {
			It("Should render different architectures based on plan", func() {
				By("Given a template with plan-based resource selection")
				template := `apiVersion: lynq.sh/v1
kind: LynqForm
metadata:
  name: {{ .uid }}-form
spec:
  hubId: {{ .hubId }}
  {{- if eq .planId "enterprise" }}
  # Enterprise: Full stack with cache, queue, and monitoring
  deployments:
  - id: app
    nameTemplate: "{{ .uid }}-app"
  - id: redis
    nameTemplate: "{{ .uid }}-cache"
  - id: worker
    nameTemplate: "{{ .uid }}-worker"
  statefulSets:
  - id: postgres
    nameTemplate: "{{ .uid }}-db"
  {{- else if eq .planId "premium" }}
  # Premium: App with database
  deployments:
  - id: app
    nameTemplate: "{{ .uid }}-app"
  statefulSets:
  - id: postgres
    nameTemplate: "{{ .uid }}-db"
  {{- else }}
  # Basic: Just the app
  deployments:
  - id: app
    nameTemplate: "{{ .uid }}-app"
  {{- end }}
  services:
  - id: main-service
    nameTemplate: "{{ .uid }}-svc"`

				By("When rendering for enterprise plan")
				varsEnterprise := Variables{"uid": "acme", "hubId": "prod", "planId": "enterprise"}
				result, err := engine.Render(template, varsEnterprise)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("id: app"))
				Expect(result).To(ContainSubstring("id: redis"))
				Expect(result).To(ContainSubstring("id: worker"))
				Expect(result).To(ContainSubstring("statefulSets:"))

				By("When rendering for premium plan")
				varsPremium := Variables{"uid": "acme", "hubId": "prod", "planId": "premium"}
				result, err = engine.Render(template, varsPremium)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("id: app"))
				Expect(result).ToNot(ContainSubstring("id: redis"))
				Expect(result).To(ContainSubstring("statefulSets:"))

				By("When rendering for basic plan")
				varsBasic := Variables{"uid": "acme", "hubId": "prod", "planId": "basic"}
				result, err = engine.Render(template, varsBasic)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("id: app"))
				Expect(result).ToNot(ContainSubstring("id: redis"))
				Expect(result).ToNot(ContainSubstring("statefulSets:"))
			})
		})

		Describe("Conditional Ingress based on environment", func() {
			It("Should include Ingress only for production", func() {
				By("Given a template with conditional Ingress")
				template := `apiVersion: lynq.sh/v1
kind: LynqForm
spec:
  hubId: {{ .hubId }}
  services:
  - id: app-service
    nameTemplate: "{{ .uid }}-svc"
  {{- if eq .environment "production" }}
  ingresses:
  - id: app-ingress
    nameTemplate: "{{ .uid }}-ingress"
    spec:
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      spec:
        rules:
        - host: "{{ .uid }}.example.com"
  {{- end }}`

				By("When environment is production")
				varsProd := Variables{"uid": "acme", "hubId": "main", "environment": "production"}
				result, err := engine.Render(template, varsProd)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("ingresses:"))
				Expect(result).To(ContainSubstring("id: app-ingress"))

				By("When environment is staging")
				varsStaging := Variables{"uid": "acme", "hubId": "main", "environment": "staging"}
				result, err = engine.Render(template, varsStaging)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(ContainSubstring("ingresses:"))
			})
		})

		Describe("Conditional PersistentVolumeClaims", func() {
			It("Should include PVCs only when persistence is enabled", func() {
				By("Given a template with conditional PVCs")
				template := `apiVersion: lynq.sh/v1
kind: LynqForm
spec:
  deployments:
  - id: app
    nameTemplate: "{{ .uid }}-app"
  {{- if .enablePersistence }}
  persistentVolumeClaims:
  - id: data-volume
    nameTemplate: "{{ .uid }}-data"
    spec:
      apiVersion: v1
      kind: PersistentVolumeClaim
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: {{ .storageSize | default "10Gi" }}
  {{- end }}`

				By("When persistence is enabled")
				varsPersistence := Variables{"uid": "acme", "enablePersistence": true, "storageSize": "50Gi"}
				result, err := engine.Render(template, varsPersistence)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("persistentVolumeClaims:"))
				Expect(result).To(ContainSubstring("storage: 50Gi"))

				By("When persistence is disabled")
				varsNoPersistence := Variables{"uid": "acme", "enablePersistence": false}
				result, err = engine.Render(template, varsNoPersistence)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(ContainSubstring("persistentVolumeClaims:"))
			})
		})

		Describe("Region-based resource selection", func() {
			It("Should include region-specific resources", func() {
				By("Given a template with region-based conditionals")
				template := `apiVersion: lynq.sh/v1
kind: LynqForm
spec:
  deployments:
  - id: app
    nameTemplate: "{{ .uid }}-app"
  {{- if eq .region "us-east-1" }}
  configMaps:
  - id: aws-config
    nameTemplate: "{{ .uid }}-aws"
    spec:
      apiVersion: v1
      kind: ConfigMap
      data:
        region: us-east-1
        availability_zone: us-east-1a
  {{- else if eq .region "eu-west-1" }}
  configMaps:
  - id: aws-config
    nameTemplate: "{{ .uid }}-aws"
    spec:
      apiVersion: v1
      kind: ConfigMap
      data:
        region: eu-west-1
        availability_zone: eu-west-1a
  {{- end }}`

				By("When region is us-east-1")
				varsUS := Variables{"uid": "acme", "region": "us-east-1"}
				result, err := engine.Render(template, varsUS)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("configMaps:"))
				Expect(result).To(ContainSubstring("region: us-east-1"))

				By("When region is eu-west-1")
				varsEU := Variables{"uid": "acme", "region": "eu-west-1"}
				result, err = engine.Render(template, varsEU)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(ContainSubstring("configMaps:"))
				Expect(result).To(ContainSubstring("region: eu-west-1"))

				By("When region is not specified")
				varsNoRegion := Variables{"uid": "acme"}
				result, err = engine.Render(template, varsNoRegion)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(ContainSubstring("configMaps:"))
			})
		})
	})

	Context("Edge Cases and Error Handling", func() {
		Describe("Empty string handling", func() {
			It("Should handle empty template strings", func() {
				By("Given an empty template")
				template := ""
				vars := Variables{"uid": "test"}

				By("When rendering empty template")
				result, err := engine.Render(template, vars)

				By("Then empty string should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(""))
			})

			It("Should handle empty variable values", func() {
				By("Given a template with empty variable")
				template := `{{ .uid }}-{{ .suffix }}`
				vars := Variables{"uid": "test", "suffix": ""}

				By("When rendering with empty suffix")
				result, err := engine.Render(template, vars)

				By("Then empty value should be used")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("test-"))
			})
		})

		Describe("Special character handling", func() {
			It("Should handle dots in resource identifiers", func() {
				By("Given a UID with dots")
				template := `{{ .uid | replace "." "-" }}`
				vars := Variables{"uid": "node.example.com"}

				By("When replacing dots with dashes")
				result, err := engine.Render(template, vars)

				By("Then dots should be replaced for K8s compatibility")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("node-example-com"))
			})
		})

		Describe("URL edge cases", func() {
			It("Should handle URLs with ports in toHost", func() {
				By("Given a URL with port")
				template := `{{ .url | toHost }}` //nolint:goconst
				vars := Variables{"url": "https://example.com:8080/path"}

				By("When extracting host")
				result, err := engine.Render(template, vars)

				By("Then host without port should be returned")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("example.com"))
			})

			It("Should handle plain hostnames in toHost", func() {
				By("Given a plain hostname")
				template := `{{ .host | toHost }}`
				vars := Variables{"host": "example.com"}

				By("When extracting host")
				result, err := engine.Render(template, vars)

				By("Then hostname should be returned as-is")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("example.com"))
			})
		})
	})
})
