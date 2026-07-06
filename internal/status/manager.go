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
	"context"
	"fmt"
	"sync"
	"time"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/metrics"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// DefaultBatchSize is the default number of nodes to batch before flushing
	DefaultBatchSize = 10

	// DefaultFlushInterval is the default time interval to flush batches
	DefaultFlushInterval = 1 * time.Second

	// DefaultEventBufferSize is the default size of the event channel buffer
	DefaultEventBufferSize = 100
)

// Manager manages status updates for nodes using an event-driven approach
// It collects status events, aggregates them, and performs batch updates to minimize API calls
type Manager struct {
	client        client.Client
	events        chan StatusEvent
	stopCh        chan struct{}
	wg            sync.WaitGroup
	batchSize     int
	flushInterval time.Duration

	// For testing - allows synchronous mode
	syncMode  bool
	syncMutex sync.Mutex
}

// ManagerOption is a function that configures a Manager
type ManagerOption func(*Manager)

// WithBatchSize sets the batch size for the manager
func WithBatchSize(size int) ManagerOption {
	return func(m *Manager) {
		m.batchSize = size
	}
}

// WithFlushInterval sets the flush interval for the manager
func WithFlushInterval(interval time.Duration) ManagerOption {
	return func(m *Manager) {
		m.flushInterval = interval
	}
}

// WithEventBufferSize sets the event buffer size for the manager
func WithEventBufferSize(size int) ManagerOption {
	return func(m *Manager) {
		m.events = make(chan StatusEvent, size)
	}
}

// WithSyncMode enables synchronous mode (for testing)
func WithSyncMode() ManagerOption {
	return func(m *Manager) {
		m.syncMode = true
	}
}

// NewManager creates a new status manager
func NewManager(c client.Client, opts ...ManagerOption) *Manager {
	m := &Manager{
		client:        c,
		events:        make(chan StatusEvent, DefaultEventBufferSize),
		stopCh:        make(chan struct{}),
		batchSize:     DefaultBatchSize,
		flushInterval: DefaultFlushInterval,
		syncMode:      false,
	}

	// Apply options
	for _, opt := range opts {
		opt(m)
	}

	// Start background aggregator only in async mode
	if !m.syncMode {
		m.wg.Add(1)
		go m.run()
	}

	return m
}

// Publish publishes a status event (non-blocking in async mode, blocking in sync mode)
func (m *Manager) Publish(event StatusEvent) {
	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	if m.syncMode {
		// In sync mode, process immediately for testing
		m.syncMutex.Lock()
		defer m.syncMutex.Unlock()

		batch := make(map[client.ObjectKey]*StatusUpdate)
		update := batch[event.NodeKey]
		if update == nil {
			update = NewStatusUpdate(event.NodeKey)
			batch[event.NodeKey] = update
		}
		update.Apply(event)
		m.flushBatch(context.Background(), batch)
	} else {
		// In async mode, send to channel (non-blocking)
		select {
		case m.events <- event:
			// Event sent successfully
		default:
			// Channel is full, drop event to prevent blocking reconciliation
			// In production, this should be rare due to buffer size and flush interval
			logger := log.Log.WithName("status-manager")
			logger.V(1).Info("Dropping status event due to full buffer",
				"node", event.NodeKey,
				"type", event.Type)
		}
	}
}

// Stop stops the status manager gracefully
func (m *Manager) Stop() {
	if !m.syncMode {
		close(m.stopCh)
		m.wg.Wait()
	}
}

// Start implements the manager.Runnable interface
// This allows the StatusManager to be added to the controller-runtime manager
func (m *Manager) Start(ctx context.Context) error {
	// Wait for context to be cancelled
	<-ctx.Done()

	// Stop the manager gracefully
	m.Stop()

	return nil
}

// run is the main event loop that aggregates and flushes status updates
func (m *Manager) run() {
	defer m.wg.Done()

	logger := log.Log.WithName("status-manager")
	logger.Info("Status manager started",
		"batchSize", m.batchSize,
		"flushInterval", m.flushInterval)

	ticker := time.NewTicker(m.flushInterval)
	defer ticker.Stop()

	batch := make(map[client.ObjectKey]*StatusUpdate)

	for {
		select {
		case <-m.stopCh:
			// Final flush before stopping
			if len(batch) > 0 {
				logger.Info("Flushing final batch before shutdown", "size", len(batch))
				m.flushBatch(context.Background(), batch)
			}
			logger.Info("Status manager stopped")
			return

		case <-ticker.C:
			// Periodic flush
			if len(batch) > 0 {
				logger.V(1).Info("Flushing batch on timer", "size", len(batch))
				m.flushBatch(context.Background(), batch)
				batch = make(map[client.ObjectKey]*StatusUpdate)
			}

		case event := <-m.events:
			// Aggregate event
			update := batch[event.NodeKey]
			if update == nil {
				update = NewStatusUpdate(event.NodeKey)
				batch[event.NodeKey] = update
			}
			update.Apply(event)

			// Flush if batch is full
			if len(batch) >= m.batchSize {
				logger.V(1).Info("Flushing batch on size limit", "size", len(batch))
				m.flushBatch(context.Background(), batch)
				batch = make(map[client.ObjectKey]*StatusUpdate)
			}
		}
	}
}

