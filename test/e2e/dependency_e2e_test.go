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

var _ = Describe("Dependency Graph", Ordered, func() {
	BeforeAll(func() {
		By("setting up policy test namespace")
		setupPolicyTestNamespace()
	})

	AfterAll(func() {
		By("cleaning up policy test namespace")
		cleanupPolicyTestNamespace()
	})

	Context("when resources have dependencies via dependIds", func() {
		const (
			hubName  = "dependency-hub"
			formName = "dependency-form"
			uid      = "dep-test-uid"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestData(uid)

			// Delete all resources created by the test
			cmd := exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "secret", "-n", policyTestNamespace,
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

		Describe("Topological ordering", func() {
			It("should apply resources in dependency order (C → B → A when A depends on B, B depends on C)", func() {
				By("Given a LynqForm with chained dependencies: resource-a → resource-b → resource-c")
				createForm(formName, hubName, `
  configMaps:
    - id: resource-c
      nameTemplate: "{{ .uid }}-resource-c"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          order: "1-first"
          name: resource-c
    - id: resource-b
      nameTemplate: "{{ .uid }}-resource-b"
      dependIds:
        - resource-c
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          order: "2-second"
          name: resource-b
    - id: resource-a
      nameTemplate: "{{ .uid }}-resource-a"
      dependIds:
        - resource-b
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          order: "3-third"
          name: resource-a
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created and reconciled")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then all resources should be created")
				resources := []string{
					fmt.Sprintf("%s-resource-a", uid),
					fmt.Sprintf("%s-resource-b", uid),
					fmt.Sprintf("%s-resource-c", uid),
				}

				for _, name := range resources {
					Eventually(func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "configmap", name, "-n", policyTestNamespace)
						_, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "ConfigMap %s should exist", name)
					}, policyTestTimeout, policyTestInterval).Should(Succeed())
				}

				By("And LynqNode should be Ready")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And all resources should be tracked as ready")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.readyResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("3"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Multiple dependencies (fan-in)", func() {
			It("should wait for all dependencies before applying a resource", func() {
				By("Given a LynqForm where resource-final depends on both resource-a and resource-b")
				createForm(formName, hubName, `
  configMaps:
    - id: resource-a
      nameTemplate: "{{ .uid }}-resource-a"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: resource-a
    - id: resource-b
      nameTemplate: "{{ .uid }}-resource-b"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: resource-b
    - id: resource-final
      nameTemplate: "{{ .uid }}-resource-final"
      dependIds:
        - resource-a
        - resource-b
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: resource-final
          depends-on: "resource-a and resource-b"
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created and reconciled")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then all resources including the fan-in resource should be created")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", fmt.Sprintf("%s-resource-final", uid),
						"-n", policyTestNamespace, "-o", "jsonpath={.data.depends-on}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("resource-a and resource-b"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And LynqNode should be Ready with all 3 resources")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.readyResources}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("3"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("Diamond dependency pattern", func() {
			It("should correctly handle diamond dependencies (A → B,C → D)", func() {
				By("Given a diamond dependency pattern: D ← B ← A and D ← C ← A")
				createForm(formName, hubName, `
  configMaps:
    - id: resource-a
      nameTemplate: "{{ .uid }}-diamond-a"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          level: root
    - id: resource-b
      nameTemplate: "{{ .uid }}-diamond-b"
      dependIds:
        - resource-a
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          level: middle-left
    - id: resource-c
      nameTemplate: "{{ .uid }}-diamond-c"
      dependIds:
        - resource-a
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          level: middle-right
    - id: resource-d
      nameTemplate: "{{ .uid }}-diamond-d"
      dependIds:
        - resource-b
        - resource-c
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          level: bottom
`)

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created and reconciled")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				By("Then all 4 resources in the diamond should be created")
				resources := []struct {
					name  string
					level string
				}{
					{fmt.Sprintf("%s-diamond-a", uid), "root"},
					{fmt.Sprintf("%s-diamond-b", uid), "middle-left"},
					{fmt.Sprintf("%s-diamond-c", uid), "middle-right"},
					{fmt.Sprintf("%s-diamond-d", uid), "bottom"},
				}

				for _, r := range resources {
					Eventually(func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "configmap", r.name, "-n", policyTestNamespace,
							"-o", "jsonpath={.data.level}")
						output, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "ConfigMap %s should exist", r.name)
						g.Expect(output).To(Equal(r.level))
					}, policyTestTimeout, policyTestInterval).Should(Succeed())
				}

				By("And LynqNode should be Ready")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})
	})

	Context("Dependency cycle detection", func() {
		const (
			hubName  = "cycle-hub"
			formName = "cycle-form"
			uid      = "cycle-test-uid"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			deleteTestData(uid)

			// Delete all resources created by the test
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

		Describe("Direct cycle (A → B → A)", func() {
			It("should be rejected by webhook at LynqForm creation time", func() {
				By("Given a LynqForm with circular dependency: A → B → A")
				// The ValidatingWebhook should reject this LynqForm creation
				// This is better behavior - fail-fast at admission time rather than runtime
				formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  configMaps:
    - id: resource-a
      nameTemplate: "{{ .uid }}-cycle-a"
      dependIds:
        - resource-b
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: cycle-a
    - id: resource-b
      nameTemplate: "{{ .uid }}-cycle-b"
      dependIds:
        - resource-a
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: cycle-b
`, formName, policyTestNamespace, hubName)

				By("When attempting to create the LynqForm")
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = strings.NewReader(formYAML)
				output, err := cmd.CombinedOutput()

				By("Then the webhook should reject the request")
				Expect(err).To(HaveOccurred(), "LynqForm with cycle should be rejected")

				By("And the error message should indicate circular dependency")
				Expect(string(output)).To(Or(
					ContainSubstring("circular dependency"),
					ContainSubstring("dependency cycle"),
					ContainSubstring("cycle detected"),
				))
			})
		})

		Describe("Indirect cycle (A → B → C → A)", func() {
			It("should be rejected by webhook at LynqForm creation time", func() {
				By("Given a LynqForm with indirect circular dependency: A → B → C → A")
				// The ValidatingWebhook should reject this LynqForm creation
				formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  configMaps:
    - id: resource-a
      nameTemplate: "{{ .uid }}-indirect-a"
      dependIds:
        - resource-c
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: indirect-a
    - id: resource-b
      nameTemplate: "{{ .uid }}-indirect-b"
      dependIds:
        - resource-a
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: indirect-b
    - id: resource-c
      nameTemplate: "{{ .uid }}-indirect-c"
      dependIds:
        - resource-b
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: indirect-c
`, formName, policyTestNamespace, hubName)

				By("When attempting to create the LynqForm")
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = strings.NewReader(formYAML)
				output, err := cmd.CombinedOutput()

				By("Then the webhook should reject the request")
				Expect(err).To(HaveOccurred(), "LynqForm with cycle should be rejected")

				By("And the error message should indicate circular dependency")
				Expect(string(output)).To(Or(
					ContainSubstring("circular dependency"),
					ContainSubstring("dependency cycle"),
					ContainSubstring("cycle detected"),
				))
			})
		})

		Describe("Self-reference cycle", func() {
			It("should be rejected by webhook at LynqForm creation time", func() {
				By("Given a LynqForm with self-referencing dependency: A → A")
				// The ValidatingWebhook should reject this LynqForm creation
				formYAML := fmt.Sprintf(`
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: %s
  namespace: %s
spec:
  hubId: %s
  configMaps:
    - id: resource-self
      nameTemplate: "{{ .uid }}-self-ref"
      dependIds:
        - resource-self
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          name: self-ref
`, formName, policyTestNamespace, hubName)

				By("When attempting to create the LynqForm")
				cmd := exec.Command("kubectl", "apply", "-f", "-")
				cmd.Stdin = strings.NewReader(formYAML)
				output, err := cmd.CombinedOutput()

				By("Then the webhook should reject the request")
				Expect(err).To(HaveOccurred(), "LynqForm with self-reference should be rejected")

				By("And the error message should indicate circular dependency or self-reference")
				Expect(string(output)).To(Or(
					ContainSubstring("circular dependency"),
					ContainSubstring("dependency cycle"),
					ContainSubstring("cycle detected"),
					ContainSubstring("self-reference"),
				))
			})
		})
	})
})
