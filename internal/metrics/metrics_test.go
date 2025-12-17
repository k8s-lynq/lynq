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
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestLynqNodeReconcileDuration(t *testing.T) {
	// Reset the metric before testing
	LynqNodeReconcileDuration.Reset()

	// Observe a reconciliation duration
	duration := 2.5 // 2.5 seconds
	LynqNodeReconcileDuration.WithLabelValues("success").Observe(duration)

	// Collect metrics
	count := testutil.CollectAndCount(LynqNodeReconcileDuration)
	assert.Equal(t, 1, count, "Expected 1 metric to be collected")

	// Verify metric can be collected and has the expected type
	problems, err := testutil.CollectAndLint(LynqNodeReconcileDuration)
	assert.NoError(t, err)
	assert.Empty(t, problems, "Metric should have no lint problems")

	// Test with error result
	LynqNodeReconcileDuration.WithLabelValues("error").Observe(1.0)
	count = testutil.CollectAndCount(LynqNodeReconcileDuration)
	assert.Equal(t, 2, count, "Expected 2 metrics to be collected after error observation")
}

func TestLynqNodeReconcileDuration_Timer(t *testing.T) {
	LynqNodeReconcileDuration.Reset()

	// Test using a timer
	timer := prometheus.NewTimer(LynqNodeReconcileDuration.WithLabelValues("success"))
	time.Sleep(10 * time.Millisecond)
	timer.ObserveDuration()

	count := testutil.CollectAndCount(LynqNodeReconcileDuration)
	assert.Equal(t, 1, count)
}

