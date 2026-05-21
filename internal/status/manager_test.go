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

package status

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	imetrics "github.com/k8s-lynq/lynq/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestStatusUpdate_Apply(t *testing.T) {
	key := client.ObjectKey{Name: "test-node", Namespace: "default"}
	update := NewStatusUpdate(key)

	// Test resource counts update
	event1 := StatusEvent{
		Type:    EventResourceCountsUpdated,
		NodeKey: key,
		Payload: ResourceCountsPayload{
			Ready:      5,
			Failed:     1,
			Desired:    6,
			Conflicted: 0,
		},
		Timestamp: time.Now(),
	}
	update.Apply(event1)

	assert.NotNil(t, update.ReadyResources)
	assert.Equal(t, int32(5), *update.ReadyResources)
	assert.NotNil(t, update.FailedResources)
	assert.Equal(t, int32(1), *update.FailedResources)
	assert.NotNil(t, update.DesiredResources)
	assert.Equal(t, int32(6), *update.DesiredResources)

	// Test condition update
	event2 := StatusEvent{
		Type:    EventConditionChanged,
		NodeKey: key,
		Payload: ConditionPayload{
			Condition: metav1.Condition{
				Type:   "Ready",
				Status: metav1.ConditionTrue,
				Reason: "AllResourcesReady",
			},
		},
		Timestamp: time.Now(),
	}
	update.Apply(event2)

	assert.Len(t, update.Conditions, 1)
	assert.Equal(t, "Ready", update.Conditions["Ready"].Type)
	assert.Equal(t, metav1.ConditionTrue, update.Conditions["Ready"].Status)

	// Test applied resources update
	event3 := StatusEvent{
		Type:    EventAppliedResourcesUpdated,
		NodeKey: key,
		Payload: AppliedResourcesPayload{
			Keys: []string{"Deployment/default/app1@dep1", "Service/default/svc1@svc1"},
		},
		Timestamp: time.Now(),
	}
	update.Apply(event3)

	assert.Len(t, update.AppliedResources, 2)
	assert.Contains(t, update.AppliedResources, "Deployment/default/app1@dep1")
}

func TestStatusUpdate_HasChanges(t *testing.T) {
	key := client.ObjectKey{Name: "test-node", Namespace: "default"}

	// Empty update has no changes
	update := NewStatusUpdate(key)
	assert.False(t, update.HasChanges())

	// Update with resource counts has changes
	ready := int32(5)
	update.ReadyResources = &ready
	assert.True(t, update.HasChanges())

	// New update with conditions has changes
	update2 := NewStatusUpdate(key)
	update2.Conditions["Ready"] = metav1.Condition{Type: "Ready"}
	assert.True(t, update2.HasChanges())
}

func TestManager_PublishSync(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	err := lynqv1.AddToScheme(scheme)
	require.NoError(t, err)

	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-node",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: lynqv1.LynqNodeSpec{
			UID:         "node-123",
			TemplateRef: "test-template",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		Build()

	// Create manager in sync mode for testing
	manager := NewManager(fakeClient, WithSyncMode())

	// Publish resource counts
	manager.PublishResourceCounts(node, 5, 1, 6, 0)

	// Verify status was updated
	updated := &lynqv1.LynqNode{}
	err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(node), updated)
	require.NoError(t, err)

	assert.Equal(t, int32(5), updated.Status.ReadyResources)
	assert.Equal(t, int32(1), updated.Status.FailedResources)
	assert.Equal(t, int32(6), updated.Status.DesiredResources)
}

func TestManager_PublishConditionSync(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	err := lynqv1.AddToScheme(scheme)
	require.NoError(t, err)

	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-node",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: lynqv1.LynqNodeSpec{
			UID:         "node-123",
			TemplateRef: "test-template",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		Build()

	manager := NewManager(fakeClient, WithSyncMode())

	// Publish Ready condition
	manager.PublishReadyCondition(node, true, "AllResourcesReady", "All 6 resources are ready")

	// Verify condition was updated
	updated := &lynqv1.LynqNode{}
	err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(node), updated)
	require.NoError(t, err)

	require.Len(t, updated.Status.Conditions, 1)
	assert.Equal(t, "Ready", updated.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, updated.Status.Conditions[0].Status)
	assert.Equal(t, "AllResourcesReady", updated.Status.Conditions[0].Reason)
}

