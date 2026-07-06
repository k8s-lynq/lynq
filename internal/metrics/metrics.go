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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// LynqNodeReconcileDuration measures the duration of LynqNode reconciliation
	LynqNodeReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lynqnode_reconcile_duration_seconds",
			Help:    "Duration of LynqNode reconciliation in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"result"}, // success or error
	)

	// LynqNodeResourcesReady tracks the number of ready resources per LynqNode
	LynqNodeResourcesReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resources_ready",
			Help: "Number of ready resources for a LynqNode",
		},
		[]string{"lynqnode", "namespace"},
	)

	// LynqNodeResourcesDesired tracks the total number of desired resources per LynqNode
	LynqNodeResourcesDesired = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resources_desired",
			Help: "Total number of desired resources for a LynqNode",
		},
		[]string{"lynqnode", "namespace"},
	)

	// LynqNodeResourcesFailed tracks the number of failed resources per LynqNode.
	// A resource enters Failed only via: rollout timeout while Progressing,
	// Kubernetes-native ProgressDeadlineExceeded, apply error, or Job Failed
	// condition. Steady-state pod-level disruption does NOT count here — see
	// LynqNodeResourcesDegraded for that signal.
	LynqNodeResourcesFailed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resources_failed",
			Help: "Number of failed resources for a LynqNode (rollout timeout / ProgressDeadlineExceeded / apply error / Job Failed; NOT steady-state degradation)",
		},
		[]string{"lynqnode", "namespace"},
	)

	// LynqNodeResourcesDegraded tracks the number of resources currently in
	// the Degraded phase — rollout completed for the current generation but
	// availability has since dropped due to causes outside Lynq's rollout
	// (node drain, HPA scale-up, pod eviction, image GC, kubelet restart).
	// Kubernetes is converging these; Lynq does NOT mark them Failed.
	// Primary new alert source for steady-state partial unavailability.
	LynqNodeResourcesDegraded = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resources_degraded",
			Help: "Number of resources in Degraded phase — rollout complete but availability dropped (node drain / HPA / eviction). NOT counted as Failed.",
		},
		[]string{"lynqnode", "namespace"},
	)

	// LynqNodeResourcesProgressing tracks resources currently rolling out
	// (observedGeneration matches but rollout criteria not yet met).
	LynqNodeResourcesProgressing = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resources_progressing",
			Help: "Number of resources currently rolling out for a LynqNode",
		},
		[]string{"lynqnode", "namespace"},
	)

	// LynqNodeResourcesPending tracks resources whose controllers have not
	// yet observed the latest generation (observedGeneration < generation),
	// or that are scaled to zero.
	LynqNodeResourcesPending = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resources_pending",
			Help: "Number of resources awaiting controller observation for a LynqNode",
		},
		[]string{"lynqnode", "namespace"},
	)

	// LynqNodeResourcePhase is a stateset: for each (lynqnode, namespace,
	// resource_id, kind, phase) tuple, exactly one phase reads 1 and the
	// other four read 0. Lets dashboards plot phase distribution as
	// `sum by(phase)(lynqnode_resource_phase == 1)`.
	//
	// Cardinality: N_lynqnodes × N_resources_per_node × 5 phases. For 1000
	// LynqNodes with 10 resources each, ~50k series — comfortable for
	// Prometheus. If a deployment is much larger, gate per-resource metrics
	// behind a controller flag (see SetResourcePhase docs).
	LynqNodeResourcePhase = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resource_phase",
			Help: "Stateset of per-resource phase: value=1 for the current phase, 0 for the other four. See ResourcePhase enum.",
		},
		[]string{"lynqnode", "namespace", "resource_id", "kind", "phase"},
	)

	// LynqNodeResourceReplicasDesired is the per-resource spec.replicas
	// (Deployment/StatefulSet) or desiredNumberScheduled (DaemonSet).
	LynqNodeResourceReplicasDesired = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resource_replicas_desired",
			Help: "Desired replica count per resource (spec.replicas / desiredNumberScheduled). Workloads only.",
		},
		[]string{"lynqnode", "namespace", "resource_id", "kind"},
	)

	// LynqNodeResourceReplicasAvailable is the per-resource availableReplicas
	// (Deployment) / numberAvailable (DaemonSet) / readyReplicas (StatefulSet).
	LynqNodeResourceReplicasAvailable = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resource_replicas_available",
			Help: "Available replicas per resource. Workloads only.",
		},
		[]string{"lynqnode", "namespace", "resource_id", "kind"},
	)

	// LynqNodeResourceReplicasReady is the per-resource readyReplicas /
	// numberReady.
	LynqNodeResourceReplicasReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resource_replicas_ready",
			Help: "Ready replicas per resource. Workloads only.",
		},
		[]string{"lynqnode", "namespace", "resource_id", "kind"},
	)

	// LynqNodeResourceReplicasUpdated is the per-resource updatedReplicas /
	// updatedNumberScheduled. Enables PromQL rollout-progress queries:
	// `lynqnode_resource_replicas_updated / lynqnode_resource_replicas_desired`.
	LynqNodeResourceReplicasUpdated = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resource_replicas_updated",
			Help: "Updated replicas per resource. Workloads only.",
		},
		[]string{"lynqnode", "namespace", "resource_id", "kind"},
	)

	// LynqNodeResourceDegradedSinceSeconds is the duration (in seconds)
	// since the resource entered Degraded phase. Reset to 0 when the
	// resource transitions out of Degraded (back to Available, or to
	// Progressing on a new spec apply). Powers the
	// LynqNodeWorkloadSeverelyDegraded alert (single resource > 30 min).
	LynqNodeResourceDegradedSinceSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resource_degraded_since_seconds",
			Help: "Seconds since the resource entered Degraded phase (0 when not Degraded)",
		},
		[]string{"lynqnode", "namespace", "resource_id", "kind"},
	)

	// LynqNodeResourceRolloutDurationSeconds observes the wall-clock time
	// between apply-start-time and first observation of Available. Recorded
	// once per (resource, generation). Buckets chosen to span typical
	// container startup (10–60s), slow image pulls (60–300s), and
	// pathological rollouts (>30 min).
	//
	// `result` label values:
	//   - "complete": rollout reached Available within timeout.
	//   - "timeout":  rollout exceeded timeoutSeconds (Phase→Failed via
	//                 RolloutTimedOut). The observed duration equals the
	//                 timeout, not real convergence time.
	//   - "aborted":  rollout hit ProgressDeadlineExceeded or another
	//                 non-timeout Failed cause.
	LynqNodeResourceRolloutDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lynqnode_resource_rollout_duration_seconds",
			Help:    "Rollout duration from apply-start-time to first Available (per resource per generation)",
			Buckets: []float64{10, 30, 60, 120, 300, 600, 1800, 3600},
		},
		[]string{"kind", "result"},
	)

	// LynqNodeResourcePhaseTransitionsTotal counts phase transitions, e.g.,
	// Available→Degraded (workload disruption), Progressing→Available
	// (rollout completion), Available→Progressing (new spec applied).
	// Powers SLO queries like
	// `rate(...{from="Available",to="Degraded"}[15m])` (= disruption rate).
	LynqNodeResourcePhaseTransitionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynqnode_resource_phase_transitions_total",
			Help: "Total phase transitions observed per resource (labeled by from/to phase)",
		},
		[]string{"kind", "from", "to"},
	)

	// HubDesired tracks the desired LynqNode count per hub
	HubDesired = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hub_desired",
			Help: "Number of desired LynqNodes from the hub data source",
		},
		[]string{"hub", "namespace"},
	)

	// HubReady tracks the ready LynqNode count per hub
	HubReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hub_ready",
			Help: "Number of ready LynqNodes for a hub",
		},
		[]string{"hub", "namespace"},
	)

	// HubFailed tracks the failed LynqNode count per hub
	HubFailed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hub_failed",
			Help: "Number of failed LynqNodes for a hub",
		},
		[]string{"hub", "namespace"},
	)

	// ApplyAttemptsTotal counts resource apply attempts
	ApplyAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "apply_attempts_total",
			Help: "Total number of resource apply attempts",
		},
		[]string{"kind", "result", "conflict_policy"},
	)

	// LynqNodeConditionStatus tracks the status of LynqNode conditions
	// status: 0=False, 1=True, 2=Unknown
	LynqNodeConditionStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_condition_status",
			Help: "Status of LynqNode conditions (0=False, 1=True, 2=Unknown)",
		},
		[]string{"lynqnode", "namespace", "type"},
	)

	// LynqNodeConflictsTotal counts the total number of conflicts encountered
	LynqNodeConflictsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynqnode_conflicts_total",
			Help: "Total number of resource conflicts encountered during reconciliation",
		},
		[]string{"lynqnode", "namespace", "resource_kind", "conflict_policy"},
	)

	// LynqNodeResourcesConflicted tracks the current number of resources in conflict state
	LynqNodeResourcesConflicted = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resources_conflicted",
			Help: "Number of resources currently in conflict state for a LynqNode",
		},
		[]string{"lynqnode", "namespace"},
	)

	// LynqNodeDegradedStatus indicates if a LynqNode is in degraded state (1=degraded, 0=not degraded)
	LynqNodeDegradedStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_degraded_status",
			Help: "Indicates if a LynqNode is in degraded state (1=degraded, 0=not degraded)",
		},
		[]string{"lynqnode", "namespace", "reason"},
	)

	// FormRolloutUpdatingNodes tracks the number of nodes currently being updated for a LynqForm
	FormRolloutUpdatingNodes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqform_rollout_updating_nodes",
			Help: "Number of nodes currently being updated for a LynqForm (updated but not Ready yet)",
		},
		[]string{"form", "namespace"},
	)

	// FormRolloutPhase tracks the current rollout phase for a LynqForm
	// Values: 0=Idle, 1=InProgress, 2=Failed, 3=Complete
	FormRolloutPhase = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqform_rollout_phase",
			Help: "Current rollout phase for a LynqForm (0=Idle, 1=InProgress, 2=Failed, 3=Complete)",
		},
		[]string{"form", "namespace"},
	)

	// FormRolloutProgress tracks the rollout progress percentage for a LynqForm
	FormRolloutProgress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqform_rollout_progress",
			Help: "Rollout progress percentage for a LynqForm (readyUpdatedNodes/totalNodes * 100)",
		},
		[]string{"form", "namespace"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		LynqNodeReconcileDuration,
		LynqNodeResourcesReady,
		LynqNodeResourcesDesired,
		LynqNodeResourcesFailed,
		HubDesired,
		HubReady,
		HubFailed,
		ApplyAttemptsTotal,
		LynqNodeConditionStatus,
		LynqNodeConflictsTotal,
		LynqNodeResourcesConflicted,
		LynqNodeDegradedStatus,
		// Rollout metrics
		FormRolloutUpdatingNodes,
		FormRolloutPhase,
		FormRolloutProgress,
		// Per-resource phase metrics
		LynqNodeResourcesDegraded,
		LynqNodeResourcesProgressing,
		LynqNodeResourcesPending,
		LynqNodeResourcePhase,
		LynqNodeResourceReplicasDesired,
		LynqNodeResourceReplicasAvailable,
		LynqNodeResourceReplicasReady,
		LynqNodeResourceReplicasUpdated,
		LynqNodeResourceDegradedSinceSeconds,
		LynqNodeResourceRolloutDurationSeconds,
		LynqNodeResourcePhaseTransitionsTotal,
	)
}

