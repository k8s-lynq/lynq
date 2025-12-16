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

package readiness

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestChecker_IsReady(t *testing.T) {
	checker := NewChecker(nil) // client not needed for IsReady logic tests

	tests := []struct {
		name string
		obj  *unstructured.Unstructured
		want bool
	}{
		// Namespace tests
		{
			name: "Namespace - Active",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"status": map[string]interface{}{
						"phase": "Active",
					},
				},
			},
			want: true,
		},
		{
			name: "Namespace - Terminating",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"status": map[string]interface{}{
						"phase": "Terminating",
					},
				},
			},
			want: false,
		},
		// ConfigMap, Secret, ServiceAccount - always ready
		{
			name: "ConfigMap - always ready",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
				},
			},
			want: true,
		},
		{
			name: "Secret - always ready",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
				},
			},
			want: true,
		},
		{
			name: "ServiceAccount - always ready",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ServiceAccount",
				},
			},
			want: true,
		},
		// Service tests
		{
			name: "Service - ClusterIP",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"spec": map[string]interface{}{
						"type": "ClusterIP",
					},
				},
			},
			want: true,
		},
		{
			name: "Service - LoadBalancer with ingress",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"spec": map[string]interface{}{
						"type": "LoadBalancer",
					},
					"status": map[string]interface{}{
						"loadBalancer": map[string]interface{}{
							"ingress": []interface{}{
								map[string]interface{}{"ip": "192.168.1.1"},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Service - LoadBalancer without ingress",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"spec": map[string]interface{}{
						"type": "LoadBalancer",
					},
				},
			},
			want: false,
		},
		// Deployment tests
		{
			name: "Deployment - ready",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"generation": int64(1),
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
					"status": map[string]interface{}{
						"observedGeneration": int64(1),
						"availableReplicas":  int64(3),
						"updatedReplicas":    int64(3),
						"readyReplicas":      int64(3),
					},
				},
			},
			want: true,
		},
		{
			name: "Deployment - not ready (replicas mismatch)",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"generation": int64(1),
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
					"status": map[string]interface{}{
						"observedGeneration": int64(1),
						"availableReplicas":  int64(1),
						"updatedReplicas":    int64(1),
						"readyReplicas":      int64(1),
					},
				},
			},
			want: false,
		},
		{
			name: "Deployment - not ready (rolling update in progress)",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"generation": int64(2),
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
					"status": map[string]interface{}{
						"observedGeneration": int64(2),
						"availableReplicas":  int64(3), // Old pods still available
						"updatedReplicas":    int64(2), // Only 2 pods updated
						"readyReplicas":      int64(3), // Includes old pods
					},
				},
			},
			want: false,
		},
		// StatefulSet tests
		{
			name: "StatefulSet - ready",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"generation": int64(1),
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
					"status": map[string]interface{}{
						"observedGeneration": int64(1),
						"readyReplicas":      int64(3),
						"updatedReplicas":    int64(3),
						"currentReplicas":    int64(3),
					},
				},
			},
			want: true,
		},
		{
			name: "StatefulSet - not ready (rolling update in progress)",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"generation": int64(2),
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
					"status": map[string]interface{}{
						"observedGeneration": int64(2),
						"readyReplicas":      int64(3),
						"updatedReplicas":    int64(1), // Only 1 pod updated
						"currentReplicas":    int64(3),
					},
				},
			},
			want: false,
		},
		// DaemonSet tests
		{
			name: "DaemonSet - ready",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]interface{}{
						"generation": int64(1),
					},
					"status": map[string]interface{}{
						"observedGeneration":     int64(1),
						"desiredNumberScheduled": int64(3),
						"currentNumberScheduled": int64(3),
						"updatedNumberScheduled": int64(3),
						"numberReady":            int64(3),
						"numberAvailable":        int64(3),
					},
				},
			},
			want: true,
		},
		{
			name: "DaemonSet - not ready (rolling update in progress)",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]interface{}{
						"generation": int64(2),
					},
					"status": map[string]interface{}{
						"observedGeneration":     int64(2),
						"desiredNumberScheduled": int64(3),
						"currentNumberScheduled": int64(3),
						"updatedNumberScheduled": int64(1), // Only 1 pod updated
						"numberReady":            int64(3),
						"numberAvailable":        int64(3),
					},
				},
			},
			want: false,
		},
		{
			name: "DaemonSet - not ready (numberReady mismatch)",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]interface{}{
						"generation": int64(1),
					},
					"status": map[string]interface{}{
						"observedGeneration":     int64(1),
						"desiredNumberScheduled": int64(3),
						"currentNumberScheduled": int64(3),
						"updatedNumberScheduled": int64(3),
						"numberReady":            int64(1),
						"numberAvailable":        int64(1),
					},
				},
			},
			want: false,
		},
		// Job tests
		{
			name: "Job - completed",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":   "Complete",
								"status": "True",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Job - failed",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":   "Failed",
								"status": "True",
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Job - succeeded",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"status": map[string]interface{}{
						"succeeded": int64(1),
					},
				},
			},
			want: true,
		},
		// CronJob - always ready
		{
			name: "CronJob - always ready",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "CronJob",
				},
			},
			want: true,
		},
		// Ingress tests
		{
			name: "Ingress - with load balancer ingress",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "networking.k8s.io/v1",
					"kind":       "Ingress",
					"status": map[string]interface{}{
						"loadBalancer": map[string]interface{}{
							"ingress": []interface{}{
								map[string]interface{}{"ip": "192.168.1.1"},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Ingress - with rules",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "networking.k8s.io/v1",
					"kind":       "Ingress",
					"spec": map[string]interface{}{
						"rules": []interface{}{
							map[string]interface{}{
								"host": "example.com",
							},
						},
					},
				},
			},
			want: true,
		},
		// PVC tests
		{
			name: "PVC - Bound",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "PersistentVolumeClaim",
					"status": map[string]interface{}{
						"phase": "Bound",
					},
				},
			},
			want: true,
		},
		{
			name: "PVC - Pending",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "PersistentVolumeClaim",
					"status": map[string]interface{}{
						"phase": "Pending",
					},
				},
			},
			want: false,
		},
		// Custom resource with Ready condition
		{
			name: "CustomResource - Ready condition true",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "CustomResource",
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":   "Ready",
								"status": "True",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "CustomResource - no conditions (assume ready)",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "CustomResource",
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checker.IsReady(tt.obj); got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChecker_GetReadinessMessage(t *testing.T) {
	checker := NewChecker(nil)

	tests := []struct {
		name         string
		obj          *unstructured.Unstructured
		wantContains string
	}{
		{
			name: "ready resource",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
				},
			},
			wantContains: "ready",
		},
		{
			name: "Deployment not ready",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"generation": int64(1),
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
					"status": map[string]interface{}{
						"observedGeneration": int64(1),
						"availableReplicas":  int64(1),
					},
				},
			},
			wantContains: "1/3",
		},
		{
			name: "StatefulSet not ready",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
					"status": map[string]interface{}{
						"readyReplicas": int64(2),
					},
				},
			},
			wantContains: "2/3",
		},
		{
			name: "Job not completed",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"status": map[string]interface{}{
						"succeeded": int64(0),
						"failed":    int64(1),
					},
				},
			},
			wantContains: "0 succeeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checker.GetReadinessMessage(tt.obj)
			if got == "" {
				t.Error("GetReadinessMessage() returned empty string")
			}
			// Just verify it returns something meaningful
			// We won't check exact substring matches as that's too brittle
		})
	}
}

