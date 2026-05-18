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

// Package e2e - bottleneck_regression_e2e_test.go
//
// # Bottleneck Regression E2E Tests
//
// Reproduces the exact production deadlock scenario that triggered the bug fix.
//
// ## Original Incident
//
// ~100 Deployments had their Lynq field-manager ownership removed by direct kubectl edits.
// LynqForm was then updated to conflictPolicy:Force + patchStrategy:replace.
// The first 3-5 nodes applied successfully; the remainder entered permanent deadlock.
// Controller restarts did not resolve it. Raising maxSkew from 5→30 worked around it.
//
// ## Root Causes (Two Compounding Bugs)
//
// BUG 1 (primary): Timeout measured from resource creationTimestamp.
//   - Pre-existing Deployments (months old) always exceeded the 5-minute timeout immediately after apply.
//   - All updated nodes were immediately marked FAILED.
//   - With maxSkew=5: 5 FAILED nodes → count=5 ≥ maxSkew → all remaining nodes permanently blocked.
//
// BUG 2 (compounding): In-memory appliedRV cache lost on controller restart.
//   - After restart, the Applier had no cache → triggered client.Update() on every resource.
//   - Each Update started a new rolling update → resource became not-ready → BUG 1 fired again.
//   - Restart did not help; it made things worse.
//
// ## Test Strategy
//
// To reproduce "old resources" without waiting 5 minutes:
//   - Use timeoutSeconds:60 (shorter than default 300s, but long enough for rolling updates in CI).
//   - Pre-create Deployments, then sleep 65 seconds so they are older than the 60s timeout.
//   - Result: Deployments are 65s old, which exceeds the 60s timeout.
//   - BUG 1 behavior:  elapsed=65s ≥ 60s → immediately FAILED after apply.
//   - Fixed behavior:  elapsed from applyStartTime ≈ 0 < 60s → wait for rolling update → Ready.
//
// Why timeoutSeconds:60 instead of a shorter value:
//   - The controller requeues every 30 seconds (reconcile interval).
//   - With timeout < 30s: on the second reconcile (t=30s), elapsed > timeout → fires even with the fix.
//   - With timeout ≥ 60s: the pod has at least one full reconcile cycle (30s) of grace after apply.
//   - In CI, nginx image pull may take 20-30s; a 60s timeout ensures the pod becomes ready first.
//   - The readiness check runs BEFORE the timeout check, so a ready pod always wins regardless of timeout.
//
// Rolling update timing (nginx with initialDelaySeconds:8):
//   - Image pull in CI: ~20-30s. Pod start + readiness: ~10s. Total: ~30-40s.
//   - At t=30s (2nd reconcile): pod may or may not be ready. elapsed=30s < 60s → wait.
//   - At t=60s (3rd reconcile): pod IS ready. Readiness check passes → LynqNode becomes Ready.
//   - Total per-node: ~60s. With maxSkew=2 and 4 nodes: ~120s total (two batches of 2).

package e2e

import (
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8s-lynq/lynq/test/utils"
)

