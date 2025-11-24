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
