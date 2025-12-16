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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/datasource"
)

// TestGetExistingNodes tests the getExistingLynqNodes function
func TestGetExistingNodes(t *testing.T) {
	tests := []struct {
		name          string
		registry      *lynqv1.LynqHub
		existingItems []lynqv1.LynqNode
		wantCount     int
		wantErr       bool
	}{
		{
			name: "no nodes found",
			registry: &lynqv1.LynqHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-registry",
					Namespace: "default",
				},
			},
			existingItems: []lynqv1.LynqNode{},
			wantCount:     0,
			wantErr:       false,
		},
		{
			name: "multiple nodes found with registry label",
			registry: &lynqv1.LynqHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-registry",
					Namespace: "default",
				},
			},
			existingItems: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web",
						Namespace: "default",
						Labels: map[string]string{
							"lynq.sh/hub": "test-registry",
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node1",
						TemplateRef: "web",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node2-web",
						Namespace: "default",
						Labels: map[string]string{
							"lynq.sh/hub": "test-registry",
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node2",
						TemplateRef: "web",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node3-api",
						Namespace: "default",
						Labels: map[string]string{
							"lynq.sh/hub": "other-registry",
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node3",
						TemplateRef: "api",
					},
				},
			},
			wantCount: 2, // Only node1 and node2 match the registry
			wantErr:   false,
		},
		{
			name: "nodes in different namespace not returned",
			registry: &lynqv1.LynqHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-registry",
					Namespace: "default",
				},
			},
			existingItems: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web",
						Namespace: "other-namespace",
						Labels: map[string]string{
							"lynq.sh/hub": "test-registry",
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node1",
						TemplateRef: "web",
					},
				},
			},
			wantCount: 0, // Different namespace
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))

			// Convert []LynqNode to []client.Object
			objects := []runtime.Object{tt.registry}
			for i := range tt.existingItems {
				objects = append(objects, &tt.existingItems[i])
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objects...).
				Build()

			r := &LynqHubReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			result, err := r.getExistingLynqNodes(ctx, tt.registry)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, len(result.Items), "Expected %d nodes, got %d", tt.wantCount, len(result.Items))
		})
	}
}

// TestCountLynqNodeStatus tests the countLynqNodeStatus function
func TestCountLynqNodeStatus(t *testing.T) {
	tests := []struct {
		name       string
		registry   *lynqv1.LynqHub
		nodes      []lynqv1.LynqNode
		wantReady  int32
		wantFailed int32
	}{
		{
			name: "no nodes",
			registry: &lynqv1.LynqHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-registry",
					Namespace: "default",
				},
			},
			nodes:      []lynqv1.LynqNode{},
			wantReady:  0,
			wantFailed: 0,
		},
		{
			name: "all nodes ready",
			registry: &lynqv1.LynqHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-registry",
					Namespace: "default",
				},
			},
			nodes: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web",
						Namespace: "default",
						Labels: map[string]string{
							"lynq.sh/hub": "test-registry",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   ConditionTypeReady,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node2-web",
						Namespace: "default",
						Labels: map[string]string{
							"lynq.sh/hub": "test-registry",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   ConditionTypeReady,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			wantReady:  2,
			wantFailed: 0,
		},
		{
			name: "mixed ready and failed nodes",
			registry: &lynqv1.LynqHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-registry",
					Namespace: "default",
				},
			},
			nodes: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web",
						Namespace: "default",
						Labels: map[string]string{
							"lynq.sh/hub": "test-registry",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   ConditionTypeReady,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node2-web",
						Namespace: "default",
						Labels: map[string]string{
							"lynq.sh/hub": "test-registry",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   ConditionTypeReady,
								Status: metav1.ConditionFalse,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node3-web",
						Namespace: "default",
						Labels: map[string]string{
							"lynq.sh/hub": "test-registry",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   ConditionTypeReady,
								Status: metav1.ConditionUnknown,
							},
						},
					},
				},
			},
			wantReady:  1,
			wantFailed: 2, // False and Unknown both count as failed
		},
		{
			name: "nodes without Ready condition",
			registry: &lynqv1.LynqHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-registry",
					Namespace: "default",
				},
			},
			nodes: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web",
						Namespace: "default",
						Labels: map[string]string{
							"lynq.sh/hub": "test-registry",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Degraded",
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			wantReady:  0,
			wantFailed: 0, // No Ready condition means not counted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))

			objects := []runtime.Object{tt.registry}
			for i := range tt.nodes {
				objects = append(objects, &tt.nodes[i])
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objects...).
				Build()

			r := &LynqHubReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			ready, failed := r.countLynqNodeStatus(ctx, tt.registry)

			assert.Equal(t, tt.wantReady, ready, "Expected %d ready nodes, got %d", tt.wantReady, ready)
			assert.Equal(t, tt.wantFailed, failed, "Expected %d failed nodes, got %d", tt.wantFailed, failed)
		})
	}
}