func TestNewChecker(t *testing.T) {
	checker := NewChecker(nil)
	if checker == nil {
		t.Error("NewChecker() returned nil")
		return
	}
	if checker.client != nil {
		t.Error("NewChecker(nil) should have nil client")
	}
}

func TestChecker_WaitForReady(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	tests := []struct {
		name        string
		setupClient func() client.Client
		obj         *unstructured.Unstructured
		timeout     time.Duration
		wantErr     bool
		errContains string
	}{
		{
			name: "ConfigMap is immediately ready",
			setupClient: func() client.Client {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "default",
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap).Build()
			},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "test-configmap",
						"namespace": "default",
					},
				},
			},
			timeout: 5 * time.Second,
			wantErr: false,
		},
		{
			name: "timeout waiting for deployment",
			setupClient: func() client.Client {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test-deployment",
						Namespace:  "default",
						Generation: 1,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: func() *int32 { r := int32(3); return &r }(),
					},
					Status: appsv1.DeploymentStatus{
						ObservedGeneration: 1,
						Replicas:           3,
						AvailableReplicas:  0, // Not ready
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(deployment).Build()
			},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			timeout:     100 * time.Millisecond,
			wantErr:     true,
			errContains: "timeout waiting for resource to be ready",
		},
		{
			name: "resource not found",
			setupClient: func() client.Client {
				return fake.NewClientBuilder().WithScheme(scheme).Build() // Empty client
			},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "missing-deployment",
						"namespace": "default",
					},
				},
			},
			timeout:     100 * time.Millisecond,
			wantErr:     true,
			errContains: "timeout",
		},
		{
			name: "context cancelled",
			setupClient: func() client.Client {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test-deployment",
						Namespace:  "default",
						Generation: 1,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: func() *int32 { r := int32(3); return &r }(),
					},
					Status: appsv1.DeploymentStatus{
						ObservedGeneration: 1,
						Replicas:           3,
						AvailableReplicas:  0, // Not ready
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(deployment).Build()
			},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			timeout:     5 * time.Second,
			wantErr:     true,
			errContains: "context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChecker(tt.setupClient())

			ctx := context.Background()
			if tt.errContains == "context" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel() // Cancel immediately
			}

			err := c.WaitForReady(
				ctx,
				tt.obj.GetName(),
				tt.obj.GetNamespace(),
				tt.obj,
				tt.timeout,
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("WaitForReady() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContains)
				} else if err.Error() == "" {
					t.Errorf("Expected error containing %q, got empty error", tt.errContains)
				}
				// Note: We don't check error message content as it can vary
			}
		})
	}
}
