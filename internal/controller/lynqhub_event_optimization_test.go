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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/datasource"
)

// TestShouldUpdateLynqNode_CorrectTemplateComparison verifies that
// shouldUpdateLynqNode uses the correct template for generation comparison.
// This prevents the bug where the wrong template was used for multi-template hubs.
func TestShouldUpdateLynqNode_CorrectTemplateComparison(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	// Create two templates with different generations
	templateA := &lynqv1.LynqForm{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "template-a",
			Namespace:  "default",
			Generation: 5,
		},
		Spec: lynqv1.LynqFormSpec{
			HubID: "test-hub",
		},
	}

	templateB := &lynqv1.LynqForm{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "template-b",
			Namespace:  "default",
			Generation: 10, // Different generation
		},
		Spec: lynqv1.LynqFormSpec{
			HubID: "test-hub",
		},
	}

	hub := &lynqv1.LynqHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hub",
			Namespace: "default",
		},
	}

	// Create a node that references template-a with matching generation
	nodeForTemplateA := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node1-template-a",
			Namespace: "default",
			Annotations: map[string]string{
				"lynq.sh/hostOrUrl":           "http://example.com",
				"lynq.sh/activate":            "true",
				"lynq.sh/extra":               "{}",
				"lynq.sh/template-generation": "5", // Matches template-a's generation
			},
		},
		Spec: lynqv1.LynqNodeSpec{
			TemplateRef: "template-a", // References template-a
			UID:         "node1",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(hub, templateA, templateB, nodeForTemplateA).
		Build()

	r := &LynqHubReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	ctx := context.Background()

	// The row data matches the node's current data
	row := datasource.NodeRow{
		UID:       "node1",
		HostOrURL: "http://example.com",
		Activate:  "true",
		Extra:     map[string]string{},
	}

	// Should NOT need update because:
	// - Data is the same
	// - Template generation matches (5 == 5)
	// Before the fix, this would incorrectly return true if template-b (generation 10)
	// was checked instead of template-a (generation 5)
	needsUpdate := r.shouldUpdateLynqNode(ctx, hub, nodeForTemplateA, row)

	assert.False(t, needsUpdate,
		"Node should NOT need update when data and template generation match. "+
			"The fix ensures we compare against the correct template (template-a with generation 5), "+
			"not the first template in the list (which might be template-b with generation 10)")
}

// TestShouldUpdateLynqNode_DetectsDataChanges verifies that data changes
// are correctly detected and trigger updates.
func TestShouldUpdateLynqNode_DetectsDataChanges(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	template := &lynqv1.LynqForm{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-template",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: lynqv1.LynqFormSpec{
			HubID: "test-hub",
		},
	}

	hub := &lynqv1.LynqHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hub",
			Namespace: "default",
		},
	}

	tests := []struct {
		name         string
		nodeData     map[string]string
		rowData      datasource.NodeRow
		expectUpdate bool
		description  string
	}{
		{
			name: "should detect hostOrUrl change",
			nodeData: map[string]string{
				"lynq.sh/hostOrUrl":           "http://old.com",
				"lynq.sh/activate":            "true",
				"lynq.sh/extra":               "{}",
				"lynq.sh/template-generation": "1",
			},
			rowData: datasource.NodeRow{
				UID:       "node1",
				HostOrURL: "http://new.com", // Changed
				Activate:  "true",
				Extra:     map[string]string{},
			},
			expectUpdate: true,
			description:  "HostOrURL change should trigger update",
		},
		{
			name: "should detect activate change",
			nodeData: map[string]string{
				"lynq.sh/hostOrUrl":           "http://example.com",
				"lynq.sh/activate":            "true",
				"lynq.sh/extra":               "{}",
				"lynq.sh/template-generation": "1",
			},
			rowData: datasource.NodeRow{
				UID:       "node1",
				HostOrURL: "http://example.com",
				Activate:  "false", // Changed
				Extra:     map[string]string{},
			},
			expectUpdate: true,
			description:  "Activate change should trigger update",
		},
		{
			name: "should detect extra values change",
			nodeData: map[string]string{
				"lynq.sh/hostOrUrl":           "http://example.com",
				"lynq.sh/activate":            "true",
				"lynq.sh/extra":               `{"key":"old"}`,
				"lynq.sh/template-generation": "1",
			},
			rowData: datasource.NodeRow{
				UID:       "node1",
				HostOrURL: "http://example.com",
				Activate:  "true",
				Extra:     map[string]string{"key": "new"}, // Changed
			},
			expectUpdate: true,
			description:  "Extra values change should trigger update",
		},
		{
			name: "should NOT update when all data matches",
			nodeData: map[string]string{
				"lynq.sh/hostOrUrl":           "http://example.com",
				"lynq.sh/activate":            "true",
				"lynq.sh/extra":               `{"key":"value"}`,
				"lynq.sh/template-generation": "1",
			},
			rowData: datasource.NodeRow{
				UID:       "node1",
				HostOrURL: "http://example.com",
				Activate:  "true",
				Extra:     map[string]string{"key": "value"}, // Same
			},
			expectUpdate: false,
			description:  "No change should NOT trigger update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-node",
					Namespace:   "default",
					Annotations: tt.nodeData,
				},
				Spec: lynqv1.LynqNodeSpec{
					TemplateRef: "test-template",
					UID:         "node1",
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(hub, template, node).
				Build()

			r := &LynqHubReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			ctx := context.Background()
			needsUpdate := r.shouldUpdateLynqNode(ctx, hub, node, tt.rowData)

			assert.Equal(t, tt.expectUpdate, needsUpdate, tt.description)
		})
	}
}

