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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
)

// The rollout-skew race reproduced here.
//
// The hub is re-enqueued from several independent sources — the LynqNode watch, the LynqHub
// watch fed by our own status writes, the LynqForm watch, and timed requeues. Only the LynqNode
// watch guarantees the informer cache already contains the node update that triggered it. Any
// other trigger can start a reconcile whose cached node list predates an update we just made, so
// a node already moved to the new generation still looks old, is not counted as updating, and a
// second node is admitted past maxSkew.
//
// These tests model that directly: the cached client is frozen at the pre-update state while the
// APIReader reflects the write that actually happened. Nothing here depends on timing, so the
// race is reproduced deterministically rather than by hoping a wall-clock window lines up as it
// did in the "maxSkew strict enforcement with slow-starting Pods" E2E flake.

const (
	skewTestNamespace = "default"
	skewTestHub       = "test-hub"
	skewTestForm      = "web-app"
)

// staleCacheReconciler returns a reconciler whose cached Client is stuck at cachedNodes while
// its APIReader sees apiNodes, plus the LynqHub and LynqForm the skew decision is made for.
func staleCacheReconciler(
	t *testing.T,
	cachedNodes []*lynqv1.LynqNode,
	apiNodes []*lynqv1.LynqNode,
	workloads ...client.Object,
) (*LynqHubReconciler, *lynqv1.LynqHub, *lynqv1.LynqForm) {
	t.Helper()
	scheme := setupTestScheme(t)

	toObjects := func(nodes []*lynqv1.LynqNode) []client.Object {
		objs := make([]client.Object, 0, len(nodes))
		for _, n := range nodes {
			objs = append(objs, n)
		}
		return objs
	}

	cached := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(toObjects(cachedNodes)...).Build()

	apiObjects := append(toObjects(apiNodes), workloads...)
	apiReader := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(apiObjects...).Build()

	hub := &lynqv1.LynqHub{
		ObjectMeta: metav1.ObjectMeta{Name: skewTestHub, Namespace: skewTestNamespace},
	}
	form := &lynqv1.LynqForm{
		ObjectMeta: metav1.ObjectMeta{
			Name:       skewTestForm,
			Namespace:  skewTestNamespace,
			Generation: 2, // the generation being rolled out to
		},
		Spec: lynqv1.LynqFormSpec{
			HubID:   skewTestHub,
			Rollout: &lynqv1.RolloutConfig{MaxSkew: 1},
		},
	}

	return &LynqHubReconciler{
		Client:    cached,
		APIReader: apiReader,
		Scheme:    scheme,
	}, hub, form
}

// skewTestNode builds a LynqNode carrying the rollout markers the skew count reads.
func skewTestNode(name string, templateGen int64, ready bool, updateStarted time.Time) *lynqv1.LynqNode {
	node := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  skewTestNamespace,
			Generation: 1,
			Labels:     map[string]string{"lynq.sh/hub": skewTestHub},
			Annotations: map[string]string{
				lynqv1.AnnotationTemplateGeneration: fmt.Sprintf("%d", templateGen),
			},
		},
		Spec: lynqv1.LynqNodeSpec{UID: name, TemplateRef: skewTestForm},
		Status: lynqv1.LynqNodeStatus{
			ObservedGeneration: 1,
		},
	}
	if !updateStarted.IsZero() {
		node.Annotations[lynqv1.AnnotationRolloutUpdateStartTime] = updateStarted.Format(time.RFC3339Nano)
	}
	if ready {
		node.Status.Conditions = []metav1.Condition{{
			Type:               ConditionTypeReady,
			Status:             metav1.ConditionTrue,
			Reason:             "Reconciled",
			LastTransitionTime: metav1.Now(),
		}}
	}
	return node
}

// TestRegression_SkewCountSeesWritesTheCacheHasNotObserved is the core regression test: a node
// that has already been moved to the new generation must be counted as updating even when the
// informer cache still shows it at the old one.
func TestRegression_SkewCountSeesWritesTheCacheHasNotObserved(t *testing.T) {
	ctx := context.Background()
	longAgo := time.Now().Add(-time.Hour) // past the safety margin, so only the generation matters

	// Given node-1 has been updated to the new template generation in the API server...
	updated := skewTestNode("node-1", 2, false, longAgo)
	// ...but the cache still shows it at the old generation, as if the watch has not arrived.
	stale := skewTestNode("node-1", 1, true, time.Time{})

	r, hub, form := staleCacheReconciler(t,
		[]*lynqv1.LynqNode{stale},
		[]*lynqv1.LynqNode{updated})

	// When the rollout skew is evaluated
	skew := newRolloutSkewCounter(r, hub)
	count := skew.count(ctx, form)

	// Then the in-flight update is counted...
	assert.Equal(t, int32(1), count,
		"a node already updated in the API server must count as updating even if the cache is behind")

	// ...and no further node may be admitted, because maxSkew is 1.
	assert.False(t, canUpdateNodeWithPrecount(form, count, 0),
		"admitting a second node here is the maxSkew violation this guards against")
}