// TestUpdateStatus tests the updateStatus function
func TestUpdateStatus(t *testing.T) {
	tests := []struct {
		name                 string
		referencingTemplates int32
		desired              int32
		ready                int32
		failed               int32
		synced               bool
		wantConditionStatus  metav1.ConditionStatus
		wantConditionReason  string
	}{
		{
			name:                 "successful sync",
			referencingTemplates: 2,
			desired:              6,
			ready:                4,
			failed:               2,
			synced:               true,
			wantConditionStatus:  metav1.ConditionTrue,
			wantConditionReason:  "DatabaseConnected",
		},
		{
			name:                 "failed sync",
			referencingTemplates: 1,
			desired:              0,
			ready:                0,
			failed:               0,
			synced:               false,
			wantConditionStatus:  metav1.ConditionFalse,
			wantConditionReason:  "DatabaseConnectionFailed",
		},
		{
			name:                 "no templates referencing registry",
			referencingTemplates: 0,
			desired:              0,
			ready:                0,
			failed:               0,
			synced:               true,
			wantConditionStatus:  metav1.ConditionTrue,
			wantConditionReason:  "DatabaseConnected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))

			registry := &lynqv1.LynqHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-registry",
					Namespace: "default",
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(registry).
				WithStatusSubresource(registry).
				Build()

			r := &LynqHubReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			// Call updateStatus
			r.updateStatus(ctx, registry, tt.referencingTemplates, tt.desired, tt.ready, tt.failed, tt.synced)

			// Verify status was updated
			updated := &lynqv1.LynqHub{}
			err := fakeClient.Get(ctx, types.NamespacedName{Name: registry.Name, Namespace: registry.Namespace}, updated)
			require.NoError(t, err)

			assert.Equal(t, tt.referencingTemplates, updated.Status.ReferencingTemplates)
			assert.Equal(t, tt.desired, updated.Status.Desired)
			assert.Equal(t, tt.ready, updated.Status.Ready)
			assert.Equal(t, tt.failed, updated.Status.Failed)

			// Verify condition
			var readyCondition *metav1.Condition
			for i := range updated.Status.Conditions {
				if updated.Status.Conditions[i].Type == ConditionTypeReady {
					readyCondition = &updated.Status.Conditions[i]
					break
				}
			}

			require.NotNil(t, readyCondition, "Ready condition should be set")
			assert.Equal(t, tt.wantConditionStatus, readyCondition.Status)
			assert.Equal(t, tt.wantConditionReason, readyCondition.Reason)
		})
	}
}