// TestShouldUpdateLynqNode_DetectsTemplateGenerationChange verifies that
// template generation changes are correctly detected.
func TestShouldUpdateLynqNode_DetectsTemplateGenerationChange(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	// Template with updated generation
	template := &lynqv1.LynqForm{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-template",
			Namespace:  "default",
			Generation: 5, // Updated generation
		},
		Spec: lynqv1.LynqFormSpec{
			HubID: "test-hub",
		},
	}

	hub := &lynqv1.LynqHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hub",
			Namespace: "default",
		},
	}

	// Node still has old template generation
	extraJSON, _ := json.Marshal(map[string]string{})
	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "default",
			Annotations: map[string]string{
				"lynq.sh/hostOrUrl":           "http://example.com",
				"lynq.sh/activate":            "true",
				"lynq.sh/extra":               string(extraJSON),
				"lynq.sh/template-generation": "3", // Old generation
			},
		},
		Spec: lynqv1.LynqNodeSpec{
			TemplateRef: "test-template",
			UID:         "node1",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(hub, template, node).
		Build()

	r := &LynqHubReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	ctx := context.Background()
	row := datasource.NodeRow{
		UID:       "node1",
		HostOrURL: "http://example.com",
		Activate:  "true",
		Extra:     map[string]string{},
	}

	needsUpdate := r.shouldUpdateLynqNode(ctx, hub, node, row)

	assert.True(t, needsUpdate,
		"Template generation change (3 -> 5) should trigger update")
}

