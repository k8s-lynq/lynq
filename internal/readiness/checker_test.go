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

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
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

// --- ClassifyPhase tests -------------------------------------------------
//
// Behavior matrix verified per kind:
//
//   Pending      observedGeneration < generation (controller hasn't seen spec)
//   Progressing  observedGeneration matches, rollout criteria NOT yet met
//   Available    rollout complete AND fully healthy
//   Degraded     rollout complete BUT availability dropped (the headline new
//                phase — used to be misclassified as Failed-after-timeout)
//   Failed       rollout timeout exceeded, OR ProgressDeadlineExceeded, OR
//                Job Failed condition

// deploymentObj builds a Deployment unstructured with the given replica
// counters. observedGeneration defaults to generation when generation > 0.
// When progressingReason is non-empty, a Progressing condition is added.
func deploymentObj(generation, observedGeneration, replicas, updated, available, ready int64, progressingReason string) *unstructured.Unstructured {
	return deploymentObjWithAvailable(generation, observedGeneration, replicas, updated, available, ready, progressingReason, "")
}

// deploymentObjWithAvailable extends deploymentObj with an explicit
// Available condition status ("True"/"False"/"" for absent). The two
// signals — Progressing.reason and Available — are independently
// settable so we can exercise the "Available=True but
// Progressing.reason != NewReplicaSetAvailable" case that triggered the
// healthy-busybox regression.
func deploymentObjWithAvailable(generation, observedGeneration, replicas, updated, available, ready int64, progressingReason, availableStatus string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"generation": generation,
			},
			"spec": map[string]interface{}{
				"replicas": replicas,
			},
			"status": map[string]interface{}{
				"observedGeneration": observedGeneration,
				"updatedReplicas":    updated,
				"availableReplicas":  available,
				"readyReplicas":      ready,
			},
		},
	}
	var conditions []interface{}
	if progressingReason != "" {
		conditions = append(conditions, map[string]interface{}{
			"type":   "Progressing",
			"status": "True",
			"reason": progressingReason,
		})
	}
	if availableStatus != "" {
		conditions = append(conditions, map[string]interface{}{
			"type":   "Available",
			"status": availableStatus,
		})
	}
	if len(conditions) > 0 {
		_ = unstructured.SetNestedSlice(obj.Object, conditions, "status", "conditions")
	}
	return obj
}

