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

// Package controller - readiness_regression_test.go
//
// # Steady-State Readiness Regression Tests
//
// lynq.sh/apply-start-time only moves when Lynq applies a changed spec. The readiness timeout
// used to be measured from it unconditionally, so a Deployment applied months ago that briefly
// lost a pod (spot drain, eviction, external `kubectl rollout restart`) had "been not ready"
// for months the moment it blipped: it was marked failed on the spot, its dependents were
// DependencySkipped, and the node went Degraded — paging for a disruption the workload
// controller was already repairing.
//
// The timeout now measures only "Lynq's change never became Ready". A resource that was
// observed Ready after the current apply and then regressed is reported not-ready, never
// timed out; and without Ready history (controller restart) the clock starts no earlier
// than the first not-ready observation, so a restart can delay detection but not fake it.

package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/apply"
	"github.com/k8s-lynq/lynq/internal/graph"
	"github.com/k8s-lynq/lynq/internal/readiness"
	"github.com/k8s-lynq/lynq/internal/status"
)

const steadyStateDeployName = "steady-deploy"

// readinessScenario is a LynqNode with a single waitForReady Deployment, wired to a fake
// cluster and an event recorder that keeps what it was given.
type readinessScenario struct {
	t        *testing.T
	scheme   *runtime.Scheme
	client   client.Client
	node     *lynqv1.LynqNode
	sorted   []*graph.Node
	recorder *record.FakeRecorder
	r        *LynqNodeReconciler
}

func newReadinessScenario(t *testing.T, replicas int64, timeoutSeconds int32) *readinessScenario {
	t.Helper()
	return newReadinessScenarioWith(t, lynqv1.PatchStrategyReplace, replicas, timeoutSeconds)
}

// newReadinessScenarioWith builds the scenario with the Deployment applied through the given
// patch strategy, plus any extra resources (e.g. dependents of the Deployment).
func newReadinessScenarioWith(t *testing.T, strategy lynqv1.PatchStrategy, replicas int64, timeoutSeconds int32, extra ...lynqv1.TResource) *readinessScenario {
	t.Helper()
	scheme := makeBottleneckScheme(t)
	node := makeTestNode("test-node", "default")

	resource := makeDeploymentResource("deploy-id", steadyStateDeployName, strategy, true, timeoutSeconds)
	resource.Spec.Object["spec"].(map[string]interface{})["replicas"] = replicas
	g := graph.NewDependencyGraph()
	for _, res := range append([]lynqv1.TResource{resource}, extra...) {
		require.NoError(t, g.AddResource(res))
	}
	sorted, err := g.TopologicalSort()
	require.NoError(t, err)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).
		WithInterceptorFuncs(generationBumpingFuncs()).Build()
	s := &readinessScenario{t: t, scheme: scheme, client: c, node: node, sorted: sorted}
	s.restartController()
	return s
}

// generationBumpingFuncs makes the fake client move metadata.generation the way the API
// server does for the Deployments these tests reason about: to 1 on create, and by one on every
// write that changes .spec. The fake client
// leaves generation untouched, which would hide whether an apply started a rollout.
func generationBumpingFuncs() interceptor.Funcs {
	write := func(ctx context.Context, c client.WithWatch, obj client.Object, do func() error) error {
		gvk := gvkOf(c, obj)
		if gvk.Kind != resourceKindDeployment {
			return do()
		}
		key := client.ObjectKeyFromObject(obj)
		before := &unstructured.Unstructured{}
		before.SetGroupVersionKind(gvk)
		if err := c.Get(ctx, key, before); err != nil {
			return do()
		}
		if err := do(); err != nil {
			return err
		}
		after := &unstructured.Unstructured{}
		after.SetGroupVersionKind(before.GroupVersionKind())
		if err := c.Get(ctx, key, after); err != nil {
			return err
		}
		// Generation is server-managed: a write cannot set it, only a spec change moves it.
		want := before.GetGeneration()
		if !equality.Semantic.DeepEqual(before.Object["spec"], after.Object["spec"]) {
			want++
		}
		if after.GetGeneration() == want {
			return nil
		}
		after.SetGeneration(want)
		if err := c.Update(ctx, after); err != nil {
			return err
		}
		return c.Get(ctx, key, obj)
	}
	return interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if gvkOf(c, obj).Kind == resourceKindDeployment {
				obj.SetGeneration(1)
			}
			return c.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			return write(ctx, c, obj, func() error { return c.Update(ctx, obj, opts...) })
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			return write(ctx, c, obj, func() error { return c.Patch(ctx, obj, patch, opts...) })
		},
	}
}

