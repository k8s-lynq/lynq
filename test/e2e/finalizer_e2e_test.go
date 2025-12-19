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

var _ = Describe("Finalizer Behavior", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up test table")
		testTable = setupTestTable("finalizer")
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	Context("LynqNode finalizer", func() {
		const (
			hubName       = "finalizer-hub"
			formName      = "finalizer-form"
			uid           = "finalizer-test-uid"
			finalizerName = "lynqnode.operator.lynq.sh/finalizer"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHubWithTable(hubName, testTable)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestDataFromTable(testTable, uid)

			// Clean up any remaining resources
			cmd := exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/orphaned=true", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(5 * time.Second)
		})

		Describe("Finalizer presence", func() {
			It("should add finalizer to LynqNode on creation", func() {
				By("Given a LynqForm with resources")
				createForm(formName, hubName, `
  configMaps:
    - id: finalizer-config
      nameTemplate: "{{ .uid }}-finalizer-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)

				By("And active data in MySQL")
				insertTestDataToTable(testTable, uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the LynqNode should have the finalizer")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.metadata.finalizers}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring(finalizerName))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Cleanup on deletion with DeletionPolicy=Delete", func() {
			It("should delete resources and remove finalizer when LynqNode is deleted", func() {
				By("Given a LynqForm with DeletionPolicy=Delete")
				createForm(formName, hubName, `
  configMaps:
    - id: delete-config
      nameTemplate: "{{ .uid }}-delete-config"
      deletionPolicy: Delete
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)

				By("And active data in MySQL")
				insertTestDataToTable(testTable, uid, true)

				By("And LynqNode and its resources are created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-delete-config", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When MySQL data is deleted (triggering LynqNode deletion)")
				deleteTestDataFromTable(testTable, uid)

				By("Then the LynqNode should be deleted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should be deleted (via ownerReference)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Cleanup on deletion with DeletionPolicy=Retain", func() {
			It("should mark resources as orphaned and remove finalizer when LynqNode is deleted", func() {
				By("Given a LynqForm with DeletionPolicy=Retain")
				createForm(formName, hubName, `
  configMaps:
    - id: retain-config
      nameTemplate: "{{ .uid }}-retain-config"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)

				By("And active data in MySQL")
				insertTestDataToTable(testTable, uid, true)

				By("And LynqNode and its resources are created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-retain-config", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When MySQL data is deleted (triggering LynqNode deletion)")
				deleteTestDataFromTable(testTable, uid)

				By("Then the LynqNode should be deleted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should still exist")
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 10*time.Second, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should be marked as orphaned")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.metadata.labels.lynq\\.sh/orphaned}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("true"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should have orphan reason annotation")
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.annotations.lynq\\.sh/orphaned-reason}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("LynqNodeDeleted"))
			})
		})

		Describe("Mixed deletion policies", func() {
			It("should handle resources with different deletion policies correctly", func() {
				By("Given a LynqForm with mixed deletion policies")
				createForm(formName, hubName, `
  configMaps:
    - id: delete-config
      nameTemplate: "{{ .uid }}-to-delete"
      deletionPolicy: Delete
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          policy: delete
    - id: retain-config
      nameTemplate: "{{ .uid }}-to-retain"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          policy: retain
`)

				By("And active data in MySQL")
				insertTestDataToTable(testTable, uid, true)

				By("And both resources are created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				deleteConfigMap := fmt.Sprintf("%s-to-delete", uid)
				retainConfigMap := fmt.Sprintf("%s-to-retain", uid)

				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", deleteConfigMap, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", retainConfigMap, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When MySQL data is deleted")
				deleteTestDataFromTable(testTable, uid)

				By("Then the LynqNode should be deleted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the Delete-policy ConfigMap should be deleted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", deleteConfigMap, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the Retain-policy ConfigMap should still exist and be orphaned")
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", retainConfigMap, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 10*time.Second, policyTestInterval).Should(Succeed())

				cmd := exec.Command("kubectl", "get", "configmap", retainConfigMap, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.labels.lynq\\.sh/orphaned}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("true"))
			})
		})

		Describe("Finalizer prevents premature deletion", func() {
			It("should block LynqNode deletion until cleanup is complete", func() {
				By("Given a LynqForm with multiple resources")
				createForm(formName, hubName, `
  configMaps:
    - id: config-1
      nameTemplate: "{{ .uid }}-config-1"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          id: "1"
    - id: config-2
      nameTemplate: "{{ .uid }}-config-2"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          id: "2"
`)

				By("And active data in MySQL")
				insertTestDataToTable(testTable, uid, true)

				By("And LynqNode is Ready with all resources")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.readyResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("2"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When MySQL data is deleted")
				deleteTestDataFromTable(testTable, uid)

				By("Then the LynqNode deletion should complete (finalizer handled cleanup)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred()) // Should not exist
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And all resources should be cleaned up")
				config1 := fmt.Sprintf("%s-config-1", uid)
				config2 := fmt.Sprintf("%s-config-2", uid)

				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", config1, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", config2, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})
	})
})
