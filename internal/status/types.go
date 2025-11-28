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

// ResourceCountsPayload contains resource count information
type ResourceCountsPayload struct {
	Ready      int32
	Failed     int32
	Desired    int32
	Conflicted int32
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

// ObservedGenerationPayload contains ObservedGeneration information
type ObservedGenerationPayload struct {
	ObservedGeneration int64
}

// MetricsPayload contains metrics update information
type MetricsPayload struct {
	Ready          int32
	Failed         int32
	Desired        int32
	Conflicted     int32
	Conditions     []metav1.Condition
	IsDegraded     bool
	DegradedReason string
}

// StatusUpdate represents accumulated status changes for a single LynqNode
type StatusUpdate struct {
	// Key is the LynqNode's namespaced name
	Key client.ObjectKey

	// Generation to update
	ObservedGeneration *int64

	// Resource counts (nil means no update)
	ReadyResources   *int32
	FailedResources  *int32
	DesiredResources *int32

	// Applied resources (nil means no update)
	AppliedResources []string

	// Skipped resources (nil means no update)
	SkippedResources   *int32
	SkippedResourceIds []string

	// Conditions to update (map by type for deduplication)
	Conditions map[string]metav1.Condition

	// Metrics to update
	Metrics *MetricsPayload

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

	case EventMetricsUpdate:
		payload := event.Payload.(MetricsPayload)
		u.Metrics = &payload
	}
}

// HasChanges returns true if this update has any changes
func (u *StatusUpdate) HasChanges() bool {
	return u.ObservedGeneration != nil ||
		u.ReadyResources != nil ||
		u.FailedResources != nil ||
		u.DesiredResources != nil ||
		u.AppliedResources != nil ||
		u.SkippedResources != nil ||
		u.SkippedResourceIds != nil ||
		len(u.Conditions) > 0 ||
		u.Metrics != nil
}
