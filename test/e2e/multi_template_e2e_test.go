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

var _ = Describe("Multi-Template Support", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up test table")
		testTable = setupTestTable("multi_template")
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	Context("when multiple LynqForms reference the same LynqHub", func() {
		const (
			hubName   = "multi-template-hub"
			formName1 = "web-tier"
			formName2 = "worker-tier"
			uid1      = "tenant-alpha"
			uid2      = "tenant-beta"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHubWithTable(hubName, testTable)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestDataFromTable(testTable, uid1)
			deleteTestDataFromTable(testTable, uid2)

			// Delete all ConfigMaps created by the test
			cmd := exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			// Delete LynqForms
			cmd = exec.Command("kubectl", "delete", "lynqform", formName1, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "lynqform", formName2, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			// Delete LynqHub
			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(5 * time.Second)
		})

		Describe("LynqNode creation for multiple templates", func() {
			It("should create separate LynqNodes for each template-row combination", func() {
				By("Given two LynqForms referencing the same Hub")
				createForm(formName1, hubName, `
  configMaps:
    - id: web-config
      nameTemplate: "{{ .uid }}-web-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tier: web
`)
				createForm(formName2, hubName, `
  configMaps:
    - id: worker-config
      nameTemplate: "{{ .uid }}-worker-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tier: worker
`)

				By("And two active rows in MySQL")
				insertTestDataToTable(testTable, uid1, true)
				insertTestDataToTable(testTable, uid2, true)

				By("When LynqHub controller syncs")
				// Wait for sync to happen (syncInterval: 5s)
				time.Sleep(7 * time.Second)

				By("Then 4 LynqNodes should be created (2 templates × 2 rows)")
				expectedNodes := []string{
					fmt.Sprintf("%s-%s", uid1, formName1), // tenant-alpha-web-tier
					fmt.Sprintf("%s-%s", uid1, formName2), // tenant-alpha-worker-tier
					fmt.Sprintf("%s-%s", uid2, formName1), // tenant-beta-web-tier
					fmt.Sprintf("%s-%s", uid2, formName2), // tenant-beta-worker-tier
				}

				for _, nodeName := range expectedNodes {
					Eventually(func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace)
						_, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "LynqNode %s should exist", nodeName)
					}, policyTestTimeout, policyTestInterval).Should(Succeed())
				}

				By("And each LynqNode should create its corresponding ConfigMap")
				expectedConfigMaps := []struct {
					name string
					tier string
				}{
					{fmt.Sprintf("%s-web-config", uid1), "web"},
					{fmt.Sprintf("%s-worker-config", uid1), "worker"},
					{fmt.Sprintf("%s-web-config", uid2), "web"},
					{fmt.Sprintf("%s-worker-config", uid2), "worker"},
				}

				for _, cm := range expectedConfigMaps {
					Eventually(func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "configmap", cm.name, "-n", policyTestNamespace,
							"-o", "jsonpath={.data.tier}")
						output, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "ConfigMap %s should exist", cm.name)
						g.Expect(output).To(Equal(cm.tier))
					}, policyTestTimeout, policyTestInterval).Should(Succeed())
				}
			})
		})

		Describe("LynqHub status fields", func() {
			It("should correctly report referencingTemplates, desired, and ready counts", func() {
				By("Given two LynqForms referencing the same Hub")
				createForm(formName1, hubName, `
  configMaps:
    - id: web-config
      nameTemplate: "{{ .uid }}-web-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tier: web
`)
				createForm(formName2, hubName, `
  configMaps:
    - id: worker-config
      nameTemplate: "{{ .uid }}-worker-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tier: worker
`)

				By("And two active rows in MySQL")
				insertTestDataToTable(testTable, uid1, true)
				insertTestDataToTable(testTable, uid2, true)

				By("When all LynqNodes become Ready")
				expectedNodes := []string{
					fmt.Sprintf("%s-%s", uid1, formName1),
					fmt.Sprintf("%s-%s", uid1, formName2),
					fmt.Sprintf("%s-%s", uid2, formName1),
					fmt.Sprintf("%s-%s", uid2, formName2),
				}

				for _, nodeName := range expectedNodes {
					Eventually(func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
							"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
						output, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(output).To(Equal("True"))
					}, policyTestTimeout, policyTestInterval).Should(Succeed())
				}

				By("Then LynqHub status.referencingTemplates should be 2")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqhub", hubName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.referencingTemplates}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("2"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And LynqHub status.desired should be 4 (2 templates × 2 rows)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqhub", hubName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.desired}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("4"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And LynqHub status.ready should be 4")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqhub", hubName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.ready}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("4"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("LynqNode garbage collection on row deletion", func() {
			It("should delete all LynqNodes for a row when it becomes inactive", func() {
				By("Given two LynqForms and one active row")
				createForm(formName1, hubName, `
  configMaps:
    - id: web-config
      nameTemplate: "{{ .uid }}-web-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tier: web
`)
				createForm(formName2, hubName, `
  configMaps:
    - id: worker-config
      nameTemplate: "{{ .uid }}-worker-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tier: worker
`)
				insertTestDataToTable(testTable, uid1, true)

				By("And LynqNodes are created for both templates")
				node1 := fmt.Sprintf("%s-%s", uid1, formName1)
				node2 := fmt.Sprintf("%s-%s", uid1, formName2)
				waitForLynqNode(node1)
				waitForLynqNode(node2)

				By("When the row is deleted from MySQL")
				deleteTestDataFromTable(testTable, uid1)

				By("Then both LynqNodes should be deleted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", node1, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred()) // Should not exist
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", node2, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred()) // Should not exist
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Template removal from Hub", func() {
			It("should delete LynqNodes when a template no longer references the Hub", func() {
				By("Given two LynqForms and one active row")
				createForm(formName1, hubName, `
  configMaps:
    - id: web-config
      nameTemplate: "{{ .uid }}-web-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tier: web
`)
				createForm(formName2, hubName, `
  configMaps:
    - id: worker-config
      nameTemplate: "{{ .uid }}-worker-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tier: worker
`)
				insertTestDataToTable(testTable, uid1, true)

				By("And LynqNodes exist for both templates")
				node1 := fmt.Sprintf("%s-%s", uid1, formName1)
				node2 := fmt.Sprintf("%s-%s", uid1, formName2)
				waitForLynqNode(node1)
				waitForLynqNode(node2)

				By("When one LynqForm is deleted")
				cmd := exec.Command("kubectl", "delete", "lynqform", formName2, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("Then only the LynqNodes for the remaining template should exist")
				// node1 should still exist
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", node1, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 10*time.Second, policyTestInterval).Should(Succeed())

				// node2 should be deleted
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", node2, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred()) // Should not exist
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And LynqHub status should reflect the change")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqhub", hubName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.referencingTemplates}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("1"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})
	})
})
