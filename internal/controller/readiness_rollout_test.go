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

// Package controller - readiness_rollout_test.go
//
// # Rollouts Lynq Starts vs. Disruptions It Did Not
//
// Ready history only shields a resource from timeoutSeconds while Lynq has not changed it.
// The apply-start-time stamp alone cannot say that: a force-reapply that reverts external
// drift keeps the stamp (the desired hash did not change) yet starts a real rollout. What
// does say it is metadata.generation — an apply that moves it started a rollout, one that
// does not was a no-op. These tests pin both sides, for every patch strategy the fake client
// can exercise, and the dependency-ordering consequence: a resource that already satisfied
// its dependents under the current apply does not block them when it later regresses.

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
)

const (
	dependentServiceID   = "svc-id"
	dependentServiceName = "steady-svc"
)

// SSA (PatchStrategyApply) is not in these tables: the controller-runtime fake client cannot
// serve apply patches. ApplyResource derives GenerationBumped after applyByStrategy returns,
// the same way for every strategy, so the SSA branch shares the logic exercised here.
var rolloutStrategies = []lynqv1.PatchStrategy{lynqv1.PatchStrategyReplace, lynqv1.PatchStrategyMerge}

// driftReplicasPreservingHash edits the Deployment's spec the way `kubectl scale` or
// `kubectl edit` does: the spec changes, Lynq's applied-hash stays, so only the periodic
// force-reapply will revert it.
func (s *readinessScenario) driftReplicasPreservingHash(replicas int32) {
	s.mutateDeployment(func(d *appsv1.Deployment) {
		d.Spec.Replicas = &replicas
	})
}

func (s *readinessScenario) deploymentGeneration() int64 {
	s.t.Helper()
	d := &appsv1.Deployment{}
	require.NoError(s.t, s.client.Get(context.Background(),
		client.ObjectKey{Namespace: "default", Name: steadyStateDeployName}, d))
	return d.Generation
}

func TestReadiness_ForceReapplyThatRevertsDriftStartsATimedRollout(t *testing.T) {
	for _, strategy := range rolloutStrategies {
		t.Run(string(strategy), func(t *testing.T) {
			// Given a Deployment Ready for months under a 1s timeout
			s := newReadinessScenarioWith(t, strategy, 2, 1)
			s.givenDeploymentReadyForMonths(2)

			// And an external edit drifted its spec without touching applied-hash, so it is
			// now a regression that the timeout does not apply to
			s.driftReplicasPreservingHash(5)
			_, failed := s.reconcile()
			require.Equal(t, int32(0), failed, "drift alone is a regression, not a failure")

			// When the periodic force-reapply reverts the drift, moving the generation
			generationBefore := s.deploymentGeneration()
			_, failed = s.reconcileForced()
			require.Greater(t, s.deploymentGeneration(), generationBefore,
				"reverting the drift must be a real spec write for this scenario to mean anything")
			require.Equal(t, int32(0), failed, "the rollout Lynq just started is within its timeout")

			// And that rollout does not become Ready within timeoutSeconds
			time.Sleep(1100 * time.Millisecond)
			_, failed = s.reconcile()

			// Then the old Ready history does not shield it: it times out
			assert.Equal(t, int32(1), failed)
			assert.NotEmpty(t, eventsWithReason(s.drainEvents(), "ReadinessTimeout"))
		})
	}
}

func TestReadiness_NoOpForceReapplyKeepsTheRegressionVerdict(t *testing.T) {
	for _, strategy := range rolloutStrategies {
		t.Run(string(strategy), func(t *testing.T) {
			// Given a Deployment Ready for months under a 1s timeout, now short a pod
			s := newReadinessScenarioWith(t, strategy, 2, 1)
			s.givenDeploymentReadyForMonths(2)
			s.setAvailable(2, 1, 1)

			// When force-reapply passes run on it without changing its spec
			generationBefore := s.deploymentGeneration()
			_, failed := s.reconcileForced()
			require.Equal(t, int32(0), failed)
			time.Sleep(1100 * time.Millisecond)
			_, failed = s.reconcileForced()
			require.Equal(t, generationBefore, s.deploymentGeneration(), "the re-apply must have been a no-op")

			// Then it is still a regression: not ready, never timed out
			assert.Equal(t, int32(0), failed)
			assert.Empty(t, eventsWithReason(s.drainEvents(), "ReadinessTimeout"))
		})
	}
}

func TestReadiness_RegressedUpstreamDoesNotBlockItsDependents(t *testing.T) {
	// Given a Deployment that has been Ready for months, and a Service that depends on it
	s := newReadinessScenarioWith(t, lynqv1.PatchStrategyReplace, 2, 1, makeDependentServiceResource(80))
	s.givenDeploymentAndDependentReady()

	// And the Deployment has lost a pod
	s.setAvailable(2, 1, 1)

	// When the template changes the Service
	s.changeDependentServicePort(8080)
	ready, failed, _, _, skipped, _ := s.r.applyResources(context.Background(), s.node, s.sorted, defaultVars(), false)

	// Then the Service is applied and counted Ready — neither skipped nor blocked
	assert.Equal(t, int32(8080), s.dependentServicePort(), "the dependent's new spec must reach the cluster")
	assert.Equal(t, int32(1), ready, "the Service is Ready, the regressed Deployment is not")
	assert.Equal(t, int32(0), failed)
	assert.Equal(t, int32(0), skipped)
	assert.Empty(t, eventsWithReason(s.drainEvents(), "DependencySkipped"))
}