// flushBatch applies all accumulated status updates
func (m *Manager) flushBatch(ctx context.Context, batch map[client.ObjectKey]*StatusUpdate) {
	logger := log.Log.WithName("status-manager")

	// Create a timeout context for the batch
	batchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	successCount := 0
	failCount := 0

	for key, update := range batch {
		if !update.HasChanges() {
			continue
		}

		if err := m.applyUpdate(batchCtx, update); err != nil {
			logger.Error(err, "Failed to apply status update",
				"node", key.Name,
				"namespace", key.Namespace)
			failCount++
		} else {
			successCount++
		}
	}

	if successCount > 0 || failCount > 0 {
		logger.V(1).Info("Batch flush completed",
			"success", successCount,
			"failed", failCount,
			"total", len(batch))
	}
}

// applyUpdate applies a single status update to a node
func (m *Manager) applyUpdate(ctx context.Context, update *StatusUpdate) error {
	logger := log.Log.WithName("status-manager")

	// Tracks whether the LynqNode still exists. Queued (batched) events can
	// arrive AFTER the node's finalizer ran CleanupNodeMetrics; writing
	// metrics for a deleted node would silently re-create the just-deleted
	// series and leave stale gauges/alerts behind forever.
	nodeGone := false

	// Update Kubernetes status
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get latest version
		node := &lynqv1.LynqNode{}
		if err := m.client.Get(ctx, update.Key, node); err != nil {
			if errors.IsNotFound(err) {
				// LynqNode was deleted, skip update
				logger.V(1).Info("LynqNode not found, skipping status update",
					"node", update.Key.Name,
					"namespace", update.Key.Namespace)
				nodeGone = true
				return nil
			}
			return err
		}

		// Apply changes to status
		statusChanged := false

		if update.ObservedVariablesHash != nil {
			node.Status.ObservedVariablesHash = *update.ObservedVariablesHash
			statusChanged = true
		}

		if update.ObservedGeneration != nil {
			node.Status.ObservedGeneration = *update.ObservedGeneration
			statusChanged = true
		}

		if update.ReadyResources != nil {
			node.Status.ReadyResources = *update.ReadyResources
			statusChanged = true
		}

		if update.FailedResources != nil {
			node.Status.FailedResources = *update.FailedResources
			statusChanged = true
		}

		if update.DesiredResources != nil {
			node.Status.DesiredResources = *update.DesiredResources
			statusChanged = true
		}

		if update.AppliedResources != nil {
			node.Status.AppliedResources = update.AppliedResources
			statusChanged = true
		}

		if update.SkippedResources != nil {
			node.Status.SkippedResources = *update.SkippedResources
			statusChanged = true
		}

		if update.SkippedResourceIds != nil {
			node.Status.SkippedResourceIds = update.SkippedResourceIds
			statusChanged = true
		}

		if update.LastFullReconcileAt != nil {
			node.Status.LastFullReconcileAt = update.LastFullReconcileAt
			statusChanged = true
		}

		if update.DegradedResources != nil {
			node.Status.DegradedResources = *update.DegradedResources
			statusChanged = true
		}

		if update.ProgressingResources != nil {
			node.Status.ProgressingResources = *update.ProgressingResources
			statusChanged = true
		}

		if update.PendingResources != nil {
			node.Status.PendingResources = *update.PendingResources
			statusChanged = true
		}

		if update.DegradedResourceIds != nil {
			node.Status.DegradedResourceIds = update.DegradedResourceIds
			statusChanged = true
		}

		if update.ResourcePhases != nil {
			node.Status.ResourcePhases = update.ResourcePhases
			statusChanged = true
		}

		// Update conditions
		for _, cond := range update.Conditions {
			if m.updateCondition(&node.Status, cond) {
				statusChanged = true
			}
		}

		// Only call Update if something changed
		if !statusChanged {
			// Even if status didn't change, we still need to update metrics
			// Return nil to skip the K8s API call, but continue to metrics update
			return nil
		}

		// Update status subresource
		return m.client.Status().Update(ctx, node)
	})

	// Update metrics if provided (metrics don't require retry)
	// CRITICAL: Always update metrics regardless of status changes
	// This ensures Prometheus always gets the latest values during scraping.
	// EXCEPT when the node no longer exists — a late queued event must not
	// re-create series that CleanupNodeMetrics already deleted.
	if update.Metrics != nil && !nodeGone {
		m.updateMetrics(update.Key, update.Metrics)
	}

	// Per-resource phase + replica metrics. The phases array drives the
	// lynqnode_resource_phase stateset; the replica map drives the
	// lynqnode_resource_replicas_* gauges and lynqnode_resource_degraded_since_seconds.
	if (update.ResourcePhases != nil || update.ResourceReplicaMetrics != nil) && !nodeGone {
		m.updatePerResourceMetrics(update.Key, update.ResourcePhases, update.ResourceReplicaMetrics)
	}

	if err != nil {
		return fmt.Errorf("failed to update node status: %w", err)
	}

	return nil
}

