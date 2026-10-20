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
	"sort"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8s-lynq/lynq/test/utils"
)

var _ = Describe("Resource Timeout and Failure Handling", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up test table")
		testTable = setupTestTable("resource_timeout")
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	Context("when resources fail to become ready within timeout", func() {
		const (
			hubName  = "timeout-hub"
			formName = "timeout-form"
			uid      = "timeout-test-uid"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHubWithTable(hubName, testTable)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestDataFromTable(testTable, uid)

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
				insertTestDataToTable(testTable, uid, true)

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
				insertTestDataToTable(testTable, uid, true)

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

	// A Deployment that Lynq applied long ago and that later loses pods is being repaired by its
	// own controller. timeoutSeconds measures whether Lynq's change became Ready, so such a
	// disruption must surface as Ready=False only: no ReadinessTimeout, no failed resources, no
	// Degraded condition (which drives the critical alerts).
	Context("when a previously ready Deployment is disrupted after its timeout has elapsed", func() {
		// Replacement pods stay unready longer than the 30s periodic requeue, so the
		// controller is guaranteed to observe the Deployment not ready while it recovers.
		const replacementUnreadySeconds = 45
		disruptionWindow := (replacementUnreadySeconds + 15) * time.Second

		var (
			run int
			d   *steadyDeployment
		)

		BeforeEach(func() {
			run++
			d = newSteadyDeployment("disruption", run, replacementUnreadySeconds)
			d.givenReadyPastItsTimeout(testTable)
		})

		AfterEach(func() {
			d.cleanup(testTable)
		})

		It("should not fail the Deployment when one of its pods is lost", func() {
			timeoutEventsBefore := readinessTimeoutEventCount(d.nodeName)

			By("When one of the Deployment's pods is deleted")
			cmd := exec.Command("kubectl", "get", "pods", "-n", policyTestNamespace, "-l", d.appLabel,
				"-o", "jsonpath={.items[0].metadata.name}")
			podName, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(podName).NotTo(BeEmpty())
			cmd = exec.Command("kubectl", "delete", "pod", podName, "-n", policyTestNamespace, "--wait=false")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			d.expectDisruptionIsNotAFailure(timeoutEventsBefore, disruptionWindow)
		})

		It("should not fail the Deployment when all of its pods are lost", func() {
			timeoutEventsBefore := readinessTimeoutEventCount(d.nodeName)

			By("When every pod of the Deployment is deleted")
			cmd := exec.Command("kubectl", "delete", "pod", "-n", policyTestNamespace, "-l", d.appLabel, "--wait=false")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			d.expectDisruptionIsNotAFailure(timeoutEventsBefore, disruptionWindow)
		})
	})
})

// A `kubectl rollout restart` of a long-Ready Deployment is a disruption Lynq did not start, and
// Lynq keeps applying template changes to the Deployment while it runs. Only a change to the
// Deployment's .spec starts a rollout of Lynq's own, which timeoutSeconds measures; anything else
// must leave the restart a disruption. metadata.generation is not that signal: the Deployment
// registry strategy also moves it when metadata.annotations change, because annotations are
// copied onto the ReplicaSet, and every Lynq apply that changes the desired hash re-stamps its
// own annotations. The scenarios below therefore compare .spec, never generation, to tell a
// metadata-only change from a rollout.
//
// The Deployment is gated: a pod that carries the kubectl.kubernetes.io/restartedAt annotation a
// restart adds stays unready until the test opens its gate. A restart therefore keeps the
// Deployment not ready for exactly as long as a scenario needs, however fast or slow the cluster.
var _ = Describe("Readiness Regression", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up test table")
		testTable = setupTestTable("readiness_regression")
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	Context("when a previously ready Deployment is restarted or edited after its timeout has elapsed", func() {
		// Longer than the 10s timeout plus the 30s periodic requeue, with margin: a controller
		// that timed the restart would have failed it before the gate opens.
		const gateHoldWindow = 60 * time.Second
		// Long enough for the 30s periodic requeue to judge the Deployment after an edit, more
		// than 10s later.
		const editWindow = 45 * time.Second

		var (
			run int
			d   *steadyDeployment
		)

		BeforeEach(func() {
			run++
			d = newGatedDeployment("restart", run)
			d.givenReadyPastItsTimeout(testTable)
		})

		AfterEach(func() {
			d.cleanup(testTable)
		})

		It("should not fail the Deployment while an external rollout restart replaces its pods", func() {
			timeoutEventsBefore := readinessTimeoutEventCount(d.nodeName)

			By("When the Deployment is restarted outside Lynq")
			d.rolloutRestart()
			d.waitUntilSeenNotReady()

			d.expectHeldRestartIsNotAFailure(timeoutEventsBefore, gateHoldWindow)
			d.openGateUntilReady()
			Expect(readinessTimeoutEventCount(d.nodeName)).To(Equal(timeoutEventsBefore))
		})

		It("should see a spec change applied during a restart become ready as Lynq's own rollout", func() {
			timeoutEventsBefore := readinessTimeoutEventCount(d.nodeName)

			By("Given the Deployment is being restarted outside Lynq")
			d.rolloutRestart()
			d.waitUntilSeenNotReady()
			generationBefore := d.deploymentInt(Default, generationPath)

			By("When the LynqForm changes the Deployment's spec, with a timeout its rollout can meet")
			d.applyForm(steadyDeploymentChange{release: "v2", timeoutSeconds: 180})

			By("Then the change reaches the Deployment as a new generation")
			var generationAfter int64
			Eventually(func(g Gomega) {
				g.Expect(d.deploymentField(releasePath)).To(Equal("v2"))
				generationAfter = d.deploymentInt(g, generationPath)
				g.Expect(generationAfter).To(BeNumerically(">", generationBefore))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the new generation is rolling out and not ready yet")
			Eventually(func(g Gomega) {
				g.Expect(d.deploymentInt(g, "{.status.observedGeneration}")).To(Equal(generationAfter))
				g.Expect(d.deploymentInt(g, "{.status.availableReplicas}")).To(BeNumerically("<", 2))
				g.Expect(lynqNodeField(d.nodeName, readyConditionPath)).To(Equal("False"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
			d.expectNotFailed(timeoutEventsBefore)

			By("And once its pods are ready, the new rollout becomes Ready without ever being failed")
			d.openGateUntilReady()
			Expect(readinessTimeoutEventCount(d.nodeName)).To(Equal(timeoutEventsBefore))

			By("And the Deployment is fully rolled out at the new generation")
			Eventually(func(g Gomega) {
				g.Expect(d.deploymentInt(g, generationPath)).To(Equal(generationAfter))
				g.Expect(d.deploymentInt(g, "{.status.observedGeneration}")).To(Equal(generationAfter))
				g.Expect(d.deploymentInt(g, "{.status.updatedReplicas}")).To(Equal(int64(2)))
				g.Expect(d.deploymentInt(g, "{.status.readyReplicas}")).To(Equal(int64(2)))
				g.Expect(d.deploymentInt(g, "{.status.availableReplicas}")).To(Equal(int64(2)))
			}, time.Minute, policyTestInterval).Should(Succeed())
		})

		It("should still time out a spec change applied during a restart that never becomes ready", func() {
			timeoutEventsBefore := readinessTimeoutEventCount(d.nodeName)

			By("Given the Deployment is being restarted outside Lynq, and the restart alone has not failed it")
			d.rolloutRestart()
			d.waitUntilSeenNotReady()
			d.expectNotFailed(timeoutEventsBefore)
			generationBefore := d.deploymentInt(Default, generationPath)
			applyStampBefore, err := d.deploymentField(applyStartTimePath)
			Expect(err).NotTo(HaveOccurred())

			By("When the LynqForm changes the Deployment's readiness probe to one that always fails")
			d.applyForm(steadyDeploymentChange{readinessCommand: []string{"false"}})

			By("Then Lynq applies it as a new generation of its own")
			Eventually(func(g Gomega) {
				g.Expect(d.deploymentField("{.spec.template.spec.containers[0].readinessProbe.exec.command[0]}")).To(Equal("false"))
				g.Expect(d.deploymentInt(g, generationPath)).To(BeNumerically(">", generationBefore))
				g.Expect(d.deploymentField(applyStartTimePath)).NotTo(Equal(applyStampBefore))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
			// Lynq's change cannot be failed before its 10s timeout has elapsed and a later
			// reconcile has judged it, so whatever is failed from here on is that change.
			d.expectNotFailed(timeoutEventsBefore)

			By("And that change is failed once its 10s timeout elapses, the restart notwithstanding")
			Eventually(func(g Gomega) {
				failed, err := lynqNodeField(d.nodeName, failedResourcesPath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strconv.Atoi(failed)).To(BeNumerically(">=", 1))
				g.Expect(readinessTimeoutEventCount(d.nodeName)).To(BeNumerically(">", timeoutEventsBefore))
				g.Expect(lynqNodeField(d.nodeName, degradedConditionPath)).To(Equal("True"))
			}, 3*time.Minute, policyTestInterval).Should(Succeed())
		})

		// Regression guard. A template change that only touches the Deployment's metadata changes
		// Lynq's desired hash, so its apply re-stamps lynq.sh/apply-start-time and
		// lynq.sh/applied-hash. Those annotation changes move the Deployment's metadata.generation
		// (the registry strategy bumps it on annotation changes too), but the .spec is untouched
		// and no rollout starts. Judging the restart against that stamp, or treating the moved
		// generation as a rollout of Lynq's own, re-timed it from the metadata apply and failed it:
		// a ReadinessTimeout and a Degraded alert for a restart Lynq did not start. The gate keeps
		// the restart not ready across the whole timeout and the next periodic reconcile after
		// that apply, so such a controller fails this test every time.
		It("should not fail a restarting Deployment when the LynqForm only changes its metadata", func() {
			timeoutEventsBefore := readinessTimeoutEventCount(d.nodeName)

			By("Given the Deployment is being restarted outside Lynq, and the restart alone has not failed it")
			d.rolloutRestart()
			d.waitUntilSeenNotReady()
			d.expectNotFailed(timeoutEventsBefore)
			specBefore, err := d.deploymentField(specPath)
			Expect(err).NotTo(HaveOccurred())
			applyStampBefore, err := d.deploymentField(applyStartTimePath)
			Expect(err).NotTo(HaveOccurred())

			By("When the LynqForm only adds a label to the Deployment")
			d.applyForm(steadyDeploymentChange{labels: map[string]string{"team": "payments"}})

			By("Then Lynq applies it and re-stamps apply-start-time")
			Eventually(func(g Gomega) {
				g.Expect(d.deploymentField("{.metadata.labels.team}")).To(Equal("payments"))
				g.Expect(d.deploymentField(applyStartTimePath)).NotTo(Equal(applyStampBefore))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And that apply left the Deployment's spec untouched, so the restart is still not ready")
			Expect(d.deploymentField(specPath)).To(Equal(specBefore))
			Expect(d.deploymentInt(Default, "{.status.availableReplicas}")).To(BeNumerically("<", 2))

			d.expectHeldRestartIsNotAFailure(timeoutEventsBefore, gateHoldWindow)
			Expect(d.deploymentField(specPath)).To(Equal(specBefore))
			d.openGateUntilReady()
			Expect(readinessTimeoutEventCount(d.nodeName)).To(Equal(timeoutEventsBefore))
		})

		It("should keep the Deployment Ready when its metadata is edited outside Lynq", func() {
			timeoutEventsBefore := readinessTimeoutEventCount(d.nodeName)
			specBefore, err := d.deploymentField(specPath)
			Expect(err).NotTo(HaveOccurred())
			revisionBefore, err := d.deploymentField(revisionPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(revisionBefore).NotTo(BeEmpty())

			By("When another party annotates the Deployment")
			cmd := exec.Command("kubectl", "annotate", "deployment", d.deploymentName, "-n", policyTestNamespace,
				"example.com/owner=other", "--overwrite")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Neither resourceVersion nor generation is compared: the annotation itself moves the
			// Deployment's generation (the registry strategy bumps it on annotation changes), and
			// the Deployment controller's status writes and Lynq's periodic force-reapply may
			// legitimately move resourceVersion. A spurious rollout would change the spec and
			// create a new ReplicaSet, which bumps the revision.
			By("Then the annotation is kept and no rollout starts")
			d.expectStaysHealthy(timeoutEventsBefore, editWindow, func(g Gomega) {
				g.Expect(d.deploymentField(`{.metadata.annotations.example\.com/owner}`)).To(Equal("other"))
				g.Expect(d.deploymentField(specPath)).To(Equal(specBefore))
				g.Expect(d.deploymentField(revisionPath)).To(Equal(revisionBefore))
			})

			By("When Lynq's applied-hash annotation is removed from the Deployment")
			appliedHashBefore, err := d.deploymentField(appliedHashPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(appliedHashBefore).NotTo(BeEmpty())
			// The removal's own response shows the annotation gone, which a later read could not:
			// Lynq may already have restored it by then.
			cmd = exec.Command("kubectl", "annotate", "deployment", d.deploymentName, "-n", policyTestNamespace,
				"lynq.sh/applied-hash-", "-o", "jsonpath=applied-hash=[{.metadata.annotations.lynq\\.sh/applied-hash}]")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("applied-hash=[]"))

			By("Then Lynq re-applies the Deployment and restores it")
			Eventually(func(g Gomega) {
				g.Expect(d.deploymentField(appliedHashPath)).To(Equal(appliedHashBefore))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			d.expectStaysHealthy(timeoutEventsBefore, editWindow, nil)
		})
	})
})

const (
	readyConditionPath    = "{.status.conditions[?(@.type=='Ready')].status}"
	degradedConditionPath = "{.status.conditions[?(@.type=='Degraded')].status}"
	failedResourcesPath   = "{.status.failedResources}"
	generationPath        = "{.metadata.generation}"
	specPath              = "{.spec}"
	revisionPath          = `{.metadata.annotations.deployment\.kubernetes\.io/revision}`
	applyStartTimePath    = `{.metadata.annotations.lynq\.sh/apply-start-time}`
	appliedHashPath       = `{.metadata.annotations.lynq\.sh/applied-hash}`
	releasePath           = "{.spec.template.spec.containers[0].env[?(@.name=='RELEASE')].value}"
)

// steadyDeployment is one It's LynqHub, LynqForm and LynqNode around a 2-replica Deployment
// with a 10s readiness timeout. Every It gets its own names and pod label, so a previous It's
// objects that are still terminating can neither collide with this one's nor match its pod
// selector.
type steadyDeployment struct {
	hubName, formName, uid, nodeName string
	deploymentName, label, appLabel  string
	// readinessDelaySeconds is how long every pod is unready after it starts.
	readinessDelaySeconds int
	// gated Deployments are recreated rather than rolled, and a pod that a rollout restart
	// created stays unready until openGate marks it ready.
	gated bool
}

func newSteadyDeployment(prefix string, run, readinessDelaySeconds int) *steadyDeployment {
	d := &steadyDeployment{
		hubName:               fmt.Sprintf("%s-hub-%d", prefix, run),
		formName:              fmt.Sprintf("%s-form-%d", prefix, run),
		uid:                   fmt.Sprintf("%s-uid-%d", prefix, run),
		label:                 fmt.Sprintf("%s-test-%d", prefix, run),
		readinessDelaySeconds: readinessDelaySeconds,
	}
	d.nodeName = fmt.Sprintf("%s-%s", d.uid, d.formName)
	d.deploymentName = d.uid + "-steady-deploy"
	d.appLabel = "app=" + d.label
	return d
}

// newGatedDeployment returns a steadyDeployment whose original pods are ready at once and whose
// restarted pods stay unready until the test opens their gate.
//
// The default rolling update of a 2-replica Deployment surges one pod and keeps both old pods
// available, so a restart would leave availableReplicas at 2 and the Deployment Ready the whole
// time. Recreate takes every old pod down first, so the Deployment is not ready until the gate
// opens.
func newGatedDeployment(prefix string, run int) *steadyDeployment {
	d := newSteadyDeployment(prefix, run, 0)
	d.gated = true
	return d
}

// steadyDeploymentChange is a LynqForm edit relative to the initial template. Zero fields keep
// the initial value.
type steadyDeploymentChange struct {
	release          string            // value of the RELEASE env var (a spec change)
	timeoutSeconds   int               // the resource's timeoutSeconds (a Lynq setting, not spec)
	labels           map[string]string // labelsTemplate entries (a metadata-only change)
	readinessCommand []string          // the readiness probe's exec command (a spec change)
}

// applyForm creates or updates the LynqForm with the given change applied.
func (d *steadyDeployment) applyForm(c steadyDeploymentChange) {
	if c.release == "" {
		c.release = "v1"
	}
	if c.timeoutSeconds == 0 {
		c.timeoutSeconds = 10
	}
	command := []string{"sh", "-c", "sleep 3600"}
	strategy, volumeMounts, volumes := "", "", ""
	if d.gated {
		// The pod reads its own annotations through the downward API: only a pod without
		// restartedAt marks itself ready.
		command = []string{"sh", "-c", "grep -q restartedAt /etc/podinfo/annotations || touch /tmp/ready; exec sleep 3600"}
		strategy = `
          strategy:
            type: Recreate`
		volumeMounts = `
                volumeMounts:
                - name: podinfo
                  mountPath: /etc/podinfo`
		volumes = `
              volumes:
              - name: podinfo
                downwardAPI:
                  items:
                  - path: annotations
                    fieldRef:
                      fieldPath: metadata.annotations`
	}
	if c.readinessCommand == nil {
		c.readinessCommand = []string{"true"}
		if d.gated {
			c.readinessCommand = []string{"test", "-f", "/tmp/ready"}
		}
	}
	labels := ""
	if len(c.labels) > 0 {
		keys := make([]string, 0, len(c.labels))
		for k := range c.labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		labels = "      labelsTemplate:\n"
		for _, k := range keys {
			labels += fmt.Sprintf("        %s: %q\n", k, c.labels[k])
		}
	}
	createForm(d.formName, d.hubName, fmt.Sprintf(`
  deployments:
    - id: steady-deployment
      nameTemplate: "{{ .uid }}-steady-deploy"
      waitForReady: true
      timeoutSeconds: %[3]d
%[4]s      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 2%[8]s
          selector:
            matchLabels:
              app: %[1]s
          template:
            metadata:
              labels:
                app: %[1]s
            spec:
              terminationGracePeriodSeconds: 0
              containers:
              - name: app
                image: busybox:1.36
                command: %[7]s
                env:
                - name: RELEASE
                  value: %[5]q
                readinessProbe:
                  exec:
                    command: %[6]s
                  initialDelaySeconds: %[2]d
                  periodSeconds: 2%[9]s%[10]s
`, d.label, d.readinessDelaySeconds, c.timeoutSeconds, labels, c.release,
		yamlFlowList(c.readinessCommand), yamlFlowList(command), strategy, volumeMounts, volumes))
}

// yamlFlowList renders strings as a YAML flow sequence, e.g. ["sh", "-c", "sleep 3600"].
func yamlFlowList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = strconv.Quote(item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// givenReadyPastItsTimeout creates the hub, the form and the row, waits for the LynqNode to be
// Ready, and then lets it stay Ready for longer than its timeout.
func (d *steadyDeployment) givenReadyPastItsTimeout(testTable string) {
	By("creating a LynqHub")
	createHubWithTable(d.hubName, testTable)

	By("and a LynqForm with a 2-replica Deployment and a 10s readiness timeout")
	d.applyForm(steadyDeploymentChange{})

	By("and active data in MySQL")
	insertTestDataToTable(testTable, d.uid, true)
	waitForLynqNode(d.nodeName)

	By("and the LynqNode has become Ready")
	Eventually(func(g Gomega) {
		g.Expect(lynqNodeField(d.nodeName, readyConditionPath)).To(Equal("True"))
	}, policyTestTimeout, policyTestInterval).Should(Succeed())

	By("and it has stayed Ready for longer than its timeout")
	time.Sleep(15 * time.Second)
	Expect(lynqNodeField(d.nodeName, readyConditionPath)).To(Equal("True"))
}

// cleanup deletes this run's objects and waits until every one of them is gone.
func (d *steadyDeployment) cleanup(testTable string) {
	By("cleaning up test data and resources")
	deleteTestDataFromTable(testTable, d.uid)

	cmd := exec.Command("kubectl", "delete", "lynqform", d.formName, "-n", policyTestNamespace, "--ignore-not-found=true")
	_, _ = utils.Run(cmd)
	cmd = exec.Command("kubectl", "delete", "deployment", d.deploymentName, "-n", policyTestNamespace,
		"--ignore-not-found=true", "--wait=false")
	_, _ = utils.Run(cmd)
	cmd = exec.Command("kubectl", "delete", "lynqhub", d.hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
	_, _ = utils.Run(cmd)

	By("and waiting until every object of this run is gone")
	for _, obj := range [][2]string{
		{"lynqnode", d.nodeName},
		{"lynqform", d.formName},
		{"lynqhub", d.hubName},
		{"deployment", d.deploymentName},
	} {
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", obj[0], obj[1], "-n", policyTestNamespace)
			output, err := utils.Run(cmd)
			g.Expect(err).To(HaveOccurred(), "%s/%s still exists", obj[0], obj[1])
			g.Expect(output).To(ContainSubstring("NotFound"))
		}, policyTestTimeout, policyTestInterval).Should(Succeed())
	}
}

// rolloutRestart restarts the Deployment the way an operator would, outside Lynq.
func (d *steadyDeployment) rolloutRestart() {
	cmd := exec.Command("kubectl", "rollout", "restart", "deployment/"+d.deploymentName, "-n", policyTestNamespace)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
}

// waitUntilSeenNotReady waits until the controller has reported the LynqNode not Ready, so a
// following template change is applied while the controller already knows of the disruption.
func (d *steadyDeployment) waitUntilSeenNotReady() {
	Eventually(func(g Gomega) {
		g.Expect(lynqNodeField(d.nodeName, readyConditionPath)).To(Equal("False"))
	}, time.Minute, time.Second).Should(Succeed())
}

// openGate marks every running pod of a gated Deployment ready. A pod that is starting or
// terminating may refuse the exec; the caller retries until the Deployment is ready.
func (d *steadyDeployment) openGate() {
	cmd := exec.Command("kubectl", "get", "pods", "-n", policyTestNamespace, "-l", d.appLabel,
		"-o", `jsonpath={range .items[?(@.status.phase=="Running")]}{.metadata.name}{" "}{end}`)
	output, err := utils.Run(cmd)
	if err != nil {
		return
	}
	for _, pod := range strings.Fields(output) {
		cmd := exec.Command("kubectl", "exec", pod, "-n", policyTestNamespace, "-c", "app", "--", "touch", "/tmp/ready")
		_, _ = utils.Run(cmd)
	}
}

// openGateUntilReady opens the gate of every pod, including pods that start meanwhile, until the
// LynqNode is Ready again, and fails if the node counted the Deployment as failed meanwhile.
func (d *steadyDeployment) openGateUntilReady() {
	By("And once the restarted pods are let become ready, the LynqNode returns to Ready without being failed")
	sawFailed := false
	Eventually(func(g Gomega) {
		d.openGate()
		failed, err := lynqNodeField(d.nodeName, failedResourcesPath)
		g.Expect(err).NotTo(HaveOccurred())
		if failed != "" && failed != "0" {
			sawFailed = true
		}
		g.Expect(lynqNodeField(d.nodeName, readyConditionPath)).To(Equal("True"))
		g.Expect(lynqNodeField(d.nodeName, "{.status.readyResources}")).To(Equal("1"))
	}, policyTestTimeout, policyTestInterval).Should(Succeed())
	Expect(sawFailed).To(BeFalse(), "a Deployment that became ready must never have been counted as failed")
}

// deploymentField returns a jsonpath field of the Deployment.
func (d *steadyDeployment) deploymentField(jsonPath string) (string, error) {
	cmd := exec.Command("kubectl", "get", "deployment", d.deploymentName, "-n", policyTestNamespace, "-o", "jsonpath="+jsonPath)
	output, err := utils.Run(cmd)
	return strings.TrimSpace(output), err
}

// deploymentInt returns an integer jsonpath field of the Deployment; an absent field, such as a
// status counter the API server omits at zero, is 0.
func (d *steadyDeployment) deploymentInt(g Gomega, jsonPath string) int64 {
	raw, err := d.deploymentField(jsonPath)
	g.Expect(err).NotTo(HaveOccurred())
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	g.Expect(err).NotTo(HaveOccurred())
	return n
}

// expectNotFailed verifies the LynqNode has not counted the Deployment as failed, is not
// Degraded, and has recorded no ReadinessTimeout since the baseline.
func (d *steadyDeployment) expectNotFailed(timeoutEventsBefore int) {
	Expect(lynqNodeField(d.nodeName, failedResourcesPath)).To(BeElementOf("", "0"))
	Expect(lynqNodeField(d.nodeName, degradedConditionPath)).NotTo(Equal("True"))
	Expect(readinessTimeoutEventCount(d.nodeName)).To(Equal(timeoutEventsBefore))
}

// expectDisruptionIsNotAFailure verifies the node rides out a disruption: it is seen not
// Ready, never reports a failed resource, a ReadinessTimeout or Degraded=True for the whole
// window, and returns to Ready once the replacement pods are ready.
func (d *steadyDeployment) expectDisruptionIsNotAFailure(timeoutEventsBefore int, window time.Duration) {
	By("Then the LynqNode is reported not Ready while the Deployment recovers, but never failed or Degraded")
	sawNotReady := false
	Consistently(func(g Gomega) {
		ready, err := lynqNodeField(d.nodeName, readyConditionPath)
		g.Expect(err).NotTo(HaveOccurred())
		if ready == "False" {
			sawNotReady = true
		}
		g.Expect(lynqNodeField(d.nodeName, failedResourcesPath)).To(BeElementOf("", "0"),
			"a disrupted, previously ready Deployment must not be counted as failed")
		g.Expect(lynqNodeField(d.nodeName, degradedConditionPath)).NotTo(Equal("True"),
			"a not-ready-only LynqNode must not be Degraded")
	}, window, policyTestInterval).Should(Succeed())
	Expect(sawNotReady).To(BeTrue(), "the controller must have observed the disruption for this test to be meaningful")

	By("And no ReadinessTimeout event was emitted for the disruption")
	Expect(readinessTimeoutEventCount(d.nodeName)).To(Equal(timeoutEventsBefore))

	By("And the LynqNode returns to Ready once the replacement pods are ready")
	Eventually(func(g Gomega) {
		g.Expect(lynqNodeField(d.nodeName, readyConditionPath)).To(Equal("True"))
		g.Expect(lynqNodeField(d.nodeName, "{.status.readyResources}")).To(Equal("1"))
	}, policyTestTimeout, policyTestInterval).Should(Succeed())
}

// expectHeldRestartIsNotAFailure holds a gated Deployment's restart not ready for the whole
// window and verifies the LynqNode reports it not Ready only: never a failed resource,
// Degraded=True or a ReadinessTimeout.
func (d *steadyDeployment) expectHeldRestartIsNotAFailure(timeoutEventsBefore int, window time.Duration) {
	By("Then, while the restarted pods are held unready, the LynqNode is not Ready but never failed or Degraded")
	Consistently(func(g Gomega) {
		g.Expect(d.deploymentInt(g, "{.status.availableReplicas}")).To(BeNumerically("<", 2),
			"the restart must stay not ready for this window to mean anything")
		g.Expect(lynqNodeField(d.nodeName, readyConditionPath)).To(Equal("False"))
		g.Expect(lynqNodeField(d.nodeName, failedResourcesPath)).To(BeElementOf("", "0"),
			"a restarting, previously ready Deployment must not be counted as failed")
		g.Expect(lynqNodeField(d.nodeName, degradedConditionPath)).NotTo(Equal("True"),
			"a not-ready-only LynqNode must not be Degraded")
		g.Expect(readinessTimeoutEventCount(d.nodeName)).To(Equal(timeoutEventsBefore),
			"a restart Lynq did not start must not be timed out")
	}, window, policyTestInterval).Should(Succeed())
}

// expectStaysHealthy verifies an edit that is not a disruption leaves the node alone: no
// failed resource, no Degraded=True and no ReadinessTimeout for the whole window, and Ready.
// also, when given, is checked on every poll of the window too.
func (d *steadyDeployment) expectStaysHealthy(timeoutEventsBefore int, window time.Duration, also func(g Gomega)) {
	By("Then the Deployment is never counted as failed and the LynqNode never Degraded")
	Consistently(func(g Gomega) {
		g.Expect(lynqNodeField(d.nodeName, failedResourcesPath)).To(BeElementOf("", "0"))
		g.Expect(lynqNodeField(d.nodeName, degradedConditionPath)).NotTo(Equal("True"))
		g.Expect(readinessTimeoutEventCount(d.nodeName)).To(Equal(timeoutEventsBefore))
		if also != nil {
			also(g)
		}
	}, window, policyTestInterval).Should(Succeed())

	By("And the LynqNode is Ready")
	Eventually(func(g Gomega) {
		g.Expect(lynqNodeField(d.nodeName, readyConditionPath)).To(Equal("True"))
	}, policyTestTimeout, policyTestInterval).Should(Succeed())
}

// lynqNodeField returns a jsonpath field of a LynqNode in the policy test namespace.
func lynqNodeField(nodeName, jsonPath string) (string, error) {
	cmd := exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace, "-o", "jsonpath="+jsonPath)
	output, err := utils.Run(cmd)
	return strings.TrimSpace(output), err
}

// readinessTimeoutEventCount returns how many ReadinessTimeout events have been recorded for a
// LynqNode, including repeats aggregated into a single Event's count.
func readinessTimeoutEventCount(nodeName string) int {
	cmd := exec.Command("kubectl", "get", "events", "-n", policyTestNamespace,
		"--field-selector", "involvedObject.name="+nodeName+",reason=ReadinessTimeout",
		"-o", "jsonpath={range .items[*]}count={.count}{\"\\n\"}{end}")
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())

	total := 0
	for _, line := range strings.Fields(output) {
		raw := strings.TrimPrefix(line, "count=")
		if raw == "" {
			total++ // an Event without an aggregated count occurred once
			continue
		}
		n, err := strconv.Atoi(raw)
		Expect(err).NotTo(HaveOccurred())
		total += n
	}
	return total
}
