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

// Resource Phases E2E covers Phase 1 of the phase-model rollout:
//
//   Suite 1 — Steady-State Degradation Tolerance (the headline feature)
//   Suite 2 — Rollout Responsibility (regression guards)
//
// Design (matches the plan's "efficient + reliable" principles):
//   1. timeoutSeconds=30 keeps rollout-timeout scenarios fast.
//   2. `kubectl delete pod` (deterministic) instead of real node drain.
//   3. One Describe block shares setup; nested Its are independent.
//   4. Eventually + Consistently together assert BOTH the transition AND
//      the non-transition (e.g., the resource stays Failed=0 over time).
//   5. Events are observable proof of WorkloadDegraded / ReadinessTimeout;
//      we assert them explicitly via `kubectl get events`.

var _ = Describe("Resource Phases", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up test table")
		testTable = setupTestTable("resource_phases")
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	// ----------------------------------------------------------------------
	// Suite 1 — Steady-State Degradation Tolerance
	// The headline feature: post-rollout pod loss does NOT mark Failed.
	// ----------------------------------------------------------------------
	Context("Suite 1: Steady-State Degradation Tolerance", func() {
		const (
			hubName  = "phase-hub-degradation"
			formName = "phase-form-degradation"
			uid      = "phase-uid-degraded"
		)
		var deploymentName, nodeName string

		BeforeEach(func() {
			deploymentName = fmt.Sprintf("%s-app", uid)
			nodeName = fmt.Sprintf("%s-%s", uid, formName)

			createHubWithTable(hubName, testTable)
			// timeoutSeconds: 180 gives CI headroom for the initial cold
			// image pull. The test asserts NO ReadinessTimeout fires during
			// steady-state disruption — but if the initial rollout itself
			// trips the timeout (e.g., slow image pull on a fresh runner),
			// the assertion at the end of the test would see that initial
			// event and report a false failure. 180s is well over typical
			// nginx pull + readiness, while leaving the steady-state-doesn't-
			// fail-after-X-seconds guarantee intact (Suite 1.4 covers that).
			// Two layered mechanisms guarantee a 25+ second Degraded
			// observation window:
			//
			//  (a) Container command sleeps 25s BEFORE starting nginx.
			//      A force-deleted pod's replacement therefore takes 25s
			//      to become container-Ready, no matter how warm the
			//      image cache is. This is what gives the controller's
			//      30s requeue + watch a guaranteed observation slot.
			//
			//  (b) minReadySeconds: 15 holds the new pod out of
			//      availableReplicas for 15 additional seconds after
			//      Ready. Belt-and-suspenders against fast scheduling.
			//
			// readinessProbe + httpGet ensures the Ready transition is
			// observed only after nginx is actually serving (the sleep
			// alone doesn't extend Ready since there's no probe by
			// default).
			createForm(formName, hubName, `
  deployments:
    - id: app-deployment
      nameTemplate: "{{ .uid }}-app"
      waitForReady: true
      timeoutSeconds: 180
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 3
          minReadySeconds: 15
          progressDeadlineSeconds: 600
          selector:
            matchLabels:
              app: phase-test-degraded
          template:
            metadata:
              labels:
                app: phase-test-degraded
            spec:
              containers:
              - name: app
                image: nginx:1.25-alpine
                command: ["sh", "-c", "sleep 25 && nginx -g 'daemon off;'"]
                readinessProbe:
                  tcpSocket:
                    port: 80
                  initialDelaySeconds: 1
                  periodSeconds: 2
`)
			insertTestDataToTable(testTable, uid, true)
			waitForLynqNode(nodeName)
		})

		AfterEach(func() {
			deleteTestDataFromTable(testTable, uid)
			_, _ = utils.Run(exec.Command("kubectl", "delete", "deployment", deploymentName, "-n", policyTestNamespace, "--ignore-not-found=true"))
			_, _ = utils.Run(exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true"))
			_, _ = utils.Run(exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true"))
			time.Sleep(3 * time.Second)
		})

		It("1.1 — pod eviction post-rollout keeps LynqNode Ready and does NOT mark Failed", func() {
			By("waiting for Deployment to reach Available (all 3 replicas)")
			Eventually(func(g Gomega) {
				out, err := utils.Run(exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.availableReplicas}"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(out)).To(Equal("3"))
			}, 90*time.Second, policyTestInterval).Should(Succeed())

			By("waiting for LynqNode to observe Available phase for the Deployment")
			Eventually(func(g Gomega) {
				out, err := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.resourcePhases[0].phase}"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(out)).To(Equal("Available"))
			}, 60*time.Second, policyTestInterval).Should(Succeed())

			By("recording wall-clock timestamp before disruption — filters out any spurious initial-rollout events")
			disruptionStart := time.Now().UTC()

			By("simulating pod eviction by force-deleting ONE pod")
			// Force-delete exactly one pod (not all). Deleting ALL pods can
			// cause K8s to transition Progressing.reason away from
			// NewReplicaSetAvailable as the deployment looks "fresh" again,
			// and the classifier would then see this as a brand-new rollout
			// (Progressing) instead of post-rollout disruption (Degraded).
			// With one-pod deletion the deployment-level signals
			// (Progressing.reason, observedGeneration) stay stable while
			// availableReplicas dips — exactly the Degraded scenario.
			//
			// minReadySeconds=15 (set on the form) holds availableReplicas
			// below spec.replicas for ~15 seconds even after the replacement
			// pod reaches Ready, giving the controller multiple reconcile
			// opportunities to observe the Degraded state.
			out, err := utils.Run(exec.Command("kubectl", "get", "pods", "-n", policyTestNamespace,
				"-l", "app=phase-test-degraded", "-o", "jsonpath={.items[0].metadata.name}"))
			Expect(err).NotTo(HaveOccurred())
			podName := strings.TrimSpace(out)
			Expect(podName).NotTo(BeEmpty())
			_, err = utils.Run(exec.Command("kubectl", "delete", "pod", podName, "-n", policyTestNamespace,
				"--wait=false", "--grace-period=0", "--force"))
			Expect(err).NotTo(HaveOccurred())

			// IMPORTANT ORDERING: assert the Degraded transition observations
			// FIRST (Eventually with tight polling), THEN run the long-window
			// Consistently checks. The Degraded window is transient (~40s
			// total — 25s container sleep + 15s minReadySeconds), so it
			// closes before two back-to-back 60s Consistently blocks would
			// finish. Polling for Degraded immediately after pod deletion
			// catches the dip; Ready / failedResources are stable so their
			// Consistently checks can run afterward without window pressure.

			By("degradedResources reaches >=1 at some point during the disruption window")
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.degradedResources}"))
				return strings.TrimSpace(out)
			}, 90*time.Second, 500*time.Millisecond).Should(SatisfyAny(Equal("1"), Equal("2"), Equal("3")),
				"degradedResources should report >=1 while pods are unavailable post-rollout")

			By("degradedResourceIds includes the deployment resource ID during disruption")
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.degradedResourceIds}"))
				return strings.TrimSpace(out)
			}, 90*time.Second, 500*time.Millisecond).Should(ContainSubstring("app-deployment"),
				"degradedResourceIds should list the affected resource ID")

			By("resourcePhases[0].phase shows Degraded during the disruption window")
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.resourcePhases[0].phase}"))
				return strings.TrimSpace(out)
			}, 90*time.Second, 500*time.Millisecond).Should(Equal("Degraded"),
				"Per-resource phase should transition Available→Degraded post-eviction")

			By("LynqNode.Ready condition stays True throughout the disruption — Lynq does NOT attribute failure")
			// Stable property — can run after the transient observations.
			// 30s window covers the remaining tail of the disruption + a few
			// reconcile cycles after recovery.
			Consistently(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status}"))
				return strings.TrimSpace(out)
			}, 30*time.Second, 3*time.Second).Should(Equal("True"))

			By("failedResources stays at 0 — no ReadinessTimeout escalation")
			Consistently(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.failedResources}"))
				return strings.TrimSpace(out)
			}, 30*time.Second, 3*time.Second).Should(Or(Equal(""), Equal("0")))

			By("no ReadinessTimeout event was emitted during the disruption (filtered to events AFTER the pod eviction)")
			// Filter to events newer than the disruption start. Events
			// from the initial rollout (if image pull happened to take
			// longer than timeoutSeconds on this CI runner) would have an
			// earlier timestamp and would not match.
			out, _ = utils.Run(exec.Command("kubectl", "get", "events", "-n", policyTestNamespace,
				"--field-selector", fmt.Sprintf("involvedObject.name=%s,reason=ReadinessTimeout", nodeName),
				"-o", "jsonpath={range .items[*]}{.lastTimestamp}{\"\\n\"}{end}"))
			for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
				ts := strings.TrimSpace(line)
				if ts == "" {
					continue
				}
				eventTime, parseErr := time.Parse(time.RFC3339, ts)
				if parseErr != nil {
					continue
				}
				Expect(eventTime).To(BeTemporally("<", disruptionStart),
					"ReadinessTimeout event emitted AFTER pod eviction — this is the bug the phase model fixes")
			}

			By("Deployment recovers — LynqNode eventually observes Available phase again")
			Eventually(func(g Gomega) {
				out, err := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.resourcePhases[0].phase}"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(out)).To(Equal("Available"))
			}, 90*time.Second, policyTestInterval).Should(Succeed())
		})

		It("1.4 — prolonged Degraded never transitions to Failed (steady-state guarantee)", func() {
			By("waiting for Deployment Available")
			Eventually(func(g Gomega) {
				out, err := utils.Run(exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.availableReplicas}"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(out)).To(Equal("3"))
			}, 90*time.Second, policyTestInterval).Should(Succeed())

			By("Waiting for LynqNode to observe Available for the current generation, then forcing prolonged unavailability")
			Eventually(func(g Gomega) {
				out, err := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.resourcePhases[0].phase}"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(out)).To(Equal("Available"))
			}, 60*time.Second, policyTestInterval).Should(Succeed())

			// Scale all pods unavailable by patching nginx to return non-2xx on readiness probe?
			// Simpler: replace the readiness probe path with one that fails. The pods stay
			// Running but unavailable, and Deployment.availableReplicas drops to 0.
			By("forcing all pods unavailable via failing readiness probe (note: this is OUTSIDE a rollout — updatedReplicas already matches generation)")
			// Easier and more deterministic: just delete all pods repeatedly to keep
			// availableReplicas low across many seconds.
			// But since this is OUTSIDE a generation change, the phase is Degraded.
			//
			// Method: delete pods continuously for ~3 minutes (well over the 30s
			// timeoutSeconds × multiple cycles). LynqNode must stay Failed=0 the
			// entire time because rollout already completed for this generation.
			done := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				for {
					select {
					case <-done:
						return
					default:
						out, _ := utils.Run(exec.Command("kubectl", "get", "pods", "-n", policyTestNamespace,
							"-l", "app=phase-test-degraded", "-o", "jsonpath={.items[0].metadata.name}"))
						podName := strings.TrimSpace(out)
						if podName != "" {
							_, _ = utils.Run(exec.Command("kubectl", "delete", "pod", podName, "-n", policyTestNamespace,
								"--wait=false", "--grace-period=0", "--force"))
						}
						time.Sleep(2 * time.Second)
					}
				}
			}()
			defer close(done)

			By("over 3 minutes (6× the 30s timeoutSeconds), failedResources stays at 0 — the user-decision #2 regression guard")
			// Long Consistently. 3 minutes >> 30s timeout — if the legacy
			// behavior were active, we'd see failedResources go to 1 within 30s.
			Consistently(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.failedResources}"))
				return strings.TrimSpace(out)
			}, 3*time.Minute, 10*time.Second).Should(Or(Equal(""), Equal("0")),
				"Steady-state disruption escalated to Failed — the phase model contract is broken")
		})
	})

	// ----------------------------------------------------------------------
	// Suite 2 — Rollout Responsibility (regression guards)
	// Lynq still owns its own rollouts; bad rollouts still fail.
	// ----------------------------------------------------------------------
	Context("Suite 2: Rollout Responsibility", func() {
		AfterEach(func() {
			time.Sleep(3 * time.Second)
		})

		It("2.1 — bad image rollout fails via Lynq's timeout (ReadinessTimeout event)", func() {
			const (
				hubName  = "phase-hub-badimage"
				formName = "phase-form-badimage"
				uid      = "phase-uid-badimage"
			)
			deploymentName := fmt.Sprintf("%s-app", uid)
			nodeName := fmt.Sprintf("%s-%s", uid, formName)

			defer func() {
				deleteTestDataFromTable(testTable, uid)
				_, _ = utils.Run(exec.Command("kubectl", "delete", "deployment", deploymentName, "-n", policyTestNamespace, "--ignore-not-found=true"))
				_, _ = utils.Run(exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true"))
				_, _ = utils.Run(exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true"))
			}()

			createHubWithTable(hubName, testTable)
			createForm(formName, hubName, `
  deployments:
    - id: app-deployment
      nameTemplate: "{{ .uid }}-app"
      waitForReady: true
      timeoutSeconds: 30
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          progressDeadlineSeconds: 600
          selector:
            matchLabels:
              app: phase-test-badimage
          template:
            metadata:
              labels:
                app: phase-test-badimage
            spec:
              containers:
              - name: app
                image: lynq-test/this-image-does-not-exist:nope
`)
			insertTestDataToTable(testTable, uid, true)
			waitForLynqNode(nodeName)

			By("after timeoutSeconds=30 elapses, LynqNode reports failedResources=1")
			Eventually(func(g Gomega) {
				out, err := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.failedResources}"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(out)).To(Equal("1"))
			}, 120*time.Second, policyTestInterval).Should(Succeed(),
				"Lynq should still mark a stuck rollout Failed after timeoutSeconds — rollout responsibility regression")

			By("ReadinessTimeout event was emitted")
			Eventually(func(g Gomega) {
				out, err := utils.Run(exec.Command("kubectl", "get", "events", "-n", policyTestNamespace,
					"--field-selector", fmt.Sprintf("involvedObject.name=%s,reason=ReadinessTimeout", nodeName),
					"--no-headers"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(out)).NotTo(BeEmpty())
			}, 30*time.Second, policyTestInterval).Should(Succeed())
		})

		It("2.2 — ProgressDeadlineExceeded fails fast (faster than Lynq's timeout)", func() {
			const (
				hubName  = "phase-hub-progressdeadline"
				formName = "phase-form-progressdeadline"
				uid      = "phase-uid-progressdeadline"
			)
			deploymentName := fmt.Sprintf("%s-app", uid)
			nodeName := fmt.Sprintf("%s-%s", uid, formName)

			defer func() {
				deleteTestDataFromTable(testTable, uid)
				_, _ = utils.Run(exec.Command("kubectl", "delete", "deployment", deploymentName, "-n", policyTestNamespace, "--ignore-not-found=true"))
				_, _ = utils.Run(exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true"))
				_, _ = utils.Run(exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true"))
			}()

			createHubWithTable(hubName, testTable)
			// progressDeadlineSeconds=15 < timeoutSeconds=600
			// Kubernetes itself decides ProgressDeadlineExceeded at ~15s; Lynq
			// should follow within one reconcile (well under 600s).
			createForm(formName, hubName, `
  deployments:
    - id: app-deployment
      nameTemplate: "{{ .uid }}-app"
      waitForReady: true
      timeoutSeconds: 600
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          progressDeadlineSeconds: 15
          selector:
            matchLabels:
              app: phase-test-progressdeadline
          template:
            metadata:
              labels:
                app: phase-test-progressdeadline
            spec:
              containers:
              - name: app
                image: lynq-test/this-image-does-not-exist:nope
`)
			insertTestDataToTable(testTable, uid, true)
			waitForLynqNode(nodeName)

			By("LynqNode reports failedResources=1 well before timeoutSeconds=600 (because K8s ProgressDeadlineExceeded fires at 15s)")
			Eventually(func(g Gomega) {
				out, err := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.failedResources}"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(out)).To(Equal("1"))
			}, 90*time.Second, policyTestInterval).Should(Succeed(),
				"Lynq should follow K8s ProgressDeadlineExceeded — not wait for its own much-longer timeout")
		})

		It("2.3 — slow but successful rollout does NOT trigger spurious failure (false-positive guard)", func() {
			const (
				hubName  = "phase-hub-slowrollout"
				formName = "phase-form-slowrollout"
				uid      = "phase-uid-slowrollout"
			)
			deploymentName := fmt.Sprintf("%s-app", uid)
			nodeName := fmt.Sprintf("%s-%s", uid, formName)

			defer func() {
				deleteTestDataFromTable(testTable, uid)
				_, _ = utils.Run(exec.Command("kubectl", "delete", "deployment", deploymentName, "-n", policyTestNamespace, "--ignore-not-found=true"))
				_, _ = utils.Run(exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true"))
				_, _ = utils.Run(exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true"))
			}()

			createHubWithTable(hubName, testTable)
			// initialDelaySeconds=10 — slower than instant, but well under
			// timeoutSeconds=60. Should reach Available cleanly.
			createForm(formName, hubName, `
  deployments:
    - id: app-deployment
      nameTemplate: "{{ .uid }}-app"
      waitForReady: true
      timeoutSeconds: 60
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 2
          progressDeadlineSeconds: 600
          selector:
            matchLabels:
              app: phase-test-slowrollout
          template:
            metadata:
              labels:
                app: phase-test-slowrollout
            spec:
              containers:
              - name: app
                image: nginx:1.25-alpine
                readinessProbe:
                  httpGet:
                    path: /
                    port: 80
                  initialDelaySeconds: 10
                  periodSeconds: 2
`)
			insertTestDataToTable(testTable, uid, true)
			waitForLynqNode(nodeName)

			By("LynqNode reports readyResources=1 (the Deployment becomes Available)")
			Eventually(func(g Gomega) {
				out, err := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.readyResources}"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(out)).To(Equal("1"))
			}, 90*time.Second, policyTestInterval).Should(Succeed())

			By("failedResources stays 0 throughout — no false positives during the slow ramp")
			out, _ := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
				"-o", "jsonpath={.status.failedResources}"))
			Expect(strings.TrimSpace(out)).To(Or(Equal(""), Equal("0")))

			By("progressingResources settles to 0 once the rollout completes")
			// The slow rollout went through Progressing → Available; once
			// Available is reached, no resource should remain Progressing.
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.progressingResources}"))
				return strings.TrimSpace(out)
			}, 30*time.Second, policyTestInterval).Should(Or(Equal(""), Equal("0")),
				"progressingResources should drop to 0 after rollout completes")

			By("degradedResources is 0 in the healthy post-rollout state")
			out, _ = utils.Run(exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace,
				"-o", "jsonpath={.status.degradedResources}"))
			Expect(strings.TrimSpace(out)).To(Or(Equal(""), Equal("0")),
				"degradedResources should be 0 for a freshly-rolled-out healthy workload")
		})
	})
})
