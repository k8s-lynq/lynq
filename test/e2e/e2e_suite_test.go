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
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8s-lynq/lynq/test/utils"
)

var (
	// Optional Environment Variables:
	// - CERT_MANAGER_INSTALL_SKIP=true: Skips CertManager installation during test setup.
	// These variables are useful if CertManager is already installed, avoiding
	// re-installation and conflicts.
	skipCertManagerInstall = os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true"
	// isCertManagerAlreadyInstalled will be set true when CertManager CRDs be found on the cluster
	isCertManagerAlreadyInstalled = false

	// projectImage is the name of the image which will be build and loaded
	// with the code source changes to be tested.
	projectImage = "example.com/lynq:v0.0.1"
)

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment to validate project changes with the purposed to be used in CI jobs.
// The default setup requires Kind, builds/loads the Manager Docker image locally, and installs
// CertManager.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting lynq-operator integration test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("building the manager(Operator) image")
	cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectImage))
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager(Operator) image")

	// TODO(user): If you want to change the e2e test vendor from Kind, ensure the image is
	// built and available before running the tests. Also, remove the following block.
	By("loading the manager(Operator) image on Kind")
	err = utils.LoadImageToKindClusterWithName(projectImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager(Operator) image into Kind")

	// The tests-e2e are intended to run on a temporary cluster that is created and destroyed for testing.
	// To prevent errors when tests run in environments with CertManager already installed,
	// we check for its presence before execution.
	// Setup CertManager before the suite if not skipped and if not already installed
	if !skipCertManagerInstall {
		By("checking if cert manager is installed already")
		isCertManagerAlreadyInstalled = utils.IsCertManagerCRDsInstalled()
		if !isCertManagerAlreadyInstalled {
			// Wait for cert-manager namespace to be fully deleted if it's in Terminating state
			By("ensuring cert-manager namespace is not in terminating state")
			_, _ = fmt.Fprintf(GinkgoWriter, "Checking for existing cert-manager namespace...\n")
			checkCmd := exec.Command("kubectl", "get", "namespace", "cert-manager", "-o", "jsonpath={.status.phase}")
			output, err := utils.Run(checkCmd)
			if err == nil && output == "Terminating" {
				_, _ = fmt.Fprintf(GinkgoWriter, "cert-manager namespace is terminating, waiting for deletion...\n")
				// Wait up to 2 minutes for namespace to be deleted
				Eventually(func() error {
					cmd := exec.Command("kubectl", "get", "namespace", "cert-manager")
					_, err := utils.Run(cmd)
					return err // Returns error when namespace doesn't exist (which is what we want)
				}, "2m", "5s").Should(HaveOccurred(), "cert-manager namespace should be deleted")
				_, _ = fmt.Fprintf(GinkgoWriter, "cert-manager namespace deleted successfully\n")
			}

			// Ensure nodes are fully ready before installing cert-manager
			// This prevents "node(s) had untolerated taint {node.kubernetes.io/not-ready}" errors
			By("ensuring all nodes are ready before cert-manager installation")
			_, _ = fmt.Fprintf(GinkgoWriter, "Waiting for all nodes to be ready...\n")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "nodes", "-o", "jsonpath={.items[*].status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// All nodes should report "True" for Ready condition
				g.Expect(output).To(ContainSubstring("True"))
				// Verify no NotReady taints
				cmd = exec.Command("kubectl", "get", "nodes", "-o", "jsonpath={.items[*].spec.taints[?(@.key=='node.kubernetes.io/not-ready')]}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(BeEmpty(), "Nodes should not have not-ready taints")
			}, "2m", "5s").Should(Succeed(), "All nodes should be ready without taints")
			_, _ = fmt.Fprintf(GinkgoWriter, "All nodes are ready\n")

			_, _ = fmt.Fprintf(GinkgoWriter, "Installing CertManager...\n")
			Expect(utils.InstallCertManager()).To(Succeed(), "Failed to install CertManager")
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: CertManager is already installed. Skipping installation...\n")
		}

		// Wait for cert-manager webhook to be fully ready and accepting requests.
		// This is critical - the webhook deployment can be Available but certificates
		// may not be ready yet, causing webhook validation failures during operator deployment.
		// cert-manager uses a self-signed CA and needs time to generate and trust certificates.
		By("waiting for cert-manager webhook to be fully ready")
		_, _ = fmt.Fprintf(GinkgoWriter, "Waiting for webhook certificates to be generated and trusted...\n")

		// Wait for webhook pods to be ready (not just deployment)
		cmd := exec.Command("kubectl", "wait", "pod",
			"-l", "app.kubernetes.io/name=webhook",
			"--for", "condition=Ready",
			"--namespace", "cert-manager",
			"--timeout", "2m",
		)
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to wait for cert-manager webhook pods")

		// Wait for cert-manager CA injector to update webhook configurations
		// The cainjector watches for webhook configurations and injects the CA bundle
		cmd = exec.Command("kubectl", "wait", "pod",
			"-l", "app.kubernetes.io/name=cainjector",
			"--for", "condition=Ready",
			"--namespace", "cert-manager",
			"--timeout", "2m",
		)
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to wait for cert-manager cainjector pods")

		// Wait for the ValidatingWebhookConfiguration to have the caBundle injected
		// This is critical because even if cainjector is running, it might take a moment to update the webhook config
		By("waiting for cert-manager webhook caBundle injection")
		_, _ = fmt.Fprintf(GinkgoWriter, "Waiting for caBundle to be injected into ValidatingWebhookConfiguration...\n")

		verifyCABundleInjection := func() error {
			cmd := exec.Command("kubectl", "get", "validatingwebhookconfiguration", "cert-manager-webhook",
				"-o", "jsonpath={.webhooks[0].clientConfig.caBundle}")
			output, err := utils.Run(cmd)
			if err != nil {
				return err
			}
			if len(output) == 0 {
				return fmt.Errorf("caBundle is empty")
			}
			return nil
		}
		Eventually(verifyCABundleInjection, "2m", "1s").Should(Succeed(), "caBundle was not injected into ValidatingWebhookConfiguration")

		// Verify cert-manager webhook is working by creating a test Certificate.
		// This ensures the webhook can validate Certificate resources before we deploy lynq operator
		// (which creates its own Certificate/Issuer resources).
		By("verifying cert-manager webhook is functional")
		_, _ = fmt.Fprintf(GinkgoWriter, "Creating test Certificate to verify webhook...\n")

		testCertYAML := `
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: test-selfsigned
  namespace: cert-manager
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: test-webhook-cert
  namespace: cert-manager
spec:
  secretName: test-webhook-cert-secret
  dnsNames:
  - test.example.com
  issuerRef:
    name: test-selfsigned
    kind: Issuer
`
		cmd = exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = utils.StringReader(testCertYAML)
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to create test Certificate")

		// Wait for test certificate to become Ready
		_, _ = fmt.Fprintf(GinkgoWriter, "Waiting for test Certificate to be Ready...\n")
		cmd = exec.Command("kubectl", "wait", "certificate",
			"test-webhook-cert",
			"--for", "condition=Ready",
			"--namespace", "cert-manager",
			"--timeout", "2m")
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Test Certificate did not become Ready")

		// Clean up test resources
		cmd = exec.Command("kubectl", "delete", "certificate", "test-webhook-cert", "-n", "cert-manager")
		_, _ = utils.Run(cmd)
		cmd = exec.Command("kubectl", "delete", "issuer", "test-selfsigned", "-n", "cert-manager")
		_, _ = utils.Run(cmd)

		_, _ = fmt.Fprintf(GinkgoWriter, "cert-manager webhook verified successfully\n")
	}

	// Install CRDs and deploy operator for all tests to use
	By("creating lynq-system namespace")
	cmd = exec.Command("kubectl", "create", "ns", "lynq-system")
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to create lynq-system namespace")

	By("labeling the namespace to enforce the restricted security policy")
	cmd = exec.Command("kubectl", "label", "--overwrite", "ns", "lynq-system",
		"pod-security.kubernetes.io/enforce=restricted")
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to label namespace")

	By("installing CRDs")
	cmd = exec.Command("make", "install")
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to install CRDs")

	By("deploying the lynq controller-manager")
	cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to deploy controller-manager")

	By("waiting for controller-manager to be ready")
	cmd = exec.Command("kubectl", "wait", "deployment", "lynq-controller-manager",
		"--for", "condition=Available",
		"--namespace", "lynq-system",
		"--timeout", "5m")
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to wait for controller-manager")

	By("waiting for lynq webhook caBundle injection")
	_, _ = fmt.Fprintf(GinkgoWriter, "Waiting for caBundle to be injected into Lynq WebhookConfigurations...\n")

	verifyLynqCABundleInjection := func() error {
		// Check ValidatingWebhookConfiguration
		cmd := exec.Command("kubectl", "get", "validatingwebhookconfiguration", "lynq-validating-webhook-configuration",
			"-o", "jsonpath={.webhooks[0].clientConfig.caBundle}")
		output, err := utils.Run(cmd)
		if err != nil {
			return err
		}
		if len(output) == 0 {
			return fmt.Errorf("validating webhook caBundle is empty")
		}

		// Check MutatingWebhookConfiguration
		cmd = exec.Command("kubectl", "get", "mutatingwebhookconfiguration", "lynq-mutating-webhook-configuration",
			"-o", "jsonpath={.webhooks[0].clientConfig.caBundle}")
		output, err = utils.Run(cmd)
		if err != nil {
			return err
		}
		if len(output) == 0 {
			return fmt.Errorf("mutating webhook caBundle is empty")
		}
		return nil
	}
	Eventually(verifyLynqCABundleInjection, "2m", "1s").Should(Succeed(), "caBundle was not injected into Lynq WebhookConfigurations")
})