// allResourcePhases is the closed enum of phases SetResourcePhase iterates
// over to keep the stateset consistent (exactly one phase reads 1).
var allResourcePhases = [...]string{"Pending", "Progressing", "Available", "Degraded", "Failed"}

// SetResourcePhase writes the per-resource phase stateset: 1 for the active
// phase, 0 for the other four. Always call this — never set a single phase
// label to 1 in isolation, since stale labels would otherwise read 1
// indefinitely.
//
// The active phase argument is stringified so callers can pass either the
// lynqv1.ResourcePhase enum or a raw string (keeps this package
// dependency-free of the v1 types).
func SetResourcePhase(lynqnode, namespace, resourceID, kind, activePhase string) {
	for _, phase := range allResourcePhases {
		value := 0.0
		if phase == activePhase {
			value = 1.0
		}
		LynqNodeResourcePhase.WithLabelValues(lynqnode, namespace, resourceID, kind, phase).Set(value)
	}
}

// DeleteResourceSeries removes all per-resource metric series for the given
// (lynqnode, namespace, resource_id, kind) tuple. Call when a resource is
// removed from a LynqNode's template (orphan cleanup) so stale series don't
// accumulate. The phase stateset is wiped across all five phase labels.
func DeleteResourceSeries(lynqnode, namespace, resourceID, kind string) {
	for _, phase := range allResourcePhases {
		LynqNodeResourcePhase.DeleteLabelValues(lynqnode, namespace, resourceID, kind, phase)
	}
	LynqNodeResourceReplicasDesired.DeleteLabelValues(lynqnode, namespace, resourceID, kind)
	LynqNodeResourceReplicasAvailable.DeleteLabelValues(lynqnode, namespace, resourceID, kind)
	LynqNodeResourceReplicasReady.DeleteLabelValues(lynqnode, namespace, resourceID, kind)
	LynqNodeResourceReplicasUpdated.DeleteLabelValues(lynqnode, namespace, resourceID, kind)
	LynqNodeResourceDegradedSinceSeconds.DeleteLabelValues(lynqnode, namespace, resourceID, kind)
}