func TestManager_PublishMultipleConditionsSync(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	err := lynqv1.AddToScheme(scheme)
	require.NoError(t, err)

	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-node",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: lynqv1.LynqNodeSpec{
			UID:         "node-123",
			TemplateRef: "test-template",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		Build()

	manager := NewManager(fakeClient, WithSyncMode())

	// Publish multiple conditions
	manager.PublishReadyCondition(node, true, "AllResourcesReady", "All resources ready")
	manager.PublishProgressingCondition(node, false, "ReconcileComplete", "Reconciliation completed")
	manager.PublishConflictedCondition(node, false)
	manager.PublishDegradedCondition(node, false, "Healthy", "All resources healthy")

	// Verify all conditions were updated
	updated := &lynqv1.LynqNode{}
	err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(node), updated)
	require.NoError(t, err)

	assert.Len(t, updated.Status.Conditions, 4)

	// Check each condition
	conditionMap := make(map[string]metav1.Condition)
	for _, cond := range updated.Status.Conditions {
		conditionMap[cond.Type] = cond
	}

	assert.Equal(t, metav1.ConditionTrue, conditionMap["Ready"].Status)
	assert.Equal(t, metav1.ConditionFalse, conditionMap["Progressing"].Status)
	assert.Equal(t, metav1.ConditionFalse, conditionMap["Conflicted"].Status)
	assert.Equal(t, metav1.ConditionFalse, conditionMap["Degraded"].Status)
}

func TestManager_PublishAppliedResourcesSync(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	err := lynqv1.AddToScheme(scheme)
	require.NoError(t, err)

	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-node",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: lynqv1.LynqNodeSpec{
			UID:         "node-123",
			TemplateRef: "test-template",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		Build()

	manager := NewManager(fakeClient, WithSyncMode())

	// Publish applied resources
	keys := []string{
		"Deployment/default/app1@dep1",
		"Service/default/svc1@svc1",
	}
	manager.PublishAppliedResources(node, keys)

	// Verify applied resources were updated
	updated := &lynqv1.LynqNode{}
	err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(node), updated)
	require.NoError(t, err)

	assert.Len(t, updated.Status.AppliedResources, 2)
	assert.Contains(t, updated.Status.AppliedResources, "Deployment/default/app1@dep1")
	assert.Contains(t, updated.Status.AppliedResources, "Service/default/svc1@svc1")
}

func TestManager_PublishFullStatusSync(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	err := lynqv1.AddToScheme(scheme)
	require.NoError(t, err)

	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-node",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: lynqv1.LynqNodeSpec{
			UID:         "node-123",
			TemplateRef: "test-template",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		Build()

	manager := NewManager(fakeClient, WithSyncMode())

	// Publish full status
	conditions := []metav1.Condition{
		{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "AllResourcesReady",
			Message:            "All resources ready",
			LastTransitionTime: metav1.Now(),
		},
		{
			Type:               "Progressing",
			Status:             metav1.ConditionFalse,
			Reason:             "ReconcileComplete",
			Message:            "Reconciliation completed",
			LastTransitionTime: metav1.Now(),
		},
	}
	appliedKeys := []string{"Deployment/default/app1@dep1"}

	manager.PublishFullStatus(node, 5, 1, 6, 0, conditions, appliedKeys, false, "")

	// Verify everything was updated
	updated := &lynqv1.LynqNode{}
	err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(node), updated)
	require.NoError(t, err)

	// Check resource counts
	assert.Equal(t, int32(5), updated.Status.ReadyResources)
	assert.Equal(t, int32(1), updated.Status.FailedResources)
	assert.Equal(t, int32(6), updated.Status.DesiredResources)

	// Check conditions
	assert.Len(t, updated.Status.Conditions, 2)

	// Check applied resources
	assert.Len(t, updated.Status.AppliedResources, 1)
	assert.Contains(t, updated.Status.AppliedResources, "Deployment/default/app1@dep1")
}

func TestManager_UpdateConditionDeduplication(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	err := lynqv1.AddToScheme(scheme)
	require.NoError(t, err)

	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-node",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: lynqv1.LynqNodeSpec{
			UID:         "node-123",
			TemplateRef: "test-template",
		},
		Status: lynqv1.LynqNodeStatus{
			Conditions: []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					Reason:             "Progressing",
					Message:            "Resources are being applied",
					LastTransitionTime: metav1.Now(),
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		Build()

	manager := NewManager(fakeClient, WithSyncMode())

	// Update the same condition with different status
	manager.PublishReadyCondition(node, true, "AllResourcesReady", "All resources ready")

	// Verify condition was updated (not duplicated)
	updated := &lynqv1.LynqNode{}
	err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(node), updated)
	require.NoError(t, err)

	assert.Len(t, updated.Status.Conditions, 1)
	assert.Equal(t, "Ready", updated.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, updated.Status.Conditions[0].Status)
	assert.Equal(t, "AllResourcesReady", updated.Status.Conditions[0].Reason)
}