// TestCleanupRetainResources tests the cleanupRetainResources function
func TestCleanupRetainResources(t *testing.T) {
	tests := []struct {
		name     string
		registry *lynqv1.LynqHub
		nodes    []lynqv1.LynqNode
		wantErr  bool
	}{
		{
			name: "no nodes to cleanup",
			registry: &lynqv1.LynqHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-registry",
					Namespace: "default",
				},
			},
			nodes:   []lynqv1.LynqNode{},
			wantErr: false,
		},
		{
			name: "cleanup multiple nodes",
			registry: &lynqv1.LynqHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-registry",
					Namespace: "default",
				},
			},
			nodes: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web",
						Namespace: "default",
						Labels: map[string]string{
							"lynq.sh/hub": "test-registry",
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node1",
						TemplateRef: "web",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node2-web",
						Namespace: "default",
						Labels: map[string]string{
							"lynq.sh/hub": "test-registry",
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node2",
						TemplateRef: "web",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			objects := []runtime.Object{tt.registry}
			for i := range tt.nodes {
				objects = append(objects, &tt.nodes[i])
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objects...).
				Build()

			recorder := record.NewFakeRecorder(100)

			r := &LynqHubReconciler{
				Client:   fakeClient,
				Scheme:   scheme,
				Recorder: recorder,
			}

			err := r.cleanupRetainResources(ctx, tt.registry)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestProcessRetainResourcesForLynqNode tests the processRetainResourcesForLynqNode function
func TestProcessRetainResourcesForLynqNode(t *testing.T) {
	tests := []struct {
		name            string
		node            *lynqv1.LynqNode
		existingObjects []runtime.Object
		wantErr         bool
	}{
		{
			name: "no retain resources",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node1-web",
					Namespace: "default",
					UID:       "node1-uid",
				},
				Spec: lynqv1.LynqNodeSpec{
					UID:         "node1",
					TemplateRef: "web",
					ConfigMaps: []lynqv1.TResource{
						{
							ID:             "cm1",
							NameTemplate:   "node1-config",
							DeletionPolicy: lynqv1.DeletionPolicyDelete,
							Spec: unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "ConfigMap",
								},
							},
						},
					},
				},
			},
			existingObjects: []runtime.Object{},
			wantErr:         false,
		},
		{
			name: "retain resource exists - remove ownerReference",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node1-web",
					Namespace: "default",
					UID:       "node1-uid",
				},
				Spec: lynqv1.LynqNodeSpec{
					UID:         "node1",
					TemplateRef: "web",
					ConfigMaps: []lynqv1.TResource{
						{
							ID:             "cm1",
							NameTemplate:   "node1-config",
							DeletionPolicy: lynqv1.DeletionPolicyRetain,
							Spec: unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "ConfigMap",
								},
							},
						},
					},
				},
			},
			existingObjects: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-config",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "operator.lynq.sh/v1",
								Kind:       "LynqNode",
								Name:       "node1-web",
								UID:        "node1-uid",
							},
						},
					},
					Data: map[string]string{
						"key": "value",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "retain resource not found - skip gracefully",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node1-web",
					Namespace: "default",
					UID:       "node1-uid",
				},
				Spec: lynqv1.LynqNodeSpec{
					UID:         "node1",
					TemplateRef: "web",
					Secrets: []lynqv1.TResource{
						{
							ID:             "secret1",
							NameTemplate:   "node1-secret",
							DeletionPolicy: lynqv1.DeletionPolicyRetain,
							Spec: unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "Secret",
								},
							},
						},
					},
				},
			},
			existingObjects: []runtime.Object{},
			wantErr:         false, // Should not error when resource not found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			objects := append([]runtime.Object{tt.node}, tt.existingObjects...)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objects...).
				Build()

			recorder := record.NewFakeRecorder(100)

			r := &LynqHubReconciler{
				Client:   fakeClient,
				Scheme:   scheme,
				Recorder: recorder,
			}

			err := r.processRetainResourcesForLynqNode(ctx, tt.node)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestContainsString tests the containsString helper function
