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
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8s-lynq/lynq/test/utils"
)

var _ = Describe("Rollout maxSkew Feature", Ordered, func() {
	BeforeAll(func() {
		By("setting up policy test namespace")
		setupPolicyTestNamespace()
	})

	AfterAll(func() {
		By("cleaning up policy test namespace")
		cleanupPolicyTestNamespace()
	})

	Context("when maxSkew is configured in LynqForm", func() {
		const (
			hubName  = "rollout-hub"
			formName = "rollout-form"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			// Delete test data for all nodes
			for i := 1; i <= 10; i++ {
				deleteTestData(fmt.Sprintf("rollout-node-%d", i))
			}

			// Delete all ConfigMaps created by the test
			cmd := exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
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

		Describe("maxSkew=0 (unlimited) behavior", func() {
			It("should update all nodes simultaneously when maxSkew is 0", func() {
				By("Given a LynqForm with maxSkew=0 (default)")
				createFormWithRollout(formName, hubName, 0, `
  configMaps:
    - id: app-config
      nameTemplate: "{{ .uid }}-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          version: v1
`)

				By("And 5 active nodes")
				for i := 1; i <= 5; i++ {
					insertTestData(fmt.Sprintf("rollout-node-%d", i), true)
				}

				By("When LynqHub syncs")
				time.Sleep(7 * time.Second)

				By("Then all 5 LynqNodes should be created")
				for i := 1; i <= 5; i++ {
					nodeName := fmt.Sprintf("rollout-node-%d-%s", i, formName)
					Eventually(func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace)
						_, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred())
					}, policyTestTimeout, policyTestInterval).Should(Succeed())
				}

				By("And LynqForm status.rollout should be nil (not tracked)")
				time.Sleep(3 * time.Second)
				cmd := exec.Command("kubectl", "get", "lynqform", formName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.rollout}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(BeEmpty(), "Rollout status should be empty when maxSkew=0")
			})
		})

		Describe("maxSkew enforcement", func() {
			It("should limit simultaneous updates based on maxSkew value", func() {
				By("Given a LynqForm with maxSkew=2")
				createFormWithRollout(formName, hubName, 2, `
  configMaps:
    - id: app-config
      nameTemplate: "{{ .uid }}-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          version: v1
`)

				By("And 6 active nodes")
				for i := 1; i <= 6; i++ {
					insertTestData(fmt.Sprintf("rollout-node-%d", i), true)
				}

				By("When LynqHub syncs")
				time.Sleep(7 * time.Second)

				By("Then LynqForm status.rollout should track progress")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqform", formName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.rollout.phase}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					// Phase should be InProgress or Complete
					g.Expect(output).To(Or(Equal("InProgress"), Equal("Complete"), Equal("Idle")))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And status.rollout.updatingNodes should not exceed maxSkew")
				// Check multiple times during rollout
				for i := 0; i < 5; i++ {
					cmd := exec.Command("kubectl", "get", "lynqform", formName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.rollout.updatingNodes}")
					output, err := utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred())
					if output != "" {
						updatingNodes, parseErr := strconv.Atoi(output)
						if parseErr == nil {
							Expect(updatingNodes).To(BeNumerically("<=", 2),
								"Updating nodes should not exceed maxSkew=2")
						}
					}
					time.Sleep(2 * time.Second)
				}

				By("And eventually all nodes should become Ready")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqform", formName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.rollout.phase}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("Complete"))
				}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("maxSkew=1 serial rollout", func() {
			It("should update nodes one at a time when maxSkew=1", func() {
				By("Given a LynqForm with maxSkew=1")
				createFormWithRollout(formName, hubName, 1, `
  configMaps:
    - id: app-config
      nameTemplate: "{{ .uid }}-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          version: v1
`)

				By("And 3 active nodes")
				for i := 1; i <= 3; i++ {
					insertTestData(fmt.Sprintf("rollout-node-%d", i), true)
				}

				By("When LynqHub syncs")
				time.Sleep(7 * time.Second)

				By("Then at most 1 node should be updating at any time")
				// Check multiple times during rollout
				for i := 0; i < 5; i++ {
					cmd := exec.Command("kubectl", "get", "lynqform", formName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.rollout.updatingNodes}")
					output, err := utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred())
					if output != "" {
						updatingNodes, parseErr := strconv.Atoi(output)
						if parseErr == nil {
							Expect(updatingNodes).To(BeNumerically("<=", 1),
								"Updating nodes should not exceed maxSkew=1 for serial rollout")
						}
					}
					time.Sleep(2 * time.Second)
				}

				By("And eventually the rollout should complete")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqform", formName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.rollout.phase}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("Complete"))
				}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Rollout status tracking", func() {
			It("should correctly track rollout metrics", func() {
				By("Given a LynqForm with maxSkew=3")
				createFormWithRollout(formName, hubName, 3, `
  configMaps:
    - id: app-config
      nameTemplate: "{{ .uid }}-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          version: v1
`)

				By("And 4 active nodes")
				for i := 1; i <= 4; i++ {
					insertTestData(fmt.Sprintf("rollout-node-%d", i), true)
				}

				By("When LynqHub syncs and rollout progresses")
				time.Sleep(7 * time.Second)

				By("Then LynqForm status.rollout should have correct values")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqform", formName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.rollout.totalNodes}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("4"), "totalNodes should be 4")
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And targetGeneration should match template generation")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqform", formName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.rollout.targetGeneration}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).NotTo(BeEmpty())
					targetGen, parseErr := strconv.ParseInt(output, 10, 64)
					g.Expect(parseErr).NotTo(HaveOccurred())
					g.Expect(targetGen).To(BeNumerically(">", 0))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And when rollout completes, readyUpdatedNodes should equal totalNodes")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqform", formName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.rollout.phase}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("Complete"))
				}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())

				cmd := exec.Command("kubectl", "get", "lynqform", formName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.rollout.readyUpdatedNodes}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("4"), "readyUpdatedNodes should equal totalNodes when Complete")
			})
		})
	})

	Context("Template-isolated maxSkew strategy", func() {
		const (
			hubName   = "multi-form-rollout-hub"
			formName1 = "web-form"
			formName2 = "worker-form"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			for i := 1; i <= 5; i++ {
				deleteTestData(fmt.Sprintf("multi-node-%d", i))
			}

			cmd := exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName1, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "lynqform", formName2, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(5 * time.Second)
		})

		Describe("Multiple LynqForms with independent maxSkew", func() {
			It("should apply maxSkew independently for each LynqForm", func() {
				By("Given two LynqForms with different maxSkew values")
				createFormWithRollout(formName1, hubName, 2, `
  configMaps:
    - id: web-config
      nameTemplate: "{{ .uid }}-web-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tier: web
`)
				createFormWithRollout(formName2, hubName, 1, `
  configMaps:
    - id: worker-config
      nameTemplate: "{{ .uid }}-worker-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tier: worker
`)

				By("And 4 active nodes")
				for i := 1; i <= 4; i++ {
					insertTestData(fmt.Sprintf("multi-node-%d", i), true)
				}

				By("When LynqHub syncs")
				time.Sleep(7 * time.Second)

				By("Then web-form should track its rollout independently")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqform", formName1, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.rollout.totalNodes}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("4"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And worker-form should track its rollout independently")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqform", formName2, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.rollout.totalNodes}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("4"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And both forms should eventually complete their rollouts")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqform", formName1, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.rollout.phase}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("Complete"))
				}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())

				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqform", formName2, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.rollout.phase}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("Complete"))
				}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})
	})
})

// createFormWithRollout creates a LynqForm with rollout config
func createFormWithRollout(name, hubName string, maxSkew int32, resources string) {
	rolloutConfig := ""
	if maxSkew > 0 {
		rolloutConfig = fmt.Sprintf(`
  rollout:
    maxSkew: %d
    progressDeadlineSeconds: 600`, maxSkew)
	}

	formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s%s
  %s
`, name, policyTestNamespace, hubName, rolloutConfig, resources)

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = utils.StringReader(formYAML)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
}
