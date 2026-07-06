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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/readiness"
)

// TestProcessResourcePhase_DegradedSince is the F2 regression guard: the
// degraded-since clock must be anchored to the phase-transition timestamp
// (DegradedSince), NOT to lynq.sh/apply-start-time. A workload applied long
// ago that degrades now must report a SMALL SinceSeconds, not the whole
// apply-elapsed — otherwise LynqNodeWorkloadSeverelyDegraded (>1800s) misfires.
func TestProcessResourcePhase_DegradedSince(t *testing.T) {
	r := &LynqNodeReconciler{Recorder: &fakeRecorder{}}
	ctx := context.Background()
	res := lynqv1.TResource{ID: "app"}
	degraded := readiness.PhaseResult{Phase: lynqv1.ResourcePhaseDegraded, Reason: "availableReplicas=2/3"}
	available := readiness.PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}

	// A deliberately huge apply-elapsed to prove SinceSeconds does NOT track it.
	const hugeApplyElapsed = 72 * time.Hour

	t.Run("fresh degrade stamps DegradedSince ~now, SinceSeconds ~0", func(t *testing.T) {
		prev := lynqv1.ResourcePhaseEntry{ID: "app", Phase: lynqv1.ResourcePhaseAvailable}
		got := r.processResourcePhase(ctx, nil, res, "app-x", "Deployment", degraded, prev, hugeApplyElapsed)

		if got.DegradedSince == nil {
			t.Fatal("DegradedSince should be stamped on Available→Degraded")
		}
		if got.SinceSeconds > 5 {
			t.Errorf("SinceSeconds = %d, want ~0 (must NOT track the %s apply-elapsed)", got.SinceSeconds, hugeApplyElapsed)
		}
	})

	t.Run("continued degrade carries DegradedSince forward, SinceSeconds accumulates", func(t *testing.T) {
		enteredAt := metav1.NewTime(time.Now().Add(-90 * time.Second))
		// Carry-forward requires the previous entry to describe the SAME
		// concrete object — ID + Kind + Name must all match.
		prev := lynqv1.ResourcePhaseEntry{ID: "app", Kind: "Deployment", Name: "app-x", Phase: lynqv1.ResourcePhaseDegraded, DegradedSince: &enteredAt}
		got := r.processResourcePhase(ctx, nil, res, "app-x", "Deployment", degraded, prev, hugeApplyElapsed)

		if got.DegradedSince == nil || !got.DegradedSince.Equal(&enteredAt) {
			t.Fatalf("DegradedSince should be carried forward from prev (%v), got %v", enteredAt, got.DegradedSince)
		}
		// ~90s since entry — must be near 90, NOT 72h worth of apply-elapsed.
		if got.SinceSeconds < 88 || got.SinceSeconds > 100 {
			t.Errorf("SinceSeconds = %d, want ~90 (anchored to DegradedSince, not apply-elapsed)", got.SinceSeconds)
		}
	})

	t.Run("same ID but different object (renamed) does NOT inherit the old clock", func(t *testing.T) {
		enteredAt := metav1.NewTime(time.Now().Add(-45 * time.Minute))
		// Previous entry: same TResource ID, but the rendered name differs —
		// a template edit produced a NEW concrete object. The old object's
		// degraded clock must not leak onto it (it would instantly trip the
		// >30m severe alert for a workload that just appeared).
		prev := lynqv1.ResourcePhaseEntry{ID: "app", Kind: "Deployment", Name: "app-OLD", Phase: lynqv1.ResourcePhaseDegraded, DegradedSince: &enteredAt}
		got := r.processResourcePhase(ctx, nil, res, "app-x", "Deployment", degraded, prev, hugeApplyElapsed)

		if got.DegradedSince == nil {
			t.Fatal("DegradedSince should be stamped fresh for the new object")
		}
		if got.DegradedSince.Equal(&enteredAt) {
			t.Error("DegradedSince leaked from a different concrete object (same ID, different name)")
		}
		if got.SinceSeconds > 5 {
			t.Errorf("SinceSeconds = %d, want ~0 for a freshly-observed object", got.SinceSeconds)
		}
	})

	t.Run("recovery clears DegradedSince and SinceSeconds", func(t *testing.T) {
		enteredAt := metav1.NewTime(time.Now().Add(-30 * time.Minute))
		prev := lynqv1.ResourcePhaseEntry{ID: "app", Phase: lynqv1.ResourcePhaseDegraded, DegradedSince: &enteredAt}
		got := r.processResourcePhase(ctx, nil, res, "app-x", "Deployment", available, prev, hugeApplyElapsed)

		if got.DegradedSince != nil {
			t.Errorf("DegradedSince should be nil once recovered to Available, got %v", got.DegradedSince)
		}
		if got.SinceSeconds != 0 {
			t.Errorf("SinceSeconds = %d, want 0 on recovery", got.SinceSeconds)
		}
	})
}