func TestManager_HandleDeletedLynqNode(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	err := lynqv1.AddToScheme(scheme)
	require.NoError(t, err)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	manager := NewManager(fakeClient, WithSyncMode())

	// Try to publish to a non-existent node
	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deleted-node",
			Namespace: "default",
		},
	}

	// This should not error - it should gracefully handle missing node
	manager.PublishResourceCounts(node, 5, 1, 6, 0)

	// Verify node still doesn't exist
	updated := &lynqv1.LynqNode{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "deleted-node",
		Namespace: "default",
	}, updated)
	assert.Error(t, err)
}

func TestStatusBatch_ResetOnUIDChange(t *testing.T) {
	// When a LynqNode is deleted and immediately recreated with the same name,
	// old-instance events and new-instance events can land in the same batch window.
	// The batch entry must be reset when the UID changes so that stale payloads from
	// the old instance do not bleed into the new instance's status/metrics flush.
	key := client.ObjectKey{Name: "node", Namespace: "default"}

	// Mirror the batch building logic from manager.run() so the test stays in sync
	// with the production code.
	applyToBatch := func(batch map[client.ObjectKey]*StatusUpdate, event StatusEvent) {
		update := batch[event.NodeKey]
		if update == nil {
			update = NewStatusUpdate(event.NodeKey)
			batch[event.NodeKey] = update
		}
		if event.NodeUID != "" && update.UID != "" && update.UID != event.NodeUID {
			update = NewStatusUpdate(event.NodeKey)
			batch[event.NodeKey] = update
		}
		update.Apply(event)
	}

	batch := make(map[client.ObjectKey]*StatusUpdate)

	// Old instance: Ready=99, Failed=5 accumulated in the batch
	applyToBatch(batch, StatusEvent{
		Type:    EventResourceCountsUpdated,
		NodeKey: key,
		NodeUID: "old-uid",
		Payload: ResourceCountsPayload{Ready: 99, Failed: 5, Desired: 10},
	})
	require.Equal(t, int32(99), *batch[key].ReadyResources)

	// New instance arrives with different UID — old state must be discarded
	applyToBatch(batch, StatusEvent{
		Type:    EventResourceCountsUpdated,
		NodeKey: key,
		NodeUID: "new-uid",
		Payload: ResourceCountsPayload{Ready: 4, Failed: 0, Desired: 4},
	})

	assert.Equal(t, types.UID("new-uid"), batch[key].UID)
	assert.Equal(t, int32(4), *batch[key].ReadyResources)
	// Old instance's Failed=5 must not bleed through into the new instance's update
	assert.Equal(t, int32(0), *batch[key].FailedResources, "stale Failed count must not persist")
}

func TestNewStatusUpdate(t *testing.T) {
	key := client.ObjectKey{Name: "test", Namespace: "default"}
	update := NewStatusUpdate(key)

	assert.Equal(t, key, update.Key)
	assert.NotNil(t, update.Conditions)
	assert.Empty(t, update.Conditions)
	assert.Nil(t, update.ReadyResources)
	assert.Nil(t, update.FailedResources)
}

func TestManager_MetricsNotFoundGuard(t *testing.T) {
	// Flush-after-deletion race: the node is deleted before the status batch is
	// flushed, but a stale MetricsPayload sits in the event queue.
	// Expected: CleanupLynqNodeMetrics is called — the leaked series disappears
	// rather than being resurrected to the stale value.
	imetrics.LynqNodeResourcesReady.Reset()
	// Simulate a series that was never cleaned up (the bug scenario).
	imetrics.LynqNodeResourcesReady.WithLabelValues("ghost-node", "default").Set(5)

	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build() // no LynqNode in cluster
	manager := NewManager(fakeClient, WithSyncMode())

	manager.Publish(StatusEvent{
		Type:      EventMetricsUpdate,
		NodeKey:   client.ObjectKey{Name: "ghost-node", Namespace: "default"},
		NodeUID:   "some-uid",
		Payload:   MetricsPayload{Ready: 3, Failed: 0, Desired: 3},
		Timestamp: time.Now(),
	})

	// The pre-existing leaked series must be gone (not resurrected to 3).
	assert.Equal(t, 0, testutil.CollectAndCount(imetrics.LynqNodeResourcesReady),
		"leaked series should be cleaned up when node is NotFound")
}

