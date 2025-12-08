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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/status"
	"github.com/k8s-lynq/lynq/internal/template"
)

// TestReconcile_NodeNotFound tests that Reconcile handles missing LynqNode gracefully
func TestReconcile_NodeNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	recorder := record.NewFakeRecorder(100)

	r := &LynqNodeReconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		Recorder:      recorder,
		StatusManager: status.NewManager(fakeClient, status.WithSyncMode()),
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "nonexistent-node",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := r.Reconcile(ctx, req)

	// Should not return error for NotFound
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestReconcile_NodeWithFinalizer tests finalizer handling
func TestReconcile_NodeWithFinalizer(t *testing.T) {
	tests := []struct {
		name                 string
		node                 *lynqv1.LynqNode
		hasDeletionTimestamp bool
		hasFinalizer         bool
		wantRequeue          bool
	}{
		{
			name: "node without finalizer gets finalizer added",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "default",
					Annotations: map[string]string{
						"lynq.sh/hostOrUrl": "https://example.com",
						"lynq.sh/activate":  "true",
					},
				},
				Spec: lynqv1.LynqNodeSpec{
					UID:         "test-uid",
					TemplateRef: "test-template",
				},
			},
			hasDeletionTimestamp: false,
			hasFinalizer:         false,
			wantRequeue:          false,
		},
		{
			name: "node with finalizer but no deletion timestamp",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-node",
					Namespace:  "default",
					Finalizers: []string{LynqNodeFinalizer},
					Annotations: map[string]string{
						"lynq.sh/hostOrUrl": "https://example.com",
						"lynq.sh/activate":  "true",
					},
				},
				Spec: lynqv1.LynqNodeSpec{
					UID:         "test-uid",
					TemplateRef: "test-template",
				},
			},
			hasDeletionTimestamp: false,
			hasFinalizer:         true,
			wantRequeue:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.node).
				WithStatusSubresource(tt.node).
				Build()

			recorder := record.NewFakeRecorder(100)

			r := &LynqNodeReconciler{
				Client:        fakeClient,
				Scheme:        scheme,
				Recorder:      recorder,
				StatusManager: status.NewManager(fakeClient, status.WithSyncMode()),
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.node.Name,
					Namespace: tt.node.Namespace,
				},
			}

			ctx := context.Background()
			result, err := r.Reconcile(ctx, req)

			// Should complete without error
			assert.NoError(t, err)

			if tt.wantRequeue {
				assert.True(t, result.RequeueAfter > 0)
			}

			// Verify finalizer was added if it wasn't present
			updatedLynqNode := &lynqv1.LynqNode{}
			err = fakeClient.Get(ctx, req.NamespacedName, updatedLynqNode)
			require.NoError(t, err)

			if !tt.hasFinalizer && !tt.hasDeletionTimestamp {
				// Finalizer should have been added
				assert.Contains(t, updatedLynqNode.Finalizers, LynqNodeFinalizer)
			}
		})
	}
}

// TestRenderResource tests resource rendering
func TestRenderResource(t *testing.T) {
	tests := []struct {
		name      string
		resource  lynqv1.TResource
		node      *lynqv1.LynqNode
		wantErr   bool
		checkName string
		checkNS   string
	}{
		{
			name: "basic deployment resource",
			resource: lynqv1.TResource{
				ID:           "deploy-1",
				NameTemplate: "test-deployment",
				Spec: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"spec": map[string]interface{}{
							"replicas": int64(1),
						},
					},
				},
			},
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "default",
				},
				Spec: lynqv1.LynqNodeSpec{
					UID: "test-uid",
				},
			},
			wantErr:   false,
			checkName: "test-deployment",
			checkNS:   "default",
		},
		{
			name: "cross-namespace resource with tracking labels",
			resource: lynqv1.TResource{
				ID:              "svc-1",
				NameTemplate:    "test-service",
				TargetNamespace: "other-namespace",
				Spec: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Service",
					},
				},
			},
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "default",
				},
				Spec: lynqv1.LynqNodeSpec{
					UID: "test-uid",
				},
			},
			wantErr:   false,
			checkName: "test-service",
			checkNS:   "other-namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			r := &LynqNodeReconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
				Scheme: scheme,
			}

			ctx := context.Background()
			vars, err := r.buildTemplateVariablesFromAnnotations(tt.node)
			require.NoError(t, err)

			engine := template.NewEngine()
			obj, err := r.renderResource(ctx, engine, tt.resource, vars, tt.node)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, obj)
			assert.Equal(t, tt.checkName, obj.GetName())
			assert.Equal(t, tt.checkNS, obj.GetNamespace())

			// Check cross-namespace tracking labels
			if tt.resource.TargetNamespace != "" && tt.resource.TargetNamespace != tt.node.Namespace {
				labels := obj.GetLabels()
				assert.Equal(t, tt.node.Name, labels["lynq.sh/node"])
				assert.Equal(t, tt.node.Namespace, labels["lynq.sh/node-namespace"])
			}
		})
	}
}

// TestCleanupNodeResources tests resource cleanup with different deletion policies
func TestCleanupNodeResources(t *testing.T) {
	tests := []struct {
		name             string
		node             *lynqv1.LynqNode
		appliedResources []string
		wantErr          bool
	}{
		{
			name: "no resources to clean up",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "default",
				},
				Status: lynqv1.LynqNodeStatus{
					AppliedResources: []string{},
				},
			},
			wantErr: false,
		},
		{
			name: "clean up resources",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "default",
				},
				Status: lynqv1.LynqNodeStatus{
					AppliedResources: []string{
						"ConfigMap/default/test-cm@cm1",
						"Service/default/test-svc@svc1",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			// Create some resources that match the appliedResources
			var objects []client.Object
			for _, key := range tt.node.Status.AppliedResources {
				kind, namespace, name, _, err := parseResourceKey(key)
				require.NoError(t, err)

				switch kind {
				case resourceKindConfigMap:
					cm := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
					}
					objects = append(objects, cm)
				case "Service":
					svc := &corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
						Spec: corev1.ServiceSpec{
							Ports: []corev1.ServicePort{
								{Port: 80},
							},
						},
					}
					objects = append(objects, svc)
				}
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			recorder := record.NewFakeRecorder(100)

			r := &LynqNodeReconciler{
				Client:   fakeClient,
				Scheme:   scheme,
				Recorder: recorder,
			}

			ctx := context.Background()
			err := r.cleanupLynqNodeResources(ctx, tt.node)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
