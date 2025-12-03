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

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
)

// TestDetermineReconcileType tests the determineReconcileType function
// to ensure correct reconcile path selection based on node state.
//
// This test prevents regression of the log/event noise fix where:
// - ObservedGeneration tracking prevents repeated full reconciles
// - Degraded/failed states trigger full reconcile for recovery
func TestDetermineReconcileType(t *testing.T) {
	tests := []struct {
		name           string
		node           *lynqv1.LynqNode
		expectedType   ReconcileType
		expectedReason string
	}{
		{
			name: "should return Spec type when generation differs from observedGeneration",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-node",
					Namespace:  "default",
					Generation: 2,
					Finalizers: []string{LynqNodeFinalizer},
				},
				Status: lynqv1.LynqNodeStatus{
					ObservedGeneration: 1,
					FailedResources:    0,
				},
			},
			expectedType:   ReconcileTypeSpec,
			expectedReason: "generation (2) != observedGeneration (1)",
		},
		{
			name: "should return Spec type when generation equals observedGeneration for annotation change detection",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-node",
					Namespace:  "default",
					Generation: 1,
					Finalizers: []string{LynqNodeFinalizer},
				},
				Status: lynqv1.LynqNodeStatus{
					ObservedGeneration: 1,
					FailedResources:    0,
					Conditions:         []metav1.Condition{},
				},
			},
			expectedType:   ReconcileTypeSpec,
			expectedReason: "always full reconcile to detect annotation changes (template variable updates)",
		},
		{
			name: "should return Spec type when Degraded condition is True (for recovery)",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-node",
					Namespace:  "default",
					Generation: 1,
					Finalizers: []string{LynqNodeFinalizer},
				},
				Status: lynqv1.LynqNodeStatus{
					ObservedGeneration: 1,
					FailedResources:    0,
					Conditions: []metav1.Condition{
						{
							Type:   ConditionTypeDegraded,
							Status: metav1.ConditionTrue,
							Reason: "ConflictDetected",
						},
					},
				},
			},
			expectedType:   ReconcileTypeSpec,
			expectedReason: "Degraded=True triggers full reconcile for conflict recovery",
		},
		{
			name: "should return Spec type when failedResources > 0 (for retry)",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-node",
					Namespace:  "default",
					Generation: 1,
					Finalizers: []string{LynqNodeFinalizer},
				},
				Status: lynqv1.LynqNodeStatus{
					ObservedGeneration: 1,
					FailedResources:    2,
					Conditions:         []metav1.Condition{},
				},
			},
			expectedType:   ReconcileTypeSpec,
			expectedReason: "failedResources > 0 triggers full reconcile for retry",
		},
		{
			name: "should return Init type when finalizer is missing",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-node",
					Namespace:  "default",
					Generation: 1,
					Finalizers: []string{}, // No finalizer
				},
				Status: lynqv1.LynqNodeStatus{
					ObservedGeneration: 0,
				},
			},
			expectedType:   ReconcileTypeInit,
			expectedReason: "missing finalizer requires initialization",
		},
		{
			name: "should return Cleanup type when deletion timestamp is set",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-node",
					Namespace:         "default",
					Generation:        1,
					Finalizers:        []string{LynqNodeFinalizer},
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Status: lynqv1.LynqNodeStatus{
					ObservedGeneration: 1,
				},
			},
			expectedType:   ReconcileTypeCleanup,
			expectedReason: "deletion timestamp set triggers cleanup",
		},
		{
			name: "should return Spec type even when Degraded condition is False for annotation change detection",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-node",
					Namespace:  "default",
					Generation: 1,
					Finalizers: []string{LynqNodeFinalizer},
				},
				Status: lynqv1.LynqNodeStatus{
					ObservedGeneration: 1,
					FailedResources:    0,
					Conditions: []metav1.Condition{
						{
							Type:   ConditionTypeDegraded,
							Status: metav1.ConditionFalse,
							Reason: "AllResourcesHealthy",
						},
					},
				},
			},
			expectedType:   ReconcileTypeSpec,
			expectedReason: "always full reconcile to detect annotation changes (template variable updates)",
		},
		{
			name: "should return Spec type when both Degraded and failedResources indicate problems",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-node",
					Namespace:  "default",
					Generation: 1,
					Finalizers: []string{LynqNodeFinalizer},
				},
				Status: lynqv1.LynqNodeStatus{
					ObservedGeneration: 1,
					FailedResources:    3,
					Conditions: []metav1.Condition{
						{
							Type:   ConditionTypeDegraded,
							Status: metav1.ConditionTrue,
							Reason: "ResourcesFailed",
						},
					},
				},
			},
			expectedType:   ReconcileTypeSpec,
			expectedReason: "multiple failure indicators should trigger full reconcile",
		},
		{
			name: "should return Spec type for fresh node (generation=1, observedGeneration=0)",
			node: &lynqv1.LynqNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-node",
					Namespace:  "default",
					Generation: 1,
					Finalizers: []string{LynqNodeFinalizer},
				},
				Status: lynqv1.LynqNodeStatus{
					ObservedGeneration: 0, // Not yet reconciled
					FailedResources:    0,
				},
			},
			expectedType:   ReconcileTypeSpec,
			expectedReason: "fresh node with observedGeneration=0 needs initial reconcile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &LynqNodeReconciler{}

			result := r.determineReconcileType(tt.node)

			assert.Equal(t, tt.expectedType, result,
				"Expected %v for case: %s", tt.expectedType, tt.expectedReason)
		})
	}
}

