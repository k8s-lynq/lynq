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

var _ = Describe("Pprof Endpoint", Ordered, func() {
	const (
		pprofPodName   = "curl-pprof"
		deploymentName = "lynq-controller-manager"
	)

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

	AfterAll(func() {
		// Cleanup curl pod only. Do NOT restore the deployment (remove --enable-pprof)
		// because the rolling restart would cause webhook unavailability and break
		// subsequent tests. Leaving pprof enabled is harmless — it only adds an HTTP
		// server on :6060 that doesn't affect webhook, metrics, or reconciliation.
		cmd := exec.Command("kubectl", "delete", "pod", pprofPodName,
			"-n", namespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
	})

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
		By("getting the controller manager pod IP")
		// IMPORTANT: filter to Ready pods with no deletionTimestamp. Without this filter
		// the kubectl jsonpath {.items[0]} can pick up a Terminating pod (e.g. from a
		// previous test's `rollout restart`) whose IP is briefly still assigned but
		// becomes invalid moments later — curl then fails with connection refused.
		// This was the root cause of pprof becoming flaky once the bottleneck regression
		// suite added a BUG 2 test that performs `kubectl rollout restart` immediately
		// before pprof BeforeAll re-patches the same Deployment.
		var podIP string
		Eventually(func(g Gomega) {
			// Capture the current pod via `$pod := .` so we can reference its podIP
			// from inside the nested {{ range .status.conditions }}. Using plain `$`
			// inside the inner range would resolve to the PodList root (the template's
			// initial dot), not the iterated pod — which would silently expand to
			// "<no value>" and produce a bogus URL like http://<no value>:6060/...
			cmd := exec.Command("kubectl", "get", "pods",
				"-l", "control-plane=controller-manager",
				"-n", namespace,
				"-o", "go-template={{ range .items }}{{ $pod := . }}"+
					"{{ if not $pod.metadata.deletionTimestamp }}"+
					"{{ range $pod.status.conditions }}"+
					"{{ if and (eq .type \"Ready\") (eq .status \"True\") }}"+
					"{{ $pod.status.podIP }}{{ \"\\n\" }}"+
					"{{ end }}{{ end }}{{ end }}{{ end }}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			lines := utils.GetNonEmptyLines(output)
			g.Expect(lines).ToNot(BeEmpty(), "no Ready controller-manager pod yet (terminating pods filtered out)")
			podIP = lines[0]
			// Defense in depth: reject empty/sentinel values that would otherwise produce
			// a malformed URL. "<no value>" was the symptom of the original template bug.
			g.Expect(podIP).ToNot(BeEmpty())
			g.Expect(podIP).ToNot(Equal("<no value>"))
		}, 2*time.Minute, 2*time.Second).Should(Succeed())

		pprofURL := fmt.Sprintf("http://%s:6060/debug/pprof/", podIP)

		By("creating a curl pod to access the pprof endpoint via Pod IP")
		cmd := exec.Command("kubectl", "delete", "pod", pprofPodName,
			"-n", namespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)

		// curl --retry-connrefused + --retry-all-errors makes the request tolerant of
		// the brief window after a pod restart where the IP is assigned but the pprof
		// HTTP server hasn't bound :6060 yet. Without these flags, the very first
		// connection-refused fails the pod permanently and the Eventually below times
		// out for 120s waiting for a never-arriving Succeeded phase.
		cmd = exec.Command("kubectl", "run", pprofPodName, "--restart=Never",
			"--namespace", namespace,
			"--image=curlimages/curl:latest",
			"--overrides",
			fmt.Sprintf(`{
				"spec": {
					"containers": [{
						"name": "curl",
						"image": "curlimages/curl:latest",
						"command": ["/bin/sh", "-c"],
						"args": ["curl -sf --max-time 10 --retry 5 --retry-delay 3 --retry-connrefused --retry-all-errors %s"],
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {"drop": ["ALL"]},
							"runAsNonRoot": true,
							"runAsUser": 1000,
							"seccompProfile": {"type": "RuntimeDefault"}
						}
					}],
					"serviceAccount": "%s"
				}
			}`, pprofURL, serviceAccountName))
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create curl-pprof pod")

		By("waiting for the curl-pprof pod to complete")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "pods", pprofPodName,
				"-o", "jsonpath={.status.phase}", "-n", namespace)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("Succeeded"), "curl-pprof pod should succeed")
		}, 2*time.Minute, 2*time.Second).Should(Succeed())

		By("verifying pprof index page contains expected profile links")
		cmd = exec.Command("kubectl", "logs", pprofPodName, "-n", namespace)
		output, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		Expect(output).To(ContainSubstring("/debug/pprof/"),
			"Pprof index page should contain profile links")
		Expect(output).To(ContainSubstring("heap"),
			"Pprof index should list heap profile")
		Expect(output).To(ContainSubstring("goroutine"),
			"Pprof index should list goroutine profile")
	})
})
