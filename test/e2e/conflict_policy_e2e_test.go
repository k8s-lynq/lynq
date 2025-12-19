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

var _ = Describe("ConflictPolicy", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up test table")
		testTable = setupTestTable("conflict_policy")
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	Context("ConflictPolicy", func() {
		Describe("Stuck policy", func() {
			const (
				hubName       = "policy-hub-stuck"
				formName      = "policy-form-stuck"
				nodeName      = "test-node-stuck"
				uid           = "test-uid-stuck"
				configMapName = "test-uid-stuck-config-stuck"
			)

			BeforeEach(func() {
				createHubWithTable(hubName, testTable)
				createForm(formName, hubName, `
  configMaps:
    - id: config-stuck
      nameTemplate: "{{ .uid }}-config-stuck"
      conflictPolicy: Stuck
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: managed-value
`)
			})

			AfterEach(func() {
				By("cleaning up test data and resources")
				cmd := exec.Command("kubectl", "delete", "configmap", configMapName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				deleteTestDataFromTable(testTable, uid)

				// Delete LynqForm (LynqNode will be auto-cleaned)
				cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(5 * time.Second)
			})

			It("should stop reconciliation if resource exists with different owner", func() {
				By("Given a pre-existing ConfigMap not managed by Lynq")
				cmYAML := fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  key: existing-value
`, configMapName, policyTestNamespace)
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(cmYAML)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("When test data is inserted and LynqHub creates LynqNode")
				insertTestDataToTable(testTable, uid, true)
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the resource should NOT be updated (remain existing-value)")
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.data.key}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("existing-value"))
				}, 15*time.Second, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Force policy", func() {
			const (
				hubName       = "policy-hub-force"
				formName      = "policy-form-force"
				nodeName      = "test-node-force"
				uid           = "test-uid-force"
				configMapName = "test-uid-force-config-force"
			)

			BeforeEach(func() {
				createHubWithTable(hubName, testTable)
				createForm(formName, hubName, `
  configMaps:
    - id: config-force
      nameTemplate: "{{ .uid }}-config-force"
      conflictPolicy: Force
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: managed-value
`)
			})

			AfterEach(func() {
				By("cleaning up test data and resources")
				cmd := exec.Command("kubectl", "delete", "configmap", configMapName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				deleteTestDataFromTable(testTable, uid)

				// Delete LynqForm (LynqNode will be auto-cleaned)
				cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(5 * time.Second)
			})

			It("should overwrite resource even if it exists with different owner", func() {
				By("Given a pre-existing ConfigMap not managed by Lynq")
				cmYAML := fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  key: existing-value
`, configMapName, policyTestNamespace)
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(cmYAML)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("When test data is inserted and LynqHub creates LynqNode")
				insertTestDataToTable(testTable, uid, true)
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the resource SHOULD be updated to the managed value")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.data.key}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("managed-value"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Stuck policy with recovery", func() {
			const (
				hubName       = "policy-hub-stuck-recovery"
				formName      = "policy-form-stuck-recovery"
				uid           = "test-uid-stuck-recovery"
				configMapName = "test-uid-stuck-recovery-config"
			)

			BeforeEach(func() {
				createHubWithTable(hubName, testTable)
				createForm(formName, hubName, `
  configMaps:
    - id: config-stuck-recovery
      nameTemplate: "{{ .uid }}-config"
      conflictPolicy: Stuck
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: managed-value
`)
			})

			AfterEach(func() {
				By("cleaning up test data and resources")
				cmd := exec.Command("kubectl", "delete", "configmap", configMapName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				deleteTestDataFromTable(testTable, uid)

				cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(5 * time.Second)
			})

			It("should get stuck on conflict and recover when conflict is resolved", func() {
				By("Given a pre-existing ConfigMap not managed by Lynq (conflict)")
				cmYAML := fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
  labels:
    external: "true"
data:
  key: conflicting-value
`, configMapName, policyTestNamespace)
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(cmYAML)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("When test data is inserted and LynqHub creates LynqNode")
				insertTestDataToTable(testTable, uid, true)
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the LynqNode should become Degraded due to conflict")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Degraded')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should remain unchanged (not taken over)")
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.data.key}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("conflicting-value"))

				By("And the ConfigMap should still have the external label")
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.metadata.labels.external}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("true"))

				By("When the conflicting ConfigMap is deleted (resolving the conflict)")
				cmd = exec.Command("kubectl", "delete", "configmap", configMapName, "-n", policyTestNamespace)
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("Then the operator should recover and create the managed ConfigMap")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.data.key}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("managed-value"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the LynqNode should become Ready")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the Degraded condition should be cleared (False or removed)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Degraded')].status}")
					output, _ := utils.Run(cmd)
					// Degraded should be False or empty (removed)
					g.Expect(output).To(Or(Equal("False"), BeEmpty()))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})
	})
})
