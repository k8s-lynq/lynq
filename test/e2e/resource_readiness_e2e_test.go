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

var _ = Describe("Resource Readiness and Types", Ordered, func() {
	BeforeAll(func() {
		setupPolicyTestNamespace()
	})

	AfterAll(func() {
		cleanupPolicyTestNamespace()
	})

	//nolint:dupl // Test contexts intentionally have similar structure for readability
	Context("StatefulSet Readiness", func() {
		const (
			hubName  = "statefulset-hub"
			formName = "statefulset-form"
			uid      = "statefulset-tenant"
		)

		BeforeEach(func() {
			createHub(hubName)
		})

		AfterEach(func() {
			deleteTestData(uid)

			cmd := exec.Command("kubectl", "delete", "statefulset", uid+"-sts", "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(3 * time.Second)
		})

		It("should wait for StatefulSet to have all replicas ready before marking LynqNode as Ready", func() {
			By("Given a LynqForm with a StatefulSet that has replicas=2")
			createForm(formName, hubName, `
  statefulSets:
    - id: test-sts
      nameTemplate: "{{ .uid }}-sts"
      waitForReady: true
      timeoutSeconds: 180
      spec:
        apiVersion: apps/v1
        kind: StatefulSet
        spec:
          replicas: 2
          serviceName: "{{ .uid }}-svc"
          selector:
            matchLabels:
              app: test-sts
          template:
            metadata:
              labels:
                app: test-sts
            spec:
              containers:
              - name: busybox
                image: busybox:1.36
                command: ["sh", "-c", "sleep 3600"]
`)

			By("And test data in MySQL")
			insertTestData(uid, true)

			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			By("When LynqNode is created")
			waitForLynqNode(expectedNodeName)

			stsName := fmt.Sprintf("%s-sts", uid)

			By("Then the StatefulSet should be created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "statefulset", stsName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the LynqNode should become Ready when StatefulSet has readyReplicas=2")
			Eventually(func(g Gomega) {
				// Check StatefulSet ready replicas
				cmd := exec.Command("kubectl", "get", "statefulset", stsName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("2"))

				// Check LynqNode is Ready
				cmd = exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		})
	})

	//nolint:dupl // Test contexts intentionally have similar structure for readability
	Context("Job Completion", func() {
		const (
			hubName  = "job-hub"
			formName = "job-form"
			uid      = "job-tenant"
		)

		BeforeEach(func() {
			createHub(hubName)
		})

		AfterEach(func() {
			deleteTestData(uid)

			cmd := exec.Command("kubectl", "delete", "job", uid+"-job", "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(3 * time.Second)
		})

		It("should wait for Job to complete before marking LynqNode as Ready", func() {
			By("Given a LynqForm with a Job that runs a quick task")
			createForm(formName, hubName, `
  jobs:
    - id: test-job
      nameTemplate: "{{ .uid }}-job"
      waitForReady: true
      timeoutSeconds: 120
      spec:
        apiVersion: batch/v1
        kind: Job
        spec:
          template:
            spec:
              containers:
              - name: job
                image: busybox:1.36
                command: ["sh", "-c", "echo 'Job completed' && sleep 5"]
              restartPolicy: Never
          backoffLimit: 3
`)

			By("And test data in MySQL")
			insertTestData(uid, true)

			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			By("When LynqNode is created")
			waitForLynqNode(expectedNodeName)

			jobName := fmt.Sprintf("%s-job", uid)

			By("Then the Job should be created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "job", jobName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the LynqNode should become Ready when Job succeeds")
			Eventually(func(g Gomega) {
				// Check Job succeeded
				cmd := exec.Command("kubectl", "get", "job", jobName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.succeeded}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"))

				// Check LynqNode is Ready
				cmd = exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		})
	})

	Context("ConfigMap and Secret Immediate Readiness", func() {
		const (
			hubName  = "immediate-ready-hub"
			formName = "immediate-ready-form"
			uid      = "immediate-ready-tenant"
		)

		BeforeEach(func() {
			createHub(hubName)
		})

		AfterEach(func() {
			deleteTestData(uid)

			cmd := exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(3 * time.Second)
		})

		It("should count ConfigMap and Secret as Ready immediately after creation", func() {
			By("Given a LynqForm with ConfigMap and Secret")
			createForm(formName, hubName, `
  configMaps:
    - id: test-cm
      nameTemplate: "{{ .uid }}-cm"
      spec:
        apiVersion: v1
        kind: ConfigMap
        data:
          key: value
  secrets:
    - id: test-secret
      nameTemplate: "{{ .uid }}-secret"
      spec:
        apiVersion: v1
        kind: Secret
        type: Opaque
        stringData:
          password: "secret-value"
`)

			By("And test data in MySQL")
			insertTestData(uid, true)

			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			By("When LynqNode is created")
			waitForLynqNode(expectedNodeName)

			By("Then the LynqNode should become Ready quickly with 2 ready resources")
			Eventually(func(g Gomega) {
				// Check readyResources count
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.readyResources}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("2"))

				// Check desiredResources count
				cmd = exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.desiredResources}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("2"))

				// Check Ready condition
				cmd = exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, 30*time.Second, policyTestInterval).Should(Succeed()) // Should be quick
		})
	})

	Context("waitForReady=false Behavior", func() {
		const (
			hubName  = "skip-wait-hub"
			formName = "skip-wait-form"
			uid      = "skip-wait-tenant"
		)

		BeforeEach(func() {
			createHub(hubName)
		})

		AfterEach(func() {
			deleteTestData(uid)

			cmd := exec.Command("kubectl", "delete", "deployment", uid+"-deploy", "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(3 * time.Second)
		})

		It("should count resource as Ready immediately when waitForReady=false", func() {
			By("Given a LynqForm with a Deployment that has waitForReady=false")
			createForm(formName, hubName, `
  deployments:
    - id: test-deploy
      nameTemplate: "{{ .uid }}-deploy"
      waitForReady: false
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 3
          selector:
            matchLabels:
              app: skip-wait
          template:
            metadata:
              labels:
                app: skip-wait
            spec:
              containers:
              - name: busybox
                image: busybox:1.36
                command: ["sh", "-c", "sleep 3600"]
`)

			By("And test data in MySQL")
			insertTestData(uid, true)

			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			By("When LynqNode is created")
			waitForLynqNode(expectedNodeName)

			By("Then the LynqNode should become Ready immediately, even if Deployment is not fully available")
			// LynqNode should become Ready very quickly since waitForReady=false
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}, 30*time.Second, policyTestInterval).Should(Succeed())

			By("And the Deployment may still be rolling out")
			// At this point, Deployment might not be fully ready, but LynqNode is Ready
			deployName := fmt.Sprintf("%s-deploy", uid)
			cmd := exec.Command("kubectl", "get", "deployment", deployName, "-n", policyTestNamespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Deployment should exist")
		})
	})

	Context("CronJob Creation", func() {
		const (
			hubName  = "cronjob-hub"
			formName = "cronjob-form"
			uid      = "cronjob-tenant"
		)

		BeforeEach(func() {
			createHub(hubName)
		})

		AfterEach(func() {
			deleteTestData(uid)

			cmd := exec.Command("kubectl", "delete", "cronjob", uid+"-cron", "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(3 * time.Second)
		})

		It("should create and track CronJob resource", func() {
			By("Given a LynqForm with a CronJob")
			createForm(formName, hubName, `
  cronJobs:
    - id: test-cronjob
      nameTemplate: "{{ .uid }}-cron"
      spec:
        apiVersion: batch/v1
        kind: CronJob
        spec:
          schedule: "*/5 * * * *"
          jobTemplate:
            spec:
              template:
                spec:
                  containers:
                  - name: job
                    image: busybox:1.36
                    command: ["echo", "scheduled job"]
                  restartPolicy: OnFailure
`)

			By("And test data in MySQL")
			insertTestData(uid, true)

			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			By("When LynqNode is created")
			waitForLynqNode(expectedNodeName)

			cronJobName := fmt.Sprintf("%s-cron", uid)

			By("Then the CronJob should be created in the cluster")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "cronjob", cronJobName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the CronJob should have correct schedule")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "cronjob", cronJobName, "-n", policyTestNamespace,
					"-o", "jsonpath={.spec.schedule}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("*/5 * * * *"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the LynqNode should track the CronJob in appliedResources")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.appliedResources}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("CronJob"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		})
	})

	Context("DaemonSet Creation", func() {
		const (
			hubName  = "daemonset-hub"
			formName = "daemonset-form"
			uid      = "daemonset-tenant"
		)

		BeforeEach(func() {
			createHub(hubName)
		})

		AfterEach(func() {
			deleteTestData(uid)

			cmd := exec.Command("kubectl", "delete", "daemonset", uid+"-ds", "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(3 * time.Second)
		})

		It("should create and track DaemonSet resource", func() {
			By("Given a LynqForm with a DaemonSet")
			createForm(formName, hubName, `
  daemonSets:
    - id: test-ds
      nameTemplate: "{{ .uid }}-ds"
      waitForReady: false
      spec:
        apiVersion: apps/v1
        kind: DaemonSet
        spec:
          selector:
            matchLabels:
              app: test-ds
          template:
            metadata:
              labels:
                app: test-ds
            spec:
              containers:
              - name: busybox
                image: busybox:1.36
                command: ["sh", "-c", "sleep 3600"]
`)

			By("And test data in MySQL")
			insertTestData(uid, true)

			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			By("When LynqNode is created")
			waitForLynqNode(expectedNodeName)

			dsName := fmt.Sprintf("%s-ds", uid)

			By("Then the DaemonSet should be created in the cluster")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "daemonset", dsName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the LynqNode should track the DaemonSet in appliedResources")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "lynqnode", expectedNodeName, "-n", policyTestNamespace,
					"-o", "jsonpath={.status.appliedResources}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("DaemonSet"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		})
	})

	Context("NetworkPolicy Creation", func() {
		const (
			hubName  = "netpol-hub"
			formName = "netpol-form"
			uid      = "netpol-tenant"
		)

		BeforeEach(func() {
			createHub(hubName)
		})

		AfterEach(func() {
			deleteTestData(uid)

			cmd := exec.Command("kubectl", "delete", "networkpolicy", uid+"-netpol", "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(3 * time.Second)
		})

		It("should create NetworkPolicy resource", func() {
			By("Given a LynqForm with a NetworkPolicy")
			createForm(formName, hubName, `
  networkPolicies:
    - id: test-netpol
      nameTemplate: "{{ .uid }}-netpol"
      spec:
        apiVersion: networking.k8s.io/v1
        kind: NetworkPolicy
        spec:
          podSelector:
            matchLabels:
              app: test-app
          policyTypes:
          - Ingress
          ingress:
          - from:
            - podSelector:
                matchLabels:
                  role: frontend
`)

			By("And test data in MySQL")
			insertTestData(uid, true)

			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			By("When LynqNode is created")
			waitForLynqNode(expectedNodeName)

			netpolName := fmt.Sprintf("%s-netpol", uid)

			By("Then the NetworkPolicy should be created in the cluster")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "networkpolicy", netpolName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		})
	})

	Context("HorizontalPodAutoscaler Creation", func() {
		const (
			hubName  = "hpa-hub"
			formName = "hpa-form"
			uid      = "hpa-tenant"
		)

		BeforeEach(func() {
			createHub(hubName)
		})

		AfterEach(func() {
			deleteTestData(uid)

			cmd := exec.Command("kubectl", "delete", "hpa", uid+"-hpa", "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "deployment", uid+"-deploy", "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(3 * time.Second)
		})

		It("should create HPA targeting a Deployment", func() {
			By("Given a LynqForm with a Deployment and HPA")
			createForm(formName, hubName, `
  deployments:
    - id: test-deploy
      nameTemplate: "{{ .uid }}-deploy"
      waitForReady: false
      spec:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: hpa-test
          template:
            metadata:
              labels:
                app: hpa-test
            spec:
              containers:
              - name: busybox
                image: busybox:1.36
                command: ["sh", "-c", "sleep 3600"]
                resources:
                  requests:
                    cpu: "50m"
  horizontalPodAutoscalers:
    - id: test-hpa
      nameTemplate: "{{ .uid }}-hpa"
      dependIds: ["test-deploy"]
      spec:
        apiVersion: autoscaling/v2
        kind: HorizontalPodAutoscaler
        spec:
          scaleTargetRef:
            apiVersion: apps/v1
            kind: Deployment
            name: "{{ .uid }}-deploy"
          minReplicas: 1
          maxReplicas: 10
          metrics:
          - type: Resource
            resource:
              name: cpu
              target:
                type: Utilization
                averageUtilization: 80
`)

			By("And test data in MySQL")
			insertTestData(uid, true)

			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			By("When LynqNode is created")
			waitForLynqNode(expectedNodeName)

			hpaName := fmt.Sprintf("%s-hpa", uid)

			By("Then the HPA should be created in the cluster")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "hpa", hpaName, "-n", policyTestNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the HPA should target the correct Deployment")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "hpa", hpaName, "-n", policyTestNamespace,
					"-o", "jsonpath={.spec.scaleTargetRef.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(uid + "-deploy"))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())
		})
	})

	Context("Namespace Resource with Label-based Tracking", func() {
		const (
			hubName  = "namespace-hub"
			formName = "namespace-form"
			uid      = "ns-tenant"
		)

		BeforeEach(func() {
			createHub(hubName)
		})

		AfterEach(func() {
			deleteTestData(uid)

			// Delete namespace created by test
			cmd := exec.Command("kubectl", "delete", "namespace", uid+"-ns", "--ignore-not-found=true", "--wait=false")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqform", formName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "lynqhub", hubName, "-n", policyTestNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			time.Sleep(3 * time.Second)
		})

		It("should create Namespace with label-based tracking instead of ownerReference", func() {
			By("Given a LynqForm with a Namespace resource")
			createForm(formName, hubName, `
  namespaces:
    - id: test-ns
      nameTemplate: "{{ .uid }}-ns"
      spec:
        apiVersion: v1
        kind: Namespace
        metadata:
          labels:
            tenant: "{{ .uid }}"
`)

			By("And test data in MySQL")
			insertTestData(uid, true)

			expectedNodeName := fmt.Sprintf("%s-%s", uid, formName)
			By("When LynqNode is created")
			waitForLynqNode(expectedNodeName)

			nsName := fmt.Sprintf("%s-ns", uid)

			By("Then the Namespace should be created in the cluster")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "namespace", nsName)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the Namespace should have tracking labels (not ownerReference)")
			Eventually(func(g Gomega) {
				// Check for lynq.sh/node label
				cmd := exec.Command("kubectl", "get", "namespace", nsName,
					"-o", "jsonpath={.metadata.labels.lynq\\.sh/node}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(expectedNodeName))

				// Check for lynq.sh/node-namespace label
				cmd = exec.Command("kubectl", "get", "namespace", nsName,
					"-o", "jsonpath={.metadata.labels.lynq\\.sh/node-namespace}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(policyTestNamespace))
			}, policyTestTimeout, policyTestInterval).Should(Succeed())

			By("And the Namespace should NOT have ownerReferences")
			cmd := exec.Command("kubectl", "get", "namespace", nsName,
				"-o", "jsonpath={.metadata.ownerReferences}")
			output, _ := utils.Run(cmd)
			Expect(output).To(BeEmpty(), "Namespace should not have ownerReferences")
		})
	})
})