func gvkOf(c client.Client, obj client.Object) schema.GroupVersionKind {
	gvk, err := apiutil.GVKForObject(obj, c.Scheme())
	if err != nil {
		return obj.GetObjectKind().GroupVersionKind()
	}
	return gvk
}

// restartController replaces the reconciler with a fresh one, as a controller restart does:
// cluster state survives, in-memory readiness history does not.
func (s *readinessScenario) restartController() {
	s.recorder = record.NewFakeRecorder(100)
	s.r = &LynqNodeReconciler{
		Client:           s.client,
		Scheme:           s.scheme,
		Recorder:         s.recorder,
		StatusManager:    status.NewManager(s.client, status.WithSyncMode()),
		ReadinessChecker: readiness.NewChecker(s.client),
		Applier:          apply.NewApplier(s.client, s.scheme),
	}
}

func (s *readinessScenario) reconcile() (ready, failed int32) {
	s.t.Helper()
	ready, failed, _, _, _, _ = s.r.applyResources(context.Background(), s.node, s.sorted, defaultVars(), false)
	return ready, failed
}

// reconcileForced runs the periodic force-reapply pass, which re-applies every resource even
// when its applied-hash matches.
func (s *readinessScenario) reconcileForced() (ready, failed int32) {
	s.t.Helper()
	ready, failed, _, _, _, _ = s.r.applyResources(context.Background(), s.node, s.sorted, defaultVars(), true)
	return ready, failed
}

// mutateDeployment edits the live Deployment the way the cluster would, leaving Lynq's
// applied-hash in place so the next reconcile skips the apply.
func (s *readinessScenario) mutateDeployment(mutate func(d *appsv1.Deployment)) {
	s.t.Helper()
	d := &appsv1.Deployment{}
	require.NoError(s.t, s.client.Get(context.Background(),
		client.ObjectKey{Namespace: "default", Name: steadyStateDeployName}, d))
	mutate(d)
	desiredStatus := d.Status
	require.NoError(s.t, s.client.Update(context.Background(), d))
	d.Status = desiredStatus
	require.NoError(s.t, s.client.Status().Update(context.Background(), d))
}

func (s *readinessScenario) setAvailable(updated, ready, available int32) {
	s.mutateDeployment(func(d *appsv1.Deployment) {
		d.Status.ObservedGeneration = d.Generation
		d.Status.UpdatedReplicas = updated
		d.Status.ReadyReplicas = ready
		d.Status.AvailableReplicas = available
	})
}

// givenDeploymentReadyForMonths applies the Deployment, lets it become fully available, and
// back-dates its apply-start-time — the state of every long-running production Deployment.
func (s *readinessScenario) givenDeploymentReadyForMonths(replicas int32) {
	s.t.Helper()
	_, failed := s.reconcile()
	require.Equal(s.t, int32(0), failed, "the initial apply must not fail")

	s.setAvailable(replicas, replicas, replicas)
	s.backdateApplyStartTime(90 * 24 * time.Hour)

	ready, failed := s.reconcile()
	require.Equal(s.t, int32(len(s.sorted)), ready, "the Deployment (and anything after it) must be observed Ready before the disruption")
	require.Equal(s.t, int32(0), failed)
	s.drainEvents()
}

func (s *readinessScenario) backdateApplyStartTime(age time.Duration) {
	s.mutateDeployment(func(d *appsv1.Deployment) {
		d.Annotations[apply.AnnotationApplyStartTime] = time.Now().Add(-age).UTC().Format(time.RFC3339Nano)
	})
}

func (s *readinessScenario) drainEvents() []string {
	var events []string
	for {
		select {
		case e := <-s.recorder.Events:
			events = append(events, e)
		default:
			return events
		}
	}
}

func eventsWithReason(events []string, reason string) []string {
	var out []string
	for _, e := range events {
		if strings.Contains(e, " "+reason+" ") {
			out = append(out, e)
		}
	}
	return out
}