func TestReadiness_UpstreamInANewRolloutBlocksItsDependents(t *testing.T) {
	// Given a Deployment that has been Ready for months, and a Service that depends on it
	s := newReadinessScenarioWith(t, lynqv1.PatchStrategyReplace, 2, 60, makeDependentServiceResource(80))
	s.givenDeploymentAndDependentReady()

	// And the Deployment's spec drifted, so it is regressed and its dependents still flow
	s.driftReplicasPreservingHash(5)

	// When the force-reapply reverts the drift — a rollout Lynq started — while the template
	// also changes the Service
	s.changeDependentServicePort(8080)
	_, failed := s.reconcileForced()

	// Then the Service waits for the new rollout, silently, exactly like a first rollout
	require.Equal(t, int32(0), failed)
	assert.Equal(t, int32(80), s.dependentServicePort(), "dependents must wait while Lynq's own rollout is in flight")
	assert.Empty(t, eventsWithReason(s.drainEvents(), "DependencySkipped"))

	// And once the rollout completes, the Service follows
	s.setAvailable(2, 2, 2)
	s.reconcile()
	assert.Equal(t, int32(8080), s.dependentServicePort())
}

func TestReadinessTracker_RecreatedLynqNodeReleasesThePreviousUID(t *testing.T) {
	// Given readiness state for a LynqNode
	var tr readinessTracker
	key := types.NamespacedName{Namespace: "default", Name: "node"}
	tr.observeReady("uid-1", key, "deploy", "stamp", 0)

	// When it is deleted and recreated under the same name, with no reconcile having seen it
	// NotFound or run its finalizer, and the new incarnation is observed
	tr.observeReady("uid-2", key, "deploy", "stamp", 0)

	// Then only the new incarnation is tracked
	_, oldTracked := tr.nodes.Load(types.UID("uid-1"))
	_, newTracked := tr.nodes.Load(types.UID("uid-2"))
	assert.False(t, oldTracked, "the previous UID's state must not outlive the recreation")
	assert.True(t, newTracked)

	// And forgetting the name releases the new incarnation, not a stale one
	tr.forgetByName(key)
	_, newTracked = tr.nodes.Load(types.UID("uid-2"))
	assert.False(t, newTracked)
}

func TestReadinessTracker_ReadyReadOlderThanOurRolloutDoesNotCount(t *testing.T) {
	// Given an apply of ours just moved the resource to generation 3
	var tr readinessTracker
	uid := types.UID("uid")
	key := types.NamespacedName{Namespace: "default", Name: "node"}
	tr.observeReady(uid, key, "deploy", "stamp", 2)
	tr.startRollout(uid, key, "deploy", time.Now(), 3)

	// When a lagging read still shows the Ready generation 2
	counted := tr.observeReady(uid, key, "deploy", "stamp", 2)

	// Then it is not taken as Ready, and a not-ready read is not a regression
	assert.False(t, counted)
	regressed, _ := tr.observeNotReady(uid, key, "deploy", "stamp", time.Now())
	assert.False(t, regressed, "the new rollout has not been Ready yet")

	// But a read of the new generation is
	assert.True(t, tr.observeReady(uid, key, "deploy", "stamp", 3))
}

func makeDependentServiceResource(port int64) lynqv1.TResource {
	waitReady := true
	return lynqv1.TResource{
		ID: dependentServiceID,
		Spec: unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   map[string]interface{}{"name": dependentServiceName, "namespace": "default"},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{"app": "test"},
				"ports":    []interface{}{map[string]interface{}{"port": port}},
			},
		}},
		NameTemplate:   dependentServiceName,
		DependIds:      []string{"deploy-id"},
		WaitForReady:   &waitReady,
		TimeoutSeconds: 60,
		PatchStrategy:  lynqv1.PatchStrategyReplace,
		ConflictPolicy: lynqv1.ConflictPolicyForce,
	}
}

// givenDeploymentAndDependentReady rolls the Deployment out and lets the Service follow it,
// with the Deployment's apply back-dated as in givenDeploymentReadyForMonths.
func (s *readinessScenario) givenDeploymentAndDependentReady() {
	s.t.Helper()
	s.givenDeploymentReadyForMonths(2)
	require.Equal(s.t, int32(80), s.dependentServicePort(), "the Service must have followed the Ready Deployment")
}

// changeDependentServicePort edits the Service in the LynqNode's resolved template, bumping
// the node generation as the API server would so the render cache picks it up.
func (s *readinessScenario) changeDependentServicePort(port int64) {
	for _, n := range s.sorted {
		if n.Resource.ID == dependentServiceID {
			n.Resource.Spec.Object["spec"].(map[string]interface{})["ports"] =
				[]interface{}{map[string]interface{}{"port": port}}
		}
	}
	s.node.Generation++
}

func (s *readinessScenario) dependentServicePort() int32 {
	s.t.Helper()
	svc := &corev1.Service{}
	err := s.client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: dependentServiceName}, svc)
	if err != nil || len(svc.Spec.Ports) == 0 {
		return 0
	}
	return svc.Spec.Ports[0].Port
}
