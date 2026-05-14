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

// metricsFlushWait is how long we wait after LynqNode deletion for the
// StatusManager's async flush cycle to complete and the resurrection race
// to be detectable (flushInterval=1s * 3 safety margin).
const metricsFlushWait = 5 * time.Second

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
			time.Sleep(5 * time.Second)
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

			By("And the metrics series are present")
			Eventually(func(g Gomega) {
				metricsOutput := getOperatorMetrics()
				v := extractMetricValue(metricsOutput, "lynqnode_resources_desired", expectedNodeName, policyTestNamespace)
				g.Expect(v).To(BeNumerically(">=", 1))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("When the row is deactivated (activate=false)")
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

			By("And after waiting for the async flush cycle, the metrics series are gone")
			time.Sleep(metricsFlushWait)
			metricsOutput := getOperatorMetrics()
			labelClauses := []string{
				fmt.Sprintf(`lynqnode="%s"`, expectedNodeName),
				fmt.Sprintf(`namespace="%s"`, policyTestNamespace),
			}
			assertMetricAbsent(metricsOutput, "lynqnode_resources_ready", labelClauses...)
			assertMetricAbsent(metricsOutput, "lynqnode_resources_desired", labelClauses...)
			assertMetricAbsent(metricsOutput, "lynqnode_resources_failed", labelClauses...)
			assertMetricAbsent(metricsOutput, "lynqnode_resources_conflicted", labelClauses...)
			assertMetricAbsent(metricsOutput, "lynqnode_condition_status", labelClauses...)
			assertMetricAbsent(metricsOutput, "lynqnode_degraded_status", labelClauses...)
		})
	})

	Context("degraded status removed after deactivation", func() {
		// This is the exact user-reported scenario:
		// LynqNode enters Degraded state (due to Stuck conflict), then the row is
		// deactivated. The lynqnode_degraded_status series must disappear, not linger.
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
			time.Sleep(5 * time.Second)
		})

		It("should remove lynqnode_degraded_status series after deactivation of a Degraded node", func() {
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

			By("And the lynqnode_degraded_status series is 1")
			Eventually(func(g Gomega) {
				metricsOutput := getOperatorMetrics()
				pattern := regexp.MustCompile(
					`lynqnode_degraded_status\{[^}]*lynqnode="` + regexp.QuoteMeta(expectedNodeName) + `"[^}]*\}\s+1`)
				g.Expect(pattern.MatchString(metricsOutput)).To(BeTrue(),
					"expected lynqnode_degraded_status=1 for %s", expectedNodeName)
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

			By("And after the flush cycle, ALL lynqnode_degraded_status series for this node are gone")
			time.Sleep(metricsFlushWait)
			metricsOutput := getOperatorMetrics()
			assertMetricAbsent(metricsOutput, "lynqnode_degraded_status",
				fmt.Sprintf(`lynqnode="%s"`, expectedNodeName),
				fmt.Sprintf(`namespace="%s"`, policyTestNamespace),
			)
			// Other per-node series must also be gone
			labelClauses := []string{
				fmt.Sprintf(`lynqnode="%s"`, expectedNodeName),
				fmt.Sprintf(`namespace="%s"`, policyTestNamespace),
			}
			assertMetricAbsent(metricsOutput, "lynqnode_resources_ready", labelClauses...)
			assertMetricAbsent(metricsOutput, "lynqnode_condition_status", labelClauses...)
		})
	})

	Context("hub metrics removed after hub deletion", func() {
		const hubName = "mhub-delete-hub"

		AfterEach(func() {
			cmd := exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			time.Sleep(5 * time.Second)
		})

		It("should remove hub_desired/ready/failed series when the LynqHub CR is deleted", func() {
			By("Given a LynqHub is created and its metrics are published")
			createHubWithTable(hubName, testTable)
			Eventually(func(g Gomega) {
				metricsOutput := getOperatorMetrics()
				// hub_desired may be 0 but the series should exist
				pattern := regexp.MustCompile(
					`hub_desired\{[^}]*hub="` + regexp.QuoteMeta(hubName) + `"[^}]*\}`)
				g.Expect(pattern.MatchString(metricsOutput)).To(BeTrue(),
					"expected hub_desired series for hub %s to exist", hubName)
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

			By("And the hub metrics series are gone")
			time.Sleep(metricsFlushWait)
			metricsOutput := getOperatorMetrics()
			hubLabels := []string{
				fmt.Sprintf(`hub="%s"`, hubName),
				fmt.Sprintf(`namespace="%s"`, policyTestNamespace),
			}
			assertMetricAbsent(metricsOutput, "hub_desired", hubLabels...)
			assertMetricAbsent(metricsOutput, "hub_ready", hubLabels...)
			assertMetricAbsent(metricsOutput, "hub_failed", hubLabels...)
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

// assertMetricAbsent fails if any /metrics line for metricName matches ALL labelClauses.
// This catches series with extra labels (e.g. reason="..." or type="...") that would be
// missed by an exact-label match.
func assertMetricAbsent(metricsOutput, metricName string, labelClauses ...string) {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(metricName) + `\{[^}]*\}`)
	for _, line := range re.FindAllString(metricsOutput, -1) {
		allMatch := true
		for _, clause := range labelClauses {
			if !strings.Contains(line, clause) {
				allMatch = false
				break
			}
		}
		Expect(allMatch).To(BeFalse(),
			"expected no %s series matching %v, but found: %s", metricName, labelClauses, line)
	}
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