func TestReadiness_PreviouslyReadyDeploymentDisruptionIsNotFailure(t *testing.T) {
	cases := []struct {
		name    string
		disrupt func(s *readinessScenario)
	}{
		{
			name:    "one of two replicas lost to a node drain",
			disrupt: func(s *readinessScenario) { s.setAvailable(2, 1, 1) },
		},
		{
			name:    "every replica lost",
			disrupt: func(s *readinessScenario) { s.setAvailable(2, 0, 0) },
		},
		{
			name: "external rollout restart still replacing pods",
			disrupt: func(s *readinessScenario) {
				s.mutateDeployment(func(d *appsv1.Deployment) {
					if d.Spec.Template.Annotations == nil {
						d.Spec.Template.Annotations = map[string]string{}
					}
					d.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().UTC().Format(time.RFC3339)
					d.Status.ObservedGeneration = d.Generation
					d.Status.UpdatedReplicas = 1
					d.Status.ReadyReplicas = 2
					d.Status.AvailableReplicas = 2
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given a 2-replica Deployment that has been Ready for months under a 60s timeout
			s := newReadinessScenario(t, 2, 60)
			s.givenDeploymentReadyForMonths(2)

			// When it drops out of Ready without Lynq changing its spec
			tc.disrupt(s)
			ready, failed := s.reconcile()

			// Then it is reported not ready, never failed
			assert.Equal(t, int32(0), ready)
			assert.Equal(t, int32(0), failed,
				"a months-old apply-start-time must not time out a resource that was already Ready")
			assert.Empty(t, eventsWithReason(s.drainEvents(), "ReadinessTimeout"))

			// And the node is not Ready, but not Degraded either
			update := s.r.calculateLynqNodeStatus(ready, failed, 0, 1, nil, false)
			assert.False(t, update.IsReady)
			assert.False(t, update.IsDegraded)
			assertCondition(t, update.Conditions, ConditionTypeReady, metav1.ConditionFalse, ReasonNotAllResourcesReady)
			assertCondition(t, update.Conditions, ConditionTypeDegraded, metav1.ConditionFalse, "Healthy")

			// And once the workload controller recovers it, it is Ready again
			s.setAvailable(2, 2, 2)
			ready, failed = s.reconcile()
			assert.Equal(t, int32(1), ready)
			assert.Equal(t, int32(0), failed)
		})
	}
}

func TestReadiness_PreviouslyReadyDeploymentPastProgressDeadlineIsFailure(t *testing.T) {
	// Given a Deployment that has been Ready for months
	s := newReadinessScenario(t, 2, 60)
	s.givenDeploymentReadyForMonths(2)

	// When Kubernetes reports its rollout stopped making progress
	s.mutateDeployment(func(d *appsv1.Deployment) {
		d.Status.UpdatedReplicas = 1
		d.Status.Conditions = []appsv1.DeploymentCondition{{
			Type:   appsv1.DeploymentProgressing,
			Status: corev1.ConditionFalse,
			Reason: "ProgressDeadlineExceeded",
		}}
	})
	_, failed := s.reconcile()

	// Then the resource is failed on the workload controller's verdict
	assert.Equal(t, int32(1), failed)
	assert.NotEmpty(t, eventsWithReason(s.drainEvents(), "ProgressDeadlineExceeded"))
	update := s.r.calculateLynqNodeStatus(0, failed, 0, 1, nil, false)
	assert.True(t, update.IsDegraded)
	assertCondition(t, update.Conditions, ConditionTypeDegraded, metav1.ConditionTrue, ReasonResourceFailures)
}

func TestReadiness_NewlyAppliedDeploymentThatNeverBecomesReadyTimesOut(t *testing.T) {
	// Given a freshly applied Deployment with a 1s timeout
	s := newReadinessScenario(t, 1, 1)
	_, failed := s.reconcile()
	require.Equal(t, int32(0), failed, "within the timeout the resource is only not ready")

	// When it is still not Ready after the timeout
	time.Sleep(1100 * time.Millisecond)
	_, failed = s.reconcile()

	// Then it fails with a ReadinessTimeout, exactly as before
	assert.Equal(t, int32(1), failed)
	assert.NotEmpty(t, eventsWithReason(s.drainEvents(), "ReadinessTimeout"))
}

func TestReadiness_RestartWithoutHistoryGrantsAFullTimeoutOfGrace(t *testing.T) {
	// Given a Deployment that has been Ready for months under a 1s timeout
	s := newReadinessScenario(t, 2, 1)
	s.givenDeploymentReadyForMonths(2)

	// And the controller restarts, losing its readiness history
	s.restartController()

	// When the Deployment is not Ready on the first reconcile after the restart
	s.setAvailable(2, 1, 1)
	_, failed := s.reconcile()

	// Then the old apply-start-time does not fail it immediately
	assert.Equal(t, int32(0), failed,
		"without Ready history the clock starts at the first not-ready observation, not the months-old apply")
	assert.Empty(t, eventsWithReason(s.drainEvents(), "ReadinessTimeout"))

	// But if it stays not Ready for a full timeout, it does fail
	time.Sleep(1100 * time.Millisecond)
	_, failed = s.reconcile()
	assert.Equal(t, int32(1), failed)
	assert.NotEmpty(t, eventsWithReason(s.drainEvents(), "ReadinessTimeout"))
}

func TestReadiness_NewApplyAfterReadyIsTimedAfresh(t *testing.T) {
	// Given a Deployment that was Ready under an earlier apply, with a 1s timeout
	s := newReadinessScenario(t, 2, 1)
	s.givenDeploymentReadyForMonths(2)

	// When Lynq applies a new spec (new apply-start-time) that never becomes Ready
	s.mutateDeployment(func(d *appsv1.Deployment) {
		d.Annotations[apply.AnnotationApplyStartTime] = time.Now().UTC().Format(time.RFC3339Nano)
		d.Status.UpdatedReplicas = 0
	})
	_, failed := s.reconcile()
	require.Equal(t, int32(0), failed)
	time.Sleep(1100 * time.Millisecond)
	_, failed = s.reconcile()

	// Then the earlier Ready observation does not shield it: the new rollout times out
	assert.Equal(t, int32(1), failed)
}

func TestCalculateLynqNodeStatus_ProgressingReflectsAnInFlightReconcile(t *testing.T) {
	r := &LynqNodeReconciler{}

	update := r.calculateLynqNodeStatus(1, 0, 0, 3, nil, true)

	assertCondition(t, update.Conditions, ConditionTypeProgressing, metav1.ConditionTrue, "Reconciling")
	assertCondition(t, update.Conditions, ConditionTypeDegraded, metav1.ConditionFalse, "Healthy")
}

func TestCalculateLynqNodeStatus_DegradedOnlyForFailuresAndConflicts(t *testing.T) {
	r := &LynqNodeReconciler{}
	cases := []struct {
		name                    string
		ready, failed, conflict int32
		wantDegraded            bool
		wantDegradedReason      string
		wantReadyReason         string
	}{
		{"all resources ready", 3, 0, 0, false, "Healthy", "Reconciled"},
		{"some resources not ready yet", 1, 0, 0, false, "Healthy", ReasonNotAllResourcesReady},
		{"no resources ready yet", 0, 0, 0, false, "Healthy", ReasonNotAllResourcesReady},
		{"a resource failed", 2, 1, 0, true, ReasonResourceFailures, ReasonResourcesFailed},
		{"a resource conflicted", 2, 0, 1, true, ReasonResourceConflicts, ReasonResourcesConflicted},
		{"failed and conflicted", 1, 1, 1, true, ReasonResourceFailuresAndConflicts, ReasonResourcesFailedAndConflicted},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			update := r.calculateLynqNodeStatus(tc.ready, tc.failed, tc.conflict, 3, nil, false)

			assert.Equal(t, tc.wantDegraded, update.IsDegraded)
			wantStatus := metav1.ConditionFalse
			if tc.wantDegraded {
				wantStatus = metav1.ConditionTrue
			}
			assertCondition(t, update.Conditions, ConditionTypeDegraded, wantStatus, tc.wantDegradedReason)
			assertCondition(t, update.Conditions, ConditionTypeReady, conditionStatusFor(update.IsReady), tc.wantReadyReason)
		})
	}
}