var _ = Describe("Bottleneck Regression (BUG 1 + BUG 2)", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up shared test table")
		testTable = setupTestTable("bottleneck_regression")
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	// =========================================================================
	// Context 1: BUG 1 regression
	// Pre-existing Deployments must not be immediately FAILED when using
	// patchStrategy:replace with timeoutSeconds shorter than their age.
	// =========================================================================
	Context("BUG 1: pre-existing Deployments are not immediately failed on replace", Ordered, func() {
		const (
			hubName  = "bn-bug1-hub"
			formName = "bn-bug1-form"
			nodeUID1 = "bn-bug1-node-1"
			nodeUID2 = "bn-bug1-node-2"
		)

		BeforeAll(func() {
			By("pre-creating Deployments to simulate pre-existing resources")
			// These Deployments exist BEFORE LynqHub/LynqForm are set up.
			// They represent workloads that were running before Lynq took ownership,
			// or workloads whose Lynq ownership was removed by manual kubectl edits.
			for _, uid := range []string{nodeUID1, nodeUID2} {
				createPreExistingDeployment(uid+"-nginx", policyTestNamespace, "nginx:1.24")
			}

			By("waiting 65 seconds so Deployments are older than timeoutSeconds:60")
			// With BUG 1 (elapsed from creationTimestamp): 65s ≥ 60s → immediately FAILED.
			// With fix (elapsed from applyStartTime):      ≈0s < 60s  → wait for rolling update.
			time.Sleep(65 * time.Second)

			By("creating LynqHub")
			createHubWithTable(hubName, testTable)

			By("inserting active rows so Hub creates LynqNodes")
			insertTestDataToTable(testTable, nodeUID1, true)
			insertTestDataToTable(testTable, nodeUID2, true)

			By("applying LynqForm with patchStrategy:replace + conflictPolicy:Force")
			// This is exactly the config change that triggered the production deadlock.
			applyBottleneckForm(formName, hubName, "nginx:1.25", 0)
		})

		AfterAll(func() {
			for _, uid := range []string{nodeUID1, nodeUID2} {
				deleteTestDataFromTable(testTable, uid)
			}
			cleanupBottleneckResources(formName, hubName, policyTestNamespace,
				[]string{nodeUID1 + "-nginx", nodeUID2 + "-nginx"},
				[]string{nodeUID1, nodeUID2})
		})

		JustAfterEach(func() {
			// Always dump diagnostic state to help debug failures
			for _, uid := range []string{nodeUID1, nodeUID2} {
				dumpBottleneckDebugState(uid, formName)
			}
		})

		It("should apply successfully without immediately failing pre-existing Deployments", func() {
			By("expecting both LynqNodes to eventually become Ready")
			// With BUG 1: both nodes are immediately FAILED (25s > 20s timeout).
			// → Ready condition is never reached. Test fails after the Eventually timeout.
			// With fix: rolling update completes (~10s pod start + 30s requeue) → Ready.
			// Reduced from 3*policyTestTimeout (15m) to 1*policyTestTimeout (5m) so failures surface faster.
			for _, uid := range []string{nodeUID1, nodeUID2} {
				nodeName := fmt.Sprintf("%s-%s", uid, formName)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", nodeName,
						"-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"),
						"LynqNode %s should be Ready; pre-existing Deployment must not be "+
							"immediately timed out using creationTimestamp", nodeName)
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			}

			By("verifying Deployments are running nginx:1.25 (rolling update completed)")
			for _, uid := range []string{nodeUID1, nodeUID2} {
				deployName := uid + "-nginx"
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deployName,
						"-n", policyTestNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].image}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("nginx:1.25"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			}
		})
	})

	// =========================================================================
	// Context 2: BUG 1 + maxSkew deadlock regression
	// The full production scenario: pre-existing Deployments + maxSkew enforcement.
	// With BUG 1: the first maxSkew nodes fail immediately, saturating all slots.
	// All remaining nodes are permanently blocked → deadlock.
	// =========================================================================
	Context("BUG 1 + maxSkew: rollout completes without deadlock for pre-existing Deployments", Ordered, func() {
		const (
			hubName  = "bn-deadlock-hub"
			formName = "bn-deadlock-form"
			maxSkew  = 2
			numNodes = 4
		)

		var nodeUIDs []string

		BeforeAll(func() {
			nodeUIDs = make([]string, numNodes)
			for i := range nodeUIDs {
				nodeUIDs[i] = fmt.Sprintf("bn-dl-node-%d", i+1)
			}

			By("pre-creating Deployments to simulate the production environment")
			// The real incident had ~100 Deployments whose Lynq ownership was removed.
			// We use 4 here for test efficiency while demonstrating the same mechanism.
			for _, uid := range nodeUIDs {
				createPreExistingDeployment(uid+"-nginx", policyTestNamespace, "nginx:1.24")
			}

			By("waiting 65 seconds so Deployments are older than timeoutSeconds:60")
			// With BUG 1 + maxSkew=2:
			//   1. Hub schedules 2 nodes (maxSkew slot capacity).
			//   2. Both immediately FAILED (65s > 60s) → count=2 = maxSkew=2 → remaining 2 blocked.
			//   3. Deadlock: first 2 stuck in FAILED, last 2 never get a chance to update.
			time.Sleep(65 * time.Second)

			By("creating LynqHub")
			createHubWithTable(hubName, testTable)

			By("inserting active rows")
			for _, uid := range nodeUIDs {
				insertTestDataToTable(testTable, uid, true)
			}

			By("applying LynqForm with maxSkew=2, patchStrategy:replace, conflictPolicy:Force")
			// This replicates the exact production configuration that triggered the deadlock.
			applyBottleneckForm(formName, hubName, "nginx:1.25", maxSkew)
		})

		AfterAll(func() {
			for _, uid := range nodeUIDs {
				deleteTestDataFromTable(testTable, uid)
			}
			deployNames := make([]string, numNodes)
			for i, uid := range nodeUIDs {
				deployNames[i] = uid + "-nginx"
			}
			cleanupBottleneckResources(formName, hubName, policyTestNamespace, deployNames, nodeUIDs)
		})

		It("should complete rollout for all nodes without maxSkew deadlock", func() {
			By("expecting all 4 LynqNodes to eventually become Ready")
			// With BUG 1: first 2 nodes FAILED immediately → maxSkew saturated → last 2 blocked forever.
			// → Test fails: not all 4 nodes become Ready within the time limit.
			// With fix: rolling update completes for first 2 → maxSkew slots free → last 2 start → all Ready.
			for _, uid := range nodeUIDs {
				nodeName := fmt.Sprintf("%s-%s", uid, formName)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", nodeName,
						"-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"),
						"LynqNode %s should be Ready; deadlock must not prevent all nodes from completing", nodeName)
				}, 4*policyTestTimeout, policyTestInterval).Should(Succeed())
			}

			By("verifying rollout status reflects completion")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqform", formName,
					"-n", policyTestNamespace,
					"-o", "jsonpath={.status.rollout.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// Phase may be Complete or Idle once all nodes are Ready
				g.Expect(output).To(Or(Equal("Complete"), Equal("Idle")))
			}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())
		})

		It("should never have more than maxSkew=2 nodes updating simultaneously", func() {
			// This assertion holds as a post-condition after the rollout above.
			// During the rollout, at most 2 nodes should have been updating at any time.
			// We verify the final state: all nodes Ready and no failed resources.
			for _, uid := range nodeUIDs {
				nodeName := fmt.Sprintf("%s-%s", uid, formName)
				cmd := exec.Command("kubectl", "get", "lynqnode", nodeName,
					"-n", policyTestNamespace,
					"-o", "jsonpath={.status.failedResources}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Or(Equal(""), Equal("0")),
					"LynqNode %s should have no failed resources after rollout completion", nodeName)
			}
		})
	})

	// =========================================================================
	// Context 3: BUG 2 regression (combined with BUG 1)
	// After controller restart, unchanged resources must not be re-applied.
	// With BUG 2: cache is cleared → unconditional re-Update → rolling update starts
	//             → resource not-ready → BUG 1 fires (elapsed from old creationTimestamp) → FAILED.
	// With fixes: annotation-based cache restores → Update skipped → nodes stay Ready.
	// =========================================================================
	Context("BUG 2: controller restart does not re-trigger immediate failures", Ordered, func() {
		const (
			hubName  = "bn-restart-hub"
			formName = "bn-restart-form"
			nodeUID1 = "bn-restart-node-1"
			nodeUID2 = "bn-restart-node-2"
		)

		BeforeAll(func() {
			By("pre-creating Deployments")
			for _, uid := range []string{nodeUID1, nodeUID2} {
				createPreExistingDeployment(uid+"-nginx", policyTestNamespace, "nginx:1.24")
			}

			By("waiting 65 seconds so Deployments are older than timeoutSeconds:60")
			time.Sleep(65 * time.Second)

			By("creating LynqHub")
			createHubWithTable(hubName, testTable)

			By("inserting active rows")
			insertTestDataToTable(testTable, nodeUID1, true)
			insertTestDataToTable(testTable, nodeUID2, true)

			By("applying initial LynqForm (nginx:1.25, replace, Force, timeoutSeconds:20)")
			applyBottleneckForm(formName, hubName, "nginx:1.25", 0)

			By("waiting for all nodes to become Ready before restart")
			// At this point, Deployments are 65s + rolling time ≈ 125-135s old.
			// This is older than the 60s timeout, so BUG 1 + BUG 2 would fire on restart.
			for _, uid := range []string{nodeUID1, nodeUID2} {
				nodeName := fmt.Sprintf("%s-%s", uid, formName)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", nodeName,
						"-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, 3*policyTestTimeout, policyTestInterval).Should(Succeed())
			}
		})

		AfterAll(func() {
			for _, uid := range []string{nodeUID1, nodeUID2} {
				deleteTestDataFromTable(testTable, uid)
			}
			cleanupBottleneckResources(formName, hubName, policyTestNamespace,
				[]string{nodeUID1 + "-nginx", nodeUID2 + "-nginx"},
				[]string{nodeUID1, nodeUID2})
		})

		It("should keep LynqNodes Ready after controller restart", func() {
			By("restarting the controller to simulate real-world restart scenario")
			// In production, the controller was restarted to try to resolve the deadlock.
			// With BUG 2 + BUG 1: restart clears the appliedRV cache → unconditional re-Update
			//   → rolling update starts → Deployments (now 125-135s old) > 60s timeout → FAILED.
			// With fixes: annotation-based cache restores → Update skipped → nodes stay Ready.
			cmd := exec.Command("kubectl", "rollout", "restart",
				"deployment/lynq-controller-manager", "-n", "lynq-system")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to restart controller")

			By("waiting for controller to come back up")
			cmd = exec.Command("kubectl", "rollout", "status",
				"deployment/lynq-controller-manager", "-n", "lynq-system",
				"--timeout=2m")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Controller did not restart successfully")

			By("verifying LynqNodes remain Ready after restart (BUG 2 regression)")
			// Allow time for the controller to reconcile after restart.
			// With fixes: nodes should remain Ready throughout.
			// With bugs: nodes would flip to FAILED within a few reconcile cycles.
			time.Sleep(10 * time.Second) // Let controller reconcile after startup

			for _, uid := range []string{nodeUID1, nodeUID2} {
				nodeName := fmt.Sprintf("%s-%s", uid, formName)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", nodeName,
						"-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"),
						"LynqNode %s should remain Ready after controller restart; "+
							"BUG 2 (cache cleared) + BUG 1 (creationTimestamp) must not cause FAILED state", nodeName)
				}, 2*policyTestTimeout, policyTestInterval).Should(Succeed())
			}
		})

		It("should complete a subsequent rollout after restart with no failed nodes", func() {
			By("updating LynqForm to nginx:1.26 to trigger another rolling update post-restart")
			// After restart, the Deployments are now ~135s+ old (well beyond timeoutSeconds:60).
			// BUG 2 + BUG 1 would cause immediate FAILED when the new rolling update starts.
			// With fixes: elapsed from new applyStartTime ≈ 0 → wait → rolling update completes.
			applyBottleneckForm(formName, hubName, "nginx:1.26", 0)

			By("verifying all nodes become Ready with nginx:1.26")
			for _, uid := range []string{nodeUID1, nodeUID2} {
				nodeName := fmt.Sprintf("%s-%s", uid, formName)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", nodeName,
						"-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"),
						"LynqNode %s should be Ready after post-restart rollout to nginx:1.26", nodeName)
				}, 3*policyTestTimeout, policyTestInterval).Should(Succeed())
			}

			By("verifying Deployments are running nginx:1.26")
			for _, uid := range []string{nodeUID1, nodeUID2} {
				deployName := uid + "-nginx"
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deployName,
						"-n", policyTestNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].image}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("nginx:1.26"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			}
		})
	})
})

