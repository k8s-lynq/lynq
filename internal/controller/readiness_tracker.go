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
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
)

// readinessTracker remembers, per LynqNode child resource, what this process has observed
// about its readiness. It is the input that separates the two meanings of "not ready":
//
//   - The resource never reached Ready after Lynq applied it. That is what timeoutSeconds
//     measures, and exceeding it marks the resource failed.
//   - The resource reached Ready after the current apply and later regressed (a pod lost to a
//     node drain, an external `kubectl rollout restart`). Lynq did not cause that and the
//     workload controller is already recovering it, so it is reported as not-ready, never
//     timed out.
//
// The state is deliberately in-memory only. Persisting it would need a write to the child or
// to the LynqNode on every readiness transition, which breaks the "a reconcile that changes
// nothing MUST NOT write" and "exactly one API write per apply" invariants. Losing it on
// restart is safe: without history a not-ready resource gets one full timeoutSeconds of grace
// from the moment it is first seen, so a restart can delay failure detection but never cause
// a false one.
type readinessTracker struct {
	nodes sync.Map // key: LynqNode UID → *nodeReadiness
	// byName maps a LynqNode's namespaced name to the UID currently tracked for it. A LynqNode
	// deleted and recreated under the same name while no reconcile observed the gap (no
	// NotFound, no finalizer cleanup) would otherwise leave the old UID's entry behind forever.
	byName sync.Map // key: types.NamespacedName → types.UID
}

// nodeReadiness holds the tracked resources of one LynqNode.
type nodeReadiness struct {
	key       types.NamespacedName
	mu        sync.Mutex
	resources map[string]*resourceReadiness // key: TResource.ID
}

// resourceReadiness is the observation history of one child resource.
type resourceReadiness struct {
	// readyApplyStamp is the lynq.sh/apply-start-time value the resource carried when it was
	// last observed Ready. It only matters for resources that do not track metadata.generation
	// (see observeNotReady); for the rest, startRollout is what ends a Ready history.
	readyApplyStamp string
	observedReady   bool
	// notReadySince is when the resource was first observed not ready in its current
	// not-ready stretch. Zero while the resource is Ready.
	notReadySince time.Time
	// rolloutGeneration is the metadata.generation an apply of ours last moved the resource
	// to. A Ready observation of an older generation is a stale read of the previous rollout
	// and does not count.
	rolloutGeneration int64
}

// node returns the tracked state for a LynqNode, creating it on first use. Creating the
// state for a new UID drops whatever was tracked for an earlier LynqNode of the same name.
func (t *readinessTracker) node(uid types.UID, key types.NamespacedName) *nodeReadiness {
	if v, ok := t.nodes.Load(uid); ok {
		return v.(*nodeReadiness)
	}
	v, loaded := t.nodes.LoadOrStore(uid, &nodeReadiness{key: key, resources: map[string]*resourceReadiness{}})
	if !loaded {
		if prev, ok := t.byName.Swap(key, uid); ok && prev.(types.UID) != uid {
			t.nodes.Delete(prev)
		}
	}
	return v.(*nodeReadiness)
}

// startRollout records that an apply of ours just moved the resource to a new generation, at
// the given instant. Lynq has started a rollout, so any earlier Ready observation no longer
// says anything about it: the resource is once again "never Ready since Lynq changed it", and
// its timeout clock starts no earlier than this apply.
func (t *readinessTracker) startRollout(uid types.UID, key types.NamespacedName, resourceID string, at time.Time, generation int64) {
	n := t.node(uid, key)
	n.mu.Lock()
	defer n.mu.Unlock()
	n.resources[resourceID] = &resourceReadiness{notReadySince: at, rolloutGeneration: generation}
}

// observeReady records that the resource is Ready under the given apply stamp and ends any
// not-ready stretch. It returns false, recording nothing, when the observed generation is
// older than the rollout we last started: that read predates our own apply.
func (t *readinessTracker) observeReady(uid types.UID, key types.NamespacedName, resourceID, applyStamp string, generation int64) bool {
	n := t.node(uid, key)
	n.mu.Lock()
	defer n.mu.Unlock()
	var rolloutGeneration int64
	if st, ok := n.resources[resourceID]; ok {
		rolloutGeneration = st.rolloutGeneration
	}
	if generation < rolloutGeneration {
		return false
	}
	n.resources[resourceID] = &resourceReadiness{readyApplyStamp: applyStamp, observedReady: true, rolloutGeneration: rolloutGeneration}
	return true
}

// observeNotReady records that the resource is not ready and reports how to judge it.
//
// regressed is true when the resource was observed Ready since Lynq last changed it, i.e. it
// is a steady-state disruption rather than a rollout that never finished. notReadySince is the
// start of the current not-ready stretch, which bounds the readiness-timeout clock for
// resources this process has no Ready history for.
//
// For a resource that tracks metadata.generation (generation > 0), only an apply of ours that
// moved the generation ends its Ready history, and startRollout already records that. The
// apply stamp is deliberately not compared: it also moves on a metadata-only apply (a label or
// annotation added to the template), which starts no rollout, so a disruption in progress at
// that moment would otherwise be re-timed from the new stamp and falsely fail. A resource
// without a generation offers no such signal, so a changed stamp is the only evidence that
// Lynq applied something new and still ends its Ready history.
func (t *readinessTracker) observeNotReady(uid types.UID, key types.NamespacedName, resourceID, applyStamp string, generation int64, now time.Time) (regressed bool, notReadySince time.Time) {
	n := t.node(uid, key)
	n.mu.Lock()
	defer n.mu.Unlock()
	st, ok := n.resources[resourceID]
	if !ok {
		st = &resourceReadiness{}
		n.resources[resourceID] = st
	}
	if st.notReadySince.IsZero() {
		st.notReadySince = now
	}
	if generation > 0 {
		return st.observedReady, st.notReadySince
	}
	return st.observedReady && st.readyApplyStamp == applyStamp, st.notReadySince
}

// retain drops tracked resources that are no longer part of the LynqNode's template.
func (t *readinessTracker) retain(uid types.UID, liveIDs map[string]struct{}) {
	v, ok := t.nodes.Load(uid)
	if !ok {
		return
	}
	n := v.(*nodeReadiness)
	n.mu.Lock()
	defer n.mu.Unlock()
	for id := range n.resources {
		if _, ok := liveIDs[id]; !ok {
			delete(n.resources, id)
		}
	}
}

// forgetNode drops all state for a LynqNode that is being deleted.
func (t *readinessTracker) forgetNode(uid types.UID) {
	if v, ok := t.nodes.LoadAndDelete(uid); ok {
		t.byName.CompareAndDelete(v.(*nodeReadiness).key, uid)
	}
}

// forgetByName drops all state for a LynqNode that is gone from the API server. Used when a
// reconcile finds the node NotFound and therefore has no UID to key on.
func (t *readinessTracker) forgetByName(key types.NamespacedName) {
	if uid, ok := t.byName.LoadAndDelete(key); ok {
		t.nodes.Delete(uid)
	}
}