func TestReadinessTracker_LifecycleOfTrackedState(t *testing.T) {
	uid := types.UID("node-uid")
	key := types.NamespacedName{Namespace: "default", Name: "node"}

	t.Run("recovering to Ready ends the not-ready stretch", func(t *testing.T) {
		var tr readinessTracker
		_, first := tr.observeNotReady(uid, key, "deploy", "stamp", time.Now().Add(-time.Hour))
		tr.observeReady(uid, key, "deploy", "stamp", 0)

		regressed, since := tr.observeNotReady(uid, key, "deploy", "stamp", time.Now())
		assert.True(t, regressed, "a resource seen Ready under the same apply has regressed")
		assert.True(t, since.After(first), "the not-ready clock restarts after a recovery")
	})

	t.Run("a Ready observation only counts for the apply it was made under", func(t *testing.T) {
		var tr readinessTracker
		tr.observeReady(uid, key, "deploy", "old-apply", 0)
		regressed, _ := tr.observeNotReady(uid, key, "deploy", "new-apply", time.Now())
		assert.False(t, regressed)
	})

	t.Run("resources removed from the template are dropped", func(t *testing.T) {
		var tr readinessTracker
		tr.observeReady(uid, key, "kept", "s", 0)
		tr.observeReady(uid, key, "removed", "s", 0)
		tr.retain(uid, map[string]struct{}{"kept": {}})

		n := tr.node(uid, key)
		assert.Contains(t, n.resources, "kept")
		assert.NotContains(t, n.resources, "removed")
	})

	t.Run("forgetting a node by UID or by name drops its state", func(t *testing.T) {
		var tr readinessTracker
		other := types.NamespacedName{Namespace: "default", Name: "other"}
		tr.observeReady(uid, key, "deploy", "s", 0)
		tr.observeReady("other-uid", other, "deploy", "s", 0)

		tr.forgetNode(uid)
		_, ok := tr.nodes.Load(uid)
		assert.False(t, ok)

		tr.forgetByName(other)
		_, ok = tr.nodes.Load(types.UID("other-uid"))
		assert.False(t, ok)
	})
}

