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

// Rollout Progress Deadline runs as its own CI group: each scenario waits out a
// progressDeadlineSeconds of 60s (the minimum) plus the LynqForm controller's 1-minute requeue.
var _ = Describe("Rollout Progress Deadline", Ordered, func() {
	const (
		hubName  = "deadline-hub"
		formName = "deadline-form"
		// The minimum the CRD allows.
		progressDeadline = 60 * time.Second
		// A stalled node is reported within one deadline plus one LynqForm requeue (1 minute).
		stallReportWindow = progressDeadline + time.Minute + 30*time.Second
	)

	var (
		testTable string
		uids      = []string{"deadline-node-1", "deadline-node-2"}
	)

	nodeName := func(uid string) string { return fmt.Sprintf("%s-%s", uid, formName) }
	deploymentName := func(uid string) string { return uid + "-gated-deploy" }

	BeforeAll(func() {
		By("setting up test table")
		testTable = setupTestTable("rollout_deadline")

		By("Given a LynqHub with two active rows")
		createHubWithTable(hubName, testTable)
		for _, uid := range uids {
			insertTestDataToTable(testTable, uid, true)
		}

		By("And a serial rollout (maxSkew=1, progressDeadlineSeconds=60) whose Deployments are Ready")
		applyDeadlineForm(formName, hubName, deadlineFormRevision{release: "v1"})
		Eventually(func(g Gomega) {
			for _, uid := range uids {
				g.Expect(lynqNodeField(nodeName(uid), readyConditionPath)).To(Equal("True"))
			}
			g.Expect(formField(formName, rolloutPhasePath)).To(Equal("Complete"))
		}, 3*time.Minute, policyTestInterval).Should(Succeed())

		By("And the hub records each node's completed update on the node")
		Eventually(func(g Gomega) {
			for _, uid := range uids {
				expectUpdateCompleted(g, nodeName(uid))
			}
		}, time.Minute, policyTestInterval).Should(Succeed())
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		for _, uid := range uids {
			deleteTestDataFromTable(testTable, uid)
		}
		cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
		cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	It("should report the rollout Failed, naming the node, when an updated node is not Ready within progressDeadlineSeconds", func() {
		By("When the LynqForm changes the Deployment to a readiness probe that never passes")
		// timeoutSeconds is far above the deadline, so the LynqNode controller does not fail the
		// resource first: the rollout deadline alone must report this stall.
		applyDeadlineForm(formName, hubName, deadlineFormRevision{release: "v2", readinessCommand: []string{"false"}, timeoutSeconds: 900})
		targetGen := formField(formName, "{.metadata.generation}")

		By("Then exactly one node is updated, and it does not become Ready")
		var stuck, waiting string
		Eventually(func(g Gomega) {
			var updated []string
			for _, uid := range uids {
				gen, err := lynqNodeField(nodeName(uid), templateGenerationPath)
				g.Expect(err).NotTo(HaveOccurred())
				if gen == targetGen {
					updated = append(updated, uid)
				}
			}
			g.Expect(updated).To(HaveLen(1))
			stuck = updated[0]
			g.Expect(lynqNodeField(nodeName(stuck), readyConditionPath)).To(Equal("False"))
		}, policyTestTimeout, policyTestInterval).Should(Succeed())
		for _, uid := range uids {
			if uid != stuck {
				waiting = uid
			}
		}

		By("And once the deadline has passed, the rollout is Failed and names the stuck node")
		Eventually(func(g Gomega) {
			g.Expect(formField(formName, rolloutPhasePath)).To(Equal("Failed"))
			g.Expect(formField(formName, rolloutMessagePath)).To(ContainSubstring(nodeName(stuck) + " (not Ready within 60s)"))
		}, stallReportWindow, policyTestInterval).Should(Succeed())
		// The event is recorded asynchronously after the status write.
		Eventually(func() int { return formEventCount(formName, "RolloutFailed") },
			30*time.Second, time.Second).Should(Equal(1))

		By("And its update is not recorded as completed")
		Expect(lynqNodeField(nodeName(stuck), rolloutCompletedPath)).To(BeEmpty())

		By("And the stuck node is progressing, not failed: the hub does not count it as failed")
		Expect(lynqNodeField(nodeName(stuck), degradedConditionPath)).NotTo(Equal("True"))
		Eventually(func(g Gomega) { expectHubHasNoFailedNodes(g, hubName, len(uids)) },
			time.Minute, policyTestInterval).Should(Succeed())

		By("And the stuck node keeps its maxSkew slot, so the other node is not updated")
		Consistently(func(g Gomega) {
			g.Expect(lynqNodeField(nodeName(waiting), templateGenerationPath)).NotTo(Equal(targetGen))
			g.Expect(formField(formName, rolloutPhasePath)).To(Equal("Failed"))
		}, 30*time.Second, policyTestInterval).Should(Succeed())
	})

	It("should resume and complete the rollout once the LynqForm is fixed", func() {
		By("When the LynqForm restores a readiness probe that passes")
		applyDeadlineForm(formName, hubName, deadlineFormRevision{release: "v3"})
		targetGen := formField(formName, "{.metadata.generation}")

		By("Then every node is updated and Ready, and the rollout completes")
		Eventually(func(g Gomega) {
			for _, uid := range uids {
				g.Expect(lynqNodeField(nodeName(uid), templateGenerationPath)).To(Equal(targetGen))
				g.Expect(lynqNodeField(nodeName(uid), readyConditionPath)).To(Equal("True"))
			}
			g.Expect(formField(formName, rolloutPhasePath)).To(Equal("Complete"))
		}, 3*time.Minute, policyTestInterval).Should(Succeed())

		By("And every node's completion is recorded for the new generation")
		Eventually(func(g Gomega) {
			for _, uid := range uids {
				g.Expect(lynqNodeField(nodeName(uid), rolloutCompletedPath)).To(Equal(targetGen))
			}
		}, time.Minute, policyTestInterval).Should(Succeed())

		By("And the recovery was announced")
		Eventually(func() int { return formEventCount(formName, "RolloutRecovered") },
			30*time.Second, time.Second).Should(BeNumerically(">=", 1))
	})

	It("should not fail a completed rollout when an updated node regresses past the deadline", func() {
		failedEventsBefore := formEventCount(formName, "RolloutFailed")
		regressed := uids[0]
		Expect(formField(formName, rolloutPhasePath)).To(Equal("Complete"))
		completionTimeBefore := formField(formName, rolloutCompletionTimePath)
		Expect(completionTimeBefore).NotTo(BeEmpty())

		By("When a node's Deployment is restarted outside Lynq and its new pod is held unready")
		cmd := exec.Command("kubectl", "rollout", "restart", "deployment/"+deploymentName(regressed), "-n", policyTestNamespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func(g Gomega) {
			g.Expect(lynqNodeField(nodeName(regressed), readyConditionPath)).To(Equal("False"))
		}, time.Minute, time.Second).Should(Succeed())

		// The LynqForm only sees the regression on its next reconcile; wait for it so the window
		// below starts from a status that already reflects it.
		Eventually(func(g Gomega) {
			g.Expect(formField(formName, rolloutMessagePath)).To(ContainSubstring("(1 recovering)"))
		}, time.Minute, time.Second).Should(Succeed())

		By("Then, for longer than the deadline plus a requeue, the rollout stays Complete")
		// Its rollout-update-start is minutes old, past the 60s deadline. A controller that timed
		// the regression from it would report Failed within one requeue. The completion recorded
		// on the node is what exempts it, and the regression must not clear it. Nor is the node
		// rolling out: the rollout stays Complete, names it as recovering, and keeps its
		// completionTime.
		Consistently(func(g Gomega) {
			g.Expect(lynqNodeField(nodeName(regressed), readyConditionPath)).To(Equal("False"),
				"the regression must last for this window to mean anything")
			expectUpdateCompleted(g, nodeName(regressed))
			g.Expect(formField(formName, rolloutPhasePath)).To(Equal("Complete"))
			g.Expect(formField(formName, rolloutMessagePath)).To(ContainSubstring("(1 recovering)"))
			g.Expect(formField(formName, rolloutCompletionTimePath)).To(Equal(completionTimeBefore))
			expectHubHasNoFailedNodes(g, hubName, len(uids))
		}, stallReportWindow, policyTestInterval).Should(Succeed())
		Expect(formEventCount(formName, "RolloutFailed")).To(Equal(failedEventsBefore))

		By("And once the pod is let become ready, the rollout is Complete again")
		Eventually(func(g Gomega) {
			openDeadlineGate(regressed)
			g.Expect(lynqNodeField(nodeName(regressed), readyConditionPath)).To(Equal("True"))
			g.Expect(formField(formName, rolloutPhasePath)).To(Equal("Complete"))
			g.Expect(formField(formName, rolloutMessagePath)).NotTo(ContainSubstring("recovering"))
		}, 3*time.Minute, policyTestInterval).Should(Succeed())
		Expect(formField(formName, rolloutCompletionTimePath)).To(Equal(completionTimeBefore))
	})
})

const (
	rolloutPhasePath          = "{.status.rollout.phase}"
	rolloutMessagePath        = "{.status.rollout.message}"
	rolloutCompletionTimePath = "{.status.rollout.completionTime}"
	templateGenerationPath    = `{.metadata.annotations.lynq\.sh/template-generation}`
	rolloutCompletedPath      = `{.metadata.annotations.lynq\.sh/rollout-completed-generation}`
)

// expectUpdateCompleted asserts that the hub recorded the node's current update as completed.
func expectUpdateCompleted(g Gomega, node string) {
	gen, err := lynqNodeField(node, templateGenerationPath)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gen).NotTo(BeEmpty())
	completed, err := lynqNodeField(node, rolloutCompletedPath)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(completed).To(Equal(gen))
}

// expectHubHasNoFailedNodes asserts that the hub's status has been written for all desired nodes
// and counts none of them as failed. status.failed is omitempty, so zero renders as "".
func expectHubHasNoFailedNodes(g Gomega, hub string, desired int) {
	gotDesired, err := hubField(hub, "{.status.desired}")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gotDesired).To(Equal(strconv.Itoa(desired)), "hub status must be initialized")
	failed, err := hubField(hub, "{.status.failed}")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(failed).To(BeEmpty(), "status.failed must be 0")
}