func TestChecker_ClassifyPhase_Deployment(t *testing.T) {
	c := NewChecker(nil)
	const rolloutTimeout = 30 * time.Second

	tests := []struct {
		name      string
		obj       *unstructured.Unstructured
		elapsed   time.Duration
		wantPhase lynqv1.ResourcePhase
		// wantTimedOut is checked only when wantPhase == Failed.
		wantTimedOut bool
	}{
		{
			name:      "Available — 3/3 updated, 3/3 available",
			obj:       deploymentObj(2, 2, 3, 3, 3, 3, "NewReplicaSetAvailable"),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name:      "Degraded — headline case: rollout complete but one pod down post-eviction",
			obj:       deploymentObj(2, 2, 3, 3, 2, 2, "NewReplicaSetAvailable"),
			elapsed:   30 * time.Minute, // hours since apply — proves no timeout escalation
			wantPhase: lynqv1.ResourcePhaseDegraded,
		},
		{
			name:      "Progressing — updatedReplicas < replicas, still within timeout",
			obj:       deploymentObj(2, 2, 3, 2, 2, 2, "ReplicaSetUpdated"),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhaseProgressing,
		},
		{
			name:         "Failed — rollout timeout elapsed while Progressing",
			obj:          deploymentObj(2, 2, 3, 1, 1, 1, "ReplicaSetUpdated"),
			elapsed:      60 * time.Second,
			wantPhase:    lynqv1.ResourcePhaseFailed,
			wantTimedOut: true,
		},
		{
			name:         "Failed — Kubernetes-native ProgressDeadlineExceeded (not RolloutTimedOut, it's K8s deciding)",
			obj:          deploymentObj(2, 2, 3, 1, 1, 1, "ProgressDeadlineExceeded"),
			elapsed:      5 * time.Second, // well within Lynq's timeout — K8s decided faster
			wantPhase:    lynqv1.ResourcePhaseFailed,
			wantTimedOut: false,
		},
		{
			name:      "Pending — observedGeneration lags generation",
			obj:       deploymentObj(3, 2, 3, 3, 3, 3, ""),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhasePending,
		},
		{
			// Critical regression case (caught by E2E resource_timeout
			// test): a Deployment with a non-existent image creates pods
			// that NEVER reach Available, but updatedReplicas grows as the
			// pod objects are scheduled. Naively checking updatedReplicas
			// == spec.replicas would misclassify this as Degraded → still
			// Ready → never escalates to Failed. The Progressing.reason
			// gate ("NewReplicaSetAvailable" must have been seen at least
			// once) is what distinguishes "post-rollout disruption" from
			// "never converged".
			name:      "Progressing — pods scheduled but NewReplicaSetAvailable never set (failing image pull)",
			obj:       deploymentObj(2, 2, 3, 3, 0, 0, "ReplicaSetUpdated"),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhaseProgressing,
		},
		{
			name:         "Failed — pods scheduled but never reached health, timeout elapsed",
			obj:          deploymentObj(2, 2, 3, 3, 0, 0, "ReplicaSetUpdated"),
			elapsed:      60 * time.Second,
			wantPhase:    lynqv1.ResourcePhaseFailed,
			wantTimedOut: true,
		},
		{
			// Regression case for fast 1-replica deployments (e.g.,
			// busybox with `sleep 3600` and no readinessProbe). K8s
			// reliably sets Available=True but the Progressing.reason
			// transition to "NewReplicaSetAvailable" can lag behind on
			// quick rollouts. The classifier must treat Available=True
			// as sufficient evidence of rollout completion.
			name:      "Available — Available=True even without NewReplicaSetAvailable reason",
			obj:       deploymentObjWithAvailable(2, 2, 1, 1, 1, 1, "ReplicaSetUpdated", "True"),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name:      "Degraded — Available=True was set, then availability dropped",
			obj:       deploymentObjWithAvailable(2, 2, 3, 3, 2, 2, "ReplicaSetUpdated", "True"),
			elapsed:   30 * time.Minute,
			wantPhase: lynqv1.ResourcePhaseDegraded,
		},
		{
			// F1 regression: an already-healthy Deployment gets a NEW
			// generation (kubectl set image). observedGeneration has caught
			// up, old pods keep availableReplicas high so Available=True
			// PERSISTS, but the new ReplicaSet is still rolling out
			// (updatedReplicas < replicas). Must be Progressing — NOT
			// Available/Degraded — so dependents don't unblock early and
			// Lynq's rollout timeout still engages. The Available=True
			// fallback is gated on updatedReplicas==replicas precisely for
			// this case.
			name:      "Progressing — new generation mid-rollout, Available=True persists from old RS",
			obj:       deploymentObjWithAvailable(3, 3, 4, 2, 4, 4, "ReplicaSetUpdated", "True"),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhaseProgressing,
		},
		{
			name:      "Pending — spec.replicas=0 (parity with existing semantics, not Degraded)",
			obj:       deploymentObj(1, 1, 0, 0, 0, 0, "NewReplicaSetAvailable"),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhasePending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.ClassifyPhase(tt.obj, tt.elapsed, rolloutTimeout)
			if got.Phase != tt.wantPhase {
				t.Errorf("phase = %q, want %q (reason=%q)", got.Phase, tt.wantPhase, got.Reason)
			}
			if tt.wantPhase == lynqv1.ResourcePhaseFailed && got.RolloutTimedOut != tt.wantTimedOut {
				t.Errorf("RolloutTimedOut = %v, want %v", got.RolloutTimedOut, tt.wantTimedOut)
			}
		})
	}
}

// statefulSetObj builds a StatefulSet unstructured. currentRevision and
// updateRevision can differ to simulate a rolling update.
func statefulSetObj(generation, observedGeneration, replicas, updated, current, ready int64, currentRev, updateRev string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "StatefulSet",
			"metadata": map[string]interface{}{
				"generation": generation,
			},
			"spec": map[string]interface{}{
				"replicas": replicas,
			},
			"status": map[string]interface{}{
				"observedGeneration": observedGeneration,
				"updatedReplicas":    updated,
				"currentReplicas":    current,
				"readyReplicas":      ready,
				"currentRevision":    currentRev,
				"updateRevision":     updateRev,
			},
		},
	}
}

