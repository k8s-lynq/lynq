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

var _ = Describe("Resource Timeout and Failure Handling", Ordered, func() {
	BeforeAll(func() {
		By("setting up policy test namespace")
		setupPolicyTestNamespace()
	})

	AfterAll(func() {
		By("cleaning up policy test namespace")
		cleanupPolicyTestNamespace()
	})

	Context("when resources fail to become ready within timeout", func() {
		const (
			hubName  = "timeout-hub"
			formName = "timeout-form"
			uid      = "timeout-test-uid"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestData(uid)

			// Delete all Deployments and ConfigMaps
			cmd := exec.Command("kubectl", "delete", "deployment", "-n", policyTestNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true", "--wait=false")
			_, _ = utils.Run(cmd)

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

		Describe("Deployment with non-existent image", func() {
			It("should mark resource as failed after timeout", func() {
				By("Given a LynqForm with a Deployment using non-existent image and short timeout")
				createForm(formName, hubName, `
  deployments:
    - id: failing-deployment
      nameTemplate: "{{ .uid }}-failing-deploy"
      waitForReady: true
      timeoutSeconds: 30
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: failing-test
          template:
            metadata:
              labels:
                app: failing-test
            spec:
              containers:
              - name: app
                image: non-existent-registry.example.com/non-existent-image:v999
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then the Deployment should be created (even if not ready)")
				deploymentName := fmt.Sprintf("%s-failing-deploy", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the LynqNode should have Ready=False after timeout")
				// Wait a bit more than the timeout to ensure the status reflects failure
				// The timeout is 30s, so wait up to 60s for the condition to be updated
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("False"))
				}, 90*time.Second, policyTestInterval).Should(Succeed())

				By("And the Ready condition should indicate resource failure")
				Eventually(func(g Gomega) {
					// Check the Ready condition's reason or message for timeout/failure indication
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].reason}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					// The reason should indicate not all resources are ready
					g.Expect(output).To(SatisfyAny(
						Equal("NotAllResourcesReady"),
						ContainSubstring("NotReady"),
						ContainSubstring("NotAllResources"),
						ContainSubstring("Failed"),
						ContainSubstring("Timeout"),
					))
				}, 30*time.Second, policyTestInterval).Should(Succeed())

				By("And readyResources should be less than desiredResources")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.readyResources}/{.status.desiredResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					// Output format: "0/1" or "ready/desired"
					// readyResources should be less than desiredResources
					g.Expect(output).To(SatisfyAny(
						Equal("0/1"),
						Equal("/1"),              // readyResources might be omitted when 0
						MatchRegexp(`^0?/[1-9]`), // 0 or empty / positive number
					))
				}, 30*time.Second, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Recovery after fixing failing resource", func() {
			It("should recover when failing resource is fixed", func() {
				By("Given a LynqForm with a failing Deployment")
				createForm(formName, hubName, `
  deployments:
    - id: recoverable-deployment
      nameTemplate: "{{ .uid }}-recover-deploy"
      waitForReady: true
      timeoutSeconds: 30
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: recover-test
          template:
            metadata:
              labels:
                app: recover-test
            spec:
              containers:
              - name: app
                image: non-existent-registry.example.com/bad-image:v1
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created and times out")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				// Wait for timeout
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("False"))
				}, 90*time.Second, policyTestInterval).Should(Succeed())

				By("And the template is fixed with a valid image")
				createForm(formName, hubName, `
  deployments:
    - id: recoverable-deployment
      nameTemplate: "{{ .uid }}-recover-deploy"
      waitForReady: true
      timeoutSeconds: 120
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: recover-test
          template:
            metadata:
              labels:
                app: recover-test
            spec:
              containers:
              - name: app
                image: busybox:1.36
                command: ["sh", "-c", "sleep 3600"]
`)

				By("Then the LynqNode should recover and become Ready")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the Deployment should be Available")
				deploymentName := fmt.Sprintf("%s-recover-deploy", uid)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

	})
})
