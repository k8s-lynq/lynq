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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
)

// TestCheckLynqNodeStatuses tests the checkLynqNodeStatuses function
func TestCheckLynqNodeStatuses(t *testing.T) {
	tests := []struct {
		name           string
		template       *lynqv1.LynqForm
		nodes          []lynqv1.LynqNode
		wantTotalNodes int32
		wantReadyNodes int32
		wantErr        bool
	}{
		{
			name: "no nodes using template",
			template: &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "web-app",
					Namespace: "default",
				},
				Spec: lynqv1.LynqFormSpec{
					HubID: "test-registry",
				},
			},
			nodes:          []lynqv1.LynqNode{},
			wantTotalNodes: 0,
			wantReadyNodes: 0,
			wantErr:        false,
		},
		{
			name: "all nodes using template are ready",
			template: &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "web-app",
					Namespace: "default",
				},
				Spec: lynqv1.LynqFormSpec{
					HubID: "test-registry",
				},
			},
			nodes: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web-app",
						Namespace: "default",
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node1",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node2-web-app",
						Namespace: "default",
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node2",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			wantTotalNodes: 2,
			wantReadyNodes: 2,
			wantErr:        false,
		},
		{
			name: "mixed ready and not ready nodes",
			template: &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "web-app",
					Namespace: "default",
				},
				Spec: lynqv1.LynqFormSpec{
					HubID: "test-registry",
				},
			},
			nodes: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web-app",
						Namespace: "default",
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node1",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node2-web-app",
						Namespace: "default",
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node2",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionFalse,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node3-web-app",
						Namespace: "default",
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node3",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{},
					},
				},
			},
			wantTotalNodes: 3,
			wantReadyNodes: 1,
			wantErr:        false,
		},
		{
			name: "exclude nodes using different template",
			template: &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "web-app",
					Namespace: "default",
				},
				Spec: lynqv1.LynqFormSpec{
					HubID: "test-registry",
				},
			},
			nodes: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web-app",
						Namespace: "default",
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node1",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node2-worker",
						Namespace: "default",
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node2",
						TemplateRef: "worker", // Different template
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			wantTotalNodes: 1, // Only node1 uses web-app template
			wantReadyNodes: 1,
			wantErr:        false,
		},
		{
			name: "exclude nodes in different namespace",
			template: &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "web-app",
					Namespace: "default",
				},
				Spec: lynqv1.LynqFormSpec{
					HubID: "test-registry",
				},
			},
			nodes: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web-app",
						Namespace: "other-namespace",
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node1",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			wantTotalNodes: 0, // Different namespace
			wantReadyNodes: 0,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))

			objects := []runtime.Object{tt.template}
			for i := range tt.nodes {
				objects = append(objects, &tt.nodes[i])
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objects...).
				Build()

			r := &LynqFormReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			totalLynqNodes, readyLynqNodes, err := r.checkLynqNodeStatuses(ctx, tt.template)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantTotalNodes, totalLynqNodes, "Expected %d total nodes, got %d", tt.wantTotalNodes, totalLynqNodes)
			assert.Equal(t, tt.wantReadyNodes, readyLynqNodes, "Expected %d ready nodes, got %d", tt.wantReadyNodes, readyLynqNodes)
		})
	}
}

// TestFindTemplateForLynqNode tests the findTemplateForLynqNode mapping function
func TestFindTemplateForLynqNode(t *testing.T) {
	tests := []struct {
		name           string
		node           *lynqv1.LynqNode
		wantRequests   []reconcile.Request
		wantNumResults int
	}{
		{
			name: "node with template reference",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node1-web-app",
					Namespace: "default",
				},
				Spec: lynqv1.LynqNodeSpec{
					UID:         "node1",
					TemplateRef: "web-app",
				},
			},
			wantRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "web-app",
						Namespace: "default",
					},
				},
			},
			wantNumResults: 1,
		},
		{
			name: "node without template reference",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node1-orphan",
					Namespace: "default",
				},
				Spec: lynqv1.LynqNodeSpec{
					UID:         "node1",
					TemplateRef: "",
				},
			},
			wantRequests:   nil,
			wantNumResults: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			r := &LynqFormReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			results := r.findTemplateForLynqNode(ctx, tt.node)

			assert.Equal(t, tt.wantNumResults, len(results))
			if tt.wantNumResults > 0 {
				assert.Equal(t, tt.wantRequests, results)
			}
		})
	}
}

