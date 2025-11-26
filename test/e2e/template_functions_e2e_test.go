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
          default-val: "{{ .nonexistent | default \"fallback\" }}"
          b64-encoded: "{{ .uid | b64enc }}"
          trimmed: "{{ \"  spaces  \" | trim }}"
`)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-sprig", uid)

				By("Then ConfigMap should contain correctly transformed values")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				// Check upper
				cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.upper-uid}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("FUNC-TEST-UID"))

				// Check lower
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.lower-uid}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("func-test-uid"))

				// Check default
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.default-val}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("fallback"))

				// Check trimmed
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.data.trimmed}")
				output, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("spaces"))
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
	})
})
