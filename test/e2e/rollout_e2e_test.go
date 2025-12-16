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
	"strings"
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

	Context("maxSkew enforcement during Deployment rolling update", func() {
		// BUG REPRODUCTION TEST
		// This test verifies that maxSkew is strictly enforced when LynqForm template changes
		// trigger Deployment rolling updates across multiple nodes.
		//
		// Expected behavior:
		// - With maxSkew=1, at most 1 node should be "updating" (not Ready) at any time
		// - The operator should wait for Deployment rolling update to complete before
		//   proceeding to update the next node
		//
		// Bug scenario:
		// - Due to async status updates, the operator may incorrectly think a node is Ready
		//   when the Deployment is still rolling out, causing maxSkew violation

		const (
			hubName  = "maxskew-deploy-hub"
			formName = "maxskew-deploy-form"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			for i := 1; i <= 3; i++ {
				deleteTestData(fmt.Sprintf("maxskew-node-%d", i))
			}

			// Delete all Deployments created by the test
			cmd := exec.Command("kubectl", "delete", "deployment", "-n", policyTestNamespace,
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

		It("should strictly enforce maxSkew=1 during template change with Deployment rolling update", func() {
			By("Given a LynqForm with maxSkew=1 and a Deployment with slow-starting pods")
			// Use initContainer with sleep to simulate slow pod startup
			// This ensures rolling update takes long enough to detect maxSkew violations
			formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  rollout:
    maxSkew: 1
    progressDeadlineSeconds: 600
  deployments:
    - id: nginx-deploy
      nameTemplate: "{{ .uid }}-nginx"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: "{{ .uid }}-nginx"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-nginx"
            spec:
              initContainers:
                - name: slow-start
                  image: busybox:1.36
                  command: ["sh", "-c", "echo 'Starting slow init v1...' && sleep 5"]
              containers:
                - name: nginx
                  image: nginx:1.24
                  ports:
                    - containerPort: 80
                  readinessProbe:
                    httpGet:
                      path: /
                      port: 80
                    initialDelaySeconds: 2
                    periodSeconds: 1
`, formName, policyTestNamespace, hubName)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(formYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("And 3 active nodes in the database")
			for i := 1; i <= 3; i++ {
				insertTestData(fmt.Sprintf("maxskew-node-%d", i), true)
			}

			By("When initial LynqNodes are created and become Ready")
			// Wait for all 3 nodes to become Ready initially
			for i := 1; i <= 3; i++ {
				nodeName := fmt.Sprintf("maxskew-node-%d-%s", i, formName)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"), "LynqNode %s should be Ready", nodeName)
				}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())
			}

			By("And all Deployments have ready replicas")
			for i := 1; i <= 3; i++ {
				deployName := fmt.Sprintf("maxskew-node-%d-nginx", i)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deployName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.readyReplicas}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("1"), "Deployment %s should have 1 ready replica", deployName)
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			}

			By("When the LynqForm template is changed (init command updated to trigger rolling update)")
			// Change the init command to trigger a rolling update
			// The 5-second sleep ensures rolling update takes long enough to detect violations
			formYAML = fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  rollout:
    maxSkew: 1
    progressDeadlineSeconds: 600
  deployments:
    - id: nginx-deploy
      nameTemplate: "{{ .uid }}-nginx"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: "{{ .uid }}-nginx"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-nginx"
            spec:
              initContainers:
                - name: slow-start
                  image: busybox:1.36
                  command: ["sh", "-c", "echo 'Starting slow init v2...' && sleep 5"]
              containers:
                - name: nginx
                  image: nginx:1.25
                  ports:
                    - containerPort: 80
                  readinessProbe:
                    httpGet:
                      path: /
                      port: 80
                    initialDelaySeconds: 2
                    periodSeconds: 1
`, formName, policyTestNamespace, hubName)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(formYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("Then at any point during rollout, at most 1 LynqNode should be not Ready (maxSkew=1)")
			// Check frequently during the rollout process
			// With 3 nodes and ~7 seconds per rolling update (5s init + 2s readiness),
			// total rollout takes ~21 seconds if serial, but much less if parallel (bug)
			maxViolations := 0
			checkCount := 60 // Check for 30 seconds (every 0.5s)
			for check := 0; check < checkCount; check++ {
				notReadyCount := 0
				notReadyNodes := []string{}

				for i := 1; i <= 3; i++ {
					nodeName := fmt.Sprintf("maxskew-node-%d-%s", i, formName)
					cmd := exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					if err == nil && output != "True" {
						notReadyCount++
						notReadyNodes = append(notReadyNodes, nodeName)
					}
				}

				// Log each check for debugging
				if notReadyCount > 0 {
					GinkgoWriter.Printf("Check %d/%d: %d nodes not Ready: %v\n",
						check+1, checkCount, notReadyCount, notReadyNodes)
				}

				// Track the maximum violation
				if notReadyCount > maxViolations {
					maxViolations = notReadyCount
				}

				// BUG ASSERTION: maxSkew=1 means at most 1 node can be updating (not Ready)
				// If more than 1 node is not Ready simultaneously, maxSkew is violated
				Expect(notReadyCount).To(BeNumerically("<=", 1),
					"maxSkew violation detected! Expected at most 1 node updating, but found %d nodes not Ready: %v",
					notReadyCount, notReadyNodes)

				time.Sleep(500 * time.Millisecond) // Check every 0.5 seconds
			}

			By("And eventually all nodes should become Ready with the new template")
			for i := 1; i <= 3; i++ {
				nodeName := fmt.Sprintf("maxskew-node-%d-%s", i, formName)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"), "LynqNode %s should be Ready after rollout", nodeName)
				}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())
			}

			By("And all Deployments should be running nginx:1.25")
			for i := 1; i <= 3; i++ {
				deployName := fmt.Sprintf("maxskew-node-%d-nginx", i)
				cmd := exec.Command("kubectl", "get", "deployment", deployName, "-n", policyTestNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].image}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("nginx:1.25"),
					"Deployment %s should have image nginx:1.25", deployName)
			}
		})

		It("should not start updating second node until first node Deployment is fully rolled out", func() {
			By("Given a LynqForm with maxSkew=1 and a Deployment with 2 replicas")
			formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  rollout:
    maxSkew: 1
    progressDeadlineSeconds: 600
  deployments:
    - id: nginx-deploy
      nameTemplate: "{{ .uid }}-nginx"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 2
          selector:
            matchLabels:
              app: "{{ .uid }}-nginx"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-nginx"
            spec:
              containers:
                - name: nginx
                  image: nginx:1.24
                  ports:
                    - containerPort: 80
`, formName, policyTestNamespace, hubName)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(formYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("And 2 active nodes in the database")
			for i := 1; i <= 2; i++ {
				insertTestData(fmt.Sprintf("maxskew-node-%d", i), true)
			}

			By("When all nodes and Deployments become Ready")
			for i := 1; i <= 2; i++ {
				nodeName := fmt.Sprintf("maxskew-node-%d-%s", i, formName)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())

				deployName := fmt.Sprintf("maxskew-node-%d-nginx", i)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deployName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.readyReplicas}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("2"))
				}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())
			}

			By("When the LynqForm template is changed")
			formYAML = fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  rollout:
    maxSkew: 1
    progressDeadlineSeconds: 600
  deployments:
    - id: nginx-deploy
      nameTemplate: "{{ .uid }}-nginx"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 2
          selector:
            matchLabels:
              app: "{{ .uid }}-nginx"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-nginx"
            spec:
              containers:
                - name: nginx
                  image: nginx:1.25
                  ports:
                    - containerPort: 80
`, formName, policyTestNamespace, hubName)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(formYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("Then during the rollout, we should never see both Deployments in rolling update state simultaneously")
			checkCount := 30
			for check := 0; check < checkCount; check++ {
				rollingUpdateCount := 0
				rollingDeployments := []string{}

				for i := 1; i <= 2; i++ {
					deployName := fmt.Sprintf("maxskew-node-%d-nginx", i)
					// A Deployment is in rolling update if updatedReplicas < replicas or readyReplicas < replicas
					cmd := exec.Command("kubectl", "get", "deployment", deployName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.updatedReplicas},{.status.readyReplicas},{.spec.replicas}")
					output, err := utils.Run(cmd)
					if err == nil && output != "" {
						// Parse: updatedReplicas,readyReplicas,replicas
						parts := splitOutput(output)
						if len(parts) == 3 {
							updated, _ := strconv.Atoi(parts[0])
							ready, _ := strconv.Atoi(parts[1])
							replicas, _ := strconv.Atoi(parts[2])

							// If updated < replicas or ready < replicas, it's rolling
							if updated < replicas || ready < replicas {
								rollingUpdateCount++
								rollingDeployments = append(rollingDeployments,
									fmt.Sprintf("%s(updated=%d,ready=%d,replicas=%d)", deployName, updated, ready, replicas))
							}
						}
					}
				}

				if rollingUpdateCount > 0 {
					GinkgoWriter.Printf("Check %d/%d: %d Deployments in rolling update: %v\n",
						check+1, checkCount, rollingUpdateCount, rollingDeployments)
				}

				// With maxSkew=1, at most 1 Deployment should be in rolling update state
				Expect(rollingUpdateCount).To(BeNumerically("<=", 1),
					"maxSkew violation! Multiple Deployments rolling simultaneously: %v", rollingDeployments)

				time.Sleep(1 * time.Second)
			}

			By("And eventually all Deployments should complete their rollout")
			for i := 1; i <= 2; i++ {
				deployName := fmt.Sprintf("maxskew-node-%d-nginx", i)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deployName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.updatedReplicas},{.status.readyReplicas}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("2,2"))
				}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())
			}
		})
	})

	Context("Deployment rolling update readiness", func() {
		const (
			hubName  = "deploy-ready-hub"
			formName = "deploy-ready-form"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestData("deploy-ready-node")

			// Delete all Deployments created by the test
			cmd := exec.Command("kubectl", "delete", "deployment", "-n", policyTestNamespace,
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

		It("should wait for Deployment rolling update to complete before marking LynqNode as Ready", func() {
			By("Given a LynqForm with a Deployment using nginx:1.24")
			formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  deployments:
    - id: nginx-deploy
      nameTemplate: "{{ .uid }}-nginx"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 2
          selector:
            matchLabels:
              app: "{{ .uid }}-nginx"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-nginx"
            spec:
              containers:
                - name: nginx
                  image: nginx:1.24
                  ports:
                    - containerPort: 80
`, formName, policyTestNamespace, hubName)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(formYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("And an active node in the database")
			insertTestData("deploy-ready-node", true)

			By("When the LynqNode is created and Deployment becomes Ready")
			nodeName := fmt.Sprintf("deploy-ready-node-%s", formName)

			// Wait for LynqNode to become Ready initially
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "LynqNode should be Ready")
			}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the Deployment should have all pods ready")
			deployName := "deploy-ready-node-nginx"
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", deployName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("2"), "Deployment should have 2 ready replicas")
			}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())

			By("When the Deployment image is updated to nginx:1.25 (triggers rolling update)")
			// Update LynqForm to change the image
			formYAML = fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  deployments:
    - id: nginx-deploy
      nameTemplate: "{{ .uid }}-nginx"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 2
          selector:
            matchLabels:
              app: "{{ .uid }}-nginx"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-nginx"
            spec:
              containers:
                - name: nginx
                  image: nginx:1.25
                  ports:
                    - containerPort: 80
`, formName, policyTestNamespace, hubName)

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(formYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("Then the Deployment should eventually have updatedReplicas == 2")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", deployName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.updatedReplicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("2"), "Deployment should have 2 updated replicas")
			}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the LynqNode should be Ready only after all pods are ready with the new image")
			Eventually(func(g Gomega) {
				// Check Deployment has all replicas ready
				cmd := exec.Command("kubectl", "get", "deployment", deployName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.readyReplicas},{.status.updatedReplicas},{.status.availableReplicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("2,2,2"), "All Deployment status fields should be 2")

				// Verify LynqNode is Ready
				cmd = exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "LynqNode should be Ready")
			}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the LynqNode ready resources should equal desired resources")
			cmd = exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
				"-o", "jsonpath={.status.readyResources},{.status.desiredResources}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("1,1"), "readyResources should equal desiredResources")
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

// splitOutput splits a comma-separated string into parts
func splitOutput(output string) []string {
	if output == "" {
		return []string{}
	}
	return strings.Split(output, ",")
}