func TestLynqNodeResourcesReady(t *testing.T) {
	LynqNodeResourcesReady.Reset()

	// Set ready resources count
	LynqNodeResourcesReady.WithLabelValues("lynqnode1", "default").Set(5)
	LynqNodeResourcesReady.WithLabelValues("lynqnode2", "production").Set(10)

	// Verify count
	count := testutil.CollectAndCount(LynqNodeResourcesReady)
	assert.Equal(t, 2, count)

	// Verify metric values
	expected := `
# HELP lynqnode_resources_ready Number of ready resources for a LynqNode
# TYPE lynqnode_resources_ready gauge
lynqnode_resources_ready{lynqnode="lynqnode1",namespace="default"} 5
lynqnode_resources_ready{lynqnode="lynqnode2",namespace="production"} 10
`
	err := testutil.CollectAndCompare(LynqNodeResourcesReady, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestLynqNodeResourcesDesired(t *testing.T) {
	LynqNodeResourcesDesired.Reset()

	// Set desired resources count
	LynqNodeResourcesDesired.WithLabelValues("lynqnode1", "default").Set(8)

	count := testutil.CollectAndCount(LynqNodeResourcesDesired)
	assert.Equal(t, 1, count)

	// Verify metric has no lint problems
	problems, err := testutil.CollectAndLint(LynqNodeResourcesDesired)
	assert.NoError(t, err)
	assert.Empty(t, problems)
}

func TestLynqNodeResourcesFailed(t *testing.T) {
	LynqNodeResourcesFailed.Reset()

	// Set failed resources count
	LynqNodeResourcesFailed.WithLabelValues("lynqnode1", "default").Set(2)

	count := testutil.CollectAndCount(LynqNodeResourcesFailed)
	assert.Equal(t, 1, count)

	// Verify metric can be incremented
	LynqNodeResourcesFailed.WithLabelValues("lynqnode1", "default").Inc()
	expected := `
# HELP lynqnode_resources_failed Number of failed resources for a LynqNode
# TYPE lynqnode_resources_failed gauge
lynqnode_resources_failed{lynqnode="lynqnode1",namespace="default"} 3
`
	err := testutil.CollectAndCompare(LynqNodeResourcesFailed, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestHubDesired(t *testing.T) {
	HubDesired.Reset()

	// Set desired LynqNode count for hubs
	HubDesired.WithLabelValues("mysql-prod", "default").Set(100)
	HubDesired.WithLabelValues("mysql-staging", "staging").Set(20)

	count := testutil.CollectAndCount(HubDesired)
	assert.Equal(t, 2, count)

	expected := `
# HELP hub_desired Number of desired LynqNodes from the hub data source
# TYPE hub_desired gauge
hub_desired{hub="mysql-prod",namespace="default"} 100
hub_desired{hub="mysql-staging",namespace="staging"} 20
`
	err := testutil.CollectAndCompare(HubDesired, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestHubReady(t *testing.T) {
	HubReady.Reset()

	// Set ready LynqNode count
	HubReady.WithLabelValues("mysql-prod", "default").Set(95)

	count := testutil.CollectAndCount(HubReady)
	assert.Equal(t, 1, count)

	problems, err := testutil.CollectAndLint(HubReady)
	assert.NoError(t, err)
	assert.Empty(t, problems)
}

func TestHubFailed(t *testing.T) {
	HubFailed.Reset()

	// Set failed LynqNode count
	HubFailed.WithLabelValues("mysql-prod", "default").Set(5)

	count := testutil.CollectAndCount(HubFailed)
	assert.Equal(t, 1, count)

	expected := `
# HELP hub_failed Number of failed LynqNodes for a hub
# TYPE hub_failed gauge
hub_failed{hub="mysql-prod",namespace="default"} 5
`
	err := testutil.CollectAndCompare(HubFailed, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestApplyAttemptsTotal(t *testing.T) {
	ApplyAttemptsTotal.Reset()

	// Increment apply attempts
	ApplyAttemptsTotal.WithLabelValues("Deployment", "success", "Stuck").Inc()
	ApplyAttemptsTotal.WithLabelValues("Deployment", "success", "Stuck").Inc()
	ApplyAttemptsTotal.WithLabelValues("Deployment", "error", "Force").Inc()
	ApplyAttemptsTotal.WithLabelValues("Service", "success", "Stuck").Inc()

	count := testutil.CollectAndCount(ApplyAttemptsTotal)
	assert.Equal(t, 3, count, "Expected 3 unique label combinations")

	// Verify counter values
	expected := `
# HELP apply_attempts_total Total number of resource apply attempts
# TYPE apply_attempts_total counter
apply_attempts_total{conflict_policy="Force",kind="Deployment",result="error"} 1
apply_attempts_total{conflict_policy="Stuck",kind="Deployment",result="success"} 2
apply_attempts_total{conflict_policy="Stuck",kind="Service",result="success"} 1
`
	err := testutil.CollectAndCompare(ApplyAttemptsTotal, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestLynqNodeConditionStatus(t *testing.T) {
	LynqNodeConditionStatus.Reset()

	// Set condition statuses (0=False, 1=True, 2=Unknown)
	LynqNodeConditionStatus.WithLabelValues("lynqnode1", "default", "Ready").Set(1)    // True
	LynqNodeConditionStatus.WithLabelValues("lynqnode2", "default", "Ready").Set(0)    // False
	LynqNodeConditionStatus.WithLabelValues("lynqnode3", "default", "Degraded").Set(2) // Unknown

	count := testutil.CollectAndCount(LynqNodeConditionStatus)
	assert.Equal(t, 3, count)

	expected := `
# HELP lynqnode_condition_status Status of LynqNode conditions (0=False, 1=True, 2=Unknown)
# TYPE lynqnode_condition_status gauge
lynqnode_condition_status{lynqnode="lynqnode1",namespace="default",type="Ready"} 1
lynqnode_condition_status{lynqnode="lynqnode2",namespace="default",type="Ready"} 0
lynqnode_condition_status{lynqnode="lynqnode3",namespace="default",type="Degraded"} 2
`
	err := testutil.CollectAndCompare(LynqNodeConditionStatus, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestLynqNodeConflictsTotal(t *testing.T) {
	LynqNodeConflictsTotal.Reset()

	// Increment conflict counters
	LynqNodeConflictsTotal.WithLabelValues("lynqnode1", "default", "Deployment", "Stuck").Inc()
	LynqNodeConflictsTotal.WithLabelValues("lynqnode1", "default", "Deployment", "Stuck").Inc()
	LynqNodeConflictsTotal.WithLabelValues("lynqnode1", "default", "Service", "Force").Inc()

	count := testutil.CollectAndCount(LynqNodeConflictsTotal)
	assert.Equal(t, 2, count)

	expected := `
# HELP lynqnode_conflicts_total Total number of resource conflicts encountered during reconciliation
# TYPE lynqnode_conflicts_total counter
lynqnode_conflicts_total{conflict_policy="Force",lynqnode="lynqnode1",namespace="default",resource_kind="Service"} 1
lynqnode_conflicts_total{conflict_policy="Stuck",lynqnode="lynqnode1",namespace="default",resource_kind="Deployment"} 2
`
	err := testutil.CollectAndCompare(LynqNodeConflictsTotal, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestLynqNodeResourcesConflicted(t *testing.T) {
	LynqNodeResourcesConflicted.Reset()

	// Set conflicted resources count
	LynqNodeResourcesConflicted.WithLabelValues("lynqnode1", "default").Set(3)

	count := testutil.CollectAndCount(LynqNodeResourcesConflicted)
	assert.Equal(t, 1, count)

	expected := `
# HELP lynqnode_resources_conflicted Number of resources currently in conflict state for a LynqNode
# TYPE lynqnode_resources_conflicted gauge
lynqnode_resources_conflicted{lynqnode="lynqnode1",namespace="default"} 3
`
	err := testutil.CollectAndCompare(LynqNodeResourcesConflicted, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestLynqNodeDegradedStatus(t *testing.T) {
	LynqNodeDegradedStatus.Reset()

	// Set degraded status (1=degraded, 0=not degraded)
	LynqNodeDegradedStatus.WithLabelValues("lynqnode1", "default", "ResourceConflict").Set(1)
	LynqNodeDegradedStatus.WithLabelValues("lynqnode2", "default", "").Set(0)

	count := testutil.CollectAndCount(LynqNodeDegradedStatus)
	assert.Equal(t, 2, count)

	expected := `
# HELP lynqnode_degraded_status Indicates if a LynqNode is in degraded state (1=degraded, 0=not degraded)
# TYPE lynqnode_degraded_status gauge
lynqnode_degraded_status{lynqnode="lynqnode1",namespace="default",reason="ResourceConflict"} 1
lynqnode_degraded_status{lynqnode="lynqnode2",namespace="default",reason=""} 0
`
	err := testutil.CollectAndCompare(LynqNodeDegradedStatus, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestMetricsRegistration(t *testing.T) {
	// Test that all metrics are properly defined and can collect data
	metrics := []prometheus.Collector{
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
	}

	for _, metric := range metrics {
		assert.NotNil(t, metric, "Metric should not be nil")

		// Verify that the metric can be collected
		count := testutil.CollectAndCount(metric)
		assert.GreaterOrEqual(t, count, 0, "Should be able to collect metric")
	}
}

func TestMetricLabels(t *testing.T) {
	// Test that metrics accept the correct label values

	// Reset all metrics
	LynqNodeReconcileDuration.Reset()
	LynqNodeResourcesReady.Reset()
	ApplyAttemptsTotal.Reset()

	// LynqNodeReconcileDuration: result
	LynqNodeReconcileDuration.WithLabelValues("success")
	LynqNodeReconcileDuration.WithLabelValues("error")

	// LynqNodeResourcesReady: lynqnode, namespace
	LynqNodeResourcesReady.WithLabelValues("test-lynqnode", "test-namespace")

	// ApplyAttemptsTotal: kind, result, conflict_policy
	ApplyAttemptsTotal.WithLabelValues("Deployment", "success", "Stuck")
	ApplyAttemptsTotal.WithLabelValues("Service", "error", "Force")

	// All label combinations should work without panicking
	assert.True(t, true, "All label combinations worked")
}

func TestFormRolloutUpdatingNodes(t *testing.T) {
	FormRolloutUpdatingNodes.Reset()

	// Set updating nodes count
	FormRolloutUpdatingNodes.WithLabelValues("web-app", "default").Set(3)
	FormRolloutUpdatingNodes.WithLabelValues("worker", "production").Set(5)

	count := testutil.CollectAndCount(FormRolloutUpdatingNodes)
	assert.Equal(t, 2, count)

	expected := `
# HELP lynqform_rollout_updating_nodes Number of nodes currently being updated for a LynqForm (updated but not Ready yet)
# TYPE lynqform_rollout_updating_nodes gauge
lynqform_rollout_updating_nodes{form="web-app",namespace="default"} 3
lynqform_rollout_updating_nodes{form="worker",namespace="production"} 5
`
	err := testutil.CollectAndCompare(FormRolloutUpdatingNodes, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestFormRolloutPhase(t *testing.T) {
	FormRolloutPhase.Reset()

	// Set rollout phases (0=Idle, 1=InProgress, 2=Failed, 3=Complete)
	FormRolloutPhase.WithLabelValues("web-app", "default").Set(1)   // InProgress
	FormRolloutPhase.WithLabelValues("worker", "production").Set(3) // Complete
	FormRolloutPhase.WithLabelValues("api", "staging").Set(0)       // Idle

	count := testutil.CollectAndCount(FormRolloutPhase)
	assert.Equal(t, 3, count)

	expected := `
# HELP lynqform_rollout_phase Current rollout phase for a LynqForm (0=Idle, 1=InProgress, 2=Failed, 3=Complete)
# TYPE lynqform_rollout_phase gauge
lynqform_rollout_phase{form="api",namespace="staging"} 0
lynqform_rollout_phase{form="web-app",namespace="default"} 1
lynqform_rollout_phase{form="worker",namespace="production"} 3
`
	err := testutil.CollectAndCompare(FormRolloutPhase, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestFormRolloutProgress(t *testing.T) {
	FormRolloutProgress.Reset()

	// Set rollout progress percentages
	FormRolloutProgress.WithLabelValues("web-app", "default").Set(50.0)  // 50% progress
	FormRolloutProgress.WithLabelValues("worker", "production").Set(100) // Complete

	count := testutil.CollectAndCount(FormRolloutProgress)
	assert.Equal(t, 2, count)

	expected := `
# HELP lynqform_rollout_progress Rollout progress percentage for a LynqForm (readyUpdatedNodes/totalNodes * 100)
# TYPE lynqform_rollout_progress gauge
lynqform_rollout_progress{form="web-app",namespace="default"} 50
lynqform_rollout_progress{form="worker",namespace="production"} 100
`
	err := testutil.CollectAndCompare(FormRolloutProgress, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestRolloutPhaseToMetric(t *testing.T) {
	tests := []struct {
		phase    string
		expected float64
	}{
		{"Idle", 0},
		{"InProgress", 1},
		{"Failed", 2},
		{"Complete", 3},
		{"Unknown", 0}, // Unknown phase defaults to 0
		{"", 0},        // Empty string defaults to 0
		{"invalid", 0}, // Invalid phase defaults to 0
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			result := RolloutPhaseToMetric(tt.phase)
			assert.Equal(t, tt.expected, result, "Phase %s should map to %f", tt.phase, tt.expected)
		})
	}
}

func TestRolloutMetricsRegistration(t *testing.T) {
	// Test that all rollout metrics are properly defined
	metrics := []prometheus.Collector{
		FormRolloutUpdatingNodes,
		FormRolloutPhase,
		FormRolloutProgress,
	}

	for _, metric := range metrics {
		assert.NotNil(t, metric, "Rollout metric should not be nil")

		// Verify that the metric can be collected
		count := testutil.CollectAndCount(metric)
		assert.GreaterOrEqual(t, count, 0, "Should be able to collect rollout metric")
	}
}

func TestRolloutMetricLabels(t *testing.T) {
	// Reset rollout metrics
	FormRolloutUpdatingNodes.Reset()
	FormRolloutPhase.Reset()
	FormRolloutProgress.Reset()

	// Test label combinations work without panicking
	FormRolloutUpdatingNodes.WithLabelValues("test-form", "test-namespace")
	FormRolloutPhase.WithLabelValues("test-form", "test-namespace")
	FormRolloutProgress.WithLabelValues("test-form", "test-namespace")

	// Verify metrics can be collected
	assert.GreaterOrEqual(t, testutil.CollectAndCount(FormRolloutUpdatingNodes), 0)
	assert.GreaterOrEqual(t, testutil.CollectAndCount(FormRolloutPhase), 0)
	assert.GreaterOrEqual(t, testutil.CollectAndCount(FormRolloutProgress), 0)
}