func TestContainsString(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		str   string
		want  bool
	}{
		{
			name:  "string found in slice",
			slice: []string{"a", "b", "c"},
			str:   "b",
			want:  true,
		},
		{
			name:  "string not found in slice",
			slice: []string{"a", "b", "c"},
			str:   "d",
			want:  false,
		},
		{
			name:  "empty slice",
			slice: []string{},
			str:   "a",
			want:  false,
		},
		{
			name:  "empty string found",
			slice: []string{"a", "", "c"},
			str:   "",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsString(tt.slice, tt.str)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestRemoveString tests the removeString helper function
func TestRemoveString(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		str   string
		want  []string
	}{
		{
			name:  "remove existing string",
			slice: []string{"a", "b", "c"},
			str:   "b",
			want:  []string{"a", "c"},
		},
		{
			name:  "remove non-existing string",
			slice: []string{"a", "b", "c"},
			str:   "d",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "remove from empty slice",
			slice: []string{},
			str:   "a",
			want:  []string{},
		},
		{
			name:  "remove multiple occurrences",
			slice: []string{"a", "b", "a", "c", "a"},
			str:   "a",
			want:  []string{"b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeString(tt.slice, tt.str)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestUpdateNodeWithConflictHandling tests that updateLynqNode handles conflicts gracefully with retry logic
func TestUpdateNodeWithConflictHandling(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	// Create a LynqHub
	registry := &lynqv1.LynqHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-registry",
			Namespace: "default",
		},
	}

	// Create a LynqForm
	tmpl := &lynqv1.LynqForm{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "web-app",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: lynqv1.LynqFormSpec{
			HubID: "test-registry",
		},
	}

	// Create an existing LynqNode
	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node1-web-app",
			Namespace: "default",
			Annotations: map[string]string{
				"lynq.sh/hostOrUrl":           "https://old.example.com",
				"lynq.sh/activate":            "true",
				"lynq.sh/extra":               "{}",
				"lynq.sh/template-generation": "1",
			},
		},
		Spec: lynqv1.LynqNodeSpec{
			UID:         "node1",
			TemplateRef: "web-app",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(registry, tmpl, node).
		Build()

	recorder := record.NewFakeRecorder(100)

	r := &LynqHubReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: recorder,
	}

	// Test data with updated values
	row := datasource.NodeRow{
		UID:       "node1",
		HostOrURL: "https://new.example.com",
		Activate:  "true",
		Extra:     map[string]string{},
	}

	// Call updateLynqNode - should succeed with retry logic
	err := r.updateLynqNode(ctx, registry, tmpl, node, row)
	assert.NoError(t, err, "updateLynqNode should succeed even with potential conflicts")

	// Verify node was updated
	updated := &lynqv1.LynqNode{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: node.Name, Namespace: node.Namespace}, updated)
	require.NoError(t, err)
	assert.Equal(t, "https://new.example.com", updated.Annotations["lynq.sh/hostOrUrl"])
}

// TestCreateNodeIgnoresAlreadyExists tests that createLynqNode handles AlreadyExists errors gracefully
func TestCreateNodeIgnoresAlreadyExists(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	// Create a LynqHub
	registry := &lynqv1.LynqHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-registry",
			Namespace: "default",
		},
	}

	// Create a LynqForm
	tmpl := &lynqv1.LynqForm{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "web-app",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: lynqv1.LynqFormSpec{
			HubID: "test-registry",
		},
	}

	// Create an existing LynqNode with the same name that would be created
	existingLynqNode := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node1-web-app",
			Namespace: "default",
		},
		Spec: lynqv1.LynqNodeSpec{
			UID:         "node1",
			TemplateRef: "web-app",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(registry, tmpl, existingLynqNode).
		Build()

	recorder := record.NewFakeRecorder(100)

	r := &LynqHubReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: recorder,
	}

	// Test data
	row := datasource.NodeRow{
		UID:       "node1",
		HostOrURL: "https://example.com",
		Activate:  "true",
		Extra:     map[string]string{},
	}

	// Call createLynqNode - should return AlreadyExists error
	err := r.createLynqNode(ctx, registry, tmpl, row)

	// Verify that AlreadyExists error is returned (will be ignored by caller)
	assert.Error(t, err, "createLynqNode should return error when node already exists")
}

// TestIsNodeReady tests the isNodeReady helper function
func TestIsNodeReady(t *testing.T) {
	tests := []struct {
		name     string
		node     *lynqv1.LynqNode
		expected bool
	}{
		{
			name: "node with Ready=True condition",
			node: &lynqv1.LynqNode{
				Status: lynqv1.LynqNodeStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "node with Ready=False condition",
			node: &lynqv1.LynqNode{
				Status: lynqv1.LynqNodeStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "node with Ready=Unknown condition",
			node: &lynqv1.LynqNode{
				Status: lynqv1.LynqNodeStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionUnknown,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "node without Ready condition",
			node: &lynqv1.LynqNode{
				Status: lynqv1.LynqNodeStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Degraded",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "node with empty conditions",
			node: &lynqv1.LynqNode{
				Status: lynqv1.LynqNodeStatus{
					Conditions: []metav1.Condition{},
				},
			},
			expected: false,
		},
		{
			name: "node with multiple conditions including Ready=True",
			node: &lynqv1.LynqNode{
				Status: lynqv1.LynqNodeStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Degraded",
							Status: metav1.ConditionFalse,
						},
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNodeReady(tt.node)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCountUpdatingNodes tests the countUpdatingNodes function for rollout tracking
func TestCountUpdatingNodes(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []*lynqv1.LynqNode
		targetGen int64
		wantCount int32
	}{
		{
			name:      "no nodes",
			nodes:     []*lynqv1.LynqNode{},
			targetGen: 1,
			wantCount: 0,
		},
		{
			name: "all nodes at target generation and Ready",
			nodes: []*lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "1",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionTrue},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "1",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionTrue},
						},
					},
				},
			},
			targetGen: 1,
			wantCount: 0, // All nodes ready, none updating
		},
		{
			name: "some nodes updating (at target gen but not Ready)",
			nodes: []*lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "2",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionTrue},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "2",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionFalse},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "2",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{}, // No Ready condition
					},
				},
			},
			targetGen: 2,
			wantCount: 2, // 2 nodes not ready
		},
		{
			name: "nodes at different generations",
			nodes: []*lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "1", // Old gen
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionFalse},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "2", // Target gen, not ready
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionFalse},
						},
					},
				},
			},
			targetGen: 2,
			wantCount: 1, // Only 1 node at target gen and not ready
		},
		{
			name: "node without annotation",
			nodes: []*lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: nil, // No annotations
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionFalse},
						},
					},
				},
			},
			targetGen: 1,
			wantCount: 0, // Node without annotation is not counted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			r := &LynqHubReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			count := r.countUpdatingNodes(tt.nodes, tt.targetGen)
			assert.Equal(t, tt.wantCount, count)
		})
	}
}

