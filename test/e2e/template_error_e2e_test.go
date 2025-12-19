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

package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8s-lynq/lynq/test/utils"
)

var _ = Describe("Template Error Handling", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up test table")
		testTable = setupTestTable("template_error")
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	Context("Template Rendering Errors", func() {
		Describe("non-existent variable reference", func() {
			const (
				hubName  = "template-error-hub"
				formName = "template-error-form"
				uid      = "template-error-tenant"
			)

			BeforeEach(func() {
				createHubWithTable(hubName, testTable)
			})

			AfterEach(func() {
				deleteTestDataFromTable(testTable, uid)

				cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				// Delete any LynqNodes that may have been created
				cmd = exec.Command("kubectl", "delete", "lynqnodes", "--all", "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(3 * time.Second)
			})

			It("should mark LynqNode as Degraded when template references non-existent variable", func() {
				By("Given a LynqForm that references a non-existent variable {{ .nonExistentVar }}")
				createForm(formName, hubName, `
  configMaps:
    - id: test-config
      nameTemplate: "{{ .uid }}-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tenant-id: "{{ .uid }}"
          missing-value: "{{ .nonExistentVar }}"
`)

				By("And test data exists in the database")
				insertTestDataToTable(testTable, uid, true)

				By("When the LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the LynqNode should have Degraded condition set to True")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Degraded')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the Degraded condition should indicate resource failures")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Degraded')].message}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					// Message should indicate failed resources (template rendering error causes resource failure)
					hasFailure := strings.Contains(strings.ToLower(output), "failed")
					g.Expect(hasFailure).To(BeTrue(), "Degraded message should indicate resource failure: %s", output)
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the LynqNode should have at least 1 failed resource")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.failedResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("1"), "Should have exactly 1 failed resource due to template error")
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should not be created")
				configMapName := fmt.Sprintf("%s-config", uid)
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				Expect(err).To(HaveOccurred(), "ConfigMap should not exist when template rendering fails")
			})
		})
	})

	Context("Template Syntax Validation", func() {
		Describe("invalid template syntax webhook rejection", func() {
			const (
				hubName  = "syntax-validation-hub"
				formName = "syntax-validation-form"
			)

			BeforeEach(func() {
				createHubWithTable(hubName, testTable)
			})

			AfterEach(func() {
				cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(2 * time.Second)
			})

			It("should reject LynqForm with invalid template function in nameTemplate", func() {
				By("Given a LynqForm with invalid template function {{ .uid | invalidFunc }}")
				formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  configMaps:
    - id: test-config
      nameTemplate: "{{ .uid | invalidFunc }}"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          test: "value"
`, formName, policyTestNamespace, hubName)

				By("When attempting to create the LynqForm")
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(formYAML)
				output, err := utils.Run(cmd)

				By("Then the webhook should reject the creation")
				Expect(err).To(HaveOccurred(), "LynqForm with invalid template should be rejected")

				By("And the error message should indicate template parsing failure")
				hasTemplateError := strings.Contains(strings.ToLower(output), "template") ||
					strings.Contains(strings.ToLower(output), "invalid") ||
					strings.Contains(strings.ToLower(output), "function") ||
					strings.Contains(output, "invalidFunc")
				Expect(hasTemplateError).To(BeTrue(), "Error should indicate template issue: %s", output)
			})

			It("should reject LynqForm with malformed template syntax", func() {
				By("Given a LynqForm with malformed template syntax {{ .uid | | trim }}")
				formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  configMaps:
    - id: test-config
      nameTemplate: "{{ .uid | | trim }}"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          test: "value"
`, formName, policyTestNamespace, hubName)

				By("When attempting to create the LynqForm")
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(formYAML)
				output, err := utils.Run(cmd)

				By("Then the webhook should reject the creation")
				Expect(err).To(HaveOccurred(), "LynqForm with malformed template should be rejected")

				By("And the error should be related to template parsing")
				hasParseError := strings.Contains(strings.ToLower(output), "template") ||
					strings.Contains(strings.ToLower(output), "parse") ||
					strings.Contains(strings.ToLower(output), "syntax") ||
					strings.Contains(strings.ToLower(output), "invalid")
				Expect(hasParseError).To(BeTrue(), "Error should indicate parsing issue: %s", output)
			})
		})
	})

	Context("Default Function for Variables", func() {
		Describe("using default function to provide fallback values", func() {
			const (
				hubName  = "default-func-hub"
				formName = "default-func-form"
				uid      = "default-func-tenant"
			)

			BeforeEach(func() {
				// Create standard hub - the `default` function is used to provide fallback values
				// when a variable exists but is empty. With strict template mode (missingkey=error),
				// all variables used in templates must be defined in the hub configuration.
				createHubWithTable(hubName, testTable)

				// Use `default` function with existing variables - the function handles empty/falsy values
				// by providing a fallback. Here we use `.uid` which always has a value, and demonstrate
				// that `default` correctly passes through non-empty values.
				createForm(formName, hubName, `
  configMaps:
    - id: test-config
      nameTemplate: "{{ .uid }}-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tenant-id: "{{ .uid }}"
          uid-with-default: "{{ default \"fallback-value\" .uid }}"
          activate-with-default: "{{ default \"false\" .activate }}"
`)
			})

			AfterEach(func() {
				deleteTestDataFromTable(testTable, uid)

				cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(3 * time.Second)
			})

			It("should use actual value when variable is non-empty, not the default", func() {
				By("Given a LynqForm using default function with existing variables")
				// Form already created in BeforeEach

				By("And test data in database with actual values")
				insertTestDataToTable(testTable, uid, true)

				By("When LynqNode is created and template is rendered")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the LynqNode should become Ready (not Degraded)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				configMapName := fmt.Sprintf("%s-config", uid)

				By("And the ConfigMap should use actual uid value, not the default")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.uid-with-default}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					// uid is non-empty, so default should NOT be used
					g.Expect(output).To(Equal(uid))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the activate value should use actual value, not the default")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.activate-with-default}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					// activate is "1" (truthy), so default should NOT be used
					g.Expect(output).To(Equal("1"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the tenant-id should be correctly set from the variable")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.tenant-id}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal(uid))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})
	})
})