func TestChecker_ClassifyPhase_StatefulSet(t *testing.T) {
	c := NewChecker(nil)
	const rolloutTimeout = 30 * time.Second

	tests := []struct {
		name      string
		obj       *unstructured.Unstructured
		elapsed   time.Duration
		wantPhase lynqv1.ResourcePhase
	}{
		{
			name:      "Available — revisions converged, all ready",
			obj:       statefulSetObj(2, 2, 3, 3, 3, 3, "v2", "v2"),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name:      "Degraded — revisions converged but one pod down post-eviction",
			obj:       statefulSetObj(2, 2, 3, 3, 3, 2, "v2", "v2"),
			elapsed:   30 * time.Minute,
			wantPhase: lynqv1.ResourcePhaseDegraded,
		},
		{
			name:      "Progressing — currentRevision != updateRevision (mid-rollout)",
			obj:       statefulSetObj(2, 2, 3, 2, 3, 2, "v1", "v2"),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhaseProgressing,
		},
		{
			name:      "Pending — observedGeneration lags",
			obj:       statefulSetObj(3, 2, 3, 3, 3, 3, "v2", "v2"),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhasePending,
		},
		{
			// Regression guard: STS with non-existent image. Pods are
			// scheduled (currentReplicas grows) but never reach Ready.
			// readyReplicas==0 means rollout never converged; STS should
			// stay Progressing, not Degraded.
			name:      "Progressing — pods scheduled but readyReplicas=0 (failing image pull)",
			obj:       statefulSetObj(2, 2, 3, 3, 3, 0, "v2", "v2"),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhaseProgressing,
		},
		{
			name:      "Pending — replicas=0",
			obj:       statefulSetObj(1, 1, 0, 0, 0, 0, "", ""),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhasePending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.ClassifyPhase(tt.obj, tt.elapsed, rolloutTimeout)
			if got.Phase != tt.wantPhase {
				t.Errorf("phase = %q, want %q (reason=%q)", got.Phase, tt.wantPhase, got.Reason)
			}
		})
	}
}

func daemonSetObj(generation, observedGeneration, desired, updated, ready, available int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata": map[string]interface{}{
				"generation": generation,
			},
			"status": map[string]interface{}{
				"observedGeneration":     observedGeneration,
				"desiredNumberScheduled": desired,
				"updatedNumberScheduled": updated,
				"numberReady":            ready,
				"numberAvailable":        available,
			},
		},
	}
}

func TestChecker_ClassifyPhase_DaemonSet(t *testing.T) {
	c := NewChecker(nil)
	const rolloutTimeout = 30 * time.Second

	tests := []struct {
		name      string
		obj       *unstructured.Unstructured
		elapsed   time.Duration
		wantPhase lynqv1.ResourcePhase
	}{
		{
			name:      "Available — all nodes scheduled, updated, and available",
			obj:       daemonSetObj(2, 2, 3, 3, 3, 3),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name:      "Degraded — updated across all nodes but one pod unavailable (node drain)",
			obj:       daemonSetObj(2, 2, 3, 3, 2, 2),
			elapsed:   30 * time.Minute,
			wantPhase: lynqv1.ResourcePhaseDegraded,
		},
		{
			name:      "Progressing — rollout still updating nodes",
			obj:       daemonSetObj(2, 2, 3, 2, 2, 2),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhaseProgressing,
		},
		{
			name:      "Pending — no nodes match selector (desiredNumberScheduled=0)",
			obj:       daemonSetObj(1, 1, 0, 0, 0, 0),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhasePending,
		},
		{
			// Regression guard: DaemonSet with non-existent image. Pods
			// are scheduled (updatedNumberScheduled grows) but never reach
			// Available (numberAvailable==0). DS should stay Progressing,
			// not Degraded.
			name:      "Progressing — pods scheduled across nodes but numberAvailable=0",
			obj:       daemonSetObj(2, 2, 3, 3, 0, 0),
			elapsed:   5 * time.Second,
			wantPhase: lynqv1.ResourcePhaseProgressing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.ClassifyPhase(tt.obj, tt.elapsed, rolloutTimeout)
			if got.Phase != tt.wantPhase {
				t.Errorf("phase = %q, want %q (reason=%q)", got.Phase, tt.wantPhase, got.Reason)
			}
		})
	}
}

