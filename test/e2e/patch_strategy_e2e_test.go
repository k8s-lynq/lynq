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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8s-lynq/lynq/test/utils"
)

var _ = Describe("PatchStrategy", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up test table")
		testTable = setupTestTable("patch_strategy")
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	Context("PatchStrategy", func() {
		Describe("Replace strategy", func() {
			const (
				hubName       = "policy-hub-replace"
				formName      = "policy-form-replace"
				nodeName      = "test-node-replace"
				uid           = "test-uid-replace"
				configMapName = "test-uid-replace-config-replace"
			)

			BeforeEach(func() {
				createHubWithTable(hubName, testTable)
				createForm(formName, hubName, `
  configMaps:
    - id: config-replace
      nameTemplate: "{{ .uid }}-config-replace"
      patchStrategy: replace
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: managed-value
`)
			})

			AfterEach(func() {
				By("cleaning up test data and resources")
				deleteTestDataFromTable(testTable, uid)

				cmd := exec.Command("kubectl", "delete", "configmap", configMapName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				// Delete LynqForm (LynqNode will be auto-cleaned)
				cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(5 * time.Second)
			})

			It("should replace the entire resource, removing unspecified fields", func() {
				By("Given test data in MySQL with active=true")
				insertTestDataToTable(testTable, uid, true)

				By("When LynqHub controller creates LynqNode automatically")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the ConfigMap resource should be created")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the resource is manually modified to add an extra field")
				cmd := exec.Command("kubectl", "patch", "configmap", configMapName, "-n", policyTestNamespace,
					"--type=merge", "-p", `{"data":{"extra":"manual-field"}}`)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				// Verify manual change took effect
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.data.extra}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("manual-field"))

				By("When the template is updated (triggering reconciliation)")
				createForm(formName, hubName, `
  configMaps:
    - id: config-replace
      nameTemplate: "{{ .uid }}-config-replace"
      patchStrategy: replace
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: managed-value-updated
`)

				By("Then the extra field should be REMOVED (full replacement)")
				Eventually(func(g Gomega) {
					// Check managed value updated
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.data.key}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("managed-value-updated"))

					// Check extra field is GONE
					cmd = exec.Command("kubectl", "get", "configmap", configMapName,
						"-n", policyTestNamespace, "-o", "jsonpath={.data.extra}")
					output, _ = utils.Run(cmd)
					g.Expect(output).To(BeEmpty())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Apply strategy (SSA - default)", func() {
			const (
				hubName       = "policy-hub-apply"
				formName      = "policy-form-apply"
				uid           = "test-uid-apply"
				configMapName = "test-uid-apply-config-apply"
			)

			BeforeEach(func() {
				createHubWithTable(hubName, testTable)
				createForm(formName, hubName, `
  configMaps:
    - id: config-apply
      nameTemplate: "{{ .uid }}-config-apply"
      patchStrategy: apply
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          managed-key: managed-value
`)
			})

			AfterEach(func() {
				By("cleaning up test data and resources")
				deleteTestDataFromTable(testTable, uid)

				cmd := exec.Command("kubectl", "delete", "configmap", configMapName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(5 * time.Second)
			})

			It("should use Server-Side Apply and preserve fields managed by other controllers", func() {
				By("Given test data in MySQL with active=true")
				insertTestDataToTable(testTable, uid, true)

				By("When LynqHub controller creates LynqNode automatically")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the ConfigMap resource should be created")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And when a different field manager adds a field")
				// Use kubectl apply with a different field manager
				patchYAML := fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  other-key: other-value
`, configMapName, policyTestNamespace)
				cmd := exec.Command("kubectl", "apply", "-f", "-", "--field-manager=other-controller", "--server-side=true")
				cmd.Stdin = utils.StringReader(patchYAML)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				// Verify other field was added
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.other-key}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("other-value"))

				By("When the template is updated (triggering reconciliation with SSA)")
				createForm(formName, hubName, `
  configMaps:
    - id: config-apply
      nameTemplate: "{{ .uid }}-config-apply"
      patchStrategy: apply
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          managed-key: updated-managed-value
`)

				By("Then the managed field should be updated")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.managed-key}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("updated-managed-value"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the field managed by another controller should be preserved")
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.other-key}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("other-value"))
			})
		})

		Describe("Merge strategy", func() {
			const (
				hubName       = "policy-hub-merge"
				formName      = "policy-form-merge"
				uid           = "test-uid-merge"
				configMapName = "test-uid-merge-config-merge"
			)

			BeforeEach(func() {
				createHubWithTable(hubName, testTable)
				createForm(formName, hubName, `
  configMaps:
    - id: config-merge
      nameTemplate: "{{ .uid }}-config-merge"
      patchStrategy: merge
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: managed-value
`)
			})

			AfterEach(func() {
				By("cleaning up test data and resources")
				deleteTestDataFromTable(testTable, uid)

				cmd := exec.Command("kubectl", "delete", "configmap", configMapName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(5 * time.Second)
			})

			It("should use strategic merge patch and preserve existing fields", func() {
				By("Given test data in MySQL with active=true")
				insertTestDataToTable(testTable, uid, true)

				By("When LynqHub controller creates LynqNode automatically")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the ConfigMap resource should be created")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the resource is manually modified to add an extra field")
				cmd := exec.Command("kubectl", "patch", "configmap", configMapName, "-n", policyTestNamespace,
					"--type=merge", "-p", `{"data":{"extra":"manual-field"}}`)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				// Verify manual change took effect
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.extra}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("manual-field"))

				By("When the template is updated (triggering reconciliation with merge)")
				createForm(formName, hubName, `
  configMaps:
    - id: config-merge
      nameTemplate: "{{ .uid }}-config-merge"
      patchStrategy: merge
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: managed-value-updated
`)

				By("Then the managed field should be updated")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.key}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("managed-value-updated"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the extra field should still exist (merged)")
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.extra}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("manual-field"))
			})
		})
	})
})
