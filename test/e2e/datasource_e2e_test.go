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

var _ = Describe("Datasource Behavior", Ordered, func() {
	BeforeAll(func() {
		setupPolicyTestNamespace()
	})

	AfterAll(func() {
		cleanupPolicyTestNamespace()
	})

	Context("Database Synchronization", func() {
		Describe("syncInterval polling behavior", func() {
			const (
				hubName  = "sync-interval-hub"
				formName = "sync-interval-form"
				uid1     = "sync-test-tenant-1"
				uid2     = "sync-test-tenant-2"
			)

			BeforeEach(func() {
				// Create hub with 10s sync interval for faster testing
				hubYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: %s
  namespace: %s
spec:
  source:
    type: mysql
    syncInterval: 10s
    mysql:
      host: mysql.%s.svc.cluster.local
      port: 3306
      database: testdb
      table: nodes
      username: root
      passwordRef:
        name: mysql-root-password
        key: password
  valueMappings:
    uid: id
    activate: active
`, hubName, policyTestNamespace, policyTestNamespace)
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(hubYAML)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				createForm(formName, hubName, `
  configMaps:
    - id: test-config
      nameTemplate: "{{ .uid }}-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tenant-id: "{{ .uid }}"
`)
			})

			AfterEach(func() {
				deleteTestData(uid1)
				deleteTestData(uid2)

				cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				// Wait for cleanup
				time.Sleep(3 * time.Second)
			})

			It("should create new LynqNode when a new row is added to the database", func() {
				By("Given test data in MySQL with one active row")
				insertTestData(uid1, true)

				expectedNodeName1 := fmt.Sprintf("%s-%s", uid1, formName)
				By("When LynqHub controller syncs and creates first LynqNode")
				waitForLynqNode(expectedNodeName1)

				By("And a new row is added to the database")
				insertTestData(uid2, true)

				By("Then after sync interval, a new LynqNode should be created automatically")
				expectedNodeName2 := fmt.Sprintf("%s-%s", uid2, formName)
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName2, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 30*time.Second, 2*time.Second).Should(Succeed())

				By("And the Hub status.desired should reflect 2 nodes")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqhub", hubName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.desired}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("2"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("activate value change behavior", func() {
			const (
				hubName  = "activate-change-hub"
				formName = "activate-change-form"
				uid      = "activate-change-tenant"
			)

			BeforeEach(func() {
				createHub(hubName)
				createForm(formName, hubName, `
  configMaps:
    - id: test-config
      nameTemplate: "{{ .uid }}-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tenant-id: "{{ .uid }}"
`)
			})

			AfterEach(func() {
				deleteTestData(uid)

				cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(3 * time.Second)
			})

			It("should delete LynqNode when activate value changes to false", func() {
				By("Given a LynqNode that is Ready with active=true")
				insertTestData(uid, true)

				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				// Wait for LynqNode to be Ready
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When the activate value is changed to false in the database")
				updateSQL := fmt.Sprintf("UPDATE nodes SET active=0 WHERE id='%s';", uid)
				cmd := exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
					"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", updateSQL)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("Then the LynqNode should be deleted after sync")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred()) // Node should not exist
				}, 30*time.Second, 2*time.Second).Should(Succeed())

				By("And the Hub status.desired should be 0 (or empty due to omitempty)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqhub", hubName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.desired}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					// With omitempty, 0 value is serialized as empty string in jsonpath
					g.Expect(output).To(SatisfyAny(Equal("0"), Equal("")))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})
	})

	Context("Activate Value Truthy Conversion", func() {
		Describe("truthy and falsy activate values", func() {
			const (
				hubName  = "truthy-test-hub"
				formName = "truthy-test-form"
			)

			BeforeEach(func() {
				// Drop and recreate table with VARCHAR active column to ensure correct type
				// This is needed for testing truthy string values like "true", "TRUE", "yes", etc.
				adapter := GetTestDatasource()
				err := RecreateNodesTableWithVarchar(adapter, policyTestNamespace)
				Expect(err).NotTo(HaveOccurred(), "Failed to recreate nodes table with VARCHAR column")

				createHub(hubName)
				createForm(formName, hubName, `
  configMaps:
    - id: test-config
      nameTemplate: "{{ .uid }}-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tenant-id: "{{ .uid }}"
`)
			})

			AfterEach(func() {
				// Drop and recreate table with original BOOLEAN column for other tests
				adapter := GetTestDatasource()
				_ = RecreateNodesTableWithBoolean(adapter, policyTestNamespace)

				cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(3 * time.Second)
			})

			It("should create LynqNodes only for truthy activate values", func() {
				adapter := GetTestDatasource()

				By("Given rows with various truthy activate values: '1', 'true', 'TRUE', 'True', 'yes', 'YES', 'Yes'")
				// Use unique UIDs to avoid MySQL case-insensitive collation issues
				// (e.g., "truthy-TRUE" and "truthy-true" are treated as same key)
				truthyUIDs := []string{"truthy-val-1", "truthy-val-true-upper", "truthy-val-true-mixed", "truthy-val-yes-upper", "truthy-val-yes-mixed", "truthy-val-true-lower", "truthy-val-yes-lower"}
				truthyValues := map[string]string{
					"truthy-val-1":          "1",
					"truthy-val-true-lower": "true",
					"truthy-val-true-upper": "TRUE",
					"truthy-val-true-mixed": "True",
					"truthy-val-yes-lower":  "yes",
					"truthy-val-yes-upper":  "YES",
					"truthy-val-yes-mixed":  "Yes",
				}
				for _, uid := range truthyUIDs {
					value := truthyValues[uid]
					err := InsertTestNodeWithValue(adapter, policyTestNamespace, uid, value)
					Expect(err).NotTo(HaveOccurred())
				}

				By("And rows with falsy activate values: '0', 'false', ''")
				falsyUIDs := []string{"falsy-0", "falsy-empty", "falsy-false"}
				falsyValues := map[string]string{
					"falsy-0":     "0",
					"falsy-false": "false",
					"falsy-empty": "",
				}
				for _, uid := range falsyUIDs {
					value := falsyValues[uid]
					err := InsertTestNodeWithValue(adapter, policyTestNamespace, uid, value)
					Expect(err).NotTo(HaveOccurred())
				}

				By("Then Hub status.desired should equal number of truthy rows (wait for sync)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqhub", hubName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.desired}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("7")) // 7 truthy values
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And LynqNodes should be created for truthy values")
				for _, uid := range truthyUIDs {
					expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
					Eventually(func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
						_, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "LynqNode should exist for truthy uid: %s", uid)
					}, policyTestTimeout, policyTestInterval).Should(Succeed())
				}

				By("And LynqNodes should NOT be created for falsy values")
				// Wait a bit for potential sync
				time.Sleep(15 * time.Second)
				for _, uid := range falsyUIDs {
					expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					Expect(err).To(HaveOccurred(), "LynqNode should NOT exist for falsy uid: %s", uid)
				}
			})
		})
	})

	Context("Extra Value Mappings", func() {
		Describe("extraValueMappings template variable injection", func() {
			const (
				hubName  = "extra-mapping-hub"
				formName = "extra-mapping-form"
				uid      = "extra-mapping-tenant"
			)

			BeforeEach(func() {
				// Add plan_id column to nodes table
				// Try to add column, ignore error if it already exists
				alterSQL := "ALTER TABLE nodes ADD COLUMN plan_id VARCHAR(50) DEFAULT 'basic';"
				cmd := exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
					"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", alterSQL)
				output, err := utils.Run(cmd)
				// Ignore "Duplicate column name" error (MySQL error 1060)
				if err != nil && !strings.Contains(output, "Duplicate column name") {
					Expect(err).NotTo(HaveOccurred(), "Failed to add plan_id column: %s", output)
				}

				// Create hub with extraValueMappings
				hubYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: %s
  namespace: %s
spec:
  source:
    type: mysql
    syncInterval: 5s
    mysql:
      host: mysql.%s.svc.cluster.local
      port: 3306
      database: testdb
      table: nodes
      username: root
      passwordRef:
        name: mysql-root-password
        key: password
  valueMappings:
    uid: id
    activate: active
  extraValueMappings:
    planId: plan_id
`, hubName, policyTestNamespace, policyTestNamespace)
				cmd = exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(hubYAML)
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				// Create form that uses the extra mapping
				createForm(formName, hubName, `
  configMaps:
    - id: test-config
      nameTemplate: "{{ .uid }}-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          tenant-id: "{{ .uid }}"
          plan: "{{ .planId }}"
`)
			})

			AfterEach(func() {
				deleteTestData(uid)

				// Drop plan_id column (ignore error if it doesn't exist)
				alterSQL := "ALTER TABLE nodes DROP COLUMN plan_id;"
				cmd := exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
					"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", alterSQL)
				_, _ = utils.Run(cmd) // Ignore error if column doesn't exist

				cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(3 * time.Second)
			})

			It("should render extraValueMappings variables in templates", func() {
				By("Given a row with plan_id='premium' in the database")
				insertSQL := fmt.Sprintf("INSERT INTO nodes (id, active, plan_id) VALUES ('%s', 1, 'premium') ON DUPLICATE KEY UPDATE active=1, plan_id='premium';", uid)
				cmd := exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
					"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", insertSQL)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("When LynqNode is created and template is rendered")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-config", uid)

				By("Then the ConfigMap should contain the planId value from extraValueMappings")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.plan}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("premium"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the tenant-id should also be correctly set")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.tenant-id}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal(uid))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})
	})

	Context("Error Handling", func() {
		Describe("missing required column in valueMappings", func() {
			const (
				hubName = "missing-column-hub"
			)

			AfterEach(func() {
				cmd := exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(2 * time.Second)
			})

			It("should report error in Hub status when uid column does not exist", func() {
				By("Given a LynqHub that references a non-existent column")
				hubYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: %s
  namespace: %s
spec:
  source:
    type: mysql
    syncInterval: 5s
    mysql:
      host: mysql.%s.svc.cluster.local
      port: 3306
      database: testdb
      table: nodes
      username: root
      passwordRef:
        name: mysql-root-password
        key: password
  valueMappings:
    uid: nonexistent_column
    activate: active
`, hubName, policyTestNamespace, policyTestNamespace)
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(hubYAML)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("When the Hub attempts to sync with the database")
				time.Sleep(10 * time.Second)

				By("Then the Hub should have an error condition or message in status")
				Eventually(func(g Gomega) {
					// Check if status has error-related fields or conditions
					cmd := exec.Command("kubectl", "get", "lynqhub", hubName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					// The Hub should either have error in conditions or remain with desired=0
					// because it cannot read valid rows
					hasError := strings.Contains(output, "error") ||
						strings.Contains(output, "Error") ||
						strings.Contains(output, "Failed") ||
						strings.Contains(output, `"desired":0`)
					g.Expect(hasError).To(BeTrue(), "Hub status should indicate error or have 0 desired: %s", output)
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And no LynqNodes should be created")
				cmd = exec.Command("kubectl", "get", "lynqnodes", "-n", policyTestNamespace,
					"-l", fmt.Sprintf("lynq.sh/hub=%s", hubName), "-o", "name")
				output, _ := utils.Run(cmd)
				Expect(output).To(BeEmpty(), "No LynqNodes should exist for hub with invalid column")
			})
		})
	})
})