// TestFindRegistryForTemplate tests the findRegistryForTemplate mapping function
func TestFindRegistryForTemplate(t *testing.T) {
	tests := []struct {
		name           string
		template       *lynqv1.LynqForm
		wantRequests   []reconcile.Request
		wantNumResults int
	}{
		{
			name: "template with registry reference",
			template: &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "web-app",
					Namespace: "default",
				},
				Spec: lynqv1.LynqFormSpec{
					HubID: "test-registry",
				},
			},
			wantRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "test-registry",
						Namespace: "default",
					},
				},
			},
			wantNumResults: 1,
		},
		{
			name: "template with empty registry reference still triggers reconcile",
			template: &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "standalone-template",
					Namespace: "default",
				},
				Spec: lynqv1.LynqFormSpec{
					HubID: "", // Empty registry ID
				},
			},
			wantRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "", // Empty name will be passed to registry reconciler
						Namespace: "default",
					},
				},
			},
			// Note: Mapping function always returns a request even for empty hubID
			// The registry reconciler will handle the NotFound case appropriately
			wantNumResults: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			r := &LynqHubReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			results := r.findRegistryForTemplate(ctx, tt.template)

			assert.Equal(t, tt.wantNumResults, len(results))
			if tt.wantNumResults > 0 {
				assert.Equal(t, tt.wantRequests, results)
			}
		})
	}
}

// TestCalculateRolloutStats tests the calculateRolloutStats function for rollout status tracking
func TestCalculateRolloutStats(t *testing.T) {
	tests := []struct {
		name                  string
		templateName          string
		templateNamespace     string
		templateGeneration    int64
		nodes                 []lynqv1.LynqNode
		wantTotalNodes        int32
		wantReadyNodes        int32
		wantUpdatedNodes      int32
		wantUpdatingNodes     int32
		wantReadyUpdatedNodes int32
	}{
		{
			name:                  "no nodes",
			templateName:          "web-app",
			templateNamespace:     "default",
			templateGeneration:    1,
			nodes:                 []lynqv1.LynqNode{},
			wantTotalNodes:        0,
			wantReadyNodes:        0,
			wantUpdatedNodes:      0,
			wantUpdatingNodes:     0,
			wantReadyUpdatedNodes: 0,
		},
		{
			name:               "all nodes updated and ready",
			templateName:       "web-app",
			templateNamespace:  "default",
			templateGeneration: 2,
			nodes: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web-app",
						Namespace: "default",
						Annotations: map[string]string{
							"lynq.sh/template-generation": "2",
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node1",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionTrue},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node2-web-app",
						Namespace: "default",
						Annotations: map[string]string{
							"lynq.sh/template-generation": "2",
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node2",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionTrue},
						},
					},
				},
			},
			wantTotalNodes:        2,
			wantReadyNodes:        2,
			wantUpdatedNodes:      2,
			wantUpdatingNodes:     0,
			wantReadyUpdatedNodes: 2,
		},
		{
			name:               "mixed state - some updating, some ready",
			templateName:       "web-app",
			templateNamespace:  "default",
			templateGeneration: 3,
			nodes: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web-app",
						Namespace: "default",
						Annotations: map[string]string{
							"lynq.sh/template-generation": "3", // Updated to target, Ready
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node1",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionTrue},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node2-web-app",
						Namespace: "default",
						Annotations: map[string]string{
							"lynq.sh/template-generation": "3", // Updated to target, not Ready
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node2",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionFalse},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node3-web-app",
						Namespace: "default",
						Annotations: map[string]string{
							"lynq.sh/template-generation": "2", // Old generation, Ready
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node3",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionTrue},
						},
					},
				},
			},
			wantTotalNodes:        3,
			wantReadyNodes:        2, // node1 and node3 are Ready
			wantUpdatedNodes:      2, // node1 and node2 are at gen 3
			wantUpdatingNodes:     1, // node2 is at gen 3 but not Ready
			wantReadyUpdatedNodes: 1, // node1 is at gen 3 and Ready
		},
		{
			name:               "exclude nodes from different template",
			templateName:       "web-app",
			templateNamespace:  "default",
			templateGeneration: 1,
			nodes: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web-app",
						Namespace: "default",
						Annotations: map[string]string{
							"lynq.sh/template-generation": "1",
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node1",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionTrue},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-worker",
						Namespace: "default",
						Annotations: map[string]string{
							"lynq.sh/template-generation": "1",
						},
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node1",
						TemplateRef: "worker", // Different template
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionTrue},
						},
					},
				},
			},
			wantTotalNodes:        1, // Only node1-web-app counts
			wantReadyNodes:        1,
			wantUpdatedNodes:      1,
			wantUpdatingNodes:     0,
			wantReadyUpdatedNodes: 1,
		},
		{
			name:               "nodes without template generation annotation",
			templateName:       "web-app",
			templateNamespace:  "default",
			templateGeneration: 1,
			nodes: []lynqv1.LynqNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node1-web-app",
						Namespace: "default",
						// No annotations
					},
					Spec: lynqv1.LynqNodeSpec{
						UID:         "node1",
						TemplateRef: "web-app",
					},
					Status: lynqv1.LynqNodeStatus{
						Conditions: []metav1.Condition{
							{Type: "Ready", Status: metav1.ConditionTrue},
						},
					},
				},
			},
			wantTotalNodes:        1,
			wantReadyNodes:        1,
			wantUpdatedNodes:      0, // No annotation means not updated to target
			wantUpdatingNodes:     0,
			wantReadyUpdatedNodes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))

			// Create template
			tmpl := &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:       tt.templateName,
					Namespace:  tt.templateNamespace,
					Generation: tt.templateGeneration,
				},
				Spec: lynqv1.LynqFormSpec{
					HubID: "test-hub",
				},
			}

			objects := []runtime.Object{tmpl}
			for i := range tt.nodes {
				objects = append(objects, &tt.nodes[i])
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objects...).
				Build()

			r := &LynqFormReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			stats := r.calculateRolloutStats(ctx, tmpl)

			assert.Equal(t, tt.wantTotalNodes, stats.totalNodes, "totalNodes mismatch")
			assert.Equal(t, tt.wantReadyNodes, stats.readyNodes, "readyNodes mismatch")
			assert.Equal(t, tt.wantUpdatedNodes, stats.updatedNodes, "updatedNodes mismatch")
			assert.Equal(t, tt.wantUpdatingNodes, stats.updatingNodes, "updatingNodes mismatch")
			assert.Equal(t, tt.wantReadyUpdatedNodes, stats.readyUpdatedNodes, "readyUpdatedNodes mismatch")
		})
	}
}

