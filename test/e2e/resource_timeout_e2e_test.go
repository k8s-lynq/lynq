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

		// Every It gets its own names and pod label, so a previous It's objects that are still
		// terminating can neither collide with this one's nor match its pod selector.
		var (
			run                                     int
			hubName, formName, uid, nodeName, label string
			deploymentName, appLabel                string
		)

		BeforeEach(func() {
			run++
			hubName = fmt.Sprintf("disruption-hub-%d", run)
			formName = fmt.Sprintf("disruption-form-%d", run)
			uid = fmt.Sprintf("disruption-uid-%d", run)
			nodeName = fmt.Sprintf("%s-%s", uid, formName)
			deploymentName = uid + "-steady-deploy"
			label = fmt.Sprintf("disruption-test-%d", run)
			appLabel = "app=" + label

			By("creating a LynqHub")
			createHubWithTable(hubName, testTable)

			By("and a LynqForm with a 2-replica Deployment and a 10s readiness timeout")
			createForm(formName, hubName, fmt.Sprintf(`
  deployments:
    - id: steady-deployment
      nameTemplate: "{{ .uid }}-steady-deploy"
      waitForReady: true
      timeoutSeconds: 10
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 2
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
                command: ["sh", "-c", "sleep 3600"]
                readinessProbe:
                  exec:
                    command: ["true"]
                  initialDelaySeconds: %[2]d
                  periodSeconds: 2
`, label, replacementUnreadySeconds))

			By("and active data in MySQL")
			insertTestDataToTable(testTable, uid, true)
			waitForLynqNode(nodeName)

			By("and the LynqNode has become Ready")
			Eventually(func(g Gomega) {
				g.Expect(getLynqNodeField(nodeName, "{.status.conditions[?(@.type=='Ready')].status}")).To(Equal("True"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("and it has stayed Ready for longer than its timeout")
			time.Sleep(15 * time.Second)
			Expect(getLynqNodeField(nodeName, "{.status.conditions[?(@.type=='Ready')].status}")).To(Equal("True"))
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestDataFromTable(testTable, uid)

			cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "deployment", deploymentName, "-n", policyTestNamespace,
				"--ignore-not-found=true", "--wait=false")
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			By("and waiting until every object of this run is gone")
			for _, obj := range [][2]string{
				{"lynqnode", nodeName},
				{"lynqform", formName},
				{"lynqhub", hubName},
				{"deployment", deploymentName},
			} {
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", obj[0], obj[1], "-n", policyTestNamespace)
					output, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred(), "%s/%s still exists", obj[0], obj[1])
					g.Expect(output).To(ContainSubstring("NotFound"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			}
		})

		// expectDisruptionIsNotAFailure verifies the node rides out the disruption: it is seen
		// not Ready, never reports a failed resource, a ReadinessTimeout or Degraded=True, and
		// returns to Ready once the replacement pods are ready.
		expectDisruptionIsNotAFailure := func(timeoutEventsBefore int) {
			By("Then the LynqNode is reported not Ready while the Deployment recovers, but never failed or Degraded")
			sawNotReady := false
			Consistently(func(g Gomega) {
				if getLynqNodeField(nodeName, "{.status.conditions[?(@.type=='Ready')].status}") == "False" {
					sawNotReady = true
				}
				g.Expect(getLynqNodeField(nodeName, "{.status.failedResources}")).To(Or(BeEmpty(), Equal("0")),
					"a disrupted, previously ready Deployment must not be counted as failed")
				g.Expect(getLynqNodeField(nodeName, "{.status.conditions[?(@.type=='Degraded')].status}")).NotTo(Equal("True"),
					"a not-ready-only LynqNode must not be Degraded")
			}, (replacementUnreadySeconds+15)*time.Second, policyTestInterval).Should(Succeed())
			Expect(sawNotReady).To(BeTrue(), "the controller must have observed the disruption for this test to be meaningful")

			By("And no ReadinessTimeout event was emitted for the disruption")
			Expect(readinessTimeoutEventCount(nodeName)).To(Equal(timeoutEventsBefore))

			By("And the LynqNode returns to Ready once the replacement pods are ready")
			Eventually(func(g Gomega) {
				g.Expect(getLynqNodeField(nodeName, "{.status.conditions[?(@.type=='Ready')].status}")).To(Equal("True"))
				g.Expect(getLynqNodeField(nodeName, "{.status.readyResources}")).To(Equal("1"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		}

		It("should not fail the Deployment when one of its pods is lost", func() {
			timeoutEventsBefore := readinessTimeoutEventCount(nodeName)

			By("When one of the Deployment's pods is deleted")
			cmd := exec.Command("kubectl", "get", "pods", "-n", policyTestNamespace, "-l", appLabel,
				"-o", "jsonpath={.items[0].metadata.name}")
			podName, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(podName).NotTo(BeEmpty())
			cmd = exec.Command("kubectl", "delete", "pod", podName, "-n", policyTestNamespace, "--wait=false")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			expectDisruptionIsNotAFailure(timeoutEventsBefore)
		})

		It("should not fail the Deployment when all of its pods are lost", func() {
			timeoutEventsBefore := readinessTimeoutEventCount(nodeName)

			By("When every pod of the Deployment is deleted")
			cmd := exec.Command("kubectl", "delete", "pod", "-n", policyTestNamespace, "-l", appLabel, "--wait=false")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			expectDisruptionIsNotAFailure(timeoutEventsBefore)
		})
	})
})

// getLynqNodeField returns a jsonpath field of a LynqNode in the policy test namespace, or ""
// when it cannot be read.
func getLynqNodeField(nodeName, jsonPath string) string {
	cmd := exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace, "-o", "jsonpath="+jsonPath)
	output, err := utils.Run(cmd)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
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
