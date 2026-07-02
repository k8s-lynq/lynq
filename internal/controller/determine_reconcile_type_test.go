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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
)

// TestDetermineReconcileType_M2 verifies the M2 routing: pure child-status
// events take the lightweight status path, while spec changes, Hub-driven
// variable changes, failures, and the periodic drift sweep take the full path.
func TestDetermineReconcileType_M2(t *testing.T) {
	r := &LynqNodeReconciler{}
	recent := metav1.NewTime(time.Now().Add(-1 * time.Minute))
	stale := metav1.NewTime(time.Now().Add(-2 * ForceReapplyInterval))

	// healthyStatusOnly returns a node that is fully reconciled and only
	// receiving a child-status update: finalizer present, generation observed,
	// variable hash matches, no failures/degradation, recent full reconcile.
	healthyStatusOnly := func() *lynqv1.LynqNode {
		n := &lynqv1.LynqNode{}
		n.Name = "n"
		// Namespace intentionally unset — determineReconcileType does not read
		// it, and avoiding the "default" literal keeps goconst quiet.
		n.Generation = 5
		n.Finalizers = []string{LynqNodeFinalizer}
		n.Annotations = map[string]string{
			"lynq.sh/uid":      "acme",
			"lynq.sh/activate": "true",
			"lynq.sh/extra":    `{"planId":"pro"}`,
			"lynq.sh/hubId":    "hub",
		}
		n.Status.ObservedGeneration = 5
		n.Status.ObservedVariablesHash = computeVariablesHash(n)
		n.Status.LastFullReconcileAt = &recent
		return n
	}

	t.Run("pure status event routes to Status", func(t *testing.T) {
		if got := r.determineReconcileType(healthyStatusOnly()); got != ReconcileTypeStatus {
			t.Errorf("got %v, want ReconcileTypeStatus", got)
		}
	})

	t.Run("generation mismatch routes to Spec", func(t *testing.T) {
		n := healthyStatusOnly()
		n.Generation = 6 // spec changed, observedGeneration still 5
		if got := r.determineReconcileType(n); got != ReconcileTypeSpec {
			t.Errorf("got %v, want ReconcileTypeSpec", got)
		}
	})

	t.Run("Hub-driven variable change routes to Spec", func(t *testing.T) {
		n := healthyStatusOnly()
		n.Annotations["lynq.sh/activate"] = "false" // Hub rewrote a var, no gen bump
		if got := r.determineReconcileType(n); got != ReconcileTypeSpec {
			t.Errorf("got %v, want ReconcileTypeSpec (variable hash mismatch)", got)
		}
	})

	t.Run("failed resources route to Spec", func(t *testing.T) {
		n := healthyStatusOnly()
		n.Status.FailedResources = 1
		if got := r.determineReconcileType(n); got != ReconcileTypeSpec {
			t.Errorf("got %v, want ReconcileTypeSpec", got)
		}
	})

	t.Run("Degraded condition routes to Spec", func(t *testing.T) {
		n := healthyStatusOnly()
		n.Status.Conditions = []metav1.Condition{
			{Type: ConditionTypeDegraded, Status: metav1.ConditionTrue, Reason: "ResourceFailures"},
		}
		if got := r.determineReconcileType(n); got != ReconcileTypeSpec {
			t.Errorf("got %v, want ReconcileTypeSpec", got)
		}
	})

	t.Run("nil LastFullReconcileAt routes to Spec (establish baseline)", func(t *testing.T) {
		n := healthyStatusOnly()
		n.Status.LastFullReconcileAt = nil
		if got := r.determineReconcileType(n); got != ReconcileTypeSpec {
			t.Errorf("got %v, want ReconcileTypeSpec", got)
		}
	})

	t.Run("stale LastFullReconcileAt routes to Spec (force-reapply backstop)", func(t *testing.T) {
		n := healthyStatusOnly()
		n.Status.LastFullReconcileAt = &stale
		if got := r.determineReconcileType(n); got != ReconcileTypeSpec {
			t.Errorf("got %v, want ReconcileTypeSpec", got)
		}
	})

	t.Run("missing finalizer routes to Init", func(t *testing.T) {
		n := healthyStatusOnly()
		n.Finalizers = nil
		if got := r.determineReconcileType(n); got != ReconcileTypeInit {
			t.Errorf("got %v, want ReconcileTypeInit", got)
		}
	})

	t.Run("deletion timestamp routes to Cleanup", func(t *testing.T) {
		n := healthyStatusOnly()
		now := metav1.Now()
		n.DeletionTimestamp = &now
		if got := r.determineReconcileType(n); got != ReconcileTypeCleanup {
			t.Errorf("got %v, want ReconcileTypeCleanup", got)
		}
	})
}
