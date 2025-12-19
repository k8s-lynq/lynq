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

var _ = Describe("DeletionPolicy", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up test table")
		testTable = setupTestTable("deletion_policy")
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	Describe("Delete policy", func() {
		const (
			hubName       = "policy-hub-delete"
			formName      = "policy-form-delete"
			uid           = "test-uid-delete"
			configMapName = "test-uid-delete-config-delete"
		)

		BeforeEach(func() {
			createHubWithTable(hubName, testTable)
			createForm(formName, hubName, `
  configMaps:
    - id: config-delete
      nameTemplate: "{{ .uid }}-config-delete"
      deletionPolicy: Delete
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
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

		It("should use ownerReference for automatic garbage collection", func() {
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

			By("And the resource should have an OwnerReference pointing to the LynqNode")
			cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.metadata.ownerReferences[0].kind}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("LynqNode"))

			By("When the MySQL data is deleted (simulating node deactivation)")
			deleteTestDataFromTable(testTable, uid)

			By("Then the LynqHub controller should delete the LynqNode")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred()) // Should not exist
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the ConfigMap should be automatically deleted via ownerReference")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred()) // Should not exist
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		})
	})

	Describe("Retain policy", func() {
		const (
			hubName       = "policy-hub-retain"
			formName      = "policy-form-retain"
			uid           = "test-uid-retain"
			configMapName = "test-uid-retain-config-retain"
		)

		BeforeEach(func() {
			createHubWithTable(hubName, testTable)
			createForm(formName, hubName, `
  configMaps:
    - id: config-retain
      nameTemplate: "{{ .uid }}-config-retain"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)
		})

		AfterEach(func() {
			By("cleaning up test data and resources (manual cleanup for Retain policy)")
			cmd := exec.Command("kubectl", "delete", "configmap", configMapName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			deleteTestDataFromTable(testTable, uid)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(5 * time.Second)
		})

		It("should use label-based tracking and preserve resource on deletion", func() {
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

			By("And the resource should NOT have an OwnerReference")
			cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.metadata.ownerReferences}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(BeEmpty())

			By("And the resource should have tracking labels")
			cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.metadata.labels.lynq\\.sh/node}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(expectedNodeName))

			By("When the MySQL data is deleted (simulating node deactivation)")
			deleteTestDataFromTable(testTable, uid)

			By("Then the LynqHub controller should delete the LynqNode")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred()) // Should not exist
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("But the ConfigMap should still exist (retained)")
			Consistently(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}, 10*time.Second, policyTestInterval).Should(Succeed())

			By("And the resource should be marked as orphaned")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.metadata.labels.lynq\\.sh/orphaned}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("true"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		})
	})

	Describe("Retain policy with LynqForm/LynqHub deletion and re-adoption", func() {
		const (
			hubName       = "policy-hub-readopt"
			formName      = "policy-form-readopt"
			uid           = "test-uid-readopt"
			configMapName = "test-uid-readopt-config-readopt"
		)

		AfterEach(func() {
			By("cleaning up test data and resources (manual cleanup for Retain policy)")
			cmd := exec.Command("kubectl", "delete", "configmap", configMapName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			deleteTestDataFromTable(testTable, uid)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(5 * time.Second)
		})

		It("should preserve retained resource after LynqForm/LynqHub deletion and re-adopt on recreation", func() {
			By("Given a LynqHub and LynqForm with DeletionPolicy:Retain")
			createHubWithTable(hubName, testTable)
			createForm(formName, hubName, `
  configMaps:
    - id: config-readopt
      nameTemplate: "{{ .uid }}-config-readopt"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: original-value
`)

			By("And active data in MySQL")
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

			By("And wait for LynqNode to be Ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the resource should have tracking labels (no ownerReference for Retain policy)")
			cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
				"-o", "jsonpath={.metadata.labels.lynq\\.sh/node}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(expectedNodeName))

			// Verify data content
			cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
				"-o", "jsonpath={.data.key}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("original-value"))

			By("When the LynqForm is deleted (but MySQL data remains)")
			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("Then the LynqNode should be deleted")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred()) // Should not exist
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("But the ConfigMap should still exist (retained)")
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

			By("And should have orphaned-at annotation")
			cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
				"-o", "jsonpath={.metadata.annotations.lynq\\.sh/orphaned-at}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())

			By("When the LynqForm is recreated with the same configuration")
			createForm(formName, hubName, `
  configMaps:
    - id: config-readopt
      nameTemplate: "{{ .uid }}-config-readopt"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: updated-value
`)

			By("Then LynqNode should be recreated")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the ConfigMap should be re-adopted (orphan markers removed)")
			Eventually(func(g Gomega) {
				// Orphan label should be removed
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.labels.lynq\\.sh/orphaned}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(BeEmpty())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the tracking labels should be updated")
			cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
				"-o", "jsonpath={.metadata.labels.lynq\\.sh/node}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(expectedNodeName))

			By("And the ConfigMap data should be updated to the new value")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.key}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("updated-value"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the LynqNode should become Ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		})

		It("should preserve retained resource after LynqHub deletion and re-adopt on recreation", func() {
			By("Given a LynqHub and LynqForm with DeletionPolicy:Retain")
			createHubWithTable(hubName, testTable)
			createForm(formName, hubName, `
  configMaps:
    - id: config-readopt
      nameTemplate: "{{ .uid }}-config-readopt"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: hub-test-value
`)

			By("And active data in MySQL")
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

			By("And wait for LynqNode to be Ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("When the LynqHub is deleted (this cascades to LynqForm and LynqNode)")
			cmd := exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("Then the LynqNode should be deleted")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred()) // Should not exist
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("But the ConfigMap should still exist (retained)")
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

			By("When both LynqHub and LynqForm are recreated")
			createHubWithTable(hubName, testTable)
			createForm(formName, hubName, `
  configMaps:
    - id: config-readopt
      nameTemplate: "{{ .uid }}-config-readopt"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: hub-recreated-value
`)

			By("Then LynqNode should be recreated")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the ConfigMap should be re-adopted (orphan markers removed)")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.labels.lynq\\.sh/orphaned}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(BeEmpty())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the ConfigMap data should be updated")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.key}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("hub-recreated-value"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the LynqNode should become Ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		})
	})
})
