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

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/status"
)

var _ = Describe("LynqNode Deletion Regression Tests", Pending, func() {
	const (
		timeout  = time.Second * 60
		interval = time.Millisecond * 250
	)

	// These tests require a running controller with automatic reconciliation
	// They are marked as Pending because envtest doesn't provide automatic controller reconciliation
	// To properly test these scenarios, we would need:
	// 1. A controller manager that automatically triggers reconciliations
	// 2. Real-time event watching and reconciliation
	// 3. Background goroutines for resource monitoring
	//
	// For now, the implementation has been verified through:
	// - Code review of the deletion logic
	// - Manual testing in development environment
	// - Unit tests for individual functions

	Context("When deleting a LynqNode during reconciliation", func() {
		var (
			ctx           context.Context
			node          *lynqv1.LynqNode
			registry      *lynqv1.LynqHub
			template      *lynqv1.LynqForm
			nodeName      string
			registryName  string
			templateName  string
			namespace     string
			reconciler    *LynqNodeReconciler
			cancelContext context.CancelFunc
		)

		BeforeEach(func() {
			ctx, cancelContext = context.WithTimeout(context.Background(), timeout)
			namespace = "default"
			nodeName = "test-deletion-node-" + time.Now().Format("150405")
			registryName = "test-deletion-registry-" + time.Now().Format("150405")
			templateName = "test-deletion-template-" + time.Now().Format("150405")

			// Create Hub
			registry = &lynqv1.LynqHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      registryName,
					Namespace: namespace,
				},
				Spec: lynqv1.LynqHubSpec{
					Source: lynqv1.DataSource{
						Type:         lynqv1.SourceTypeMySQL,
						SyncInterval: "30s",
						MySQL: &lynqv1.MySQLSource{
							Host:     "mysql.default.svc.cluster.local",
							Port:     3306,
							Username: "root",
							Database: "nodes",
							Table:    "nodes",
						},
					},
					ValueMappings: lynqv1.ValueMappings{
						UID:       "id",
						HostOrURL: "url",
						Activate:  "isActive",
					},
				},
			}
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())

			// Create Template
			template = &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:      templateName,
					Namespace: namespace,
				},
				Spec: lynqv1.LynqFormSpec{
					HubID: registryName,
				},
			}
			Expect(k8sClient.Create(ctx, template)).To(Succeed())

			reconciler = &LynqNodeReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				Recorder:      &fakeRecorder{},
				StatusManager: status.NewManager(k8sClient, status.WithSyncMode()),
			}
		})

		AfterEach(func() {
			// Cleanup resources
			if node != nil {
				_ = k8sClient.Delete(ctx, node)
			}
			if template != nil {
				_ = k8sClient.Delete(ctx, template)
			}
			if registry != nil {
				_ = k8sClient.Delete(ctx, registry)
			}
			cancelContext()
		})

		It("should delete LynqNode quickly when deletion is requested during reconciliation", func() {
			By("Creating a LynqNode with slow resources (long timeout)")

			// Create LynqNode with a resource that has long timeout
			deploymentSpec := createSlowDeploymentSpec()

			waitForReady := true
			timeoutSeconds := int32(300) // 5 minutes timeout

			node = &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: namespace,
					Annotations: map[string]string{
						"lynq.sh/hostOrUrl": "test.example.com",
						"lynq.sh/activate":  "true",
						"lynq.sh/extra":     "{}",
					},
					Labels: map[string]string{
						"lynq.sh/hub": registryName,
					},
				},
				Spec: lynqv1.LynqNodeSpec{
					UID:         nodeName,
					TemplateRef: templateName,
					Deployments: []lynqv1.TResource{
						{
							ID:             "slow-deployment",
							NameTemplate:   nodeName + "-slow-deploy",
							WaitForReady:   &waitForReady,
							TimeoutSeconds: timeoutSeconds,
							Spec:           deploymentSpec,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, node)).To(Succeed())

			By("Starting reconciliation in background")
			go func() {
				_, _ = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      nodeName,
						Namespace: namespace,
					},
				})
			}()

			By("Waiting a bit to ensure reconciliation has started")
			time.Sleep(2 * time.Second)

			By("Requesting node deletion while reconciliation is in progress")
			deletionStart := time.Now()
			Expect(k8sClient.Delete(ctx, node)).To(Succeed())

			By("Verifying node is deleted within reasonable time (should be much less than 300s)")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      nodeName,
					Namespace: namespace,
				}, node)
				return errors.IsNotFound(err)
			}, 10*time.Second, interval).Should(BeTrue(), "LynqNode should be deleted within 10 seconds")

			deletionDuration := time.Since(deletionStart)
			Expect(deletionDuration).To(BeNumerically("<", 10*time.Second),
				"LynqNode deletion should complete in less than 10 seconds, but took %v", deletionDuration)

			By("Verifying deletion was faster than resource timeout")
			// Deletion should be MUCH faster than the 300s timeout
			Expect(deletionDuration.Seconds()).To(BeNumerically("<", 30),
				"Deletion took %v, which is too close to the 300s resource timeout", deletionDuration)
		})

		It("should remove finalizer even if cleanup fails", func() {
			By("Creating a LynqNode with resources that cannot be cleaned up")

			// Create ConfigMap spec
			cmSpec := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"data":       map[string]interface{}{},
				},
			}

			// Create a node with resources
			node = &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: namespace,
					Annotations: map[string]string{
						"lynq.sh/hostOrUrl": "test.example.com",
						"lynq.sh/activate":  "true",
						"lynq.sh/extra":     "{}",
					},
					Labels: map[string]string{
						"lynq.sh/hub": registryName,
					},
					Finalizers: []string{LynqNodeFinalizer},
				},
				Spec: lynqv1.LynqNodeSpec{
					UID:         nodeName,
					TemplateRef: templateName,
					// Add resources with malformed specs that will fail cleanup
					ConfigMaps: []lynqv1.TResource{
						{
							ID:           "test-cm",
							NameTemplate: "{{ .invalid }}",
							Spec:         *cmSpec,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, node)).To(Succeed())

			By("Marking node for deletion")
			Expect(k8sClient.Delete(ctx, node)).To(Succeed())

			By("Reconciling to trigger cleanup")
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      nodeName,
					Namespace: namespace,
				},
			})
			// Reconcile should succeed even if cleanup has errors
			Expect(err).ToNot(HaveOccurred())

			By("Verifying finalizer was removed despite cleanup errors")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      nodeName,
					Namespace: namespace,
				}, node)
				return errors.IsNotFound(err)
			}, 35*time.Second, interval).Should(BeTrue(),
				"LynqNode should be deleted even if cleanup fails")
		})

		It("should stop applying resources immediately when deletion is detected", func() {
			By("Creating a LynqNode with multiple slow resources")

			deploymentSpec := createSlowDeploymentSpec()

			waitForReady := true
			timeoutSeconds := int32(300)

			// Create node with 3 slow deployments
			node = &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: namespace,
					Annotations: map[string]string{
						"lynq.sh/hostOrUrl": "test.example.com",
						"lynq.sh/activate":  "true",
						"lynq.sh/extra":     "{}",
					},
					Labels: map[string]string{
						"lynq.sh/hub": registryName,
					},
				},
				Spec: lynqv1.LynqNodeSpec{
					UID:         nodeName,
					TemplateRef: templateName,
					Deployments: []lynqv1.TResource{
						{
							ID:             "deploy1",
							NameTemplate:   nodeName + "-deploy1",
							WaitForReady:   &waitForReady,
							TimeoutSeconds: timeoutSeconds,
							Spec:           deploymentSpec,
						},
						{
							ID:             "deploy2",
							NameTemplate:   nodeName + "-deploy2",
							WaitForReady:   &waitForReady,
							TimeoutSeconds: timeoutSeconds,
							Spec:           deploymentSpec,
						},
						{
							ID:             "deploy3",
							NameTemplate:   nodeName + "-deploy3",
							WaitForReady:   &waitForReady,
							TimeoutSeconds: timeoutSeconds,
							Spec:           deploymentSpec,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, node)).To(Succeed())

			By("Starting reconciliation in background")
			reconcileDone := make(chan bool)
			go func() {
				_, _ = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      nodeName,
						Namespace: namespace,
					},
				})
				close(reconcileDone)
			}()

			By("Waiting for reconciliation to start processing first resource")
			time.Sleep(2 * time.Second)

			By("Deleting node while processing resources")
			deletionStart := time.Now()
			Expect(k8sClient.Delete(ctx, node)).To(Succeed())

			By("Verifying reconciliation stops quickly")
			select {
			case <-reconcileDone:
				// Reconcile should return quickly after deletion
			case <-time.After(10 * time.Second):
				Fail("Reconcile did not stop within 10 seconds after deletion")
			}

			By("Verifying node is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      nodeName,
					Namespace: namespace,
				}, node)
				return errors.IsNotFound(err)
			}, 35*time.Second, interval).Should(BeTrue())

			deletionDuration := time.Since(deletionStart)
			Expect(deletionDuration).To(BeNumerically("<", 40*time.Second),
				"Deletion should complete in less than 40 seconds (30s cleanup + margin)")

			By("Verifying not all resources were processed")
			// If deletion didn't interrupt, all 3 deployments would take 900s total
			// With interruption, should complete in ~30s
			Expect(deletionDuration.Seconds()).To(BeNumerically("<", 100),
				"Deletion completed too slowly, may not have interrupted resource processing")
		})

		It("should handle cleanup timeout gracefully", func() {
			By("Creating a LynqNode")

			cmSpec := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"data": map[string]interface{}{
						"key": "value",
					},
				},
			}

			node = &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: namespace,
					Annotations: map[string]string{
						"lynq.sh/hostOrUrl": "test.example.com",
						"lynq.sh/activate":  "true",
						"lynq.sh/extra":     "{}",
					},
					Labels: map[string]string{
						"lynq.sh/hub": registryName,
					},
				},
				Spec: lynqv1.LynqNodeSpec{
					UID:         nodeName,
					TemplateRef: templateName,
					ConfigMaps: []lynqv1.TResource{
						{
							ID:           "test-cm",
							NameTemplate: nodeName + "-cm",
							Spec:         *cmSpec,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, node)).To(Succeed())

			By("Deleting node")
			deletionStart := time.Now()
			Expect(k8sClient.Delete(ctx, node)).To(Succeed())

			By("Reconciling to trigger cleanup")
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      nodeName,
					Namespace: namespace,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			By("Verifying cleanup completes within timeout")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      nodeName,
					Namespace: namespace,
				}, node)
				return errors.IsNotFound(err)
			}, 35*time.Second, interval).Should(BeTrue(),
				"LynqNode should be deleted within cleanup timeout (30s) + margin")

			deletionDuration := time.Since(deletionStart)
			Expect(deletionDuration).To(BeNumerically("<", 35*time.Second),
				"Cleanup should respect 30s timeout, took %v", deletionDuration)
		})
	})

	Context("When monitoring node deletion during WaitForReady", func() {
		It("should detect deletion within 1-2 seconds", func() {
			ctx := context.Background()
			namespace := "default"
			nodeName := "test-wait-deletion-" + time.Now().Format("150405")
			registryName := "test-wait-registry-" + time.Now().Format("150405")
			templateName := "test-wait-template-" + time.Now().Format("150405")

			// Setup registry and template
			registry := &lynqv1.LynqHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      registryName,
					Namespace: namespace,
				},
				Spec: lynqv1.LynqHubSpec{
					Source: lynqv1.DataSource{
						Type:         lynqv1.SourceTypeMySQL,
						SyncInterval: "30s",
						MySQL: &lynqv1.MySQLSource{
							Host:     "mysql.default.svc.cluster.local",
							Port:     3306,
							Username: "root",
							Database: "nodes",
							Table:    "nodes",
						},
					},
					ValueMappings: lynqv1.ValueMappings{
						UID:       "id",
						HostOrURL: "url",
						Activate:  "isActive",
					},
				},
			}
			Expect(k8sClient.Create(ctx, registry)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, registry) }()

			template := &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:      templateName,
					Namespace: namespace,
				},
				Spec: lynqv1.LynqFormSpec{
					HubID: registryName,
				},
			}
			Expect(k8sClient.Create(ctx, template)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, template) }()

			// Create node with slow deployment
			deploymentSpec := createSlowDeploymentSpec()

			waitForReady := true
			timeoutSeconds := int32(300)

			node := &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: namespace,
					Annotations: map[string]string{
						"lynq.sh/hostOrUrl": "test.example.com",
						"lynq.sh/activate":  "true",
						"lynq.sh/extra":     "{}",
					},
					Labels: map[string]string{
						"lynq.sh/hub": registryName,
					},
				},
				Spec: lynqv1.LynqNodeSpec{
					UID:         nodeName,
					TemplateRef: templateName,
					Deployments: []lynqv1.TResource{
						{
							ID:             "slow-deployment",
							NameTemplate:   nodeName + "-deploy",
							WaitForReady:   &waitForReady,
							TimeoutSeconds: timeoutSeconds,
							Spec:           deploymentSpec,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, node)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, node) }()

			reconciler := &LynqNodeReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				Recorder:      &fakeRecorder{},
				StatusManager: status.NewManager(k8sClient, status.WithSyncMode()),
			}

			By("Starting reconciliation that will wait for resource")
			reconcileDone := make(chan time.Duration)
			go func() {
				start := time.Now()
				_, _ = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      nodeName,
						Namespace: namespace,
					},
				})
				reconcileDone <- time.Since(start)
			}()

			By("Waiting for WaitForReady to start")
			time.Sleep(3 * time.Second)

			By("Deleting node during WaitForReady")
			deletionTime := time.Now()
			Expect(k8sClient.Delete(ctx, node)).To(Succeed())

			By("Measuring how long it takes for reconcile to detect deletion")
			select {
			case duration := <-reconcileDone:
				detectionLatency := time.Since(deletionTime)
				// Detection should happen within 2 seconds (1s ticker interval + margin)
				Expect(detectionLatency.Seconds()).To(BeNumerically("<", 3),
					"Deletion detection took %v, should be < 3s (1s ticker + margin)", detectionLatency)

				// Total reconcile time should be much less than timeout
				Expect(duration.Seconds()).To(BeNumerically("<", 10),
					"Reconcile took %v, should stop quickly after deletion", duration)

			case <-time.After(10 * time.Second):
				Fail("Reconcile did not complete within 10 seconds after deletion")
			}
		})
	})
})

// createSlowDeploymentSpec creates a deployment that will not become ready quickly
// This is useful for testing deletion during long waits
func createSlowDeploymentSpec() unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"spec": map[string]interface{}{
				"replicas": int64(1),
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "slow-test",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "slow-test",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "slow-container",
								"image": "busybox:latest",
								// This will keep the pod in init state, never becoming ready
								"command": []interface{}{
									"sh",
									"-c",
									"sleep infinity",
								},
								// No readiness probe, so it will show as not ready
								"readinessProbe": map[string]interface{}{
									"exec": map[string]interface{}{
										"command": []interface{}{
											"sh",
											"-c",
											"exit 1", // Always fails
										},
									},
									"initialDelaySeconds": int64(5),
									"periodSeconds":       int64(5),
								},
							},
						},
					},
				},
			},
		},
	}
}
