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
	"time"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PublishResourceCounts is a helper to publish resource count updates
func (m *Manager) PublishResourceCounts(node *lynqv1.LynqNode, ready, failed, desired, conflicted int32) {
	m.Publish(StatusEvent{
		Type:    EventResourceCountsUpdated,
		NodeKey: client.ObjectKeyFromObject(node),
		Payload: ResourceCountsPayload{
			Ready:      ready,
			Failed:     failed,
			Desired:    desired,
			Conflicted: conflicted,
		},
		Timestamp: time.Now(),
	})
}

// PublishCondition is a helper to publish condition updates
func (m *Manager) PublishCondition(node *lynqv1.LynqNode, conditionType string, status metav1.ConditionStatus, reason, message string) {
	m.Publish(StatusEvent{
		Type:    EventConditionChanged,
		NodeKey: client.ObjectKeyFromObject(node),
		Payload: ConditionPayload{
			Condition: metav1.Condition{
				Type:               conditionType,
				Status:             status,
				Reason:             reason,
				Message:            message,
				LastTransitionTime: metav1.Now(),
			},
		},
		Timestamp: time.Now(),
	})
}

// PublishReadyCondition is a helper to publish the Ready condition
func (m *Manager) PublishReadyCondition(node *lynqv1.LynqNode, isReady bool, reason, message string) {
	status := metav1.ConditionTrue
	if !isReady {
		status = metav1.ConditionFalse
	}
	m.PublishCondition(node, "Ready", status, reason, message)
}

// PublishProgressingCondition is a helper to publish the Progressing condition
func (m *Manager) PublishProgressingCondition(node *lynqv1.LynqNode, isProgressing bool, reason, message string) {
	status := metav1.ConditionTrue
	if !isProgressing {
		status = metav1.ConditionFalse
	}
	if reason == "" {
		reason = "ReconcileComplete"
	}
	if message == "" {
		message = "Reconciliation completed"
	}
	m.PublishCondition(node, "Progressing", status, reason, message)
}

// PublishConflictedCondition is a helper to publish the Conflicted condition
func (m *Manager) PublishConflictedCondition(node *lynqv1.LynqNode, hasConflict bool) {
	status := metav1.ConditionFalse
	reason := "NoConflict"
	message := "No resource conflicts detected"

	if hasConflict {
		status = metav1.ConditionTrue
		reason = "ResourceConflict"
		message = "One or more resources are in conflict. Check events for details."
	}

	m.PublishCondition(node, "Conflicted", status, reason, message)
}

// PublishDegradedCondition is a helper to publish the Degraded condition
func (m *Manager) PublishDegradedCondition(node *lynqv1.LynqNode, isDegraded bool, reason, message string) {
	status := metav1.ConditionFalse
	if isDegraded {
		status = metav1.ConditionTrue
	}
	m.PublishCondition(node, "Degraded", status, reason, message)
}

// PublishObservedGeneration is a helper to publish ObservedGeneration updates
func (m *Manager) PublishObservedGeneration(node *lynqv1.LynqNode, generation int64) {
	m.Publish(StatusEvent{
		Type:    EventObservedGenerationUpdated,
		NodeKey: client.ObjectKeyFromObject(node),
		Payload: ObservedGenerationPayload{
			ObservedGeneration: generation,
		},
		Timestamp: time.Now(),
	})
}

// PublishAppliedResources is a helper to publish applied resources list
func (m *Manager) PublishAppliedResources(node *lynqv1.LynqNode, keys []string) {
	m.Publish(StatusEvent{
		Type:    EventAppliedResourcesUpdated,
		NodeKey: client.ObjectKeyFromObject(node),
		Payload: AppliedResourcesPayload{
			Keys: keys,
		},
		Timestamp: time.Now(),
	})
}

// PublishSkippedResources is a helper to publish skipped resources due to dependency failures
func (m *Manager) PublishSkippedResources(node *lynqv1.LynqNode, count int32, ids []string) {
	m.Publish(StatusEvent{
		Type:    EventSkippedResourcesUpdated,
		NodeKey: client.ObjectKeyFromObject(node),
		Payload: SkippedResourcesPayload{
			Count: count,
			Ids:   ids,
		},
		Timestamp: time.Now(),
	})
}

// PublishMetrics is a helper to publish all metrics at once
func (m *Manager) PublishMetrics(node *lynqv1.LynqNode, ready, failed, desired, conflicted int32, conditions []metav1.Condition, isDegraded bool, degradedReason string) {
	m.Publish(StatusEvent{
		Type:    EventMetricsUpdate,
		NodeKey: client.ObjectKeyFromObject(node),
		Payload: MetricsPayload{
			Ready:          ready,
			Failed:         failed,
			Desired:        desired,
			Conflicted:     conflicted,
			Conditions:     conditions,
			IsDegraded:     isDegraded,
			DegradedReason: degradedReason,
		},
		Timestamp: time.Now(),
	})
}

// PublishFullStatus is a helper to publish all status updates at once
// This is useful at the end of reconciliation to update everything together
func (m *Manager) PublishFullStatus(node *lynqv1.LynqNode, ready, failed, desired, conflicted int32, conditions []metav1.Condition, appliedKeys []string, isDegraded bool, degradedReason string) {
	key := client.ObjectKeyFromObject(node)
	now := time.Now()

	// Publish resource counts
	m.Publish(StatusEvent{
		Type:    EventResourceCountsUpdated,
		NodeKey: key,
		Payload: ResourceCountsPayload{
			Ready:      ready,
			Failed:     failed,
			Desired:    desired,
			Conflicted: conflicted,
		},
		Timestamp: now,
	})

	// Publish all conditions
	for _, cond := range conditions {
		m.Publish(StatusEvent{
			Type:    EventConditionChanged,
			NodeKey: key,
			Payload: ConditionPayload{
				Condition: cond,
			},
			Timestamp: now,
		})
	}

	// Publish applied resources
	if appliedKeys != nil {
		m.Publish(StatusEvent{
			Type:    EventAppliedResourcesUpdated,
			NodeKey: key,
			Payload: AppliedResourcesPayload{
				Keys: appliedKeys,
			},
			Timestamp: now,
		})
	}

	// Publish metrics
	m.Publish(StatusEvent{
		Type:    EventMetricsUpdate,
		NodeKey: key,
		Payload: MetricsPayload{
			Ready:          ready,
			Failed:         failed,
			Desired:        desired,
			Conflicted:     conflicted,
			Conditions:     conditions,
			IsDegraded:     isDegraded,
			DegradedReason: degradedReason,
		},
		Timestamp: now,
	})
}
