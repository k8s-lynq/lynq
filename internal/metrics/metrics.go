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

	// LynqNodeResourcesFailed tracks the number of failed resources per LynqNode
	LynqNodeResourcesFailed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynqnode_resources_failed",
			Help: "Number of failed resources for a LynqNode",
		},
		[]string{"lynqnode", "namespace"},
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
	)
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
