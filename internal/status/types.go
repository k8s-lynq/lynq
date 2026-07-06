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

// EventType represents the type of status event
type EventType string

const (
	// EventResourceCountsUpdated indicates resource counts have changed
	EventResourceCountsUpdated EventType = "ResourceCountsUpdated"

	// EventConditionChanged indicates a condition status has changed
	EventConditionChanged EventType = "ConditionChanged"

	// EventAppliedResourcesUpdated indicates the list of applied resources has changed
	EventAppliedResourcesUpdated EventType = "AppliedResourcesUpdated"

	// EventSkippedResourcesUpdated indicates skipped resources have changed
	EventSkippedResourcesUpdated EventType = "SkippedResourcesUpdated"

	// EventObservedGenerationUpdated indicates ObservedGeneration has changed
	EventObservedGenerationUpdated EventType = "ObservedGenerationUpdated"

	// EventMetricsUpdate indicates metrics should be updated
	EventMetricsUpdate EventType = "MetricsUpdate"

	// EventLastFullReconcileAtUpdated indicates the LastFullReconcileAt timestamp
	// should be updated (used to gate the periodic drift-correction force-reapply)
	EventLastFullReconcileAtUpdated EventType = "LastFullReconcileAtUpdated"

	// EventResourcePhasesUpdated carries the per-resource phase array and the
	// per-resource metric payloads (replica counters + degraded-since-seconds)
	// for one reconcile pass. Aggregate counts (degraded/progressing/pending)
	// flow through EventResourceCountsUpdated / EventMetricsUpdate as before.
	EventResourcePhasesUpdated EventType = "ResourcePhasesUpdated"
)

// StatusEvent represents a status change event for a LynqNode
type StatusEvent struct {
	// Type is the event type
	Type EventType

	// NodeKey is the namespaced name of the LynqNode
	NodeKey client.ObjectKey

	// Payload contains event-specific data
	Payload interface{}

	// Timestamp is when the event was created
	Timestamp time.Time
}

// ResourceCountsPayload contains resource count information.
// Degraded, Progressing, Pending are the new per-phase counts; existing
// callers that leave them at zero get the old behavior (no per-phase
// reporting in status / metrics).
type ResourceCountsPayload struct {
	Ready       int32
	Failed      int32
	Desired     int32
	Conflicted  int32
	Degraded    int32
	Progressing int32
	Pending     int32
}

// ConditionPayload contains condition information
type ConditionPayload struct {
	Condition metav1.Condition
}

// AppliedResourcesPayload contains the list of applied resource keys
type AppliedResourcesPayload struct {
	Keys []string
}

// SkippedResourcesPayload contains information about skipped resources
type SkippedResourcesPayload struct {
	Count int32
	Ids   []string
}

// ObservedGenerationPayload contains ObservedGeneration information.
// VariablesHash, when non-nil, also records the template-variable annotation
// hash observed at this full reconcile (M2) — used to detect Hub-driven
// variable changes that don't bump metadata.generation.
type ObservedGenerationPayload struct {
	ObservedGeneration int64
	VariablesHash      *string
}

// LastFullReconcileAtPayload contains the timestamp of the most recent force-reapply
type LastFullReconcileAtPayload struct {
	Timestamp metav1.Time
}

// MetricsPayload contains metrics update information.
// Degraded, Progressing, Pending are the new aggregate gauges; existing
// callers that leave them at zero get the old behavior.
type MetricsPayload struct {
	Ready          int32
	Failed         int32
	Desired        int32
	Conflicted     int32
	Degraded       int32
	Progressing    int32
	Pending        int32
	Conditions     []metav1.Condition
	IsDegraded     bool
	DegradedReason string
}

// ResourceReplicaMetrics carries the per-resource workload counters consumed
// by lynqnode_resource_replicas_* gauges. Zero-valued for non-workload kinds.
type ResourceReplicaMetrics struct {
	Kind                 string
	Desired              int64
	Available            int64
	Ready                int64
	Updated              int64
	DegradedSinceSeconds int64
}

// ResourcePhasesPayload bundles a per-reconcile snapshot of per-resource
// phase data: the status.resourcePhases array (source of truth for kubectl
// jsonpath queries), the degraded resource ID list (status field), and
// per-resource metric payloads keyed by resource ID.
type ResourcePhasesPayload struct {
	Phases              []lynqv1.ResourcePhaseEntry
	DegradedResourceIds []string
	// ResourceReplicas is indexed by resource ID — matches Phases[*].ID.
	ResourceReplicas map[string]ResourceReplicaMetrics
}

