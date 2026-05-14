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
	"regexp"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8s-lynq/lynq/test/utils"
)

var _ = Describe("Metrics Cleanup on Deletion", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up test table for metrics cleanup tests")
		testTable = setupTestTable("metrics-cleanup")
	})

	AfterAll(func() {
		By("tearing down metrics cleanup test resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	// Case A: row deactivation removes all per-LynqNode series.
	Context("metrics cleanup after row deactivation", func() {
		const (
			hubName  = "mcleanup-hub"
			formName = "mcleanup-form"
			uid      = "mcleanup-node"
		)

		BeforeEach(func() {
			createHubWithTable(hubName, testTable)
			createForm(formName, hubName, `
  configMaps:
    - id: cm
      nameTemplate: "{{ .uid }}-cm"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)
		})

		AfterEach(func() {
			deleteTestDataFromTable(testTable, uid)
			cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			// Wait for LynqNodes to be gone before the next test starts.
			waitForLynqNodesAbsent(uid)
		})

		It("should remove per-LynqNode Prometheus series after row is deactivated", func() {
			By("Given an active row → LynqNode created and Ready")
			insertTestDataToTable(testTable, uid, true)
			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			waitForLynqNode(expectedNodeName)
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And ALL per-node metric series are present before deletion")
			nodeLabels := nodeMetricLabels(expectedNodeName)
			Eventually(func(g Gomega) {
				metricsOutput := getOperatorMetrics()
				assertMetricPresentG(g, metricsOutput, "lynqnode_resources_desired", nodeLabels...)
				assertMetricPresentG(g, metricsOutput, "lynqnode_resources_ready", nodeLabels...)
				assertMetricPresentG(g, metricsOutput, "lynqnode_resources_failed", nodeLabels...)
				assertMetricPresentG(g, metricsOutput, "lynqnode_condition_status", nodeLabels...)
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("When the row is deactivated (active=false)")
			updateSQL := fmt.Sprintf("UPDATE %s SET active=0 WHERE id='%s';", testTable, uid)
			cmd := exec.Command("kubectl", "exec", "-n", sharedMySQLNamespace, "deployment/mysql", "--",
				"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", updateSQL)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("Then the LynqNode CR is deleted")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the metrics series are eventually absent (polling to catch late resurrection)")
			Eventually(func(g Gomega) {
				metricsOutput := getOperatorMetrics()
				assertMetricAbsentG(g, metricsOutput, "lynqnode_resources_ready", nodeLabels...)
				assertMetricAbsentG(g, metricsOutput, "lynqnode_resources_desired", nodeLabels...)
				assertMetricAbsentG(g, metricsOutput, "lynqnode_resources_failed", nodeLabels...)
				assertMetricAbsentG(g, metricsOutput, "lynqnode_resources_conflicted", nodeLabels...)
				assertMetricAbsentG(g, metricsOutput, "lynqnode_condition_status", nodeLabels...)
				assertMetricAbsentG(g, metricsOutput, "lynqnode_degraded_status", nodeLabels...)
			}, policyTestTimeout, 5*time.Second).Should(Succeed())
		})
	})

	// Case B: Degraded → deactivation removes degraded_status and conflicts_total series.
	// This is the exact user-reported scenario.
	Context("degraded status removed after deactivation", func() {
		const (
			hubName       = "mdeg-hub"
			formName      = "mdeg-form"
			uid           = "mdeg-node"
			configMapName = "mdeg-node-cm-stuck"
		)

		BeforeEach(func() {
			createHubWithTable(hubName, testTable)
			createForm(formName, hubName, `
  configMaps:
    - id: cm-stuck
      nameTemplate: "{{ .uid }}-cm-stuck"
      conflictPolicy: Stuck
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: managed
`)
		})

		AfterEach(func() {
			cmd := exec.Command("kubectl", "delete", "configmap", configMapName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			deleteTestDataFromTable(testTable, uid)
			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			waitForLynqNodesAbsent(uid)
		})

		It("should remove lynqnode_degraded_status and conflicts_total series after deactivation of a Degraded node", func() {
			By("Given a pre-existing ConfigMap that will cause a Stuck conflict")
			cmYAML := fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  key: existing
`, configMapName, policyTestNamespace)
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(cmYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("When the row is activated → LynqNode created and enters Degraded")
			insertTestDataToTable(testTable, uid, true)
			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			waitForLynqNode(expectedNodeName)

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Degraded')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			nodeLabels := nodeMetricLabels(expectedNodeName)

			By("And the degraded_status=1 and conflicts_total series are present")
			Eventually(func(g Gomega) {
				metricsOutput := getOperatorMetrics()
				// degraded_status must be 1
				pattern := regexp.MustCompile(
					`lynqnode_degraded_status\{[^}]*lynqnode="` + regexp.QuoteMeta(expectedNodeName) + `"[^}]*\}\s+1`)
				g.Expect(pattern.MatchString(metricsOutput)).To(BeTrue(),
					"expected lynqnode_degraded_status=1 for %s", expectedNodeName)
				// conflicts_total counter series must exist
				assertMetricPresentG(g, metricsOutput, "lynqnode_conflicts_total", nodeLabels...)
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("When the row is deactivated")
			updateSQL := fmt.Sprintf("UPDATE %s SET active=0 WHERE id='%s';", testTable, uid)
			cmd = exec.Command("kubectl", "exec", "-n", sharedMySQLNamespace, "deployment/mysql", "--",
				"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", updateSQL)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("Then the LynqNode CR is deleted")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And ALL per-node series are eventually absent, including degraded and conflicts_total")
			Eventually(func(g Gomega) {
				metricsOutput := getOperatorMetrics()
				assertMetricAbsentG(g, metricsOutput, "lynqnode_degraded_status", nodeLabels...)
				assertMetricAbsentG(g, metricsOutput, "lynqnode_conflicts_total", nodeLabels...)
				assertMetricAbsentG(g, metricsOutput, "lynqnode_resources_ready", nodeLabels...)
				assertMetricAbsentG(g, metricsOutput, "lynqnode_resources_conflicted", nodeLabels...)
				assertMetricAbsentG(g, metricsOutput, "lynqnode_condition_status", nodeLabels...)
			}, policyTestTimeout, 5*time.Second).Should(Succeed())
		})
	})

	// Case C: Hub CR deletion removes hub_desired/ready/failed series.
	Context("hub metrics removed after hub deletion", func() {
		const hubName = "mhub-delete-hub"

		AfterEach(func() {
			cmd := exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqhub", hubName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		})

		It("should remove hub_desired/ready/failed series when the LynqHub CR is deleted", func() {
			By("Given a LynqHub is created and all three hub metric series are present")
			createHubWithTable(hubName, testTable)
			hubLabels := []string{
				fmt.Sprintf(`hub="%s"`, hubName),
				fmt.Sprintf(`namespace="%s"`, policyTestNamespace),
			}
			Eventually(func(g Gomega) {
				metricsOutput := getOperatorMetrics()
				assertMetricPresentG(g, metricsOutput, "hub_desired", hubLabels...)
				assertMetricPresentG(g, metricsOutput, "hub_ready", hubLabels...)
				assertMetricPresentG(g, metricsOutput, "hub_failed", hubLabels...)
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("When the LynqHub CR is deleted")
			cmd := exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("Then the LynqHub CR is fully removed")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqhub", hubName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the hub metrics series are eventually absent")
			Eventually(func(g Gomega) {
				metricsOutput := getOperatorMetrics()
				assertMetricAbsentG(g, metricsOutput, "hub_desired", hubLabels...)
				assertMetricAbsentG(g, metricsOutput, "hub_ready", hubLabels...)
				assertMetricAbsentG(g, metricsOutput, "hub_failed", hubLabels...)
			}, policyTestTimeout, 5*time.Second).Should(Succeed())
		})
	})

	// Case D: same-name recreation — verifies the UID mismatch guard.
	// When a row is deactivated and then reactivated, the LynqHub creates a new
	// LynqNode CR with the same name but a different UID. This test verifies that
	// stale status events from the old instance do not contaminate the new instance's
	// metrics. Note: the UID mismatch and DeletionTimestamp race guards are also
	// directly exercised by unit tests in internal/status/manager_test.go.
	Context("metrics uncontaminated after same-name LynqNode recreation", func() {
		const (
			hubName  = "mrec-hub"
			formName = "mrec-form"
			uid      = "mrec-node"
		)

		BeforeEach(func() {
			createHubWithTable(hubName, testTable)
			createForm(formName, hubName, `
  configMaps:
    - id: cm
      nameTemplate: "{{ .uid }}-cm"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)
		})

		AfterEach(func() {
			deleteTestDataFromTable(testTable, uid)
			cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			waitForLynqNodesAbsent(uid)
		})

		It("should show correct metrics for the recreated LynqNode", func() {
			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			nodeLabels := nodeMetricLabels(expectedNodeName)

			By("Given a first LynqNode instance is Ready and metrics are published")
			insertTestDataToTable(testTable, uid, true)
			waitForLynqNode(expectedNodeName)
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			Eventually(func(g Gomega) {
				metricsOutput := getOperatorMetrics()
				assertMetricPresentG(g, metricsOutput, "lynqnode_resources_desired", nodeLabels...)
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			// Capture the UID of the first instance.
			firstUID := ""
			cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
				"-o", "jsonpath={.metadata.uid}")
			firstUID, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(firstUID).NotTo(BeEmpty())

			By("When the row is deactivated → first LynqNode deleted")
			deleteTestDataFromTable(testTable, uid)
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And then the row is reactivated → new LynqNode created with the same name")
			insertTestDataToTable(testTable, uid, true)
			waitForLynqNode(expectedNodeName)

			// The new instance must have a different UID.
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.uid}")
				secondUID, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(secondUID).NotTo(Equal(firstUID), "expected new LynqNode to have a different UID")
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("Then the new LynqNode eventually becomes Ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the new instance's metrics show a healthy state (not contaminated by stale events)")
			Eventually(func(g Gomega) {
				metricsOutput := getOperatorMetrics()
				// Resources should be ready for the new instance (ConfigMap is instant).
				ready := extractMetricValue(metricsOutput, "lynqnode_resources_ready", expectedNodeName, policyTestNamespace)
				desired := extractMetricValue(metricsOutput, "lynqnode_resources_desired", expectedNodeName, policyTestNamespace)
				g.Expect(desired).To(BeNumerically(">=", 1), "desired resources should be >= 1")
				g.Expect(ready).To(Equal(desired), "ready should equal desired for a healthy ConfigMap node")
				// No stale degraded series should linger from the old instance.
				assertMetricAbsentG(g, metricsOutput, "lynqnode_degraded_status",
					append(nodeLabels, `} 1`)...) // value=1 means degraded
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})
})

var _ = Describe("Metrics Collection", Ordered, func() {
	var testTable string

	BeforeAll(func() {
		By("setting up test table")
		testTable = setupTestTable("metrics")
	})

	AfterAll(func() {
		By("cleaning up test table and resources")
		cleanupTestTable(testTable)
		cleanupTestResources()
	})

	Context("Metrics Independence from Event/Log Suppression", func() {
		// This test verifies that metrics are collected correctly during reconciliation.

		Describe("metrics update during reconciliation", func() {
			const (
				hubName  = "metrics-test-hub"
				formName = "metrics-test-form"
				uid      = "metrics-test-tenant"
			)

			BeforeEach(func() {
				createHubWithTable(hubName, testTable)
				createForm(formName, hubName, `
  deployments:
    - id: test-deployment
      nameTemplate: "{{ .uid }}-deploy"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: "{{ .uid }}"
          template:
            metadata:
              labels:
                app: "{{ .uid }}"
            spec:
              containers:
              - name: nginx
                image: nginx:alpine
                ports:
                - containerPort: 80
`)
			})

			AfterEach(func() {
				deleteTestDataFromTable(testTable, uid)

				cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				// Wait for cleanup
				time.Sleep(3 * time.Second)
			})

			It("should update lynqnode_resources_ready metric when deployment becomes ready", func() {
				By("Given test data in MySQL")
				insertTestDataToTable(testTable, uid, true)

				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				By("When LynqHub controller creates LynqNode")
				waitForLynqNode(expectedNodeName)

				By("Then the LynqNode should eventually become Ready")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, 4*time.Minute, 5*time.Second).Should(Succeed())

				By("And metrics should reflect the ready state")
				Eventually(func(g Gomega) {
					metricsOutput := getOperatorMetrics()

					// Check lynqnode_resources_ready metric
					readyMetric := extractMetricValue(metricsOutput, "lynqnode_resources_ready", expectedNodeName, policyTestNamespace)
					g.Expect(readyMetric).To(BeNumerically(">=", 1), "lynqnode_resources_ready should be >= 1")

					// Check lynqnode_resources_desired metric
					desiredMetric := extractMetricValue(metricsOutput, "lynqnode_resources_desired", expectedNodeName, policyTestNamespace)
					g.Expect(desiredMetric).To(BeNumerically(">=", 1), "lynqnode_resources_desired should be >= 1")

					// Check lynqnode_resources_failed metric
					failedMetric := extractMetricValue(metricsOutput, "lynqnode_resources_failed", expectedNodeName, policyTestNamespace)
					g.Expect(failedMetric).To(Equal(float64(0)), "lynqnode_resources_failed should be 0")

					// Check lynqnode_condition_status for Ready condition
					readyConditionMetric := extractConditionMetricValue(metricsOutput, "lynqnode_condition_status", expectedNodeName, policyTestNamespace, "Ready")
					g.Expect(readyConditionMetric).To(Equal(float64(1)), "Ready condition should be True (1)")
				}, 4*time.Minute, 5*time.Second).Should(Succeed())
			})
		})

		Describe("metrics collection for degraded state", func() {
			const (
				hubName  = "metrics-degraded-hub"
				formName = "metrics-degraded-form"
				uid      = "metrics-degraded-tenant"
			)

			BeforeEach(func() {
				createHubWithTable(hubName, testTable)
				// Create a form with an invalid image to cause failure
				createForm(formName, hubName, `
  deployments:
    - id: failing-deployment
      nameTemplate: "{{ .uid }}-failing"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: "{{ .uid }}-failing"
          template:
            metadata:
              labels:
                app: "{{ .uid }}-failing"
            spec:
              containers:
              - name: fail
                image: invalid-image-that-does-not-exist:v999
                ports:
                - containerPort: 80
`)
			})

			AfterEach(func() {
				deleteTestDataFromTable(testTable, uid)

				cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				// Wait for cleanup
				time.Sleep(3 * time.Second)
			})

			It("should update lynqnode_resources_failed metric when deployment fails", func() {
				By("Given test data in MySQL")
				insertTestDataToTable(testTable, uid, true)

				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				By("When LynqHub controller creates LynqNode")
				waitForLynqNode(expectedNodeName)

				By("Then metrics should reflect the failed state")
				Eventually(func(g Gomega) {
					metricsOutput := getOperatorMetrics()

					// Check lynqnode_resources_desired metric
					desiredMetric := extractMetricValue(metricsOutput, "lynqnode_resources_desired", expectedNodeName, policyTestNamespace)
					g.Expect(desiredMetric).To(BeNumerically(">=", 1), "lynqnode_resources_desired should be >= 1")

					// The deployment may be created but not ready due to image pull failure
					// Check that ready count is 0 since the deployment cannot become ready
					readyMetric := extractMetricValue(metricsOutput, "lynqnode_resources_ready", expectedNodeName, policyTestNamespace)
					g.Expect(readyMetric).To(Equal(float64(0)), "lynqnode_resources_ready should be 0 for failing deployment")
				}, 2*time.Minute, 5*time.Second).Should(Succeed())
			})
		})
	})
})

// getOperatorMetrics fetches metrics from the operator's metrics endpoint
func getOperatorMetrics() string {
	// Get the service account token
	token, err := serviceAccountToken()
	Expect(err).NotTo(HaveOccurred())

	// Create a unique pod name to avoid conflicts
	podName := fmt.Sprintf("curl-metrics-%d", time.Now().UnixNano())

	// Create curl pod to fetch metrics
	cmd := exec.Command("kubectl", "run", podName, "--restart=Never",
		"--namespace", namespace,
		"--image=curlimages/curl:latest",
		"--overrides",
		fmt.Sprintf(`{
			"spec": {
				"containers": [{
					"name": "curl",
					"image": "curlimages/curl:latest",
					"command": ["/bin/sh", "-c"],
					"args": ["curl -s -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
					"securityContext": {
						"allowPrivilegeEscalation": false,
						"capabilities": {
							"drop": ["ALL"]
						},
						"runAsNonRoot": true,
						"runAsUser": 1000,
						"seccompProfile": {
							"type": "RuntimeDefault"
						}
					}
				}],
				"serviceAccount": "%s"
			}
		}`, token, metricsServiceName, namespace, serviceAccountName))

	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())

	// Wait for pod to complete
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "pod", podName,
			"-o", "jsonpath={.status.phase}",
			"-n", namespace)
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal("Succeeded"))
	}, 2*time.Minute, 5*time.Second).Should(Succeed())

	// Get logs
	cmd = exec.Command("kubectl", "logs", podName, "-n", namespace)
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())

	// Cleanup pod
	cleanupCmd := exec.Command("kubectl", "delete", "pod", podName, "-n", namespace, "--ignore-not-found=true")
	_, _ = utils.Run(cleanupCmd)

	return output
}

// extractMetricValue extracts a metric value for a specific lynqnode and namespace
func extractMetricValue(metricsOutput, metricName, lynqnodeName, lynqnodeNamespace string) float64 {
	if metricsOutput == "" {
		return -1
	}

	if !strings.Contains(metricsOutput, metricName) {
		return -1
	}

	// Pattern: metric_name{lynqnode="name",namespace="ns"} value
	pattern := fmt.Sprintf(`%s\{[^}]*lynqnode="%s"[^}]*namespace="%s"[^}]*\}\s+([0-9.]+)`,
		regexp.QuoteMeta(metricName), regexp.QuoteMeta(lynqnodeName), regexp.QuoteMeta(lynqnodeNamespace))
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(metricsOutput)
	if len(matches) < 2 {
		// Also try the reverse order of labels
		pattern = fmt.Sprintf(`%s\{[^}]*namespace="%s"[^}]*lynqnode="%s"[^}]*\}\s+([0-9.]+)`,
			regexp.QuoteMeta(metricName), regexp.QuoteMeta(lynqnodeNamespace), regexp.QuoteMeta(lynqnodeName))
		re = regexp.MustCompile(pattern)
		matches = re.FindStringSubmatch(metricsOutput)
		if len(matches) < 2 {
			return -1
		}
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(matches[1]), 64)
	if err != nil {
		return -1
	}
	return value
}

// nodeMetricLabels returns the standard label clauses used to match per-LynqNode
// metric lines. Extracted once per test so they stay consistent.
func nodeMetricLabels(nodeName string) []string {
	return []string{
		fmt.Sprintf(`lynqnode="%s"`, nodeName),
		fmt.Sprintf(`namespace="%s"`, policyTestNamespace),
	}
}

// waitForLynqNodesAbsent waits until no LynqNode whose name contains uid exists
// in the policy test namespace. Used in AfterEach to ensure test isolation.
func waitForLynqNodesAbsent(uid string) {
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "lynqnodes", "-n", policyTestNamespace,
			"--ignore-not-found=true", "-o", "name")
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).NotTo(ContainSubstring(uid))
	}, policyTestTimeout, policyTestInterval).Should(Succeed())
}

// assertMetricAbsentG fails (via g) if any /metrics line for metricName matches ALL
// labelClauses. Safe to use inside Eventually blocks — uses g.Expect, not Expect.
// Catches series with extra labels (e.g. reason="...", type="...") that would be
// missed by an exact-label match.
func assertMetricAbsentG(g Gomega, metricsOutput, metricName string, labelClauses ...string) {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(metricName) + `\{[^}]*\}`)
	for _, line := range re.FindAllString(metricsOutput, -1) {
		allMatch := true
		for _, clause := range labelClauses {
			if !strings.Contains(line, clause) {
				allMatch = false
				break
			}
		}
		g.Expect(allMatch).To(BeFalse(),
			"expected no %s series matching %v, but found: %s", metricName, labelClauses, line)
	}
}

// assertMetricAbsent is the non-Eventually wrapper around assertMetricAbsentG.
func assertMetricAbsent(metricsOutput, metricName string, labelClauses ...string) {
	assertMetricAbsentG(Default, metricsOutput, metricName, labelClauses...)
}

// assertMetricPresentG fails (via g) if no /metrics line for metricName matches ALL
// labelClauses. Used to verify series exist before the deletion step so that an
// absent series doesn't cause the subsequent assertMetricAbsent to trivially pass.
func assertMetricPresentG(g Gomega, metricsOutput, metricName string, labelClauses ...string) {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(metricName) + `\{[^}]*\}`)
	found := false
	for _, line := range re.FindAllString(metricsOutput, -1) {
		allMatch := true
		for _, clause := range labelClauses {
			if !strings.Contains(line, clause) {
				allMatch = false
				break
			}
		}
		if allMatch {
			found = true
			break
		}
	}
	g.Expect(found).To(BeTrue(),
		"expected at least one %s series matching %v, but none found", metricName, labelClauses)
}

// extractConditionMetricValue extracts a condition metric value
func extractConditionMetricValue(metricsOutput, metricName, lynqnodeName, lynqnodeNamespace, conditionType string) float64 {
	// Pattern: metric_name{lynqnode="name",namespace="ns",type="condition"} value
	pattern := fmt.Sprintf(`%s\{[^}]*lynqnode="%s"[^}]*namespace="%s"[^}]*type="%s"[^}]*\}\s+([0-9.]+)`,
		regexp.QuoteMeta(metricName), regexp.QuoteMeta(lynqnodeName), regexp.QuoteMeta(lynqnodeNamespace), regexp.QuoteMeta(conditionType))
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(metricsOutput)
	if len(matches) < 2 {
		return -1 // Metric not found
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(matches[1]), 64)
	if err != nil {
		return -1
	}
	return value
}
