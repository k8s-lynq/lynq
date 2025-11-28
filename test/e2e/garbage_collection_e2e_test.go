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

var _ = Describe("LynqHub Garbage Collection", Ordered, func() {
	BeforeAll(func() {
		By("setting up policy test namespace")
		setupPolicyTestNamespace()
	})

	AfterAll(func() {
		By("cleaning up policy test namespace")
		cleanupPolicyTestNamespace()
	})

	Context("when database rows are deactivated or deleted", func() {
		const (
			hubName  = "gc-hub"
			formName = "gc-form"
		)

		BeforeEach(func() {
			By("creating a LynqHub")
			createHub(hubName)
		})

		AfterEach(func() {
			By("cleaning up test data and resources")
			// Clean all test UIDs
			for _, uid := range []string{"gc-uid-1", "gc-uid-2", "gc-uid-3", "deactivate-uid", "delete-uid"} {
				deleteTestData(uid)
			}

			// Delete all ConfigMaps
			cmd := exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/node", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			// Delete orphaned resources
			cmd = exec.Command("kubectl", "delete", "configmap", "-n", policyTestNamespace,
				"-l", "lynq.sh/orphaned=true", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			// Delete LynqForm
			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			// Delete additional forms used in tests
			cmd = exec.Command("kubectl", "delete", "lynqform", "gc-form-2", "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			// Delete LynqHub
			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(5 * time.Second)
		})

		Describe("activate flag set to false", func() {
			It("should delete LynqNode when activate flag changes from true to false", func() {
				By("Given a LynqForm with resources")
				createForm(formName, hubName, `
  configMaps:
    - id: gc-config
      nameTemplate: "{{ .uid }}-gc-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)
				const uid = "deactivate-uid"

				By("And active data in MySQL (activate=true)")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-gc-config", uid)

				By("Then the ConfigMap should be created")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When the activate flag is set to false in MySQL")
				updateSQL := fmt.Sprintf("UPDATE nodes SET active=0 WHERE id='%s';", uid)
				cmd := exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
					"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", updateSQL)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("Then the LynqNode should be deleted after sync")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred()) // Should not exist
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should be deleted (DeletionPolicy=Delete by default)")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred()) // Should not exist
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("database row deleted", func() {
			It("should delete LynqNode when database row is deleted", func() {
				By("Given a LynqForm with resources")
				createForm(formName, hubName, `
  configMaps:
    - id: delete-config
      nameTemplate: "{{ .uid }}-delete-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)
				const uid = "delete-uid"

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-delete-config", uid)

				By("Then the ConfigMap should be created")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When the database row is deleted")
				deleteTestData(uid)

				By("Then the LynqNode should be deleted after sync")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred()) // Should not exist
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should be deleted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred()) // Should not exist
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})

		Describe("multiple rows garbage collection", func() {
			It("should clean up only deactivated rows while keeping active ones", func() {
				By("Given a LynqForm with resources")
				createForm(formName, hubName, `
  configMaps:
    - id: multi-gc-config
      nameTemplate: "{{ .uid }}-multi-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)

				By("And multiple active rows in MySQL")
				insertTestData("gc-uid-1", true)
				insertTestData("gc-uid-2", true)
				insertTestData("gc-uid-3", true)

				By("When all LynqNodes are created")
				for _, uid := range []string{"gc-uid-1", "gc-uid-2", "gc-uid-3"} {
					expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
					waitForLynqNode(expectedNodeName)
				}

				By("Then all 3 ConfigMaps should exist")
				for _, uid := range []string{"gc-uid-1", "gc-uid-2", "gc-uid-3"} {
					configMapName := fmt.Sprintf("%s-multi-config", uid)
					Eventually(func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
						_, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred())
					}, policyTestTimeout, policyTestInterval).Should(Succeed())
				}

				By("When gc-uid-2 is deactivated")
				updateSQL := "UPDATE nodes SET active=0 WHERE id='gc-uid-2';"
				cmd := exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
					"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", updateSQL)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("Then only gc-uid-2's LynqNode should be deleted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", "gc-uid-2-"+formName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred()) // Should not exist
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And gc-uid-1 and gc-uid-3 LynqNodes should still exist")
				for _, uid := range []string{"gc-uid-1", "gc-uid-3"} {
					cmd := exec.Command("kubectl", "get", "lynqnode", uid+"-"+formName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred())
				}

				By("And gc-uid-2's ConfigMap should be deleted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", "gc-uid-2-multi-config", "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And gc-uid-1 and gc-uid-3 ConfigMaps should still exist")
				for _, uid := range []string{"gc-uid-1", "gc-uid-3"} {
					cmd := exec.Command("kubectl", "get", "configmap", uid+"-multi-config", "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		Describe("garbage collection with DeletionPolicy=Retain", func() {
			It("should retain resources with orphan markers when activate flag changes to false", func() {
				By("Given a LynqForm with DeletionPolicy=Retain")
				createForm(formName, hubName, `
  configMaps:
    - id: retain-gc-config
      nameTemplate: "{{ .uid }}-retain-gc-config"
      deletionPolicy: Retain
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)
				const uid = "deactivate-uid"

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-retain-gc-config", uid)

				By("Then the ConfigMap should be created")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When the activate flag is set to false in MySQL")
				updateSQL := fmt.Sprintf("UPDATE nodes SET active=0 WHERE id='%s';", uid)
				cmd := exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
					"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", updateSQL)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("Then the LynqNode should be deleted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("But the ConfigMap should still exist (Retain policy)")
				Consistently(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, 10*time.Second, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should be marked as orphaned")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
						"-o", "jsonpath={.metadata.labels.lynq\\.sh/orphaned}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("true"))
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the orphan reason should be LynqNodeDeleted")
				cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace,
					"-o", "jsonpath={.metadata.annotations.lynq\\.sh/orphaned-reason}")
				output, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal("LynqNodeDeleted"))
			})
		})

		Describe("reactivation of deactivated row", func() {
			It("should recreate LynqNode when activate flag changes back to true", func() {
				By("Given a LynqForm with resources")
				createForm(formName, hubName, `
  configMaps:
    - id: reactivate-config
      nameTemplate: "{{ .uid }}-reactivate-config"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
`)
				const uid = "deactivate-uid"

				By("And active data in MySQL")
				insertTestData(uid, true)

				By("When LynqNode is created")
				expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
				waitForLynqNode(expectedNodeName)

				configMapName := fmt.Sprintf("%s-reactivate-config", uid)

				By("Then the ConfigMap should be created")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When the activate flag is set to false")
				updateSQL := fmt.Sprintf("UPDATE nodes SET active=0 WHERE id='%s';", uid)
				cmd := exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
					"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", updateSQL)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("Then the LynqNode should be deleted")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).To(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("When the activate flag is set back to true")
				updateSQL = fmt.Sprintf("UPDATE nodes SET active=1 WHERE id='%s';", uid)
				cmd = exec.Command("kubectl", "exec", "-n", policyTestNamespace, "deployment/mysql", "--",
					"mysql", "-h", "127.0.0.1", "-uroot", "-ptest-password", "testdb", "-e", updateSQL)
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())

				By("Then the LynqNode should be recreated")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())

				By("And the ConfigMap should be recreated")
				Eventually(func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
				}, policyTestTimeout, policyTestInterval).Should(Succeed())
			})
		})
	})
})