// StatusUpdate represents accumulated status changes for a single LynqNode
type StatusUpdate struct {
	// Key is the LynqNode's namespaced name
	Key client.ObjectKey

	// Generation to update
	ObservedGeneration *int64

	// ObservedVariablesHash to update (nil means no update) — M2.
	ObservedVariablesHash *string

	// Resource counts (nil means no update)
	ReadyResources       *int32
	FailedResources      *int32
	DesiredResources     *int32
	DegradedResources    *int32
	ProgressingResources *int32
	PendingResources     *int32

	// Applied resources (nil means no update)
	AppliedResources []string

	// Skipped resources (nil means no update)
	SkippedResources   *int32
	SkippedResourceIds []string

	// Per-resource phase array — when non-nil, replaces status.resourcePhases
	// wholesale. Non-nil status.resourcePhases here also clears the array if
	// empty (i.e., zero resources observed); pair with len() checks if you
	// need to distinguish "no update" from "empty array".
	ResourcePhases []lynqv1.ResourcePhaseEntry

	// DegradedResourceIds — when non-nil, replaces status.degradedResourceIds.
	DegradedResourceIds []string

	// ResourceReplicaMetrics is the per-resource metric snapshot for this
	// reconcile, indexed by resource ID. Drives the per-resource gauges
	// (lynqnode_resource_replicas_*, lynqnode_resource_degraded_since_seconds)
	// and the phase stateset (lynqnode_resource_phase).
	ResourceReplicaMetrics map[string]ResourceReplicaMetrics

	// Conditions to update (map by type for deduplication)
	Conditions map[string]metav1.Condition

	// Metrics to update
	Metrics *MetricsPayload

	// LastFullReconcileAt to update (nil means no update)
	LastFullReconcileAt *metav1.Time

	// Timestamp of the last event in this update
	LastEventTime time.Time
}

// NewStatusUpdate creates a new StatusUpdate for a LynqNode
func NewStatusUpdate(key client.ObjectKey) *StatusUpdate {
	return &StatusUpdate{
		Key:        key,
		Conditions: make(map[string]metav1.Condition),
	}
}

// Apply applies an event to this status update
func (u *StatusUpdate) Apply(event StatusEvent) {
	u.LastEventTime = event.Timestamp

	switch event.Type {
	case EventResourceCountsUpdated:
		payload := event.Payload.(ResourceCountsPayload)
		u.ReadyResources = &payload.Ready
		u.FailedResources = &payload.Failed
		u.DesiredResources = &payload.Desired
		// Always set the per-phase counts. The previous "only when non-zero"
		// guard prevented progressingResources from RESETTING to 0 after a
		// rollout completed: the new payload arrived with progressing=0 and
		// the guard skipped the write, leaving the previous value (e.g.,
		// "1" during rollout) stale on status. Backwards-compat callers
		// that don't supply phase counts implicitly pass zeros, which is
		// the correct "no phase activity" state.
		d := payload.Degraded
		p := payload.Progressing
		pe := payload.Pending
		u.DegradedResources = &d
		u.ProgressingResources = &p
		u.PendingResources = &pe

	case EventConditionChanged:
		payload := event.Payload.(ConditionPayload)
		// Use map to deduplicate conditions by type (last write wins)
		u.Conditions[payload.Condition.Type] = payload.Condition

	case EventAppliedResourcesUpdated:
		payload := event.Payload.(AppliedResourcesPayload)
		u.AppliedResources = payload.Keys

	case EventSkippedResourcesUpdated:
		payload := event.Payload.(SkippedResourcesPayload)
		u.SkippedResources = &payload.Count
		u.SkippedResourceIds = payload.Ids

	case EventObservedGenerationUpdated:
		payload := event.Payload.(ObservedGenerationPayload)
		u.ObservedGeneration = &payload.ObservedGeneration
		if payload.VariablesHash != nil {
			u.ObservedVariablesHash = payload.VariablesHash
		}

	case EventMetricsUpdate:
		payload := event.Payload.(MetricsPayload)
		u.Metrics = &payload

	case EventLastFullReconcileAtUpdated:
		payload := event.Payload.(LastFullReconcileAtPayload)
		ts := payload.Timestamp
		u.LastFullReconcileAt = &ts

	case EventResourcePhasesUpdated:
		payload := event.Payload.(ResourcePhasesPayload)
		u.ResourcePhases = payload.Phases
		u.DegradedResourceIds = payload.DegradedResourceIds
		u.ResourceReplicaMetrics = payload.ResourceReplicas
	}
}

// HasChanges returns true if this update has any changes
func (u *StatusUpdate) HasChanges() bool {
	return u.ObservedGeneration != nil ||
		u.ObservedVariablesHash != nil ||
		u.ReadyResources != nil ||
		u.FailedResources != nil ||
		u.DesiredResources != nil ||
		u.DegradedResources != nil ||
		u.ProgressingResources != nil ||
		u.PendingResources != nil ||
		u.AppliedResources != nil ||
		u.SkippedResources != nil ||
		u.SkippedResourceIds != nil ||
		u.ResourcePhases != nil ||
		u.DegradedResourceIds != nil ||
		u.ResourceReplicaMetrics != nil ||
		len(u.Conditions) > 0 ||
		u.Metrics != nil ||
		u.LastFullReconcileAt != nil
}