// TestRegression_SkewCountUsesFreshWorkloadState covers the slot-RELEASE side. countUpdatingNodes
// falls through to isNodeResourcesActuallyReady for nodes that look settled; reading the workload
// from cache there can show the pre-update, still-Ready Deployment and free a slot that is in
// fact occupied.
func TestRegression_SkewCountUsesFreshWorkloadState(t *testing.T) {
	ctx := context.Background()
	longAgo := time.Now().Add(-time.Hour)

	// Given a node that looks settled: current generation, Ready, past the safety margin
	node := skewTestNode("node-1", 2, true, longAgo)
	node.Status.AppliedResources = []string{"Deployment/default/node-1-app@app"}

	// ...whose Deployment is in fact still rolling out
	rolling := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1-app", Namespace: skewTestNamespace, Generation: 2,
		},
		Spec: appsv1.DeploymentSpec{Replicas: ptrInt32(2)},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 2,
			UpdatedReplicas:    1, // still replacing pods
			ReadyReplicas:      1,
			AvailableReplicas:  1,
		},
	}

	r, hub, form := staleCacheReconciler(t,
		[]*lynqv1.LynqNode{node},
		[]*lynqv1.LynqNode{node.DeepCopy()},
		rolling)

	// When the rollout skew is evaluated
	count := newRolloutSkewCounter(r, hub).count(ctx, form)

	// Then the slot is still held by the in-progress rollout
	assert.Equal(t, int32(1), count,
		"a node whose Deployment is still rolling must keep holding its slot")
}

// TestSkewCountIsFreeWhenRolloutIsUnset pins the cost profile: without a rollout limit the count
// is never consulted, so it must not read anything. This is what keeps steady-state syncs free of
// uncached reads.
func TestSkewCountIsFreeWhenRolloutIsUnset(t *testing.T) {
	ctx := context.Background()
	r, hub, form := staleCacheReconciler(t, nil, nil)

	counted := &countingReader{Reader: r.APIReader}
	r.APIReader = counted

	form.Spec.Rollout = nil
	assert.Equal(t, int32(0), newRolloutSkewCounter(r, hub).count(ctx, form))

	form.Spec.Rollout = &lynqv1.RolloutConfig{MaxSkew: 0}
	assert.Equal(t, int32(0), newRolloutSkewCounter(r, hub).count(ctx, form))

	assert.Zero(t, counted.lists, "no rollout limit must mean no API reads")
}

// TestSkewCountReadsOncePerTemplate pins the memoization. Without it the uncached LIST would be
// issued once per desired node instead of once per template.
func TestSkewCountReadsOncePerTemplate(t *testing.T) {
	ctx := context.Background()
	r, hub, form := staleCacheReconciler(t, nil,
		[]*lynqv1.LynqNode{skewTestNode("node-1", 2, false, time.Now())})

	counted := &countingReader{Reader: r.APIReader}
	r.APIReader = counted

	skew := newRolloutSkewCounter(r, hub)
	for i := 0; i < 10; i++ {
		skew.count(ctx, form)
	}

	assert.Equal(t, 1, counted.lists, "the skew snapshot must be taken once per template per reconcile")
}

// TestSkewCountDeniesAdmissionWhenTheReadFails: if we cannot establish how many updates are in
// flight, refusing to start another is the safe direction.
func TestSkewCountDeniesAdmissionWhenTheReadFails(t *testing.T) {
	ctx := context.Background()
	r, hub, form := staleCacheReconciler(t, nil, nil)
	r.APIReader = &failingReader{}

	count := newRolloutSkewCounter(r, hub).count(ctx, form)

	assert.Equal(t, form.Spec.Rollout.MaxSkew, count)
	assert.False(t, canUpdateNodeWithPrecount(form, count, 0),
		"a failed skew read must not admit an update")
}

// TestSkewCountIgnoresOtherTemplates: rollout budgets are per LynqForm, so one template's
// in-flight updates must not consume another's.
func TestSkewCountIgnoresOtherTemplates(t *testing.T) {
	ctx := context.Background()

	mine := skewTestNode("node-1", 2, false, time.Now())
	other := skewTestNode("node-2", 2, false, time.Now())
	other.Spec.TemplateRef = "worker" // different LynqForm, same hub

	r, hub, form := staleCacheReconciler(t, nil, []*lynqv1.LynqNode{mine, other})

	assert.Equal(t, int32(1), newRolloutSkewCounter(r, hub).count(ctx, form),
		"only this template's nodes may consume its rollout budget")
}

func ptrInt32(v int32) *int32 { return &v }

// countingReader counts List calls so tests can assert on read volume.
type countingReader struct {
	client.Reader
	lists int
}

func (c *countingReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	c.lists++
	return c.Reader.List(ctx, list, opts...)
}

// failingReader fails every read, standing in for an API server we cannot reach.
type failingReader struct{}

func (f *failingReader) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return fmt.Errorf("simulated API server failure")
}

func (f *failingReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return fmt.Errorf("simulated API server failure")
}

var _ client.Reader = &failingReader{}