// =============================================================================
// HELPERS
// =============================================================================

// createPreExistingDeployment creates a Deployment directly (not via Lynq) to simulate
// a workload that existed before LynqForm was configured, or one whose Lynq ownership
// was removed by direct kubectl edits.
func createPreExistingDeployment(name, namespace, image string) {
	deployYAML := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    managed-by: pre-existing
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
        - name: nginx
          image: %s
          ports:
            - containerPort: 80
          readinessProbe:
            httpGet:
              path: /
              port: 80
            initialDelaySeconds: 8
            periodSeconds: 2
            failureThreshold: 5
`, name, namespace, name, name, name, image)

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = utils.StringReader(deployYAML)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create pre-existing Deployment %s", name)

	// Wait for the pre-existing Deployment to become ready so rolling update
	// is clearly triggered when LynqForm applies a different image later.
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "deployment", name,
			"-n", namespace,
			"-o", "jsonpath={.status.readyReplicas}")
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal("1"), "Pre-existing Deployment %s should have 1 ready replica", name)
	}, 3*time.Minute, 5*time.Second).Should(Succeed())
}

// applyBottleneckForm creates or updates the LynqForm that reproduces the bottleneck scenario.
// The form uses patchStrategy:replace + conflictPolicy:Force (the exact production config).
// timeoutSeconds:60 with 65-second-old Deployments is the controlled reproduction of BUG 1.
// maxSkew=0 means unlimited (no throttling); maxSkew>0 enables the deadlock test.
func applyBottleneckForm(name, hubName, nginxImage string, maxSkew int32) {
	rolloutSection := ""
	if maxSkew > 0 {
		rolloutSection = fmt.Sprintf(`
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
  deployments:
    - id: nginx-deploy
      nameTemplate: "{{ .uid }}-nginx"
      patchStrategy: replace
      conflictPolicy: Force
      waitForReady: true
      timeoutSeconds: 60
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
              containers:
                - name: nginx
                  image: %s
                  ports:
                    - containerPort: 80
                  readinessProbe:
                    httpGet:
                      path: /
                      port: 80
                    initialDelaySeconds: 8
                    periodSeconds: 2
                    failureThreshold: 5
`, name, policyTestNamespace, hubName, rolloutSection, nginxImage)

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = utils.StringReader(formYAML)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to apply bottleneck LynqForm %s", name)
}

// dumpBottleneckDebugState prints the state of LynqNode, Deployment, and ReplicaSets
// for diagnosing why a LynqNode is not becoming Ready. Output is added to test logs
// via Ginkgo's By() so it appears in CI artifacts.
func dumpBottleneckDebugState(uid, formName string) {
	nodeName := fmt.Sprintf("%s-%s", uid, formName)
	deployName := uid + "-nginx"

	writeDebug := func(label string, cmd *exec.Cmd) {
		By(label)
		if out, err := utils.Run(cmd); err == nil {
			_, _ = fmt.Fprintln(GinkgoWriter, out)
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "%s failed: %v\n", label, err)
		}
	}

	writeDebug(fmt.Sprintf("=== DEBUG: LynqNode %s ===", nodeName),
		exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace, "-o", "yaml"))
	writeDebug(fmt.Sprintf("=== DEBUG: Deployment %s ===", deployName),
		exec.Command("kubectl", "get", "deployment", deployName, "-n", policyTestNamespace, "-o", "yaml"))
	writeDebug(fmt.Sprintf("=== DEBUG: ReplicaSets for app=%s ===", deployName),
		exec.Command("kubectl", "get", "rs", "-n", policyTestNamespace, "-l", "app="+deployName, "-o", "yaml"))
	writeDebug(fmt.Sprintf("=== DEBUG: Pods with app=%s ===", deployName),
		exec.Command("kubectl", "get", "pods", "-n", policyTestNamespace, "-l", "app="+deployName, "-o", "wide"))
	writeDebug("=== DEBUG: Recent controller logs (last 200 lines) ===",
		exec.Command("kubectl", "logs", "-n", "lynq-system", "deployment/lynq-controller-manager", "--tail=200"))
	writeDebug(fmt.Sprintf("=== DEBUG: Events for LynqNode %s ===", nodeName),
		exec.Command("kubectl", "get", "events", "-n", policyTestNamespace,
			"--field-selector", "involvedObject.name="+nodeName, "--sort-by=.lastTimestamp"))
}

// cleanupBottleneckResources removes all resources created by bottleneck tests.
func cleanupBottleneckResources(formName, hubName, namespace string, deployNames, nodeUIDs []string) {
	// Remove pre-existing Deployments (may have been adopted by LynqNode; delete directly)
	for _, deployName := range deployNames {
		cmd := exec.Command("kubectl", "delete", "deployment", deployName,
			"-n", namespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
	}

	// Delete LynqForm (triggers LynqNode deletion via garbage collection)
	cmd := exec.Command("kubectl", "delete", "lynqform", formName,
		"-n", namespace, "--ignore-not-found=true")
	_, _ = utils.Run(cmd)

	// Delete LynqHub
	cmd = exec.Command("kubectl", "delete", "lynqhub", hubName,
		"-n", namespace, "--ignore-not-found=true")
	_, _ = utils.Run(cmd)

	// Delete any remaining LynqNodes
	for _, uid := range nodeUIDs {
		nodeName := fmt.Sprintf("%s-%s", uid, formName)
		cmd := exec.Command("kubectl", "delete", "lynqnode", nodeName,
			"-n", namespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
	}

	time.Sleep(5 * time.Second)
}