// updateCondition updates or appends a condition to the status
// Returns true if the condition was changed
func (m *Manager) updateCondition(status *lynqv1.LynqNodeStatus, newCond metav1.Condition) bool {
	// Find existing condition
	for i := range status.Conditions {
		if status.Conditions[i].Type == newCond.Type {
			// Only update if status changed (avoid unnecessary LastTransitionTime updates)
			if status.Conditions[i].Status != newCond.Status ||
				status.Conditions[i].Reason != newCond.Reason ||
				status.Conditions[i].Message != newCond.Message {
				status.Conditions[i] = newCond
				return true
			}
			return false
		}
	}

	// Condition not found, append
	status.Conditions = append(status.Conditions, newCond)
	return true
}

// updateMetrics updates Prometheus metrics for a LynqNode
func (m *Manager) updateMetrics(key client.ObjectKey, metricsPayload *MetricsPayload) {
	lynqnodeName := key.Name
	lynqnodeNamespace := key.Namespace

	// Update resource metrics
	metrics.LynqNodeResourcesReady.WithLabelValues(lynqnodeName, lynqnodeNamespace).Set(float64(metricsPayload.Ready))
	metrics.LynqNodeResourcesDesired.WithLabelValues(lynqnodeName, lynqnodeNamespace).Set(float64(metricsPayload.Desired))
	metrics.LynqNodeResourcesFailed.WithLabelValues(lynqnodeName, lynqnodeNamespace).Set(float64(metricsPayload.Failed))
	metrics.LynqNodeResourcesConflicted.WithLabelValues(lynqnodeName, lynqnodeNamespace).Set(float64(metricsPayload.Conflicted))
	metrics.LynqNodeResourcesDegraded.WithLabelValues(lynqnodeName, lynqnodeNamespace).Set(float64(metricsPayload.Degraded))
	metrics.LynqNodeResourcesProgressing.WithLabelValues(lynqnodeName, lynqnodeNamespace).Set(float64(metricsPayload.Progressing))
	metrics.LynqNodeResourcesPending.WithLabelValues(lynqnodeName, lynqnodeNamespace).Set(float64(metricsPayload.Pending))

	// Update condition metrics
	for _, condition := range metricsPayload.Conditions {
		var statusValue float64
		switch condition.Status {
		case metav1.ConditionTrue:
			statusValue = 1
		case metav1.ConditionFalse:
			statusValue = 0
		case metav1.ConditionUnknown:
			statusValue = 2
		default:
			statusValue = 2 // Unknown
		}

		metrics.LynqNodeConditionStatus.WithLabelValues(
			lynqnodeName,
			lynqnodeNamespace,
			condition.Type,
		).Set(statusValue)
	}

	// Update degraded status metric
	if metricsPayload.IsDegraded {
		metrics.LynqNodeDegradedStatus.WithLabelValues(lynqnodeName, lynqnodeNamespace, metricsPayload.DegradedReason).Set(1)
	} else {
		// Reset all possible degraded reasons for this LynqNode to ensure metrics are cleared
		// This prevents stale degraded metrics from remaining after LynqNode recovers
		metrics.LynqNodeDegradedStatus.WithLabelValues(lynqnodeName, lynqnodeNamespace, "ResourceFailures").Set(0)
		metrics.LynqNodeDegradedStatus.WithLabelValues(lynqnodeName, lynqnodeNamespace, "ResourceConflicts").Set(0)
		metrics.LynqNodeDegradedStatus.WithLabelValues(lynqnodeName, lynqnodeNamespace, "ResourceFailuresAndConflicts").Set(0)
		metrics.LynqNodeDegradedStatus.WithLabelValues(lynqnodeName, lynqnodeNamespace, "ResourcesNotReady").Set(0)
		metrics.LynqNodeDegradedStatus.WithLabelValues(lynqnodeName, lynqnodeNamespace, "TemplateRenderError").Set(0)
		metrics.LynqNodeDegradedStatus.WithLabelValues(lynqnodeName, lynqnodeNamespace, "DependencyCycle").Set(0)
		metrics.LynqNodeDegradedStatus.WithLabelValues(lynqnodeName, lynqnodeNamespace, "VariablesBuildError").Set(0)
		metrics.LynqNodeDegradedStatus.WithLabelValues(lynqnodeName, lynqnodeNamespace, "ResourcesDegraded").Set(0)
		metrics.LynqNodeDegradedStatus.WithLabelValues(lynqnodeName, lynqnodeNamespace, "ResourceFailuresAndDegraded").Set(0)
	}
}