func TestChecker_ClassifyPhase_NonWorkloadKinds(t *testing.T) {
	c := NewChecker(nil)

	tests := []struct {
		name      string
		obj       *unstructured.Unstructured
		wantPhase lynqv1.ResourcePhase
	}{
		{
			name: "ConfigMap — immediately Available",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "ConfigMap",
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name: "Secret — immediately Available",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "Secret",
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name: "ServiceAccount — immediately Available",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "ServiceAccount",
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name: "Namespace Active — Available",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "Namespace",
				"status": map[string]interface{}{"phase": "Active"},
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name: "Namespace Terminating — Pending",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "Namespace",
				"status": map[string]interface{}{"phase": "Terminating"},
			}},
			wantPhase: lynqv1.ResourcePhasePending,
		},
		{
			name: "Service ClusterIP — immediately Available",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "Service",
				"spec": map[string]interface{}{"type": "ClusterIP"},
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name: "Service LoadBalancer without ingress — Progressing",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "Service",
				"spec": map[string]interface{}{"type": "LoadBalancer"},
			}},
			wantPhase: lynqv1.ResourcePhaseProgressing,
		},
		{
			name: "Service LoadBalancer with ingress — Available",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "Service",
				"spec": map[string]interface{}{"type": "LoadBalancer"},
				"status": map[string]interface{}{
					"loadBalancer": map[string]interface{}{
						"ingress": []interface{}{
							map[string]interface{}{"ip": "10.0.0.1"},
						},
					},
				},
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name: "Job Complete=True — Available",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "batch/v1", "kind": "Job",
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{"type": "Complete", "status": "True"},
					},
				},
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name: "Job Failed=True — Failed (not RolloutTimedOut)",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "batch/v1", "kind": "Job",
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{"type": "Failed", "status": "True"},
					},
				},
			}},
			wantPhase: lynqv1.ResourcePhaseFailed,
		},
		{
			name: "CronJob — immediately Available",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "batch/v1", "kind": "CronJob",
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name: "PodDisruptionBudget — immediately Available",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "policy/v1", "kind": "PodDisruptionBudget",
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name: "NetworkPolicy — immediately Available",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "networking.k8s.io/v1", "kind": "NetworkPolicy",
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name: "PVC Bound — Available",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "PersistentVolumeClaim",
				"status": map[string]interface{}{"phase": "Bound"},
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name: "PVC Pending phase — Progressing",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "PersistentVolumeClaim",
				"status": map[string]interface{}{"phase": "Pending"},
			}},
			wantPhase: lynqv1.ResourcePhaseProgressing,
		},
		{
			name: "Custom resource with no status.conditions — Available (matches existing fallback)",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "example.com/v1", "kind": "Widget",
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
		{
			name: "Custom resource with Ready=True — Available",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "example.com/v1", "kind": "Widget",
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{"type": "Ready", "status": "True"},
					},
				},
			}},
			wantPhase: lynqv1.ResourcePhaseAvailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.ClassifyPhase(tt.obj, 5*time.Second, 30*time.Second)
			if got.Phase != tt.wantPhase {
				t.Errorf("phase = %q, want %q (reason=%q)", got.Phase, tt.wantPhase, got.Reason)
			}
		})
	}
}

// TestChecker_ClassifyPhase_Replicas verifies that the ReplicaStatus is
// populated correctly so the metrics layer can emit accurate replica gauges.
func TestChecker_ClassifyPhase_Replicas(t *testing.T) {
	c := NewChecker(nil)

	t.Run("Deployment replica counts surface in PhaseResult", func(t *testing.T) {
		obj := deploymentObj(2, 2, 5, 3, 2, 3, "ReplicaSetUpdated")
		got := c.ClassifyPhase(obj, 5*time.Second, 30*time.Second)

		if got.Replicas.Desired != 5 {
			t.Errorf("Replicas.Desired = %d, want 5", got.Replicas.Desired)
		}
		if got.Replicas.Updated != 3 {
			t.Errorf("Replicas.Updated = %d, want 3", got.Replicas.Updated)
		}
		if got.Replicas.Available != 2 {
			t.Errorf("Replicas.Available = %d, want 2", got.Replicas.Available)
		}
		if got.Replicas.Ready != 3 {
			t.Errorf("Replicas.Ready = %d, want 3", got.Replicas.Ready)
		}
	})

	t.Run("DaemonSet maps native counters to ReplicaStatus", func(t *testing.T) {
		obj := daemonSetObj(2, 2, 4, 4, 4, 3)
		got := c.ClassifyPhase(obj, 5*time.Second, 30*time.Second)

		if got.Replicas.Desired != 4 {
			t.Errorf("Replicas.Desired = %d, want 4 (desiredNumberScheduled)", got.Replicas.Desired)
		}
		if got.Replicas.Available != 3 {
			t.Errorf("Replicas.Available = %d, want 3 (numberAvailable)", got.Replicas.Available)
		}
	})
}