var _ = AfterSuite(func() {
	// Cleanup all test namespaces first
	By("cleaning up all test namespaces")
	testNamespaces := []string{"policy-test", "lynq-test"}
	for _, ns := range testNamespaces {
		cmd := exec.Command("kubectl", "get", "namespace", ns)
		_, err := utils.Run(cmd)
		if err == nil {
			// Namespace exists, delete it
			_, _ = fmt.Fprintf(GinkgoWriter, "Deleting test namespace: %s\n", ns)
			cmd = exec.Command("kubectl", "delete", "namespace", ns, "--wait=false", "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
		}
	}

	// Cleanup lynq operator and CRDs
	By("undeploying the controller-manager")
	cmd := exec.Command("make", "undeploy")
	_, _ = utils.Run(cmd)

	By("uninstalling CRDs")
	cmd = exec.Command("make", "uninstall")
	_, _ = utils.Run(cmd)

	By("removing lynq-system namespace")
	cmd = exec.Command("kubectl", "delete", "ns", "lynq-system", "--wait=false", "--ignore-not-found=true")
	_, _ = utils.Run(cmd)

	// Wait a bit for namespace deletion to propagate
	By("waiting for namespaces to be deleted")
	allNamespaces := append(testNamespaces, "lynq-system")
	for _, ns := range allNamespaces {
		// Check if namespace still exists
		cmd := exec.Command("kubectl", "get", "namespace", ns)
		_, err := utils.Run(cmd)
		if err == nil {
			// Namespace still exists, try to force cleanup
			_, _ = fmt.Fprintf(GinkgoWriter, "Force cleaning namespace: %s\n", ns)
			_ = utils.CleanupNamespace(ns)
		}
	}

	// Teardown CertManager after the suite if not skipped and if it was not already installed
	if !skipCertManagerInstall && !isCertManagerAlreadyInstalled {
		_, _ = fmt.Fprintf(GinkgoWriter, "Uninstalling CertManager...\n")
		utils.UninstallCertManager()
	}

	_, _ = fmt.Fprintf(GinkgoWriter, "AfterSuite cleanup completed\n")
})
