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

var _ = Describe("SkipOnDependencyFailure Behavior", Ordered, func() {
	BeforeAll(func() {
		By("setting up policy test namespace")
		setupPolicyTestNamespace()
	})

	AfterAll(func() {
		By("cleaning up policy test namespace")
		cleanupPolicyTestNamespace()
	})

	Context("when skipOnDependencyFailure flag controls dependent resource creation", func() {
		const (
			hubName = "dep-skip-hub"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			// Cleanup all resources created by tests
			cmd := exec.Command("kubectl", "delete", "deployment", "-n", policyTestNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true", "--wait=false")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqform", "--all", "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqnode", "--all", "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(5 * time.Second)
		})

		Describe("default behavior (skipOnDependencyFailure=true)", func() {
			const (
				formName = "skip-default-form"
				uid      = "skip-default-uid"
			)

			AfterEach(func() {
				deleteTestData(uid)
			})

			It("should skip dependent resource when dependency fails and track in status", func() {
				By("Given a LynqForm with dependency chain: failing-deployment -> dependent-config")
				createForm(formName, hubName, `
  deployments:
    - id: failing-deployment
      nameTemplate: "{{ .uid }}-fail-deploy"
      waitForReady: true
      timeoutSeconds: 20
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: fail-test
          template:
            metadata:
              labels:
                app: fail-test
            spec:
              containers:
              - name: app
                image: non-existent-registry.example.com/will-fail:v1
  configMaps:
    - id: dependent-config
      nameTemplate: "{{ .uid }}-dep-config"
      dependIds:
        - failing-deployment
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          status: "should-be-skipped"
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created and dependency fails")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the Deployment should be created (but not ready)")
				deploymentName := fmt.Sprintf("%s-fail-deploy", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the dependent ConfigMap should NOT be created")
				configMapName := fmt.Sprintf("%s-dep-config", uid)
				// Wait sufficient time for timeout to occur
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred()) // Should not exist
				}, 40*time.Second, policyTestInterval).Should(Succeed())

				By("And LynqNode status should show skipped resources")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.skippedResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("1"))
				}, 60*time.Second, policyTestInterval).Should(Succeed())

				By("And skippedResourceIds should contain the dependent resource ID")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.skippedResourceIds}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring("dependent-config"))
				}, 30*time.Second, policyTestInterval).Should(Succeed())

				By("And a DependencySkipped event should be emitted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "events", "-n", policyTestNamespace,
						"--field-selector", fmt.Sprintf("involvedObject.name=%s,reason=DependencySkipped", expectedNodeName))
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring("DependencySkipped"))
				}, 30*time.Second, policyTestInterval).Should(Succeed())
			})
		})

		Describe("skipOnDependencyFailure=false behavior", func() {
			const (
				formName = "skip-false-form"
				uid      = "skip-false-uid"
			)

			AfterEach(func() {
				deleteTestData(uid)
			})

			It("should create dependent resource even when dependency fails", func() {
				By("Given a LynqForm with skipOnDependencyFailure=false on dependent resource")
				createForm(formName, hubName, `
  deployments:
    - id: failing-deployment
      nameTemplate: "{{ .uid }}-fail-deploy"
      waitForReady: true
      timeoutSeconds: 20
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: fail-test
          template:
            metadata:
              labels:
                app: fail-test
            spec:
              containers:
              - name: app
                image: non-existent-registry.example.com/will-fail:v1
  configMaps:
    - id: cleanup-config
      nameTemplate: "{{ .uid }}-cleanup-config"
      dependIds:
        - failing-deployment
      skipOnDependencyFailure: false
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          status: "created-despite-dependency-failure"
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the Deployment should be created")
				deploymentName := fmt.Sprintf("%s-fail-deploy", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should ALSO be created (despite dependency failure)")
				configMapName := fmt.Sprintf("%s-cleanup-config", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 60*time.Second, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should have the expected data")
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.status}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("created-despite-dependency-failure"))

				By("And skippedResources should be 0 (resource was not skipped)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.skippedResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					// Should be 0 or empty (omitempty)
					g.Expect(output).To(SatisfyAny(Equal("0"), Equal("")))
				}, 30*time.Second, policyTestInterval).Should(Succeed())

				By("And a DependencyFailedButProceeding event should be emitted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "events", "-n", policyTestNamespace,
						"--field-selector", fmt.Sprintf("involvedObject.name=%s,reason=DependencyFailedButProceeding", expectedNodeName))
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring("DependencyFailedButProceeding"))
				}, 30*time.Second, policyTestInterval).Should(Succeed())
			})
		})

		Describe("cascading dependency skip behavior", func() {
			const (
				formName = "cascade-skip-form"
				uid      = "cascade-skip-uid"
			)

			AfterEach(func() {
				deleteTestData(uid)
			})

			It("should cascade skip through multiple dependency levels", func() {
				By("Given a LynqForm with chain: A (fails) -> B (skipped) -> C (also skipped)")
				createForm(formName, hubName, `
  deployments:
    - id: resource-a
      nameTemplate: "{{ .uid }}-res-a"
      waitForReady: true
      timeoutSeconds: 20
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: chain-test
          template:
            metadata:
              labels:
                app: chain-test
            spec:
              containers:
              - name: app
                image: non-existent-registry.example.com/will-fail:v1
  configMaps:
    - id: resource-b
      nameTemplate: "{{ .uid }}-res-b"
      dependIds:
        - resource-a
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          level: "b"
    - id: resource-c
      nameTemplate: "{{ .uid }}-res-c"
      dependIds:
        - resource-b
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          level: "c"
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then resource-a (Deployment) should be created")
				deploymentName := fmt.Sprintf("%s-res-a", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And resource-b and resource-c should NOT be created (cascade skip)")
				configMapB := fmt.Sprintf("%s-res-b", uid)
				configMapC := fmt.Sprintf("%s-res-c", uid)

				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapB, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())

					cmd = exec.Command("kubectl", "get", "configmap", configMapC, "-n", policyTestNamespace)
					_, err = utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, 40*time.Second, policyTestInterval).Should(Succeed())

				By("And skippedResources should be 2 (both B and C skipped)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.skippedResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("2"))
				}, 60*time.Second, policyTestInterval).Should(Succeed())

				By("And skippedResourceIds should contain both resource-b and resource-c")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.skippedResourceIds}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(ContainSubstring("resource-b"))
					g.Expect(output).To(ContainSubstring("resource-c"))
				}, 30*time.Second, policyTestInterval).Should(Succeed())
			})
		})

		Describe("mixed skipOnDependencyFailure in chain", func() {
			const (
				formName = "mixed-skip-form"
				uid      = "mixed-skip-uid"
			)

			AfterEach(func() {
				deleteTestData(uid)
			})

			It("should handle mixed skip settings: A (fails) -> B (skip=false, created) -> C (skip=true, created because B succeeded)", func() {
				By("Given a chain where B has skipOnDependencyFailure=false")
				createForm(formName, hubName, `
  deployments:
    - id: resource-a
      nameTemplate: "{{ .uid }}-res-a"
      waitForReady: true
      timeoutSeconds: 20
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: mixed-test
          template:
            metadata:
              labels:
                app: mixed-test
            spec:
              containers:
              - name: app
                image: non-existent-registry.example.com/will-fail:v1
  configMaps:
    - id: resource-b
      nameTemplate: "{{ .uid }}-res-b"
      dependIds:
        - resource-a
      skipOnDependencyFailure: false
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          level: "b-created-despite-failure"
    - id: resource-c
      nameTemplate: "{{ .uid }}-res-c"
      dependIds:
        - resource-b
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          level: "c-depends-on-b"
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then resource-a (Deployment) should be created (but not ready)")
				deploymentName := fmt.Sprintf("%s-res-a", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And resource-b should be created (skipOnDependencyFailure=false)")
				configMapB := fmt.Sprintf("%s-res-b", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapB, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 60*time.Second, policyTestInterval).Should(Succeed())

				By("And resource-c should ALSO be created (B succeeded, so C's dependency is satisfied)")
				configMapC := fmt.Sprintf("%s-res-c", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapC, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 30*time.Second, policyTestInterval).Should(Succeed())

				By("And skippedResources should be 0 (no resources were skipped)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.skippedResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(SatisfyAny(Equal("0"), Equal("")))
				}, 30*time.Second, policyTestInterval).Should(Succeed())
			})
		})

		Describe("not-ready vs failed dependency distinction", func() {
			const (
				formName = "not-ready-form"
				uid      = "not-ready-uid"
			)

			AfterEach(func() {
				deleteTestData(uid)
			})

			It("should NOT emit DependencySkipped event when dependency is just not ready yet", func() {
				By("Given a LynqForm with a slow-starting (but valid) Deployment and a dependent ConfigMap")
				createForm(formName, hubName, `
  deployments:
    - id: slow-deploy
      nameTemplate: "{{ .uid }}-slow-deploy"
      waitForReady: true
      timeoutSeconds: 120
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: slow-test
          template:
            metadata:
              labels:
                app: slow-test
            spec:
              containers:
              - name: app
                image: busybox:1.36
                command: ["sh", "-c", "sleep 3600"]
  configMaps:
    - id: dependent-config
      nameTemplate: "{{ .uid }}-dep-config"
      dependIds:
        - slow-deploy
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          status: "created-after-deploy-ready"
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the Deployment should be created")
				deploymentName := fmt.Sprintf("%s-slow-deploy", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And while Deployment is starting, NO DependencySkipped event should be emitted")
				// Check early that no DependencySkipped event is emitted
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "events", "-n", policyTestNamespace,
						"--field-selector", fmt.Sprintf("involvedObject.name=%s,reason=DependencySkipped", expectedNodeName))
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					// Should NOT contain DependencySkipped - only "No resources found"
					g.Expect(output).To(ContainSubstring("No resources found"))
				}, 15*time.Second, policyTestInterval).Should(Succeed())

				By("And skippedResources should be 0 (not-ready is not skipped)")
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.skippedResources}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(SatisfyAny(Equal("0"), Equal("")))

				By("When Deployment becomes ready")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.readyReplicas}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("1"))
				}, 90*time.Second, policyTestInterval).Should(Succeed())

				By("Then the dependent ConfigMap should eventually be created")
				configMapName := fmt.Sprintf("%s-dep-config", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 60*time.Second, policyTestInterval).Should(Succeed())

				By("And still no DependencySkipped event should exist")
				cmd = exec.Command("kubectl", "get", "events", "-n", policyTestNamespace,
					"--field-selector", fmt.Sprintf("involvedObject.name=%s,reason=DependencySkipped", expectedNodeName))
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(ContainSubstring("No resources found"))
			})
		})

		Describe("kubectl get lynqnodes shows Skipped column", func() {
			const (
				formName = "skipped-column-form"
				uid      = "skipped-column-uid"
			)

			AfterEach(func() {
				deleteTestData(uid)
			})

			It("should display skipped count in kubectl get lynqnodes output", func() {
				By("Given a LynqForm with a dependency that will fail")
				createForm(formName, hubName, `
  deployments:
    - id: will-fail
      nameTemplate: "{{ .uid }}-fail"
      waitForReady: true
      timeoutSeconds: 20
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: column-test
          template:
            metadata:
              labels:
                app: column-test
            spec:
              containers:
              - name: app
                image: non-existent-registry.example.com/fail:v1
  configMaps:
    - id: to-be-skipped
      nameTemplate: "{{ .uid }}-skipped"
      dependIds:
        - will-fail
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created and dependency times out")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then kubectl get lynqnodes should show Skipped column with value 1")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnodes", expectedNodeName, "-n", policyTestNamespace)
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					// The output should contain the Skipped column header and a value of 1
					g.Expect(strings.ToUpper(output)).To(ContainSubstring("SKIPPED"))
					// The row should contain "1" for the skipped column
					lines := strings.Split(output, "\n")
					g.Expect(len(lines)).To(BeNumerically(">=", 2))
					dataLine := lines[1]
					g.Expect(dataLine).To(MatchRegexp(`\s+1\s+`))
				}, 60*time.Second, policyTestInterval).Should(Succeed())
			})
		})
	})
})