// DeleteLynqNodeSeries removes all aggregate metric series for a LynqNode.
// Call on finalizer cleanup so deleted LynqNodes don't leave stale series.
// Per-resource series for this node should be removed via repeated
// DeleteResourceSeries calls before this — the per-resource cardinality
// can't be enumerated from just the lynqnode name.
func DeleteLynqNodeSeries(lynqnode, namespace string) {
	LynqNodeResourcesReady.DeleteLabelValues(lynqnode, namespace)
	LynqNodeResourcesDesired.DeleteLabelValues(lynqnode, namespace)
	LynqNodeResourcesFailed.DeleteLabelValues(lynqnode, namespace)
	LynqNodeResourcesDegraded.DeleteLabelValues(lynqnode, namespace)
	LynqNodeResourcesProgressing.DeleteLabelValues(lynqnode, namespace)
	LynqNodeResourcesPending.DeleteLabelValues(lynqnode, namespace)
	LynqNodeResourcesConflicted.DeleteLabelValues(lynqnode, namespace)
	// LynqNodeConditionStatus / LynqNodeDegradedStatus carry a third label
	// (type / reason) whose values can't be enumerated here — delete every
	// series matching the node's identity labels so a deleted node leaves no
	// stale condition/degraded gauges (and no stale alerts) behind.
	partial := prometheus.Labels{"lynqnode": lynqnode, "namespace": namespace}
	LynqNodeConditionStatus.DeletePartialMatch(partial)
	LynqNodeDegradedStatus.DeletePartialMatch(partial)
}

// RolloutPhaseToMetric converts a RolloutPhase to a numeric value for metrics
// 0=Idle, 1=InProgress, 2=Failed, 3=Complete
func RolloutPhaseToMetric(phase string) float64 {
	switch phase {
	case "Idle":
		return 0
	case "InProgress":
		return 1
	case "Failed":
		return 2
	case "Complete":
		return 3
	default:
		return 0
	}
}
