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

var _ = Describe("CreationPolicy", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up test table")
		testTable = setupTestTable("creation_policy")
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	Describe("Once policy", func() {
		const (
			hubName       = "policy-hub-once"
			formName      = "policy-form-once"
			uid           = "test-uid-once"
			configMapName = "test-uid-once-config-once"
		)

		BeforeEach(func() {
			createHubWithTable(hubName, testTable)
			createForm(formName, hubName, `
  configMaps:
    - id: config-once
      nameTemplate: "{{ .uid }}-config-once"
      creationPolicy: Once
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: initial-value
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

		It("should create resource only once and never update", func() {
			By("Given test data in MySQL with active=true")
			insertTestDataToTable(testTable, uid, true)

			By("When LynqHub controller creates LynqNode automatically")
			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			waitForLynqNode(expectedNodeName)

			By("Then the ConfigMap resource should be created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.data.key}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("initial-value"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the resource should be marked with created-once annotation")
			cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.metadata.annotations.lynq\\.sh/created-once}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("true"))

			By("When the template is updated to change the resource value")
			createForm(formName, hubName, `
  configMaps:
    - id: config-once
      nameTemplate: "{{ .uid }}-config-once"
      creationPolicy: Once
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: updated-value
`)
			time.Sleep(10 * time.Second)

			By("Then the resource should NOT be updated")
			cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.data.key}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("initial-value"))
		})
	})

	Describe("WhenNeeded policy", func() {
		const (
			hubName       = "policy-hub-whenneeded"
			formName      = "policy-form-whenneeded"
			uid           = "test-uid-whenneeded"
			configMapName = "test-uid-whenneeded-config-whenneeded"
		)

		BeforeEach(func() {
			createHubWithTable(hubName, testTable)
			createForm(formName, hubName, `
  configMaps:
    - id: config-whenneeded
      nameTemplate: "{{ .uid }}-config-whenneeded"
      creationPolicy: WhenNeeded
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: initial-value
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

		It("should update resource when spec changes", func() {
			By("Given test data in MySQL with active=true")
			insertTestDataToTable(testTable, uid, true)

			By("When LynqHub controller creates LynqNode automatically")
			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			waitForLynqNode(expectedNodeName)

			By("Then the ConfigMap resource should be created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.data.key}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("initial-value"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("When the template is updated to change the resource value")
			createForm(formName, hubName, `
  configMaps:
    - id: config-whenneeded
      nameTemplate: "{{ .uid }}-config-whenneeded"
      creationPolicy: WhenNeeded
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: updated-value
`)

			By("Then the resource SHOULD be updated")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace, "-o", "jsonpath={.data.key}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("updated-value"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		})
	})
})
