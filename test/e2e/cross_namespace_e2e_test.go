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

var _ = Describe("Cross-Namespace Resources", Ordered, func() {
	const (
		targetNamespace = "cross-ns-target"
	)

	BeforeAll(func() {
		By("setting up policy test namespace")
		setupPolicyTestNamespace()

		By("creating target namespace for cross-namespace tests")
		cmd := exec.Command("kubectl", "create", "ns", targetNamespace)
		_, _ = utils.Run(cmd) // Ignore error if already exists
	})

	AfterAll(func() {
		By("cleaning up target namespace")
		cmd := exec.Command("kubectl", "delete", "ns", targetNamespace, "--wait=false", "--ignore-not-found=true")
		_, _ = utils.Run(cmd)

		By("cleaning up policy test namespace")
		cleanupPolicyTestNamespace()
	})

	Context("when targetNamespace is specified", func() {
		const (
			hubName  = "cross-ns-hub"
			formName = "cross-ns-form"
			uid      = "cross-ns-uid"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestData(uid)

			// Delete resources in target namespace
			cmd := exec.Command("kubectl", "delete", "configmap", "-n", targetNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			// Delete resources in source namespace
			cmd = exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			// Delete LynqForm
			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			// Delete LynqHub
			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(5 * time.Second)
		})

		Describe("Label-based tracking for cross-namespace resources", func() {
			It("should create resources in targetNamespace with tracking labels instead of ownerReference", func() {
				By("Given a LynqForm with targetNamespace specified")
				formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  configMaps:
    - id: cross-ns-config
      nameTemplate: "{{ .uid }}-cross-config"
      targetNamespace: %s
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          location: cross-namespace
`, formName, policyTestNamespace, hubName, targetNamespace)
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(formYAML)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created and reconciled")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-cross-config", uid)

				By("Then the ConfigMap should be created in the target namespace")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", targetNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should NOT have ownerReference (cross-namespace)")
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", targetNamespace,
					"-o", "jsonpath={.metadata.ownerReferences}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(BeEmpty())

				By("And the ConfigMap should have tracking labels")
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", targetNamespace,
					"-o", "jsonpath={.metadata.labels.lynq\\.sh/node}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal(expectedNodeName))

				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", targetNamespace,
					"-o", "jsonpath={.metadata.labels.lynq\\.sh/node-namespace}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal(policyTestNamespace))
			})
		})

		Describe("Cross-namespace with DeletionPolicy=Delete", func() {
			It("should delete cross-namespace resource when LynqNode is deleted", func() {
				By("Given a cross-namespace resource with DeletionPolicy=Delete")
				formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  configMaps:
    - id: delete-cross-config
      nameTemplate: "{{ .uid }}-delete-cross"
      targetNamespace: %s
      deletionPolicy: Delete
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          policy: delete
`, formName, policyTestNamespace, hubName, targetNamespace)
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(formYAML)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("And the resource is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-delete-cross", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", targetNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When the MySQL data is deleted")
				deleteTestData(uid)

				By("Then the LynqNode should be deleted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the cross-namespace resource should be deleted via finalizer")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", targetNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Cross-namespace with DeletionPolicy=Retain", func() {
			It("should retain cross-namespace resource and mark as orphaned when LynqNode is deleted", func() {
				By("Given a cross-namespace resource with DeletionPolicy=Retain")
				formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  configMaps:
    - id: retain-cross-config
      nameTemplate: "{{ .uid }}-retain-cross"
      targetNamespace: %s
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          policy: retain
`, formName, policyTestNamespace, hubName, targetNamespace)
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(formYAML)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("And the resource is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-retain-cross", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", targetNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When the MySQL data is deleted")
				deleteTestData(uid)

				By("Then the LynqNode should be deleted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("But the cross-namespace resource should still exist")
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", targetNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 10*time.Second, policyTestInterval).Should(Succeed())

				By("And the resource should be marked as orphaned")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", targetNamespace,
						"-o", "jsonpath={.metadata.labels.lynq\\.sh/orphaned}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("true"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the orphan reason should be LynqNodeDeleted")
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", targetNamespace,
					"-o", "jsonpath={.metadata.annotations.lynq\\.sh/orphaned-reason}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("LynqNodeDeleted"))
			})
		})

		Describe("Mixed same-namespace and cross-namespace resources", func() {
			It("should handle both types correctly in the same template", func() {
				By("Given a LynqForm with both same-namespace and cross-namespace resources")
				formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  configMaps:
    - id: same-ns-config
      nameTemplate: "{{ .uid }}-same-ns"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          location: same-namespace
    - id: cross-ns-config
      nameTemplate: "{{ .uid }}-cross-ns"
      targetNamespace: %s
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          location: cross-namespace
`, formName, policyTestNamespace, hubName, targetNamespace)
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(formYAML)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created and reconciled")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				sameNsConfig := fmt.Sprintf("%s-same-ns", uid)
				crossNsConfig := fmt.Sprintf("%s-cross-ns", uid)

				By("Then the same-namespace resource should have ownerReference")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", sameNsConfig, "-n", policyTestNamespace,
						"-o", "jsonpath={.metadata.ownerReferences[0].kind}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("LynqNode"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the cross-namespace resource should have tracking labels instead")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", crossNsConfig, "-n", targetNamespace,
						"-o", "jsonpath={.metadata.ownerReferences}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(BeEmpty())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				cmd = exec.Command("kubectl", "get", "configmap", crossNsConfig, "-n", targetNamespace,
					"-o", "jsonpath={.metadata.labels.lynq\\.sh/node}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal(expectedNodeName))

				By("And LynqNode should be Ready with all 2 resources")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.readyResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("2"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Template variable in targetNamespace", func() {
			It("should support templated targetNamespace", func() {
				By("Given a LynqForm with templated targetNamespace")
				// First create the target namespace matching the template
				dynamicNs := fmt.Sprintf("%s-ns", uid)
				cmd := exec.Command("kubectl", "create", "ns", dynamicNs)
				_, _ = utils.Run(cmd)

				formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  configMaps:
    - id: templated-ns-config
      nameTemplate: "{{ .uid }}-templated"
      targetNamespace: "{{ .uid }}-ns"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          dynamic: "true"
`, formName, policyTestNamespace, hubName)
				cmd = exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(formYAML)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created and reconciled")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-templated", uid)

				By("Then the ConfigMap should be created in the dynamically resolved namespace")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", dynamicNs)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				// Cleanup dynamic namespace
				cmd = exec.Command("kubectl", "delete", "ns", dynamicNs, "--wait=false", "--ignore-not-found=true")
				_, _ = utils.Run(cmd)
			})
		})
	})
})
