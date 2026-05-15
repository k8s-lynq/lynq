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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
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

		// The readiness probe (:8081) passes before the webhook server is accepting
		// connections. Subsequent tests that create LynqHub resources will hit the
		// mutating webhook and fail with "context deadline exceeded" if we proceed
		// too quickly. Poll until the webhook responds to a dry-run apply.
		//
		// Any webhook response — including admission rejection — proves the webhook
		// is up. Only connectivity failures (timeout, connection refused) mean it
		// is not ready yet.
		By("waiting for the webhook to be accepting connections after rollout")
		Eventually(func(g Gomega) {
			dryRunYAML := `
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: pprof-webhook-probe
  namespace: ` + namespace + `
spec:
  source:
    type: mysql
    syncInterval: 5s
    mysql:
      host: "probe.example.com"
      port: 3306
      database: "probe"
      username: "probe"
      passwordRef:
        name: probe-secret
        key: password
  valueMappings:
    uid: id
    activate: active
`
			cmd := exec.Command("kubectl", "apply", "--dry-run=server", "-f", "-")
			cmd.Stdin = utils.StringReader(dryRunYAML)
			_, err := utils.Run(cmd)
			if err == nil {
				return
			}
			// Admission/validation rejection means the webhook responded — it is up.
			errMsg := err.Error()
			if strings.Contains(errMsg, "denied the request") ||
				strings.Contains(errMsg, "is invalid") ||
				strings.Contains(errMsg, "Forbidden") ||
				strings.Contains(errMsg, "Unsupported value") {
				return
			}
			g.Expect(err).NotTo(HaveOccurred(), "webhook not yet accepting connections")
		}, 2*time.Minute, 3*time.Second).Should(Succeed())
	})

	// AfterAll intentionally does NOT restore the deployment (remove --enable-pprof):
	// the rolling restart would cause webhook unavailability and break subsequent tests.
	// Leaving pprof enabled is harmless — it only adds an HTTP server on :6060.

	It("should verify the pprof server log message", func() {
		By("checking controller manager logs for pprof startup message")
		Eventually(func(g Gomega) {
			podName, err := pprofReadyControllerPod()
			g.Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command("kubectl", "logs", podName, "-n", namespace)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring("Starting pprof server"),
				"Pprof server startup log not found")
		}, 2*time.Minute, 2*time.Second).Should(Succeed())
	})

	It("should serve pprof index page over HTTP on port 6060", func() {
		// Restart port-forward on every Eventually iteration.
		// kubectl rollout status completes when the readiness probe (:8081) passes,
		// independent of the pprof goroutine binding :6060. If port-forward starts
		// before :6060 is listening, kubectl exits immediately with a non-zero code.
		// Re-creating it each iteration makes the poll self-healing across all K8s versions.
		By("forwarding pprof port to localhost and verifying the endpoint")
		Eventually(func(g Gomega) {
			podName, err := pprofReadyControllerPod()
			g.Expect(err).NotTo(HaveOccurred())

			pfCtx, pfCancel := context.WithCancel(context.Background())
			defer pfCancel()

			var pfOutput lockedBuffer
			pfCmd := exec.CommandContext(pfCtx, "kubectl", "port-forward",
				"--address", "127.0.0.1", "pod/"+podName, "16060:6060", "-n", namespace)
			pfCmd.Stdout = &pfOutput
			pfCmd.Stderr = &pfOutput
			g.Expect(pfCmd.Start()).To(Succeed(), "Failed to start kubectl port-forward")
			defer func() {
				pfCancel()
				_ = pfCmd.Wait()
			}()

			g.Expect(waitForLocalPort("127.0.0.1:16060", 5*time.Second)).To(Succeed(),
				"port-forward to pod %s did not bind localhost; kubectl output: %s",
				podName, pfOutput.String())

			cmd := exec.Command("curl", "-sf", "--max-time", "5",
				"http://127.0.0.1:16060/debug/pprof/")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred(),
				"pprof endpoint should respond via port-forward to pod %s; kubectl output: %s",
				podName, pfOutput.String())
			g.Expect(output).To(ContainSubstring("/debug/pprof/"),
				"Pprof index page should contain profile links")
			g.Expect(output).To(ContainSubstring("heap"),
				"Pprof index should list heap profile")
			g.Expect(output).To(ContainSubstring("goroutine"),
				"Pprof index should list goroutine profile")
		}, 2*time.Minute, 3*time.Second).Should(Succeed())
	})
})

type controllerPodList struct {
	Items []controllerPod `json:"items"`
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type controllerPod struct {
	Metadata struct {
		Name              string  `json:"name"`
		DeletionTimestamp *string `json:"deletionTimestamp,omitempty"`
	} `json:"metadata"`
	Spec struct {
		Containers []struct {
			Name string   `json:"name"`
			Args []string `json:"args"`
		} `json:"containers"`
	} `json:"spec"`
	Status struct {
		Phase      string `json:"phase"`
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
	} `json:"status"`
}

func pprofReadyControllerPod() (string, error) {
	cmd := exec.Command("kubectl", "get", "pods",
		"-l", "control-plane=controller-manager",
		"-o", "json",
		"-n", namespace)
	output, err := utils.Run(cmd)
	if err != nil {
		return "", err
	}

	var pods controllerPodList
	if err := json.Unmarshal([]byte(output), &pods); err != nil {
		return "", fmt.Errorf("parse controller-manager pod list: %w", err)
	}

	for _, pod := range pods.Items {
		if pod.Metadata.DeletionTimestamp != nil || pod.Status.Phase != "Running" {
			continue
		}
		if !podReady(pod) || !podHasPprofEnabled(pod) {
			continue
		}
		return pod.Metadata.Name, nil
	}

	return "", fmt.Errorf("no running ready controller-manager pod with --enable-pprof found")
}

func podReady(pod controllerPod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return true
		}
	}
	return false
}

func podHasPprofEnabled(pod controllerPod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name != "manager" {
			continue
		}
		for _, arg := range container.Args {
			if arg == "--enable-pprof" || arg == "--enable-pprof=true" {
				return true
			}
		}
	}
	return false
}

func waitForLocalPort(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("%s did not accept connections within %s: %w", address, timeout, lastErr)
}
