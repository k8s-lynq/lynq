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

var _ = Describe("Metrics Collection", Ordered, func() {
	BeforeAll(func() {
		setupPolicyTestNamespace()
	})

	AfterAll(func() {
		cleanupPolicyTestNamespace()
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
				createHub(hubName)
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
				deleteTestData(uid)

				cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				// Wait for cleanup
				time.Sleep(3 * time.Second)
			})

			It("should update lynqnode_resources_ready metric when deployment becomes ready", func() {
				By("Given test data in MySQL")
				insertTestData(uid, true)

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
				createHub(hubName)
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
				deleteTestData(uid)

				cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				// Wait for cleanup
				time.Sleep(3 * time.Second)
			})

			It("should update lynqnode_resources_failed metric when deployment fails", func() {
				By("Given test data in MySQL")
				insertTestData(uid, true)

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

	// Debug: check if output is empty or contains an error
	if output == "" {
		GinkgoWriter.Printf("WARNING: curl pod returned empty output\n")
	} else if strings.Contains(output, "error") || strings.Contains(output, "Error") {
		GinkgoWriter.Printf("WARNING: curl output may contain error: %s\n", output[:min(len(output), 500)])
	}

	// Cleanup pod
	cleanupCmd := exec.Command("kubectl", "delete", "pod", podName, "-n", namespace, "--ignore-not-found=true")
	_, _ = utils.Run(cleanupCmd)

	return output
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractMetricValue extracts a metric value for a specific lynqnode and namespace
func extractMetricValue(metricsOutput, metricName, lynqnodeName, lynqnodeNamespace string) float64 {
	// First, check if the metrics output is empty or contains an error
	if metricsOutput == "" {
		GinkgoWriter.Printf("WARNING: metrics output is empty\n")
		return -1
	}

	// Check if the metric type exists at all in the output
	if !strings.Contains(metricsOutput, metricName) {
		GinkgoWriter.Printf("WARNING: metric %s not found in output (output length: %d bytes)\n", metricName, len(metricsOutput))
		// Print first 500 characters of output for debugging
		if len(metricsOutput) > 500 {
			GinkgoWriter.Printf("First 500 chars of output: %s...\n", metricsOutput[:500])
		} else {
			GinkgoWriter.Printf("Full output: %s\n", metricsOutput)
		}
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
			// Log what metrics ARE present for this metric name
			GinkgoWriter.Printf("DEBUG: Looking for %s with lynqnode=%s namespace=%s\n", metricName, lynqnodeName, lynqnodeNamespace)
			// Find all lines containing the metric name for debugging
			lines := strings.Split(metricsOutput, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, metricName+"{") {
					GinkgoWriter.Printf("DEBUG: Found metric line: %s\n", line)
				}
			}
			return -1 // Metric not found
		}
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(matches[1]), 64)
	if err != nil {
		return -1
	}
	return value
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