// updatePerResourceMetrics emits the per-resource phase stateset, replica
// gauges, and degraded-since-seconds gauge. Called from applyUpdate when the
// reconcile pass produced a ResourcePhases array.
//
// Stale series — for resources that were tracked previously but are missing
// from this reconcile's phases array — should be cleared via
// metrics.DeleteResourceSeries(). Doing that requires a diff against the
// previous reconcile's resource set; the controller owns that diff (it has
// the previous status.appliedResources / status.resourcePhases). The status
// manager just writes what the controller tells it.
func (m *Manager) updatePerResourceMetrics(
	key client.ObjectKey,
	phases []lynqv1.ResourcePhaseEntry,
	replicas map[string]ResourceReplicaMetrics,
) {
	lynqnodeName := key.Name
	lynqnodeNamespace := key.Namespace

	for _, entry := range phases {
		// Phase stateset — exactly one phase reads 1, others 0.
		metrics.SetResourcePhase(lynqnodeName, lynqnodeNamespace, entry.ID, entry.Kind, string(entry.Phase))

		// Per-resource replica gauges, when supplied. Workloads only
		// populate this map; non-workload kinds omit themselves.
		if r, ok := replicas[entry.ID]; ok {
			metrics.LynqNodeResourceReplicasDesired.WithLabelValues(lynqnodeName, lynqnodeNamespace, entry.ID, entry.Kind).Set(float64(r.Desired))
			metrics.LynqNodeResourceReplicasAvailable.WithLabelValues(lynqnodeName, lynqnodeNamespace, entry.ID, entry.Kind).Set(float64(r.Available))
			metrics.LynqNodeResourceReplicasReady.WithLabelValues(lynqnodeName, lynqnodeNamespace, entry.ID, entry.Kind).Set(float64(r.Ready))
			metrics.LynqNodeResourceReplicasUpdated.WithLabelValues(lynqnodeName, lynqnodeNamespace, entry.ID, entry.Kind).Set(float64(r.Updated))
		}

		// Degraded-since gauge: reset to 0 unless this resource is currently
		// Degraded. The controller computes the elapsed seconds from the
		// previous reconcile's phase transition timestamp; we just write
		// what it tells us.
		degradedSince := float64(0)
		if entry.Phase == lynqv1.ResourcePhaseDegraded {
			if r, ok := replicas[entry.ID]; ok {
				degradedSince = float64(r.DegradedSinceSeconds)
			} else {
				degradedSince = float64(entry.SinceSeconds)
			}
		}
		metrics.LynqNodeResourceDegradedSinceSeconds.WithLabelValues(lynqnodeName, lynqnodeNamespace, entry.ID, entry.Kind).Set(degradedSince)
	}
}

// CleanupNodeMetrics removes all metric series for a LynqNode. Call from the
// finalizer cleanup path so deleted LynqNodes don't leave stale series in
// Prometheus. resourcePhases is the LAST observed phase array (typically read
// from node.Status.ResourcePhases before finalizer removal) so per-resource
// series can be enumerated and deleted.
func (m *Manager) CleanupNodeMetrics(key client.ObjectKey, resourcePhases []lynqv1.ResourcePhaseEntry) {
	for _, entry := range resourcePhases {
		metrics.DeleteResourceSeries(key.Name, key.Namespace, entry.ID, entry.Kind)
	}
	metrics.DeleteLynqNodeSeries(key.Name, key.Namespace)
}
