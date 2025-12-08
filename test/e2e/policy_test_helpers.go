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

	. "github.com/onsi/gomega"

	"github.com/k8s-lynq/lynq/test/utils"
)

const (
	policyTestNamespace = "policy-test"
	policyTestTimeout   = 3 * time.Minute
	policyTestInterval  = 2 * time.Second
)

// setupPolicyTestNamespace creates the test namespace and deploys MySQL for testing
func setupPolicyTestNamespace() {
	// Wait for any existing namespace to be fully deleted
	Eventually(func() error {
		cmd := exec.Command("kubectl", "get", "namespace", policyTestNamespace)
		_, err := utils.Run(cmd)
		return err // Should return error when namespace doesn't exist
	}, 2*time.Minute, 2*time.Second).Should(HaveOccurred(), "Namespace should not exist before creation")

	cmd := exec.Command("kubectl", "create", "ns", policyTestNamespace)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")

	mysqlYAML := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: mysql-root-password
  namespace: %s
type: Opaque
stringData:
  password: test-password
---
apiVersion: v1
kind: Service
metadata:
  name: mysql
  namespace: %s
spec:
  ports:
  - port: 3306
  selector:
    app: mysql
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mysql
  namespace: %s
spec:
  selector:
    matchLabels:
      app: mysql
  template:
    metadata:
      labels:
        app: mysql
    spec:
      containers:
      - name: mysql
        image: mysql:8.0
        env:
        - name: MYSQL_ROOT_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mysql-root-password
              key: password
        - name: MYSQL_DATABASE
          value: testdb
        # Optimize MySQL for faster startup in CI
        - name: MYSQL_INITDB_SKIP_TZINFO
          value: "1"
        args:
        - --default-authentication-plugin=mysql_native_password
        - --skip-mysqlx
        - --skip-log-bin
        - --skip-name-resolve
        - --innodb-buffer-pool-size=64M
        - --innodb-log-file-size=16M
        - --innodb-flush-method=O_DIRECT_NO_FSYNC
        - --max-connections=50
        - --performance-schema=OFF
        ports:
        - containerPort: 3306
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        readinessProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - mysqladmin ping -h 127.0.0.1 --silent
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 10
          failureThreshold: 30
`, policyTestNamespace, policyTestNamespace, policyTestNamespace)

	cmd = exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = utils.StringReader(mysqlYAML)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy MySQL")

	// Wait for MySQL pods to be scheduled and running first
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "pods", "-n", policyTestNamespace,
			"-l", "app=mysql", "-o", "jsonpath={.items[0].status.phase}")
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal("Running"))
	}, 3*time.Minute, 5*time.Second).Should(Succeed(), "MySQL pod should be running")

	// Wait for MySQL deployment to become Available (increased timeout for CI environments)
	// Print debug info after 4 minutes if still failing
	deploymentStartTime := time.Now()
	debugPrinted := false
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "wait", "deployment", "mysql",
			"-n", policyTestNamespace,
			"--for", "condition=Available",
			"--timeout", "2m")
		_, err := utils.Run(cmd)

		// Print debug info once after 4 minutes of failures
		if err != nil && !debugPrinted && time.Since(deploymentStartTime) > 4*time.Minute {
			debugPrinted = true
			fmt.Println("\n" + strings.Repeat("=", 80))
			fmt.Println("MySQL taking too long to start - Debug Information")
			fmt.Println(strings.Repeat("=", 80))
			printMySQLDebugInfo()
			fmt.Println(strings.Repeat("=", 80))
		}

		g.Expect(err).NotTo(HaveOccurred())
	}, 8*time.Minute, 10*time.Second).Should(Succeed(), "MySQL deployment should be available")

	// Wait for MySQL to be truly ready to accept connections
	schemaSQL := `
CREATE TABLE IF NOT EXISTS nodes (
  id VARCHAR(255) PRIMARY KEY,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  replicas VARCHAR(255),
  app_port VARCHAR(255)
);
`
	schemaStartTime := time.Now()
	schemaDebugPrinted := false
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
			"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", schemaSQL)
		_, err := utils.Run(cmd)

		// Print debug info once after 90 seconds of failures
		if err != nil && !schemaDebugPrinted && time.Since(schemaStartTime) > 90*time.Second {
			schemaDebugPrinted = true
			fmt.Println("\n" + strings.Repeat("=", 80))
			fmt.Println("MySQL schema creation taking too long - Debug Information")
			fmt.Println(strings.Repeat("=", 80))
			printMySQLSchemaDebugInfo()
			fmt.Println(strings.Repeat("=", 80))
		}

		g.Expect(err).NotTo(HaveOccurred())
	}, 2*time.Minute, 5*time.Second).Should(Succeed(), "Failed to create database schema")
}

// printMySQLDebugInfo prints debugging information when MySQL deployment fails
func printMySQLDebugInfo() {
	fmt.Println("\n=== MySQL Deployment Failed - Debug Information ===")

	// Get pod status
	cmd := exec.Command("kubectl", "get", "pods", "-n", policyTestNamespace, "-l", "app=mysql", "-o", "wide")
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nPod Status:\n%s\n", output)
	}

	// Get pod events
	cmd = exec.Command("kubectl", "get", "events", "-n", policyTestNamespace, "--sort-by=.lastTimestamp")
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nNamespace Events:\n%s\n", output)
	}

	// Get pod logs
	cmd = exec.Command("kubectl", "logs", "-n", policyTestNamespace, "-l", "app=mysql", "--tail=100")
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nMySQL Logs (last 100 lines):\n%s\n", output)
	}

	// Get pod description
	cmd = exec.Command("kubectl", "describe", "pod", "-n", policyTestNamespace, "-l", "app=mysql")
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nPod Description:\n%s\n", output)
	}

	// Get deployment status
	cmd = exec.Command("kubectl", "describe", "deployment", "mysql", "-n", policyTestNamespace)
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nDeployment Description:\n%s\n", output)
	}

	fmt.Println("=== End of Debug Information ===")
}

// printMySQLSchemaDebugInfo prints debugging information when MySQL schema creation fails
func printMySQLSchemaDebugInfo() {
	fmt.Println("\n=== MySQL Schema Creation Failed - Debug Information ===")

	// Get MySQL version info
	cmd := exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
		"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "-e", "SHOW VARIABLES LIKE '%version%';")
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nMySQL Version Info:\n%s\n", output)
	}

	// Check MySQL process status
	cmd = exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
		"mysqladmin", "-h", "127.0.0.1", "-uroot", "-ptest-password", "status")
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nMySQL Status:\n%s\n", output)
	}

	// Get recent MySQL logs
	cmd = exec.Command("kubectl", "logs", "-n", policyTestNamespace, "-l", "app=mysql", "--tail=50")
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nRecent MySQL Logs:\n%s\n", output)
	}

	fmt.Println("=== End of Debug Information ===")
}

// cleanupPolicyTestNamespace cleans up the test namespace and all resources
func cleanupPolicyTestNamespace() {
	// Delete all LynqNodes first
	cmd := exec.Command("kubectl", "delete", "lynqnodes", "--all", "-n", policyTestNamespace, "--ignore-not-found=true", "--wait=false")
	_, _ = utils.Run(cmd)

	// Delete all LynqForms
	cmd = exec.Command("kubectl", "delete", "lynqforms", "--all", "-n", policyTestNamespace, "--ignore-not-found=true", "--wait=false")
	_, _ = utils.Run(cmd)

	// Delete all LynqHubs
	cmd = exec.Command("kubectl", "delete", "lynqhubs", "--all", "-n", policyTestNamespace, "--ignore-not-found=true", "--wait=false")
	_, _ = utils.Run(cmd)

	// Delete namespace
	cmd = exec.Command("kubectl", "delete", "ns", policyTestNamespace, "--wait=false", "--ignore-not-found=true")
	_, _ = utils.Run(cmd)

	// Force cleanup if namespace is stuck
	_ = utils.CleanupNamespace(policyTestNamespace)
}

// insertTestData inserts a test data row into MySQL using the adapter
func insertTestData(uid string, active bool) {
	adapter := GetTestDatasource()
	err := InsertTestNode(adapter, policyTestNamespace, uid, active)
	Expect(err).NotTo(HaveOccurred())
}

// deleteTestData deletes a test data row from MySQL using the adapter
func deleteTestData(uid string) {
	adapter := GetTestDatasource()
	_ = DeleteTestNode(adapter, policyTestNamespace, uid)
}

// createHub creates a LynqHub pointing to the configured datasource
func createHub(name string) {
	err := ApplyHub(GetTestDatasource(), name, policyTestNamespace, "5s")
	Expect(err).NotTo(HaveOccurred())
}

// createForm creates a LynqForm with the given resources
func createForm(name, hubName string, resources string) {
	formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  %s
`, name, policyTestNamespace, hubName, resources)
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = utils.StringReader(formYAML)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
}

// waitForLynqNode waits for a LynqNode to be created by the LynqHub controller
func waitForLynqNode(nodeName string) {
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "lynqnode", nodeName, "-n", policyTestNamespace)
		_, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
	}, policyTestTimeout, policyTestInterval).Should(Succeed())
}