// deadlineFormRevision is one version of the Rollout Progress Deadline LynqForm.
type deadlineFormRevision struct {
	release          string   // value of the RELEASE env var, so every revision changes the spec
	readinessCommand []string // nil: the gated probe (ready unless restarted outside Lynq)
	timeoutSeconds   int      // the resource's timeoutSeconds; 0 means 300
}

// applyDeadlineForm creates or updates the LynqForm: maxSkew=1, progressDeadlineSeconds=60 and a
// one-replica Deployment per node.
//
// The Deployment is gated like the Readiness Regression one: a pod marks itself ready unless a
// `kubectl rollout restart` created it, and Recreate takes the old pod down first, so a restart
// holds the Deployment not ready until openDeadlineGate.
func applyDeadlineForm(name, hubName string, rev deadlineFormRevision) {
	if rev.readinessCommand == nil {
		rev.readinessCommand = []string{"test", "-f", "/tmp/ready"}
	}
	if rev.timeoutSeconds == 0 {
		rev.timeoutSeconds = 300
	}
	formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  hubId: %[3]s
  rollout:
    maxSkew: 1
    progressDeadlineSeconds: 60
  deployments:
    - id: gated-deployment
      nameTemplate: "{{ .uid }}-gated-deploy"
      waitForReady: true
      timeoutSeconds: %[4]d
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          strategy:
            type: Recreate
          selector:
            matchLabels:
              app: "{{ .uid }}-gated"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-gated"
            spec:
              terminationGracePeriodSeconds: 0
              containers:
              - name: app
                image: busybox:1.36
                command: ["sh", "-c", "grep -q restartedAt /etc/podinfo/annotations || touch /tmp/ready; exec sleep 3600"]
                env:
                - name: RELEASE
                  value: %[5]q
                readinessProbe:
                  exec:
                    command: %[6]s
                  periodSeconds: 2
                volumeMounts:
                - name: podinfo
                  mountPath: /etc/podinfo
              volumes:
              - name: podinfo
                downwardAPI:
                  items:
                  - path: annotations
                    fieldRef:
                      fieldPath: metadata.annotations
`, name, policyTestNamespace, hubName, rev.timeoutSeconds, rev.release, yamlFlowList(rev.readinessCommand))

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = utils.StringReader(formYAML)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
}

// openDeadlineGate marks the running pods of a node's gated Deployment ready. A pod that is
// starting or terminating may refuse the exec; the caller retries.
func openDeadlineGate(uid string) {
	cmd := exec.Command("kubectl", "get", "pods", "-n", policyTestNamespace, "-l", "app="+uid+"-gated",
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

// formField returns a jsonpath field of a LynqForm.
func formField(name, jsonPath string) string {
	cmd := exec.Command("kubectl", "get", "lynqform", name, "-n", policyTestNamespace, "-o", "jsonpath="+jsonPath)
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(output)
}

// hubField returns a jsonpath field of a LynqHub.
func hubField(name, jsonPath string) (string, error) {
	cmd := exec.Command("kubectl", "get", "lynqhub", name, "-n", policyTestNamespace, "-o", "jsonpath="+jsonPath)
	output, err := utils.Run(cmd)
	return strings.TrimSpace(output), err
}

// formEventCount returns how many events with the given reason a LynqForm has, counting repeats
// aggregated into a single Event.
func formEventCount(name, reason string) int {
	cmd := exec.Command("kubectl", "get", "events", "-n", policyTestNamespace,
		"--field-selector", "involvedObject.kind=LynqForm,involvedObject.name="+name+",reason="+reason,
		"-o", "jsonpath={range .items[*]}count={.count}{\"\\n\"}{end}")
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())

	total := 0
	for _, line := range strings.Fields(output) {
		n, err := strconv.Atoi(strings.TrimPrefix(line, "count="))
		if err != nil || n == 0 {
			n = 1 // an Event without an aggregated count occurred once
		}
		total += n
	}
	return total
}
