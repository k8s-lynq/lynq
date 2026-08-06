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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/status"
)

// multiResourceLynqNode returns a LynqNode carrying enough resources that a map-ordered
// status.appliedResources would almost certainly differ between two reconciles (1 - 1/n!).
func multiResourceLynqNode() *lynqv1.LynqNode {
	configMap := func(id, name string) lynqv1.TResource {
		return lynqv1.TResource{
			ID:           id,
			NameTemplate: name,
			Spec: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"data":       map[string]interface{}{"app": "myapp"},
				},
			},
			CreationPolicy: lynqv1.CreationPolicyWhenNeeded,
			DeletionPolicy: lynqv1.DeletionPolicyDelete,
		}
	}

	return &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node1-web-app",
			Namespace: "default",
			Annotations: map[string]string{
				"lynq.sh/hostOrUrl": "https://tenant1.example.com",
				"lynq.sh/activate":  "true",
				"lynq.sh/extra":     `{"plan":"premium"}`,
			},
		},
		Spec: lynqv1.LynqNodeSpec{
			UID:         "node1",
			TemplateRef: "web-app",
			ConfigMaps: []lynqv1.TResource{
				configMap("zeta-config", "node1-zeta"),
				configMap("alpha-config", "node1-alpha"),
				configMap("mike-config", "node1-mike"),
				configMap("delta-config", "node1-delta"),
				configMap("bravo-config", "node1-bravo"),
				configMap("yankee-config", "node1-yankee"),
				configMap("charlie-config", "node1-charlie"),
			},
		},
	}
}

// TestRegression_SteadyStateReconcileDoesNotWriteStatus pins the invariant that broke in
// production: reconciling an unchanged LynqNode must not write to the API server.
//
// Two independent defects combined into a self-sustaining full-reconcile loop:
//
//  1. status.appliedResources was assembled by ranging over a map, so its order was
//     re-randomized on every reconcile. The API server only elides an update when the
//     serialized object is byte-identical, so a reordered slice is a genuine etcd write.
//  2. StatusManager marked every published field as changed without comparing it to the
//     stored value, so Status().Update() was issued on every reconcile regardless.
//
// Either write bumps resourceVersion, which fires a LynqNode watch event, which re-enqueues
// the node — forever. Measured in production: 200 of 360 LynqNodes written per minute on a
// fully-Ready, idle cluster, with the manager pinned at 671m CPU.
//
// Asserting on resourceVersion covers both defects at once, and covers any future field that
// reintroduces the same class of instability.
func TestRegression_SteadyStateReconcileDoesNotWriteStatus(t *testing.T) {
	ctx := context.Background()
	scheme := setupTestScheme(t)
	node := multiResourceLynqNode()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		Build()

	r := &LynqNodeReconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		Recorder:      record.NewFakeRecorder(1000),
		StatusManager: status.NewManager(fakeClient, status.WithSyncMode()),
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: node.Name, Namespace: node.Namespace}}

	resourceVersion := func() string {
		current := &lynqv1.LynqNode{}
		require.NoError(t, fakeClient.Get(ctx, req.NamespacedName, current))
		return current.ResourceVersion
	}

	// Given a LynqNode that has settled: the finalizer is on, resources have been applied
	// once, and status reflects the outcome.
	for i := 0; i < 3; i++ {
		_, err := r.Reconcile(ctx, req)
		// The fake client does not implement Server-Side Apply, so applies may fail. That is
		// irrelevant here: the loop is driven by status writes, and a stable failure produces
		// a stable status just as a stable success does.
		_ = err
	}
	settled := resourceVersion()

	// When the node is reconciled repeatedly with nothing changed
	for i := 0; i < 5; i++ {
		_, err := r.Reconcile(ctx, req)
		_ = err
	}

	// Then the object is never written again, so no watch event is produced and the loop
	// cannot sustain itself.
	assert.Equal(t, settled, resourceVersion(),
		"steady-state reconcile wrote to the LynqNode; each such write re-triggers reconciliation")
}

// TestRegression_AppliedResourcesIsSorted pins the ordering invariant directly, independent of
// whether StatusManager happens to suppress the write. status.appliedResources is built from a
// map and Go randomizes map iteration, so it must be sorted before it is published.
func TestRegression_AppliedResourcesIsSorted(t *testing.T) {
	ctx := context.Background()
	scheme := setupTestScheme(t)
	node := multiResourceLynqNode()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		WithStatusSubresource(node).
		Build()

	r := &LynqNodeReconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		Recorder:      record.NewFakeRecorder(1000),
		StatusManager: status.NewManager(fakeClient, status.WithSyncMode()),
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: node.Name, Namespace: node.Namespace}}

	// Given a reconciled LynqNode with several resources
	for i := 0; i < 2; i++ {
		_, err := r.Reconcile(ctx, req)
		_ = err
	}

	// When its recorded applied resources are read back
	current := &lynqv1.LynqNode{}
	require.NoError(t, fakeClient.Get(ctx, req.NamespacedName, current))
	require.Len(t, current.Status.AppliedResources, 7, "all resources should be tracked")

	// Then they are in a deterministic (sorted) order
	assert.True(t, sort.StringsAreSorted(current.Status.AppliedResources),
		"appliedResources must be sorted; unsorted map iteration re-randomizes it every reconcile: %v",
		current.Status.AppliedResources)
}