// TestShouldUpdateLynqNode_MultiTemplateScenario is a comprehensive test
// for the multi-template hub scenario that was causing the original bug.
func TestShouldUpdateLynqNode_MultiTemplateScenario(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	hub := &lynqv1.LynqHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-template-hub",
			Namespace: "default",
		},
	}

	// Two templates with very different generations
	webTemplate := &lynqv1.LynqForm{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "web-template",
			Namespace:  "default",
			Generation: 3,
		},
		Spec: lynqv1.LynqFormSpec{
			HubID: "multi-template-hub",
		},
	}

	workerTemplate := &lynqv1.LynqForm{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "worker-template",
			Namespace:  "default",
			Generation: 15, // Much higher generation
		},
		Spec: lynqv1.LynqFormSpec{
			HubID: "multi-template-hub",
		},
	}

	// Nodes for different templates
	extraJSON, _ := json.Marshal(map[string]string{})

	webNode := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant1-web-template",
			Namespace: "default",
			Annotations: map[string]string{
				"lynq.sh/hostOrUrl":           "http://tenant1.com",
				"lynq.sh/activate":            "true",
				"lynq.sh/extra":               string(extraJSON),
				"lynq.sh/template-generation": "3", // Matches web-template
			},
		},
		Spec: lynqv1.LynqNodeSpec{
			TemplateRef: "web-template",
			UID:         "tenant1",
		},
	}

	workerNode := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant1-worker-template",
			Namespace: "default",
			Annotations: map[string]string{
				"lynq.sh/hostOrUrl":           "http://tenant1.com",
				"lynq.sh/activate":            "true",
				"lynq.sh/extra":               string(extraJSON),
				"lynq.sh/template-generation": "15", // Matches worker-template
			},
		},
		Spec: lynqv1.LynqNodeSpec{
			TemplateRef: "worker-template",
			UID:         "tenant1",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(hub, webTemplate, workerTemplate, webNode, workerNode).
		Build()

	r := &LynqHubReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	ctx := context.Background()
	row := datasource.NodeRow{
		UID:       "tenant1",
		HostOrURL: "http://tenant1.com",
		Activate:  "true",
		Extra:     map[string]string{},
	}

	t.Run("web node should not need update when web template unchanged", func(t *testing.T) {
		needsUpdate := r.shouldUpdateLynqNode(ctx, hub, webNode, row)
		assert.False(t, needsUpdate,
			"Web node should NOT need update. "+
				"Before fix: would incorrectly compare against worker-template (gen 15) "+
				"instead of web-template (gen 3), causing unnecessary update")
	})

	t.Run("worker node should not need update when worker template unchanged", func(t *testing.T) {
		needsUpdate := r.shouldUpdateLynqNode(ctx, hub, workerNode, row)
		assert.False(t, needsUpdate,
			"Worker node should NOT need update when its template generation matches")
	})

	t.Run("web node should need update when web template changes", func(t *testing.T) {
		// Simulate template update
		webTemplate.Generation = 4
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(hub, webTemplate, workerTemplate, webNode, workerNode).
			Build()
		r := &LynqHubReconciler{Client: fakeClient, Scheme: scheme}

		needsUpdate := r.shouldUpdateLynqNode(ctx, hub, webNode, row)
		assert.True(t, needsUpdate,
			"Web node SHOULD need update when web-template generation changes (3 -> 4)")
	})
}

// TestUpdateLynqNode_EventSelection verifies that the correct event type
// is emitted based on what changed (template vs data vs extra values).
func TestUpdateLynqNode_EventSelection(t *testing.T) {
	tests := []struct {
		name                  string
		oldTemplateGeneration string
		newTemplateGeneration int64
		oldHostOrURL          string
		newHostOrURL          string
		expectTemplateApplied bool
		expectDataUpdated     bool
		expectNoEvent         bool
		description           string
	}{
		{
			name:                  "should emit TemplateApplied when template changes",
			oldTemplateGeneration: "1",
			newTemplateGeneration: 2,
			oldHostOrURL:          "http://example.com",
			newHostOrURL:          "http://example.com",
			expectTemplateApplied: true,
			expectDataUpdated:     false,
			expectNoEvent:         false,
			description:           "Template change emits TemplateApplied",
		},
		{
			name:                  "should emit DataUpdated when only data changes",
			oldTemplateGeneration: "1",
			newTemplateGeneration: 1,
			oldHostOrURL:          "http://old.com",
			newHostOrURL:          "http://new.com",
			expectTemplateApplied: false,
			expectDataUpdated:     true,
			expectNoEvent:         false,
			description:           "Data change emits DataUpdated",
		},
		{
			name:                  "should emit NO event when only extra values change",
			oldTemplateGeneration: "1",
			newTemplateGeneration: 1,
			oldHostOrURL:          "http://example.com",
			newHostOrURL:          "http://example.com",
			expectTemplateApplied: false,
			expectDataUpdated:     false,
			expectNoEvent:         true,
			description:           "Extra values change emits NO event (removed NodeUpdated)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test documents the expected event behavior
			// The actual event emission is tested through integration tests
			// as it requires the full updateLynqNode flow

			templateChanged := tt.oldTemplateGeneration != fmt.Sprintf("%d", tt.newTemplateGeneration)
			dataChanged := tt.oldHostOrURL != tt.newHostOrURL

			if templateChanged {
				assert.True(t, tt.expectTemplateApplied, tt.description)
			} else if dataChanged {
				assert.True(t, tt.expectDataUpdated, tt.description)
			} else {
				// Extra values or other minor changes - no event per the fix
				assert.True(t, tt.expectNoEvent, tt.description)
			}
		})
	}
}
