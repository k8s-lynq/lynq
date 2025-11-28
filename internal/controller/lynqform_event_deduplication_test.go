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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
)

// TestLynqFormEventDeduplication tests that ValidationPassed/ValidationFailed
// events are only fired when the validation state actually changes.
// This prevents event log flooding with redundant validation events.
func TestLynqFormEventDeduplication(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	tests := []struct {
		name                   string
		existingConditions     []metav1.Condition
		hubExists              bool
		expectValidationPassed bool
		expectValidationFailed bool
		description            string
	}{
		{
			name:                   "should fire ValidationPassed when no previous Valid condition exists",
			existingConditions:     []metav1.Condition{},
			hubExists:              true,
			expectValidationPassed: true,
			expectValidationFailed: false,
			description:            "First successful validation should emit event",
		},
		{
			name: "should NOT fire ValidationPassed when already valid",
			existingConditions: []metav1.Condition{
				{
					Type:               ConditionTypeValid,
					Status:             metav1.ConditionTrue,
					Reason:             "ValidationPassed",
					LastTransitionTime: metav1.Now(),
				},
			},
			hubExists:              true,
			expectValidationPassed: false,
			expectValidationFailed: false,
			description:            "Repeated successful validation should NOT emit event",
		},
		{
			name: "should fire ValidationFailed when transitioning from valid to invalid",
			existingConditions: []metav1.Condition{
				{
					Type:               ConditionTypeValid,
					Status:             metav1.ConditionTrue,
					Reason:             "ValidationPassed",
					LastTransitionTime: metav1.Now(),
				},
			},
			hubExists:              false, // Hub doesn't exist, validation will fail
			expectValidationPassed: false,
			expectValidationFailed: true,
			description:            "Transition from valid to invalid should emit event",
		},
		{
			name: "should NOT fire ValidationFailed when already invalid",
			existingConditions: []metav1.Condition{
				{
					Type:               ConditionTypeValid,
					Status:             metav1.ConditionFalse,
					Reason:             "ValidationFailed",
					LastTransitionTime: metav1.Now(),
				},
			},
			hubExists:              false,
			expectValidationPassed: false,
			expectValidationFailed: false,
			description:            "Repeated failed validation should NOT emit event",
		},
		{
			name: "should fire ValidationPassed when transitioning from invalid to valid",
			existingConditions: []metav1.Condition{
				{
					Type:               ConditionTypeValid,
					Status:             metav1.ConditionFalse,
					Reason:             "ValidationFailed",
					LastTransitionTime: metav1.Now(),
				},
			},
			hubExists:              true,
			expectValidationPassed: true,
			expectValidationFailed: false,
			description:            "Transition from invalid to valid should emit event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake recorder to capture events
			recorder := record.NewFakeRecorder(10)

			// Build the LynqForm with existing conditions
			form := &lynqv1.LynqForm{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-form",
					Namespace: "default",
				},
				Spec: lynqv1.LynqFormSpec{
					HubID: "test-hub",
				},
				Status: lynqv1.LynqFormStatus{
					Conditions: tt.existingConditions,
				},
			}

			// Build objects for fake client
			objects := []runtime.Object{form}

			// Add hub if it should exist
			if tt.hubExists {
				hub := &lynqv1.LynqHub{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-hub",
						Namespace: "default",
					},
					Spec: lynqv1.LynqHubSpec{
						Source: lynqv1.DataSource{
							Type:         lynqv1.SourceTypeMySQL,
							SyncInterval: "30s",
						},
					},
				}
				objects = append(objects, hub)
			}

			// Create fake client
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objects...).
				WithStatusSubresource(&lynqv1.LynqForm{}).
				Build()

			// Create reconciler
			r := &LynqFormReconciler{
				Client:   fakeClient,
				Scheme:   scheme,
				Recorder: recorder,
			}

			// Run reconcile
			ctx := context.Background()
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-form",
					Namespace: "default",
				},
			})

			// We don't check error here because validation failure is expected in some cases
			_ = err

			// Drain events from recorder
			close(recorder.Events)
			var events []string
			for event := range recorder.Events {
				events = append(events, event)
			}

			// Check for ValidationPassed event
			hasValidationPassed := false
			hasValidationFailed := false
			for _, event := range events {
				if strings.Contains(event, "ValidationPassed") {
					hasValidationPassed = true
				}
				if strings.Contains(event, "ValidationFailed") {
					hasValidationFailed = true
				}
			}

			assert.Equal(t, tt.expectValidationPassed, hasValidationPassed,
				"ValidationPassed event expectation failed for: %s. Events: %v",
				tt.description, events)
			assert.Equal(t, tt.expectValidationFailed, hasValidationFailed,
				"ValidationFailed event expectation failed for: %s. Events: %v",
				tt.description, events)
		})
	}
}

// TestLynqFormEventDeduplication_MultipleReconciles verifies that
// multiple consecutive reconciles with no state change do not
// produce duplicate events.
func TestLynqFormEventDeduplication_MultipleReconciles(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))

	// Create hub and form
	hub := &lynqv1.LynqHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hub",
			Namespace: "default",
		},
		Spec: lynqv1.LynqHubSpec{
			Source: lynqv1.DataSource{
				Type:         lynqv1.SourceTypeMySQL,
				SyncInterval: "30s",
			},
		},
	}

	form := &lynqv1.LynqForm{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-form",
			Namespace: "default",
		},
		Spec: lynqv1.LynqFormSpec{
			HubID: "test-hub",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(hub, form).
		WithStatusSubresource(&lynqv1.LynqForm{}).
		Build()

	recorder := record.NewFakeRecorder(100)
	r := &LynqFormReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: recorder,
	}

	ctx := context.Background()
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-form",
			Namespace: "default",
		},
	}

	// Run reconcile multiple times (simulating periodic requeue)
	for i := 0; i < 5; i++ {
		_, err := r.Reconcile(ctx, req)
		require.NoError(t, err)
		// Small delay to simulate real-world timing
		time.Sleep(10 * time.Millisecond)
	}

	// Drain events
	close(recorder.Events)
	validationPassedCount := 0
	for event := range recorder.Events {
		if strings.Contains(event, "ValidationPassed") {
			validationPassedCount++
		}
	}

	// Should only have 1 ValidationPassed event from the first reconcile
	// Subsequent reconciles should NOT emit the event
	assert.Equal(t, 1, validationPassedCount,
		"Expected exactly 1 ValidationPassed event after 5 reconciles, but got %d",
		validationPassedCount)
}
