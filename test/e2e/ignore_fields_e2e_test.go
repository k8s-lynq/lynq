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

var _ = Describe("Field-Level Ignore Control (ignoreFields)", Ordered, func() {
	BeforeAll(func() {
		By("setting up policy test namespace")
		setupPolicyTestNamespace()
	})

	AfterAll(func() {
		By("cleaning up policy test namespace")
		cleanupPolicyTestNamespace()
	})

	Context("when ignoreFields is configured", func() {
		const (
			hubName  = "ignore-fields-hub"
			formName = "ignore-fields-form"
			uid      = "ignore-test-uid"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestData(uid)

			// Delete all ConfigMaps and Deployments
			cmd := exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "deployment", "-n", policyTestNamespace,
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

		Describe("Preserving externally controlled fields", func() {
			It("should preserve ignored ConfigMap data fields during reconciliation", func() {
				By("Given a LynqForm with ignoreFields for $.data.externalKey")
				createForm(formName, hubName, `
  configMaps:
    - id: ignore-config
      nameTemplate: "{{ .uid }}-ignore-config"
      creationPolicy: WhenNeeded
      ignoreFields:
        - "$.data.externalKey"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          managedKey: managed-value
          externalKey: initial-external-value
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created and reconciled")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-ignore-config", uid)

				By("Then the ConfigMap should be created with all fields")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.managedKey}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("managed-value"))

					cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.externalKey}")
					output, err = utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("initial-external-value"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When the externalKey is manually changed")
				cmd := exec.Command("kubectl", "patch", "configmap", configMapName, "-n", policyTestNamespace,
					"--type=merge", "-p", `{"data":{"externalKey":"manually-changed-value"}}`)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				// Verify the change
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.externalKey}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("manually-changed-value"))

				By("And the template is updated (triggering reconciliation)")
				createForm(formName, hubName, `
  configMaps:
    - id: ignore-config
      nameTemplate: "{{ .uid }}-ignore-config"
      creationPolicy: WhenNeeded
      ignoreFields:
        - "$.data.externalKey"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          managedKey: updated-managed-value
          externalKey: template-external-value
`)

				By("Then the managedKey should be updated")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.managedKey}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("updated-managed-value"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the externalKey should be preserved (not overwritten)")
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.externalKey}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("manually-changed-value"))
				}, 15*time.Second, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Deployment replicas controlled by HPA simulation", func() {
			It("should preserve replicas field when controlled externally", func() {
				By("Given a LynqForm with ignoreFields for $.spec.replicas")
				createForm(formName, hubName, `
  deployments:
    - id: hpa-deployment
      nameTemplate: "{{ .uid }}-hpa-app"
      waitForReady: true
      timeoutSeconds: 120
      ignoreFields:
        - "$.spec.replicas"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: hpa-test
          template:
            metadata:
              labels:
                app: hpa-test
            spec:
              containers:
              - name: app
                image: busybox:1.36
                command: ["sh", "-c", "sleep 3600"]
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				deploymentName := fmt.Sprintf("%s-hpa-app", uid)

				By("Then the Deployment should be created with replicas=1")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
						"-o", "jsonpath={.spec.replicas}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("1"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When replicas is scaled up (simulating HPA)")
				cmd := exec.Command("kubectl", "scale", "deployment", deploymentName, "-n", policyTestNamespace, "--replicas=3")
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				// Verify the scale
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
						"-o", "jsonpath={.spec.replicas}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("3"))
				}, 30*time.Second, policyTestInterval).Should(Succeed())

				By("And the template is updated with new image")
				createForm(formName, hubName, `
  deployments:
    - id: hpa-deployment
      nameTemplate: "{{ .uid }}-hpa-app"
      waitForReady: true
      timeoutSeconds: 120
      ignoreFields:
        - "$.spec.replicas"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: hpa-test
          template:
            metadata:
              labels:
                app: hpa-test
            spec:
              containers:
              - name: app
                image: busybox:1.36.1
                command: ["sh", "-c", "sleep 3600"]
`)

				By("Then the image should be updated")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].image}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("busybox:1.36.1"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And replicas should remain at 3 (HPA controlled)")
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
						"-o", "jsonpath={.spec.replicas}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("3"))
				}, 15*time.Second, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Wildcard ignoreFields", func() {
			It("should preserve all container resources when using wildcard", func() {
				By("Given a LynqForm with wildcard ignoreFields for container resources")
				createForm(formName, hubName, `
  deployments:
    - id: wildcard-deployment
      nameTemplate: "{{ .uid }}-wildcard-app"
      waitForReady: true
      timeoutSeconds: 120
      ignoreFields:
        - "$.spec.template.spec.containers[*].resources"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: wildcard-test
          template:
            metadata:
              labels:
                app: wildcard-test
            spec:
              containers:
              - name: app
                image: busybox:1.36
                command: ["sh", "-c", "sleep 3600"]
                resources:
                  requests:
                    memory: "64Mi"
                    cpu: "100m"
                  limits:
                    memory: "128Mi"
                    cpu: "200m"
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				deploymentName := fmt.Sprintf("%s-wildcard-app", uid)

				By("Then the Deployment should be created with initial resources")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].resources.limits.memory}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("128Mi"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When container resources are manually tuned")
				patchCmd := `{"spec":{"template":{"spec":{"containers":[{"name":"app","resources":{"limits":{"memory":"256Mi","cpu":"500m"}}}]}}}}`
				cmd := exec.Command("kubectl", "patch", "deployment", deploymentName, "-n", policyTestNamespace,
					"--type=strategic", "-p", patchCmd)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				// Wait for patch to be applied
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].resources.limits.memory}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("256Mi"))
				}, 30*time.Second, policyTestInterval).Should(Succeed())

				By("And the template is updated")
				createForm(formName, hubName, `
  deployments:
    - id: wildcard-deployment
      nameTemplate: "{{ .uid }}-wildcard-app"
      waitForReady: true
      timeoutSeconds: 120
      ignoreFields:
        - "$.spec.template.spec.containers[*].resources"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: wildcard-test
          template:
            metadata:
              labels:
                app: wildcard-test
            spec:
              containers:
              - name: app
                image: busybox:1.36.1
                command: ["sh", "-c", "sleep 3600"]
                resources:
                  requests:
                    memory: "64Mi"
                    cpu: "100m"
                  limits:
                    memory: "128Mi"
                    cpu: "200m"
`)

				By("Then the image should be updated")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].image}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("busybox:1.36.1"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the manually tuned resources should be preserved")
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].resources.limits.memory}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("256Mi"))
				}, 15*time.Second, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Non-existent JSONPath graceful handling", func() {
			It("should handle non-existent JSONPath gracefully without errors", func() {
				By("Given a LynqForm with ignoreFields for non-existent path")
				createForm(formName, hubName, `
  configMaps:
    - id: nonexistent-path-config
      nameTemplate: "{{ .uid }}-nonexistent-config"
      ignoreFields:
        - "$.nonexistent.deeply.nested.path"
        - "$.data['nonexistent-key']"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          realKey: real-value
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-nonexistent-config", uid)

				By("Then the ConfigMap should be created successfully")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.realKey}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("real-value"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And LynqNode should be Ready (no errors)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("ignoreFields with CreationPolicy:Once", func() {
			It("should have no effect when combined with CreationPolicy:Once", func() {
				By("Given a LynqForm with CreationPolicy:Once and ignoreFields")
				createForm(formName, hubName, `
  configMaps:
    - id: once-ignore-config
      nameTemplate: "{{ .uid }}-once-ignore-config"
      creationPolicy: Once
      ignoreFields:
        - "$.data.ignoredKey"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: initial-value
          ignoredKey: initial-ignored
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-once-ignore-config", uid)

				By("Then the ConfigMap should be created")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.key}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("initial-value"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the created-once annotation should be set")
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.annotations.lynq\\.sh/created-once}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("true"))

				By("When the template is updated")
				createForm(formName, hubName, `
  configMaps:
    - id: once-ignore-config
      nameTemplate: "{{ .uid }}-once-ignore-config"
      creationPolicy: Once
      ignoreFields:
        - "$.data.ignoredKey"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: updated-value
          ignoredKey: updated-ignored
`)

				By("Then the resource should NOT be updated (Once policy takes precedence)")
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.key}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("initial-value"))
				}, 15*time.Second, policyTestInterval).Should(Succeed())
			})
		})
	})
})
