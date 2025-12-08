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

var _ = Describe("Orphan Resource Cleanup", Ordered, func() {
	BeforeAll(func() {
		By("setting up policy test namespace")
		setupPolicyTestNamespace()
	})

	AfterAll(func() {
		By("cleaning up policy test namespace")
		cleanupPolicyTestNamespace()
	})

	Context("when resources are removed from template", func() {
		const (
			hubName  = "orphan-hub"
			formName = "orphan-form"
			uid      = "orphan-test-uid"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestData(uid)

			// Delete all resources created by the test
			cmd := exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			// Delete orphaned resources (by orphan label)
			cmd = exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/orphaned=true", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			// Delete LynqForm
			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			// Delete LynqHub
			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(5 * time.Second)
		})

		Describe("DeletionPolicy=Delete (default)", func() {
			It("should delete orphaned resources when removed from template", func() {
				By("Given a LynqForm with two ConfigMaps (resource-a and resource-b)")
				createForm(formName, hubName, `
  configMaps:
    - id: resource-a
      nameTemplate: "{{ .uid }}-config-a"
      deletionPolicy: Delete
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: resource-a
    - id: resource-b
      nameTemplate: "{{ .uid }}-config-b"
      deletionPolicy: Delete
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: resource-b
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("And both resources are created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapA := fmt.Sprintf("%s-config-a", uid)
				configMapB := fmt.Sprintf("%s-config-b", uid)

				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapA, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapB, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the LynqNode is Ready with AppliedResources populated")
				Eventually(func(g Gomega) {
					// Wait for Ready condition
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				Eventually(func(g Gomega) {
					// Wait for AppliedResources to contain both ConfigMaps
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.appliedResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring(configMapA))
					g.Expect(output).To(ContainSubstring(configMapB))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When resource-b is removed from the template")
				createForm(formName, hubName, `
  configMaps:
    - id: resource-a
      nameTemplate: "{{ .uid }}-config-a"
      deletionPolicy: Delete
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: resource-a
`)

				By("Then resource-a should still exist")
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapA, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 10*time.Second, policyTestInterval).Should(Succeed())

				By("And resource-b should be deleted (orphan cleanup)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapB, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred()) // Should not exist
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("DeletionPolicy=Retain", func() {
			It("should mark orphaned resources with orphan labels/annotations when removed from template", func() {
				By("Given a LynqForm with two ConfigMaps (retain-a and retain-b with Retain policy)")
				createForm(formName, hubName, `
  configMaps:
    - id: retain-a
      nameTemplate: "{{ .uid }}-retain-a"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: retain-a
    - id: retain-b
      nameTemplate: "{{ .uid }}-retain-b"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: retain-b
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("And both resources are created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				retainA := fmt.Sprintf("%s-retain-a", uid)
				retainB := fmt.Sprintf("%s-retain-b", uid)

				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", retainA, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", retainB, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the LynqNode is Ready with AppliedResources populated")
				Eventually(func(g Gomega) {
					// Wait for Ready condition
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				Eventually(func(g Gomega) {
					// Wait for AppliedResources to contain both ConfigMaps
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.appliedResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring(retainA))
					g.Expect(output).To(ContainSubstring(retainB))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When retain-b is removed from the template")
				createForm(formName, hubName, `
  configMaps:
    - id: retain-a
      nameTemplate: "{{ .uid }}-retain-a"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: retain-a
`)

				By("Then retain-a should still exist and be managed")
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", retainA, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 10*time.Second, policyTestInterval).Should(Succeed())

				By("And retain-b should still exist (Retain policy)")
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", retainB, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 10*time.Second, policyTestInterval).Should(Succeed())

				By("And retain-b should be marked with orphan label")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", retainB, "-n", policyTestNamespace,
						"-o", "jsonpath={.metadata.labels.lynq\\.sh/orphaned}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("true"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And retain-b should have orphan-at annotation")
				cmd := exec.Command("kubectl", "get", "configmap", retainB, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.annotations.lynq\\.sh/orphaned-at}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).NotTo(BeEmpty())

				By("And retain-b should have orphan-reason annotation")
				cmd = exec.Command("kubectl", "get", "configmap", retainB, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.annotations.lynq\\.sh/orphaned-reason}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("RemovedFromTemplate"))

				By("And retain-b should have tracking labels removed")
				cmd = exec.Command("kubectl", "get", "configmap", retainB, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.labels.lynq\\.sh/node}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(BeEmpty())
			})
		})

		Describe("Re-adoption of orphaned resources", func() {
			It("should remove orphan markers when resource is re-added to template", func() {
				By("Given a LynqForm with a Retain-policy ConfigMap")
				createForm(formName, hubName, `
  configMaps:
    - id: readopt-config
      nameTemplate: "{{ .uid }}-readopt"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          version: v1
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("And the resource is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				readoptCM := fmt.Sprintf("%s-readopt", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", readoptCM, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the LynqNode is Ready with AppliedResources populated")
				Eventually(func(g Gomega) {
					// Wait for Ready condition
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				Eventually(func(g Gomega) {
					// Wait for AppliedResources to contain the ConfigMap
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.appliedResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring(readoptCM))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When the resource is removed from template (becomes orphaned)")
				createForm(formName, hubName, `
  configMaps: []
`)

				By("Then the resource should be marked as orphaned")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", readoptCM, "-n", policyTestNamespace,
						"-o", "jsonpath={.metadata.labels.lynq\\.sh/orphaned}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("true"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When the resource is re-added to the template")
				createForm(formName, hubName, `
  configMaps:
    - id: readopt-config
      nameTemplate: "{{ .uid }}-readopt"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          version: v2
`)

				By("Then the orphan label should be removed")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", readoptCM, "-n", policyTestNamespace,
						"-o", "jsonpath={.metadata.labels.lynq\\.sh/orphaned}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(BeEmpty())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the orphan annotations should be removed")
				cmd := exec.Command("kubectl", "get", "configmap", readoptCM, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.annotations.lynq\\.sh/orphaned-at}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(BeEmpty())

				By("And the resource should be updated with new content")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", readoptCM, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.version}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("v2"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And tracking labels should be restored")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", readoptCM, "-n", policyTestNamespace,
						"-o", "jsonpath={.metadata.labels.lynq\\.sh/node}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal(expectedNodeName))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})
	})

	Context("appliedResources tracking in status", func() {
		const (
			hubName  = "applied-tracking-hub"
			formName = "applied-tracking-form"
			uid      = "applied-tracking-uid"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestData(uid)

			cmd := exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(5 * time.Second)
		})

		It("should track applied resources in LynqNode status", func() {
			By("Given a LynqForm with multiple resources")
			createForm(formName, hubName, `
  configMaps:
    - id: track-a
      nameTemplate: "{{ .uid }}-track-a"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: track-a
    - id: track-b
      nameTemplate: "{{ .uid }}-track-b"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: track-b
`)

			By("And active data in MySQL")
			insertTestData(uid, true)

			By("When LynqNode reconciles successfully")
			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			waitForLynqNode(expectedNodeName)

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("Then status.appliedResources should contain both resources")
			cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
				"-o", "jsonpath={.status.appliedResources}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Check format: kind/namespace/name@id
			expectedTrackA := fmt.Sprintf("ConfigMap/%s/%s-track-a@track-a", policyTestNamespace, uid)
			expectedTrackB := fmt.Sprintf("ConfigMap/%s/%s-track-b@track-b", policyTestNamespace, uid)
			Expect(output).To(ContainSubstring(expectedTrackA))
			Expect(output).To(ContainSubstring(expectedTrackB))
		})
	})
})
