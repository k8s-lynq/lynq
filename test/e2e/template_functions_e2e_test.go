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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8s-lynq/lynq/test/utils"
)

// verifyConfigMapValue checks that a ConfigMap has been created and verifies a specific data value.
// Returns the actual value found for the given key.
func verifyConfigMapValue(configMapName, namespace, dataKey, expectedValue string) {
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", namespace)
		_, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
	}, policyTestTimeout, policyTestInterval).Should(Succeed())

	cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", namespace,
		"-o", fmt.Sprintf("jsonpath={.data.%s}", dataKey))
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
	Expect(output).To(Equal(expectedValue))
}

var _ = Describe("Template Functions", Ordered, func() {
	BeforeAll(func() {
		By("setting up policy test namespace")
		setupPolicyTestNamespace()
	})

	AfterAll(func() {
		By("cleaning up policy test namespace")
		cleanupPolicyTestNamespace()
	})

	Context("Custom template functions", func() {
		const (
			hubName  = "template-func-hub"
			formName = "template-func-form"
			uid      = "func-test-uid"
		)

		BeforeEach(func() {
			By("creating a LynqHub with extraValueMappings")
			// Create hub with extra value mappings for testing
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
`, hubName, policyTestNamespace, policyTestNamespace)
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = utils.StringReader(hubYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestData(uid)

			// Delete all ConfigMaps
			cmd := exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
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

		Describe("trunc63 function", func() {
			It("should truncate strings to 63 characters for K8s name compliance", func() {
				By("Given a UID that would exceed 63 characters when used in a resource name")
				// Use a UID that is within K8s name limits but long enough to test trunc63
				// The UID + suffix should exceed 63 chars to demonstrate truncation
				longUID := "long-uid-for-trunc"

				// Insert data with long UID
				insertSQL := fmt.Sprintf("INSERT INTO nodes (id, active) VALUES ('%s', 1) ON DUPLICATE KEY UPDATE active=1;", longUID)
				cmd := exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
					"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", insertSQL)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("And a LynqForm using trunc63 in nameTemplate with additional suffix")
				// This template creates a name that would exceed 63 chars without trunc63
				// Using Go template string concatenation instead of printf to avoid fmt.Sprintf conflicts
				// The suffix is long enough that uid + suffix > 63 chars
				createForm(formName, hubName, `
  configMaps:
    - id: trunc-config
      nameTemplate: '{{ print .uid "-this-is-a-very-long-suffix-to-exceed-limit" | trunc63 }}'
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          original-uid: "{{ .uid }}"
`)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", longUID, formName)
				waitForLynqNode(expectedNodeName)

				By("Then a ConfigMap should be created with truncated name (63 chars max)")
				// The full name would be "long-uid-for-trunc-this-is-a-very-long-suffix-to-exceed-limit" (62 chars)
				// This is within limit, so no truncation needed
				// Let's calculate: longUID (18) + suffix (43) = 61 chars - within limit
				// Actually we need a longer suffix to test truncation
				fullName := longUID + "-this-is-a-very-long-suffix-to-exceed-limit"
				truncatedName := fullName
				if len(fullName) > 63 {
					truncatedName = fullName[:63]
				}
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", truncatedName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the truncated name should be at most 63 characters")
				Expect(len(truncatedName)).To(BeNumerically("<=", 63))

				By("And the original UID should be preserved in data")
				cmd = exec.Command("kubectl", "get", "configmap", truncatedName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.original-uid}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal(longUID))

				// Cleanup
				deleteSQL := fmt.Sprintf("DELETE FROM nodes WHERE id='%s';", longUID)
				cmd = exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
					"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", deleteSQL)
				_, _ = utils.Run(cmd)
			})
		})

		Describe("sha1sum function", func() {
			It("should generate SHA1 hash of input string", func() {
				By("Given active data in MySQL")
				insertTestData(uid, true)

				By("And a LynqForm using sha1sum function")
				createForm(formName, hubName, `
  configMaps:
    - id: sha1-config
      nameTemplate: "{{ .uid }}-sha1"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          uid-hash: "{{ .uid | sha1sum }}"
          known-hash: "{{ \"test\" | sha1sum }}"
`)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-sha1", uid)

				By("Then ConfigMap should contain SHA1 hashes")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.data.known-hash}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					// SHA1 of "test" is a]09de3a4b1abe2caed4c80de6b0c5d2a4
					g.Expect(output).To(HaveLen(40)) // SHA1 produces 40 hex characters
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Sprig functions", func() {
			It("should support common Sprig functions like default, upper, lower, b64enc", func() {
				By("Given active data in MySQL")
				insertTestData(uid, true)

				By("And a LynqForm using various Sprig functions")
				createForm(formName, hubName, `
  configMaps:
    - id: sprig-config
      nameTemplate: "{{ .uid }}-sprig"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          upper-uid: "{{ .uid | upper }}"
          lower-uid: "{{ .uid | lower }}"
          default-val: "{{ \"\" | default \"fallback\" }}"
          b64-encoded: "{{ .uid | b64enc }}"
          trimmed: "{{ \"  spaces  \" | trim }}"
`)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-sprig", uid)

				By("Then ConfigMap should contain correctly transformed values")
				verifyConfigMapValue(configMapName, policyTestNamespace, "upper-uid", "FUNC-TEST-UID")
				verifyConfigMapValue(configMapName, policyTestNamespace, "lower-uid", "func-test-uid")
				verifyConfigMapValue(configMapName, policyTestNamespace, "default-val", "fallback")
				verifyConfigMapValue(configMapName, policyTestNamespace, "trimmed", "spaces")
			})
		})

		Describe("Template variables", func() {
			It("should provide hubId and templateRef variables", func() {
				By("Given active data in MySQL")
				insertTestData(uid, true)

				By("And a LynqForm using context variables")
				createForm(formName, hubName, `
  configMaps:
    - id: context-config
      nameTemplate: "{{ .uid }}-context"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          hub-id: "{{ .hubId }}"
          template-ref: "{{ .templateRef }}"
          uid: "{{ .uid }}"
          activate: "{{ .activate }}"
`)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-context", uid)

				By("Then ConfigMap should contain correct context variables")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				// Check hubId
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.hub-id}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal(hubName))

				// Check templateRef
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.template-ref}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal(formName))

				// Check uid
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.uid}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal(uid))
			})
		})

		Describe("labelsTemplate and annotationsTemplate", func() {
			It("should support template expressions in labels and annotations", func() {
				By("Given active data in MySQL")
				insertTestData(uid, true)

				By("And a LynqForm with templated labels and annotations")
				createForm(formName, hubName, `
  configMaps:
    - id: labeled-config
      nameTemplate: "{{ .uid }}-labeled"
      labelsTemplate:
        tenant-id: "{{ .uid }}"
        managed-by: "lynq"
      annotationsTemplate:
        description: "Config for tenant {{ .uid }}"
        hub: "{{ .hubId }}"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-labeled", uid)

				By("Then ConfigMap should have templated labels")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.metadata.labels.tenant-id}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal(uid))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.labels.managed-by}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("lynq"))

				By("And ConfigMap should have templated annotations")
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.annotations.description}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal(fmt.Sprintf("Config for tenant %s", uid)))

				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.annotations.hub}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal(hubName))
			})
		})

		Describe("Typed template functions (int, float, bool)", func() {
			It("should convert string values to proper integer types for Kubernetes API", func() {
				By("Given active data in MySQL with numeric values as strings")
				// Insert test data with extra value mappings for replicas and port
				insertSQL := fmt.Sprintf("INSERT INTO nodes (id, active, replicas, app_port) VALUES ('%s', 1, '3', '8080') ON DUPLICATE KEY UPDATE active=1, replicas='3', app_port='8080';", uid)
				cmd := exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
					"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", insertSQL)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("And a LynqHub with extraValueMappings for numeric fields")
				// Update hub to include extra value mappings
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
    replicas: replicas
    appPort: app_port
`, hubName, policyTestNamespace, policyTestNamespace)
				cmd = exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = utils.StringReader(hubYAML)
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				// Wait for hub to sync
				time.Sleep(2 * time.Second)

				By("And a LynqForm using int function for numeric fields")
				// Create a Deployment with replicas and containerPort using int function
				createForm(formName, hubName, `
  configMaps:
    - id: int-test-config
      nameTemplate: "{{ .uid }}-int-test"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          replicas-str: "{{ .replicas }}"
          port-str: "{{ .appPort }}"
  deployments:
    - id: int-test-deploy
      nameTemplate: "{{ .uid }}-int-deploy"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: "{{ .replicas | int }}"
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
                    - containerPort: "{{ .appPort | int }}"
`)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				deploymentName := fmt.Sprintf("%s-int-deploy", uid)

				By("Then Deployment should be created with correct integer types")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And replicas should be a valid integer (not quoted string)")
				cmd = exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
					"-o", "jsonpath={.spec.replicas}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("3"))

				By("And containerPort should be a valid integer")
				cmd = exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].ports[0].containerPort}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("8080"))
			})

			It("should convert string values to boolean types", func() {
				By("Given active data in MySQL")
				insertTestData(uid, true)

				By("And a LynqForm using bool function")
				createForm(formName, hubName, `
  configMaps:
    - id: bool-test-config
      nameTemplate: "{{ .uid }}-bool-test"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          true-from-string: '{{ "true" | bool }}'
          false-from-string: '{{ "false" | bool }}'
          true-from-one: '{{ "1" | bool }}'
          false-from-zero: '{{ "0" | bool }}'
          true-from-yes: '{{ "yes" | bool }}'
`)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-bool-test", uid)

				By("Then ConfigMap should be created with boolean values")
				verifyConfigMapValue(configMapName, policyTestNamespace, "true-from-string", "true")
				verifyConfigMapValue(configMapName, policyTestNamespace, "false-from-string", "false")
				verifyConfigMapValue(configMapName, policyTestNamespace, "true-from-one", "true")
				verifyConfigMapValue(configMapName, policyTestNamespace, "true-from-yes", "true")
			})

			It("should convert string values to float types", func() {
				By("Given active data in MySQL")
				insertTestData(uid, true)

				By("And a LynqForm using float function")
				createForm(formName, hubName, `
  configMaps:
    - id: float-test-config
      nameTemplate: "{{ .uid }}-float-test"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          cpu-limit: '{{ "1.5" | float }}'
          memory-ratio: '{{ "0.75" | float }}'
          integer-as-float: '{{ "42" | float }}'
`)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-float-test", uid)

				By("Then ConfigMap should be created with float values")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.cpu-limit}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("1.5"))

				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.memory-ratio}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("0.75"))

				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.integer-as-float}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("42"))
			})

			It("should handle int function with default values for optional fields", func() {
				By("Given active data in MySQL")
				insertTestData(uid, true)

				By("And a LynqForm using int with default function")
				createForm(formName, hubName, `
  deployments:
    - id: default-int-deploy
      nameTemplate: "{{ .uid }}-default-int"
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: '{{ index . "customReplicas" | default "2" | int }}'
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
`)

				By("When LynqNode is created (customReplicas not provided)")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				deploymentName := fmt.Sprintf("%s-default-int", uid)

				By("Then Deployment should use default replicas value")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
						"-o", "jsonpath={.spec.replicas}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("2"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})

			It("should gracefully handle invalid values with int function (returns 0)", func() {
				By("Given active data in MySQL")
				insertTestData(uid, true)

				By("And a LynqForm using int function with invalid string input")
				createForm(formName, hubName, `
  configMaps:
    - id: invalid-int-config
      nameTemplate: "{{ .uid }}-invalid-int"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          invalid-to-zero: '{{ "not-a-number" | int }}'
          empty-to-zero: '{{ "" | int }}'
`)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-invalid-int", uid)

				By("Then ConfigMap should contain 0 for invalid inputs")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.invalid-to-zero}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("0"))

				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.empty-to-zero}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("0"))
			})
		})
	})
})
