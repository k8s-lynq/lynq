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
	sharedMySQLNamespace = "mysql-shared"
	policyTestNamespace  = "policy-test"
	policyTestTimeout    = 5 * time.Minute
	policyTestInterval   = 2 * time.Second
)

// setupSharedMySQL creates mysql-shared namespace and deploys MySQL (idempotent)
func setupSharedMySQL() {
	// Check if MySQL is already running
	cmd := exec.Command("kubectl", "get", "ns", sharedMySQLNamespace)
	if _, err := utils.Run(cmd); err == nil {
		// Namespace exists, verify MySQL is ready
		verifyMySQLReady()
		return
	}

	// Create mysql-shared namespace
	cmd = exec.Command("kubectl", "create", "ns", sharedMySQLNamespace)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create mysql-shared namespace")

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
`, sharedMySQLNamespace, sharedMySQLNamespace, sharedMySQLNamespace)

	cmd = exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = utils.StringReader(mysqlYAML)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy MySQL")

	// Wait for MySQL pods to be scheduled and running first
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "pods", "-n", sharedMySQLNamespace,
			"-l", "app=mysql", "-o", "jsonpath={.items[0].status.phase}")
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal("Running"))
	}, 3*time.Minute, 5*time.Second).Should(Succeed(), "MySQL pod should be running")

	// Wait for MySQL deployment to become Available
	deploymentStartTime := time.Now()
	debugPrinted := false
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "wait", "deployment", "mysql",
			"-n", sharedMySQLNamespace,
			"--for", "condition=Available",
			"--timeout", "2m")
		_, err := utils.Run(cmd)

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

	// Verify MySQL is accepting connections
	verifyMySQLReady()
}

// verifyMySQLReady ensures MySQL is accepting connections
func verifyMySQLReady() {
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "exec", "-n", sharedMySQLNamespace, "deployment/mysql", "--",
			"mysqladmin", "-h", "127.0.0.1", "-uroot", "-ptest-password", "ping")
		_, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
	}, 2*time.Minute, 5*time.Second).Should(Succeed(), "MySQL should be accepting connections")
}

// setupPolicyTestNamespace creates the test namespace (idempotent, no MySQL)
func setupPolicyTestNamespace() {
	// Check if namespace already exists
	cmd := exec.Command("kubectl", "get", "ns", policyTestNamespace)
	if _, err := utils.Run(cmd); err == nil {
		// Namespace exists, just ensure it's active
		return
	}

	// Create namespace
	cmd = exec.Command("kubectl", "create", "ns", policyTestNamespace)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")

	// Copy MySQL secret to policy-test namespace for Hub authentication
	copySecretCmd := exec.Command("kubectl", "get", "secret", "mysql-root-password",
		"-n", sharedMySQLNamespace, "-o", "yaml")
	secretYAML, err := utils.Run(copySecretCmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to get MySQL secret")

	// Replace namespace in the secret YAML
	secretYAML = strings.Replace(secretYAML, fmt.Sprintf("namespace: %s", sharedMySQLNamespace),
		fmt.Sprintf("namespace: %s", policyTestNamespace), 1)
	// Remove resourceVersion to allow creation
	lines := strings.Split(secretYAML, "\n")
	var filteredLines []string
	for _, line := range lines {
		if !strings.Contains(line, "resourceVersion:") &&
			!strings.Contains(line, "uid:") &&
			!strings.Contains(line, "creationTimestamp:") {
			filteredLines = append(filteredLines, line)
		}
	}
	secretYAML = strings.Join(filteredLines, "\n")

	applyCmd := exec.Command("kubectl", "apply", "-f", "-")
	applyCmd.Stdin = utils.StringReader(secretYAML)
	_, err = utils.Run(applyCmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to copy MySQL secret to policy-test namespace")
}

// printMySQLDebugInfo prints debugging information when MySQL deployment fails
func printMySQLDebugInfo() {
	fmt.Println("\n=== MySQL Deployment Failed - Debug Information ===")

	// Get pod status
	cmd := exec.Command("kubectl", "get", "pods", "-n", sharedMySQLNamespace, "-l", "app=mysql", "-o", "wide")
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nPod Status:\n%s\n", output)
	}

	// Get pod events
	cmd = exec.Command("kubectl", "get", "events", "-n", sharedMySQLNamespace, "--sort-by=.lastTimestamp")
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nNamespace Events:\n%s\n", output)
	}

	// Get pod logs
	cmd = exec.Command("kubectl", "logs", "-n", sharedMySQLNamespace, "-l", "app=mysql", "--tail=100")
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nMySQL Logs (last 100 lines):\n%s\n", output)
	}

	// Get pod description
	cmd = exec.Command("kubectl", "describe", "pod", "-n", sharedMySQLNamespace, "-l", "app=mysql")
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nPod Description:\n%s\n", output)
	}

	// Get deployment status
	cmd = exec.Command("kubectl", "describe", "deployment", "mysql", "-n", sharedMySQLNamespace)
	if output, err := utils.Run(cmd); err == nil {
		fmt.Printf("\nDeployment Description:\n%s\n", output)
	}

	fmt.Println("=== End of Debug Information ===")
}

// cleanupTestResources deletes all Lynq resources in policy-test namespace
func cleanupTestResources() {
	// Delete all LynqNodes first
	cmd := exec.Command("kubectl", "delete", "lynqnodes", "--all", "-n", policyTestNamespace, "--ignore-not-found=true", "--wait=false")
	_, _ = utils.Run(cmd)

	// Delete all LynqForms
	cmd = exec.Command("kubectl", "delete", "lynqforms", "--all", "-n", policyTestNamespace, "--ignore-not-found=true", "--wait=false")
	_, _ = utils.Run(cmd)

	// Delete all LynqHubs
	cmd = exec.Command("kubectl", "delete", "lynqhubs", "--all", "-n", policyTestNamespace, "--ignore-not-found=true", "--wait=false")
	_, _ = utils.Run(cmd)

	// Wait briefly for resources to be deleted
	time.Sleep(2 * time.Second)
}

// getUniqueTableName generates unique table name for test isolation
func getUniqueTableName(testPrefix string) string {
	// Replace spaces and special chars with underscores
	safe := strings.ReplaceAll(testPrefix, " ", "_")
	safe = strings.ReplaceAll(safe, "-", "_")
	// Use timestamp for uniqueness
	return fmt.Sprintf("nodes_%s_%d", safe, time.Now().UnixNano()%1000000)
}

// setupTestTable creates a test table and returns its name
func setupTestTable(testPrefix string) string {
	tableName := getUniqueTableName(testPrefix)
	adapter := GetTestDatasource()
	err := adapter.CreateTable(sharedMySQLNamespace, tableName, []ColumnDef{
		{Name: "id", Type: "VARCHAR(255)", PrimaryKey: true},
		{Name: "active", Type: "BOOLEAN", NotNull: true, Default: "TRUE"},
		{Name: "replicas", Type: "VARCHAR(255)"},
		{Name: "app_port", Type: "VARCHAR(255)"},
	})
	Expect(err).NotTo(HaveOccurred(), "Failed to create test table: "+tableName)
	return tableName
}

// cleanupTestTable drops the test table
func cleanupTestTable(tableName string) {
	if tableName == "" {
		return
	}
	adapter := GetTestDatasource()
	_ = adapter.DropTable(sharedMySQLNamespace, tableName)
}

// createHubWithTable creates a LynqHub pointing to MySQL in sharedMySQLNamespace with specific table
func createHubWithTable(name, tableName string) {
	err := ApplyHubWithTable(GetTestDatasource(), name, policyTestNamespace, sharedMySQLNamespace, "5s", tableName)
	Expect(err).NotTo(HaveOccurred())
}

// insertTestDataToTable inserts a test data row into a specific table
func insertTestDataToTable(tableName, uid string, active bool) {
	adapter := GetTestDatasource()
	activeStr := "0"
	if active {
		activeStr = "1"
	}
	err := adapter.InsertRow(sharedMySQLNamespace, tableName, map[string]string{
		"id":     uid,
		"active": activeStr,
	})
	Expect(err).NotTo(HaveOccurred())
}

// deleteTestDataFromTable deletes a test data row from a specific table
func deleteTestDataFromTable(tableName, uid string) {
	adapter := GetTestDatasource()
	_ = adapter.DeleteRow(sharedMySQLNamespace, tableName, "id", uid)
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
