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

// Package controller - readiness_stale_cache_test.go
//
// # Rollout Detection Behind a Stale Cache
//
// In production the Applier's pre-apply read comes from the informer cache, while its write
// goes to the API server. When the cache has not yet seen an external edit, the .spec
// comparison alone can miss a rollout Lynq starts: a force-reapply that reverts drift the cache
// never saw compares the old cached spec to an identical response. The generation closes that
// gap — one write moves it by at most one, so a jump of two or more past the cached read is
// assumed to be a rollout. These tests split the Applier's reads from the live cluster to pin
// both the rollout that must not be missed and the conservative false positive the rule
// accepts in exchange.
//
// Only merge is exercised: replace sends the cached resourceVersion as a precondition, so a
// stale read makes its write conflict instead of reaching this comparison.

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/apply"
)

// staleCacheClient serves reads of one object from a frozen snapshot, the way an informer
// cache serves an object whose latest watch event it has not processed yet. Writes, and reads
// of every other object, go to the live cluster.
type staleCacheClient struct {
	client.Client
	frozen *unstructured.Unstructured
}

func (c *staleCacheClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if u, ok := obj.(*unstructured.Unstructured); ok && c.frozen != nil &&
		key == client.ObjectKeyFromObject(c.frozen) && u.GroupVersionKind() == c.frozen.GroupVersionKind() {
		c.frozen.DeepCopyInto(u)
		return nil
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

// freezeApplierCache makes the Applier's pre-apply reads of the Deployment return its current
// state until catchUp is called, while the readiness checks keep reading the live cluster.
func (s *readinessScenario) freezeApplierCache() (catchUp func()) {
	s.t.Helper()
	frozen := &unstructured.Unstructured{}
	frozen.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind(resourceKindDeployment))
	require.NoError(s.t, s.client.Get(context.Background(),
		client.ObjectKey{Namespace: "default", Name: steadyStateDeployName}, frozen))
	cache := &staleCacheClient{Client: s.client, frozen: frozen}
	s.r.Applier = apply.NewApplier(cache, s.scheme)
	return func() { cache.frozen = nil }
}

func TestReadiness_ForceReapplyBehindAStaleCacheStillStartsATimedRollout(t *testing.T) {
	// Given a Deployment Ready for months under a 1s timeout
	s := newReadinessScenarioWith(t, lynqv1.PatchStrategyMerge, 2, 1)
	s.givenDeploymentReadyForMonths(2)

	// And an external edit drifted its replicas to 5 that the Applier's cache has not seen:
	// the cache still holds the replicas Lynq desires
	catchUp := s.freezeApplierCache()
	cachedGeneration := s.deploymentGeneration()
	s.driftReplicasPreservingHash(5)

	// When the periodic force-reapply reverts the drift through that stale cache
	_, failed := s.reconcileForced()
	require.Equal(t, int32(2), *s.liveDeployment().Spec.Replicas, "the force-reapply must revert the drift")
	require.Equal(t, cachedGeneration+2, s.deploymentGeneration(),
		"the drift and its revert are two spec writes past the cached read")
	require.Equal(t, int32(0), failed, "the rollout Lynq just started is within its timeout")

	// And the cache catches up, but that rollout does not become Ready within timeoutSeconds
	catchUp()
	time.Sleep(1100 * time.Millisecond)
	_, failed = s.reconcile()

	// Then the old Ready history does not shield it: it times out
	assert.Equal(t, int32(1), failed)
	assert.NotEmpty(t, eventsWithReason(s.drainEvents(), "ReadinessTimeout"))
}

func TestReadiness_MetadataOnlyApplyBehindAStaleCacheIsConservativelyTimedAsARollout(t *testing.T) {
	// This is the false positive the stale-cache rule accepts: no apply of Lynq's changed the
	// spec, but a generation two past the cached read cannot be told apart from a missed spec
	// write, so the disruption is re-timed from Lynq's apply. Missing a real rollout instead
	// would leave it exempt from timeoutSeconds for good.

	// Given a Deployment Ready for months under a 1s timeout, now short a pod
	s := newReadinessScenarioWith(t, lynqv1.PatchStrategyMerge, 2, 1)
	s.givenDeploymentReadyForMonths(2)
	s.setAvailable(2, 1, 1)

	// And an external annotation edit the Applier's cache has not seen
	catchUp := s.freezeApplierCache()
	cachedGeneration := s.deploymentGeneration()
	specBefore := s.liveDeployment().Spec
	s.mutateDeployment(func(d *appsv1.Deployment) {
		d.Annotations["example.com/owner"] = "payments"
	})

	// When the template only adds a label to the Deployment
	s.addDeploymentLabel("team", "payments")
	_, failed := s.reconcile()
	require.Equal(t, "payments", s.deploymentLabel("team"), "the metadata change must reach the cluster")
	require.Equal(t, specBefore, s.liveDeployment().Spec, "neither write may change the spec")
	require.Equal(t, cachedGeneration+2, s.deploymentGeneration(),
		"the external annotation edit and Lynq's re-stamp each move a Deployment's generation")
	require.Equal(t, int32(0), failed)

	// And the disruption outlasts timeoutSeconds
	catchUp()
	time.Sleep(1100 * time.Millisecond)
	_, failed = s.reconcile()

	// Then it is timed as a rollout Lynq started, and times out
	assert.Equal(t, int32(1), failed)
	assert.NotEmpty(t, eventsWithReason(s.drainEvents(), "ReadinessTimeout"))
}