func TestManager_MetricsUIDMismatchGuard(t *testing.T) {
	// Same-name recreation race: old instance's stale event arrives after a new
	// instance (same name, different UID) already exists.
	// Expected: neither updateMetrics nor CleanupLynqNodeMetrics is called —
	// the new instance's series stays exactly as it was.
	imetrics.LynqNodeResourcesReady.Reset()
	imetrics.LynqNodeResourcesReady.WithLabelValues("recycled-node", "default").Set(7)

	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "recycled-node",
			Namespace: "default",
			UID:       "new-uid",
		},
		Spec: lynqv1.LynqNodeSpec{UID: "row-1", TemplateRef: "tpl"},
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		Build()
	manager := NewManager(fakeClient, WithSyncMode())

	// Old instance's stale event with old UID
	manager.Publish(StatusEvent{
		Type:      EventMetricsUpdate,
		NodeKey:   client.ObjectKey{Name: "recycled-node", Namespace: "default"},
		NodeUID:   "old-uid", // mismatches node.UID = "new-uid"
		Payload:   MetricsPayload{Ready: 99, IsDegraded: true, DegradedReason: "ResourceFailures"},
		Timestamp: time.Now(),
	})

	// New instance's series must remain at 7 — not updated to 99 and not deleted.
	expected := `
# HELP lynqnode_resources_ready Number of ready resources for a LynqNode
# TYPE lynqnode_resources_ready gauge
lynqnode_resources_ready{lynqnode="recycled-node",namespace="default"} 7
`
	err := testutil.CollectAndCompare(imetrics.LynqNodeResourcesReady, strings.NewReader(expected))
	assert.NoError(t, err, "new instance series must not be touched by stale event")
}

func TestManager_MetricsNormalPath(t *testing.T) {
	// Verify that matching UIDs result in metrics being updated as usual.
	imetrics.LynqNodeResourcesReady.Reset()

	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-node",
			Namespace: "default",
			UID:       "my-uid",
		},
		Spec: lynqv1.LynqNodeSpec{UID: "row-1", TemplateRef: "tpl"},
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		Build()
	manager := NewManager(fakeClient, WithSyncMode())

	manager.Publish(StatusEvent{
		Type:      EventMetricsUpdate,
		NodeKey:   client.ObjectKey{Name: "my-node", Namespace: "default"},
		NodeUID:   "my-uid",
		Payload:   MetricsPayload{Ready: 4, Failed: 0, Desired: 4},
		Timestamp: time.Now(),
	})

	expected := `
# HELP lynqnode_resources_ready Number of ready resources for a LynqNode
# TYPE lynqnode_resources_ready gauge
lynqnode_resources_ready{lynqnode="my-node",namespace="default"} 4
`
	err := testutil.CollectAndCompare(imetrics.LynqNodeResourcesReady, strings.NewReader(expected))
	assert.NoError(t, err, "matching UID should result in metrics being updated")
}

func TestManager_MetricsSkippedOnAPIError(t *testing.T) {
	// When the status manager's Get call fails with a non-NotFound error (e.g. API
	// server unavailable), neither nodeDeleted nor nodeVerified is set. The metrics
	// dispatch must skip updateMetrics in that case to avoid writing stale values.
	imetrics.LynqNodeResourcesReady.Reset()

	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-err-node",
			Namespace: "default",
			UID:       "uid-1",
		},
		Spec: lynqv1.LynqNodeSpec{UID: "row-1", TemplateRef: "tpl"},
	}

	// Inject an internal-server-error on every Get to simulate a transient API failure.
	errClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				return fmt.Errorf("simulated API server error")
			},
		}).
		Build()

	manager := NewManager(errClient, WithSyncMode())

	manager.Publish(StatusEvent{
		Type:      EventMetricsUpdate,
		NodeKey:   client.ObjectKey{Name: "api-err-node", Namespace: "default"},
		NodeUID:   "uid-1",
		Payload:   MetricsPayload{Ready: 99, Failed: 0, Desired: 99},
		Timestamp: time.Now(),
	})

	// No series should exist — metrics must not be written when Get fails.
	err := testutil.CollectAndCompare(imetrics.LynqNodeResourcesReady, strings.NewReader(""))
	assert.NoError(t, err, "metrics must not be written when the API Get fails")
}
