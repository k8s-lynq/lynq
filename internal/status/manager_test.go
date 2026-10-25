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
	"testing"
	"time"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func TestNewStatusUpdate(t *testing.T) {
	key := client.ObjectKey{Name: "test", Namespace: "default"}
	update := NewStatusUpdate(key)

	assert.Equal(t, key, update.Key)
	assert.NotNil(t, update.Conditions)
	assert.Empty(t, update.Conditions)
	assert.Nil(t, update.ReadyResources)
	assert.Nil(t, update.FailedResources)
}

// TestManager_NoWriteWhenValuesUnchanged verifies that republishing identical status values
// does not issue an API write.
//
// Reconcilers republish their full status on every pass, so "a field was published" is not
// evidence that anything changed. Treating it as such issued a Status().Update() on every
// reconcile; each write bumps resourceVersion, fires a LynqNode watch event, and re-enqueues
// the node, which sustained a full-reconcile loop in production.
func TestManager_NoWriteWhenValuesUnchanged(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "default"},
		Spec:       lynqv1.LynqNodeSpec{UID: "node1"},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		Build()

	m := NewManager(fakeClient, WithSyncMode())
	key := types.NamespacedName{Name: node.Name, Namespace: node.Namespace}

	resourceVersion := func() string {
		current := &lynqv1.LynqNode{}
		require.NoError(t, fakeClient.Get(ctx, key, current))
		return current.ResourceVersion
	}

	publishAll := func() {
		m.PublishObservedGeneration(node, 1)
		m.PublishResourceCounts(node, 3, 0, 3, 0)
		m.PublishAppliedResources(node, []string{"ConfigMap/default/a@a", "Secret/default/b@b"})
		m.PublishSkippedResources(node, 0, nil)
		m.PublishReadyCondition(node, true, "Reconciled", "Successfully reconciled all resources")
	}

	// Given a node whose status has been published once
	publishAll()
	settled := resourceVersion()
	assert.NotEqual(t, node.ResourceVersion, settled, "the first publish should write")

	// When the identical status is published again, repeatedly
	for i := 0; i < 5; i++ {
		publishAll()
	}

	// Then no further writes occur
	assert.Equal(t, settled, resourceVersion(),
		"republishing unchanged status wrote to the API server")
}

// TestManager_WritesWhenValueActuallyChanges guards the other direction: the equality checks
// must not suppress genuine transitions.
func TestManager_WritesWhenValueActuallyChanges(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "default"},
		Spec:       lynqv1.LynqNodeSpec{UID: "node1"},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		Build()

	m := NewManager(fakeClient, WithSyncMode())
	key := types.NamespacedName{Name: node.Name, Namespace: node.Namespace}

	read := func() *lynqv1.LynqNode {
		current := &lynqv1.LynqNode{}
		require.NoError(t, fakeClient.Get(ctx, key, current))
		return current
	}

	// Given a settled status
	m.PublishResourceCounts(node, 3, 0, 3, 0)
	m.PublishAppliedResources(node, []string{"ConfigMap/default/a@a"})
	settled := read()

	// When a resource starts failing
	m.PublishResourceCounts(node, 2, 1, 3, 0)

	// Then the change is persisted
	afterCounts := read()
	assert.NotEqual(t, settled.ResourceVersion, afterCounts.ResourceVersion, "changed counts should write")
	assert.Equal(t, int32(1), afterCounts.Status.FailedResources)
	assert.Equal(t, int32(2), afterCounts.Status.ReadyResources)

	// And when the tracked resource set changes
	m.PublishAppliedResources(node, []string{"ConfigMap/default/a@a", "Secret/default/b@b"})

	afterApplied := read()
	assert.NotEqual(t, afterCounts.ResourceVersion, afterApplied.ResourceVersion, "changed applied set should write")
	assert.Len(t, afterApplied.Status.AppliedResources, 2)
}

// TestManager_DegradedReasonTransitionClearsPreviousReasonSeries guards against a stale
// lynqnode_degraded_status series: when a node stays degraded but its reason changes, the
// previous reason's series must drop to 0 instead of staying at 1 and keeping its alert firing.
func TestManager_DegradedReasonTransitionClearsPreviousReasonSeries(t *testing.T) {
	m := &Manager{}
	key := client.ObjectKey{Name: "reason-transition-node", Namespace: "default"}
	degradedStatus := func(reason string) float64 {
		return testutil.ToFloat64(metrics.LynqNodeDegradedStatus.WithLabelValues(key.Name, key.Namespace, reason))
	}

	// Given a node degraded because of failed resources
	m.updateMetrics(key, &MetricsPayload{IsDegraded: true, DegradedReason: "ResourceFailures"})
	require.Equal(t, float64(1), degradedStatus("ResourceFailures"))

	// When it stays degraded but the reason becomes a conflict
	m.updateMetrics(key, &MetricsPayload{IsDegraded: true, DegradedReason: "ResourceConflicts"})

	// Then only the current reason reports degraded
	assert.Equal(t, float64(1), degradedStatus("ResourceConflicts"))
	assert.Equal(t, float64(0), degradedStatus("ResourceFailures"),
		"the previous reason's series must be reset when the degraded reason changes")
	assert.Equal(t, float64(0), degradedStatus("ResourcesNotReady"))

	// And when the node recovers, every reason reports healthy
	m.updateMetrics(key, &MetricsPayload{IsDegraded: false, DegradedReason: "Healthy"})
	for _, reason := range knownDegradedReasons {
		assert.Equal(t, float64(0), degradedStatus(reason), "reason %s must be cleared after recovery", reason)
	}
}

