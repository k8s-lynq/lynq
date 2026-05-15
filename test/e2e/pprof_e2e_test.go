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
	"context"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8s-lynq/lynq/test/utils"
)

var _ = Describe("Pprof Endpoint", Ordered, func() {
	const deploymentName = "lynq-controller-manager"

	BeforeAll(func() {
		By("enabling pprof on the controller manager deployment")
		// Patch the deployment to add --enable-pprof arg and port 6060
		patchJSON := `{"spec":{"template":{"spec":{"containers":[{"name":"manager","args":["--leader-elect","--health-probe-bind-address=:8081","--metrics-bind-address=:8443","--enable-pprof"],"ports":[{"containerPort":8443,"name":"https","protocol":"TCP"},{"containerPort":6060,"name":"pprof","protocol":"TCP"}]}]}}}}`
		cmd := exec.Command("kubectl", "patch", "deployment", deploymentName,
			"-n", namespace, "--type=strategic", "-p", patchJSON)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch deployment for pprof")

		By("waiting for the rollout to complete after patching")
		cmd = exec.Command("kubectl", "rollout", "status", "deployment/"+deploymentName,
			"-n", namespace, "--timeout=120s")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Deployment rollout did not complete")
	})

	// AfterAll intentionally does NOT restore the deployment (remove --enable-pprof):
	// the rolling restart would cause webhook unavailability and break subsequent tests.
	// Leaving pprof enabled is harmless — it only adds an HTTP server on :6060.

	It("should verify the pprof server log message", func() {
		By("checking controller manager logs for pprof startup message")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "pods",
				"-l", "control-plane=controller-manager",
				"-o", "go-template={{ range .items }}"+
					"{{ if not .metadata.deletionTimestamp }}"+
					"{{ .metadata.name }}"+
					"{{ \"\\n\" }}{{ end }}{{ end }}",
				"-n", namespace)
			podOutput, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			podNames := utils.GetNonEmptyLines(podOutput)
			g.Expect(podNames).ToNot(BeEmpty())

			cmd = exec.Command("kubectl", "logs", podNames[0], "-n", namespace)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring("Starting pprof server"),
				"Pprof server startup log not found")
		}, 2*time.Minute, 2*time.Second).Should(Succeed())
	})

	It("should serve pprof index page over HTTP on port 6060", func() {
		By("getting the controller manager pod name")
		var podName string
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "pods",
				"-l", "control-plane=controller-manager",
				"-o", "jsonpath={.items[0].metadata.name}",
				"-n", namespace)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).ToNot(BeEmpty())
			podName = output
		}, 2*time.Minute, 2*time.Second).Should(Succeed())

		// Use kubectl port-forward instead of direct Pod IP access.
		// Direct Pod IP access is fragile across K8s versions: kubectl rollout status
		// completes when the readiness probe (:8081) passes, which is independent of
		// the pprof goroutine binding :6060. Port-forward eliminates pod-to-pod
		// networking dependency and retries until the port is actually accepting connections.
		By("forwarding pprof port to localhost and verifying the endpoint")
		pfCtx, pfCancel := context.WithCancel(context.Background())
		defer pfCancel()

		pfCmd := exec.CommandContext(pfCtx, "kubectl", "port-forward",
			"pod/"+podName, "16060:6060", "-n", namespace)
		Expect(pfCmd.Start()).To(Succeed(), "Failed to start kubectl port-forward")
		defer func() {
			pfCancel()
			_ = pfCmd.Wait()
		}()

		Eventually(func(g Gomega) {
			cmd := exec.Command("curl", "-sf", "--max-time", "5",
				"http://localhost:16060/debug/pprof/")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred(), "pprof endpoint should respond via port-forward")
			g.Expect(output).To(ContainSubstring("/debug/pprof/"),
				"Pprof index page should contain profile links")
			g.Expect(output).To(ContainSubstring("heap"),
				"Pprof index should list heap profile")
			g.Expect(output).To(ContainSubstring("goroutine"),
				"Pprof index should list goroutine profile")
		}, 30*time.Second, 2*time.Second).Should(Succeed())
	})
})