// TestDetermineReconcileType_AlwaysFullReconcileForAnnotationChanges verifies that
// determineReconcileType always returns Spec type to ensure annotation changes
// (template variable updates from database) are properly applied.
// This is critical for database-driven configuration synchronization.
func TestDetermineReconcileType_AlwaysFullReconcileForAnnotationChanges(t *testing.T) {
	r := &LynqNodeReconciler{}

	// Simulate a node that has been successfully reconciled
	// Even though spec hasn't changed, annotations may have changed
	// (e.g., lynq.sh/extra updated by LynqHub controller due to DB changes)
	stableNode := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "stable-node",
			Namespace:  "default",
			Generation: 5, // Has been updated several times
			Finalizers: []string{LynqNodeFinalizer},
		},
		Status: lynqv1.LynqNodeStatus{
			ObservedGeneration: 5, // Matches generation - but annotations may have changed
			FailedResources:    0,
			ReadyResources:     3,
			DesiredResources:   3,
			Conditions: []metav1.Condition{
				{
					Type:   ConditionTypeReady,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}

	// This should return Spec type to ensure annotation changes are applied
	// Template variables are stored in annotations and don't change generation
	result := r.determineReconcileType(stableNode)

	assert.Equal(t, ReconcileTypeSpec, result,
		"Always full reconcile to detect annotation changes (template variable updates from database)")
}

// TestDetermineReconcileType_ConflictRecovery verifies that nodes in
// Degraded state due to conflicts will get full reconcile for recovery.
// This prevents regression of the E2E test fix for conflict recovery.
func TestDetermineReconcileType_ConflictRecovery(t *testing.T) {
	r := &LynqNodeReconciler{}

	// Simulate a node that had a conflict and is now Degraded
	// Even though generation == observedGeneration, it should still
	// use full reconcile to retry creating the conflicted resource
	conflictedNode := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "conflicted-node",
			Namespace:  "default",
			Generation: 3,
			Finalizers: []string{LynqNodeFinalizer},
		},
		Status: lynqv1.LynqNodeStatus{
			ObservedGeneration: 3, // Matches - but still degraded
			FailedResources:    0,
			ReadyResources:     2,
			DesiredResources:   3,
			Conditions: []metav1.Condition{
				{
					Type:    ConditionTypeDegraded,
					Status:  metav1.ConditionTrue,
					Reason:  "ConflictDetected",
					Message: "Resource ownership conflict detected",
				},
				{
					Type:   ConditionTypeReady,
					Status: metav1.ConditionFalse,
				},
			},
		},
	}

	result := r.determineReconcileType(conflictedNode)

	assert.Equal(t, ReconcileTypeSpec, result,
		"Degraded node should use Spec path for conflict recovery even when generation matches")
}