// TestManager_ClearsPublishedEmptyLists pins that a list published empty clears the stored
// list. Publishers build an empty list as nil, and StatusUpdate uses nil for "not published",
// so without the distinction a node whose skips resolved kept skippedResources=0 next to the
// stale skippedResourceIds of the skip that was gone.
func TestManager_ClearsPublishedEmptyLists(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	read := func(c client.Client, key types.NamespacedName) *lynqv1.LynqNode {
		current := &lynqv1.LynqNode{}
		require.NoError(t, c.Get(ctx, key, current))
		return current
	}

	t.Run("skipped resources are cleared when the skip resolves", func(t *testing.T) {
		node := &lynqv1.LynqNode{
			ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "default"},
			Spec:       lynqv1.LynqNodeSpec{UID: "node1"},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).WithStatusSubresource(node).Build()
		m := NewManager(fakeClient, WithSyncMode())
		key := types.NamespacedName{Name: node.Name, Namespace: node.Namespace}

		// Given a node whose dependents were skipped because a dependency failed
		m.PublishSkippedResources(node, 2, []string{"core-api-ingress", "core-api-service"})
		skipped := read(fakeClient, key)
		require.Equal(t, int32(2), skipped.Status.SkippedResources)
		require.Equal(t, []string{"core-api-ingress", "core-api-service"}, skipped.Status.SkippedResourceIds)

		// When the dependency recovers and the reconcile skips nothing (ids built as nil)
		m.PublishSkippedResources(node, 0, nil)

		// Then the count and the ids are cleared together
		resolved := read(fakeClient, key)
		assert.Equal(t, int32(0), resolved.Status.SkippedResources)
		assert.Empty(t, resolved.Status.SkippedResourceIds, "stale skipped ids survived the resolved skip")

		// And republishing the empty skip set does not write again
		for i := 0; i < 5; i++ {
			m.PublishSkippedResources(node, 0, nil)
			m.PublishSkippedResources(node, 0, []string{})
		}
		assert.Equal(t, resolved.ResourceVersion, read(fakeClient, key).ResourceVersion,
			"republishing an empty skip set wrote to the API server")
	})

	t.Run("stale skipped ids left by an older version are cleared once", func(t *testing.T) {
		// Given a node an older operator left with skippedResources=0 but stale ids
		node := &lynqv1.LynqNode{
			ObjectMeta: metav1.ObjectMeta{Name: "upgraded-node", Namespace: "default"},
			Spec:       lynqv1.LynqNodeSpec{UID: "node1"},
			Status: lynqv1.LynqNodeStatus{
				SkippedResources:   0,
				SkippedResourceIds: []string{"core-api-service", "core-api-ingress"},
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).WithStatusSubresource(node).Build()
		m := NewManager(fakeClient, WithSyncMode())
		key := types.NamespacedName{Name: node.Name, Namespace: node.Namespace}
		before := read(fakeClient, key)
		require.Len(t, before.Status.SkippedResourceIds, 2)

		// When the next reconcile publishes that nothing is skipped
		m.PublishSkippedResources(node, 0, nil)

		// Then the stale ids are cleared in one write
		cleaned := read(fakeClient, key)
		assert.NotEqual(t, before.ResourceVersion, cleaned.ResourceVersion, "clearing stale ids should write")
		assert.Empty(t, cleaned.Status.SkippedResourceIds)

		// And later reconciles do not write again
		m.PublishSkippedResources(node, 0, nil)
		assert.Equal(t, cleaned.ResourceVersion, read(fakeClient, key).ResourceVersion)
	})

	t.Run("applied resources are cleared when published empty", func(t *testing.T) {
		node := &lynqv1.LynqNode{
			ObjectMeta: metav1.ObjectMeta{Name: "applied-node", Namespace: "default"},
			Spec:       lynqv1.LynqNodeSpec{UID: "node1"},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).WithStatusSubresource(node).Build()
		m := NewManager(fakeClient, WithSyncMode())
		key := types.NamespacedName{Name: node.Name, Namespace: node.Namespace}

		// Given a node tracking one applied resource
		m.PublishAppliedResources(node, []string{"ConfigMap/default/a@a"})
		require.Len(t, read(fakeClient, key).Status.AppliedResources, 1)

		// When every resource is removed from the template (keys built as nil)
		m.PublishAppliedResources(node, nil)

		// Then the applied set is cleared
		assert.Empty(t, read(fakeClient, key).Status.AppliedResources)
	})
}

// TestStatusUpdate_ApplyMarksEmptyListsPublished pins the StatusUpdate side of the contract:
// an event carrying a nil list still counts as a published (empty) list.
func TestStatusUpdate_ApplyMarksEmptyListsPublished(t *testing.T) {
	key := client.ObjectKey{Name: "test-node", Namespace: "default"}
	update := NewStatusUpdate(key)

	update.Apply(StatusEvent{Type: EventSkippedResourcesUpdated, NodeKey: key, Payload: SkippedResourcesPayload{Count: 0}})
	update.Apply(StatusEvent{Type: EventAppliedResourcesUpdated, NodeKey: key, Payload: AppliedResourcesPayload{}})

	assert.NotNil(t, update.SkippedResourceIds, "a nil skipped list must still be published")
	assert.Empty(t, update.SkippedResourceIds)
	assert.NotNil(t, update.AppliedResources, "a nil applied list must still be published")
	assert.Empty(t, update.AppliedResources)
}