func TestReadinessTracker_ForgetsDeletedLynqNode(t *testing.T) {
	// Given a LynqNode whose readiness this controller has been tracking
	s := newReadinessScenario(t, 1, 60)
	s.givenDeploymentReadyForMonths(1)
	_, tracked := s.r.readyTracker.nodes.Load(s.node.UID)
	require.True(t, tracked)

	// When the LynqNode is deleted and its finalizer runs
	live := &lynqv1.LynqNode{}
	require.NoError(t, s.client.Get(context.Background(), client.ObjectKeyFromObject(s.node), live))
	live.Finalizers = []string{LynqNodeFinalizer}
	require.NoError(t, s.client.Update(context.Background(), live))
	require.NoError(t, s.client.Delete(context.Background(), live))
	require.NoError(t, s.client.Get(context.Background(), client.ObjectKeyFromObject(s.node), live))
	_, err := s.r.reconcileCleanup(context.Background(), live, time.Now())
	require.NoError(t, err)

	// Then its readiness state is released
	_, tracked = s.r.readyTracker.nodes.Load(s.node.UID)
	assert.False(t, tracked)
}

func TestReadinessTracker_ForgetsLynqNodeThatIsGone(t *testing.T) {
	// Given tracked state for a LynqNode that no longer exists in the API server
	s := newReadinessScenario(t, 1, 60)
	s.givenDeploymentReadyForMonths(1)
	require.NoError(t, s.client.Delete(context.Background(), s.node))

	// When a reconcile finds it NotFound
	_, err := s.r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(s.node)})
	require.NoError(t, err)

	// Then its readiness state is released
	_, tracked := s.r.readyTracker.nodes.Load(s.node.UID)
	assert.False(t, tracked)
}

func assertCondition(t *testing.T, conditions []metav1.Condition, condType string, wantStatus metav1.ConditionStatus, wantReason string) {
	t.Helper()
	for _, c := range conditions {
		if c.Type == condType {
			assert.Equal(t, wantStatus, c.Status, "%s status", condType)
			assert.Equal(t, wantReason, c.Reason, "%s reason", condType)
			return
		}
	}
	t.Errorf("condition %s not found", condType)
}

func conditionStatusFor(b bool) metav1.ConditionStatus {
	if b {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}