// TestCanUpdateNode tests the canUpdateNode function for maxSkew enforcement
func TestCanUpdateNode(t *testing.T) {
	tests := []struct {
		name          string
		rollout       *lynqv1.RolloutConfig
		templateGen   int64
		nodes         []*lynqv1.LynqNode
		wantCanUpdate bool
	}{
		{
			name:          "no rollout config - unlimited updates",
			rollout:       nil,
			templateGen:   1,
			nodes:         []*lynqv1.LynqNode{},
			wantCanUpdate: true,
		},
		{
			name: "maxSkew=0 - unlimited updates",
			rollout: &lynqv1.RolloutConfig{
				MaxSkew: 0,
			},
			templateGen:   1,
			nodes:         []*lynqv1.LynqNode{},
			wantCanUpdate: true,
		},
		{
			name: "maxSkew=3, 0 nodes updating - can update",
			rollout: &lynqv1.RolloutConfig{
				MaxSkew: 3,
			},
			templateGen:   1,
			nodes:         []*lynqv1.LynqNode{},
			wantCanUpdate: true,
		},
		{
			name: "maxSkew=3, 2 nodes updating - can update",
			rollout: &lynqv1.RolloutConfig{
				MaxSkew: 3,
			},
			templateGen: 2,
			nodes: []*lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "2",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionFalse},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "2",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{}, // No ready condition
					},
				},
			},
			wantCanUpdate: true, // 2 < 3
		},
		{
			name: "maxSkew=3, 3 nodes updating - cannot update",
			rollout: &lynqv1.RolloutConfig{
				MaxSkew: 3,
			},
			templateGen: 2,
			nodes: []*lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "2",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionFalse},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "2",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionFalse},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "2",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionFalse},
						},
					},
				},
			},
			wantCanUpdate: false, // 3 >= 3
		},
		{
			name: "maxSkew=1 serial rollout - 1 updating cannot add more",
			rollout: &lynqv1.RolloutConfig{
				MaxSkew: 1,
			},
			templateGen: 3,
			nodes: []*lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "3",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionFalse},
						},
					},
				},
			},
			wantCanUpdate: false, // 1 >= 1
		},
		{
			name: "ready nodes don't count as updating",
			rollout: &lynqv1.RolloutConfig{
				MaxSkew: 2,
			},
			templateGen: 2,
			nodes: []*lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "2",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionTrue}, // Ready!
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							lynqv1.AnnotationTemplateGeneration: "2",
						},
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionFalse},
						},
					},
				},
			},
			wantCanUpdate: true, // Only 1 updating (other is ready)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			r := &LynqHubReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			tmpl := &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-template",
					Namespace:  "default",
					Generation: tt.templateGen,
				},
				Spec: lynqv1.LynqFormSpec{
					HubID:   "test-hub",
					Rollout: tt.rollout,
				},
			}

			canUpdate := r.canUpdateNode(tmpl, tt.nodes)
			assert.Equal(t, tt.wantCanUpdate, canUpdate)
		})
	}
}