// TestRolloutPhasesDetermination tests the phase determination logic in updateRolloutStatus
func TestRolloutPhasesDetermination(t *testing.T) {
	tests := []struct {
		name           string
		maxSkew        int32
		stats          rolloutStats
		wantPhase      lynqv1.RolloutPhase
		wantRolloutNil bool
	}{
		{
			name:    "no rollout config",
			maxSkew: 0,
			stats: rolloutStats{
				totalNodes:        5,
				readyNodes:        5,
				updatedNodes:      5,
				updatingNodes:     0,
				readyUpdatedNodes: 5,
			},
			wantPhase:      "",
			wantRolloutNil: true, // rollout should be nil when maxSkew is 0
		},
		{
			name:    "no nodes - Idle phase",
			maxSkew: 3,
			stats: rolloutStats{
				totalNodes:        0,
				readyNodes:        0,
				updatedNodes:      0,
				updatingNodes:     0,
				readyUpdatedNodes: 0,
			},
			wantPhase:      lynqv1.RolloutPhaseIdle,
			wantRolloutNil: false,
		},
		{
			name:    "all nodes updated and ready - Complete phase",
			maxSkew: 3,
			stats: rolloutStats{
				totalNodes:        5,
				readyNodes:        5,
				updatedNodes:      5,
				updatingNodes:     0,
				readyUpdatedNodes: 5,
			},
			wantPhase:      lynqv1.RolloutPhaseComplete,
			wantRolloutNil: false,
		},
		{
			name:    "no nodes updated yet - Idle phase",
			maxSkew: 3,
			stats: rolloutStats{
				totalNodes:        5,
				readyNodes:        5,
				updatedNodes:      0,
				updatingNodes:     0,
				readyUpdatedNodes: 0,
			},
			wantPhase:      lynqv1.RolloutPhaseIdle,
			wantRolloutNil: false,
		},
		{
			name:    "some nodes updating - InProgress phase",
			maxSkew: 3,
			stats: rolloutStats{
				totalNodes:        5,
				readyNodes:        3,
				updatedNodes:      3,
				updatingNodes:     1,
				readyUpdatedNodes: 2,
			},
			wantPhase:      lynqv1.RolloutPhaseInProgress,
			wantRolloutNil: false,
		},
		{
			name:    "partial update, none currently updating - InProgress",
			maxSkew: 3,
			stats: rolloutStats{
				totalNodes:        5,
				readyNodes:        2,
				updatedNodes:      2, // Some updated
				updatingNodes:     0,
				readyUpdatedNodes: 2,
			},
			wantPhase:      lynqv1.RolloutPhaseInProgress,
			wantRolloutNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-template",
					Namespace:  "default",
					Generation: 1,
				},
				Spec: lynqv1.LynqFormSpec{
					HubID: "test-hub",
				},
			}

			if tt.maxSkew > 0 {
				tmpl.Spec.Rollout = &lynqv1.RolloutConfig{
					MaxSkew: tt.maxSkew,
				}
			}

			scheme := runtime.NewScheme()
			require.NoError(t, lynqv1.AddToScheme(scheme))

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			r := &LynqFormReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			r.updateRolloutStatus(tmpl, tt.stats)

			if tt.wantRolloutNil {
				assert.Nil(t, tmpl.Status.Rollout)
			} else {
				require.NotNil(t, tmpl.Status.Rollout)
				assert.Equal(t, tt.wantPhase, tmpl.Status.Rollout.Phase)
				assert.Equal(t, tt.stats.totalNodes, tmpl.Status.Rollout.TotalNodes)
				assert.Equal(t, tt.stats.updatedNodes, tmpl.Status.Rollout.UpdatedNodes)
				assert.Equal(t, tt.stats.updatingNodes, tmpl.Status.Rollout.UpdatingNodes)
				assert.Equal(t, tt.stats.readyUpdatedNodes, tmpl.Status.Rollout.ReadyUpdatedNodes)
			}
		})
	}
}
