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

package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/datasource"
	"github.com/k8s-lynq/lynq/internal/metrics"
)

// =============================================================================
// LynqNode classification (shared by hub status, maxSkew and rollout status)
// =============================================================================

func TestClassifyNode(t *testing.T) {
	cond := func(condType string, status metav1.ConditionStatus) metav1.Condition {
		return metav1.Condition{Type: condType, Status: status}
	}

	tests := []struct {
		name   string
		status lynqv1.LynqNodeStatus
		want   nodeHealth
	}{
		{
			name:   "Ready=True is ready",
			status: lynqv1.LynqNodeStatus{Conditions: []metav1.Condition{cond(ConditionTypeReady, metav1.ConditionTrue)}},
			want:   nodeHealthReady,
		},
		{
			name: "recovering from a regression (Ready=False, Degraded=False, nothing failed) is progressing",
			status: lynqv1.LynqNodeStatus{Conditions: []metav1.Condition{
				cond(ConditionTypeReady, metav1.ConditionFalse),
				cond(ConditionTypeDegraded, metav1.ConditionFalse),
				cond(ConditionTypeConflicted, metav1.ConditionFalse),
			}},
			want: nodeHealthProgressing,
		},
		{
			name:   "Ready=Unknown is progressing",
			status: lynqv1.LynqNodeStatus{Conditions: []metav1.Condition{cond(ConditionTypeReady, metav1.ConditionUnknown)}},
			want:   nodeHealthProgressing,
		},
		{
			name:   "no conditions yet (never reconciled) is progressing",
			status: lynqv1.LynqNodeStatus{},
			want:   nodeHealthProgressing,
		},
		{
			name: "failed resources are failed",
			status: lynqv1.LynqNodeStatus{FailedResources: 1, Conditions: []metav1.Condition{
				cond(ConditionTypeReady, metav1.ConditionFalse),
			}},
			want: nodeHealthFailed,
		},
		{
			name: "Conflicted=True is failed",
			status: lynqv1.LynqNodeStatus{Conditions: []metav1.Condition{
				cond(ConditionTypeReady, metav1.ConditionFalse),
				cond(ConditionTypeConflicted, metav1.ConditionTrue),
			}},
			want: nodeHealthFailed,
		},
		{
			name: "Degraded=True (template, variables or dependency error) is failed",
			status: lynqv1.LynqNodeStatus{Conditions: []metav1.Condition{
				cond(ConditionTypeDegraded, metav1.ConditionTrue),
			}},
			want: nodeHealthFailed,
		},
		{
			name: "a failure signal wins over a stale Ready=True",
			status: lynqv1.LynqNodeStatus{Conditions: []metav1.Condition{
				cond(ConditionTypeReady, metav1.ConditionTrue),
				cond(ConditionTypeDegraded, metav1.ConditionTrue),
			}},
			want: nodeHealthFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &lynqv1.LynqNode{Status: tt.status}
			assert.Equal(t, tt.want, classifyNode(node))
		})
	}
}

// A node that cannot recover keeps its maxSkew slot (policy: stall, do not skip). Failed and
// recovering nodes are both counted as updating, so no further node is admitted.
func TestCountUpdatingNodes_FailedAndRecoveringNodesKeepTheirSlot(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(scheme))
	r := &LynqHubReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), Scheme: scheme}

	failed := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{Name: "failed", Annotations: map[string]string{lynqv1.AnnotationTemplateGeneration: "2"}},
		Status: lynqv1.LynqNodeStatus{Conditions: []metav1.Condition{
			{Type: ConditionTypeReady, Status: metav1.ConditionTrue}, // stale
			{Type: ConditionTypeDegraded, Status: metav1.ConditionTrue},
		}},
	}
	recovering := &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{Name: "recovering", Annotations: map[string]string{lynqv1.AnnotationTemplateGeneration: "2"}},
		Status: lynqv1.LynqNodeStatus{Conditions: []metav1.Condition{
			{Type: ConditionTypeReady, Status: metav1.ConditionFalse, Reason: ReasonNotAllResourcesReady},
		}},
	}

	count := r.countUpdatingNodes(context.Background(), []*lynqv1.LynqNode{failed, recovering}, 2)
	assert.Equal(t, int32(2), count)

	tmpl := &lynqv1.LynqForm{Spec: lynqv1.LynqFormSpec{Rollout: &lynqv1.RolloutConfig{MaxSkew: 2}}}
	assert.False(t, canUpdateNodeWithPrecount(tmpl, count, 0), "both slots are held, the rollout must not advance")
}

// =============================================================================
// rollout.progressDeadlineSeconds: stall exposure on the LynqForm rollout status
// =============================================================================

// rolloutHarness drives LynqFormReconciler.updateStatus against a fake API server with a
// controllable clock, counting LynqForm status writes and LynqNode writes and collecting events.
// hubSync plays the LynqHub controller's part: recording completed updates on the nodes.
type rolloutHarness struct {
	t          *testing.T
	ctx        context.Context
	scheme     *runtime.Scheme
	client     client.Client
	recorder   *record.FakeRecorder
	r          *LynqFormReconciler
	hub        *LynqHubReconciler
	form       types.NamespacedName
	now        time.Time
	writes     int
	nodeWrites int
	listErr    error
	// afterNodeList, when set, runs once right after the next LynqNode list succeeds.
	afterNodeList func()
}

const rolloutTestFormGeneration = 2

var rolloutT0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func newRolloutHarness(t *testing.T, formName string, maxSkew int32, nodes ...*lynqv1.LynqNode) *rolloutHarness {
	t.Helper()
	h := &rolloutHarness{
		t:        t,
		ctx:      context.Background(),
		scheme:   runtime.NewScheme(),
		recorder: record.NewFakeRecorder(100),
		form:     types.NamespacedName{Namespace: "default", Name: formName},
		now:      rolloutT0,
	}
	require.NoError(t, lynqv1.AddToScheme(h.scheme))
	t.Cleanup(func() { deleteRolloutMetrics(formName, "default") })

	form := &lynqv1.LynqForm{
		ObjectMeta: metav1.ObjectMeta{Name: formName, Namespace: "default", Generation: rolloutTestFormGeneration},
		Spec: lynqv1.LynqFormSpec{
			HubID:   "test-hub",
			Rollout: &lynqv1.RolloutConfig{MaxSkew: maxSkew, ProgressDeadlineSeconds: 600},
		},
	}
	hub := &lynqv1.LynqHub{ObjectMeta: metav1.ObjectMeta{Name: "test-hub", Namespace: "default"}}
	objs := []client.Object{hub, form}
	for _, n := range nodes {
		n.Spec.TemplateRef = formName
		objs = append(objs, n)
	}

	h.client = fake.NewClientBuilder().
		WithScheme(h.scheme).
		WithObjects(objs...).
		WithStatusSubresource(&lynqv1.LynqForm{}).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
				if _, ok := obj.(*lynqv1.LynqForm); ok {
					h.writes++
				}
				return c.Status().Update(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				if _, ok := obj.(*lynqv1.LynqNode); ok {
					h.nodeWrites++
				}
				return c.Update(ctx, obj, opts...)
			},
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				_, isNodeList := list.(*lynqv1.LynqNodeList)
				if isNodeList && h.listErr != nil {
					return h.listErr
				}
				if err := c.List(ctx, list, opts...); err != nil {
					return err
				}
				if hook := h.afterNodeList; isNodeList && hook != nil {
					h.afterNodeList = nil
					hook()
				}
				return nil
			},
		}).
		Build()
	h.hub = &LynqHubReconciler{Client: h.client, Scheme: h.scheme, Recorder: h.recorder}
	h.restart()
	return h
}

// restart replaces the reconcilers with fresh ones, as a controller restart or leader change
// does: everything in memory is gone, the API server state is kept.
func (h *rolloutHarness) restart() {
	h.r = &LynqFormReconciler{
		Client:   h.client,
		Scheme:   h.scheme,
		Recorder: h.recorder,
		now:      func() time.Time { return h.now },
	}
	h.hub = &LynqHubReconciler{Client: h.client, Scheme: h.scheme, Recorder: h.recorder}
}

func (h *rolloutHarness) getForm() *lynqv1.LynqForm {
	h.t.Helper()
	form := &lynqv1.LynqForm{}
	require.NoError(h.t, h.client.Get(h.ctx, h.form, form))
	return form
}

// reconcile runs the status pass of one LynqForm reconcile at the given offset from rolloutT0.
func (h *rolloutHarness) reconcile(at time.Duration) *lynqv1.RolloutStatus {
	h.t.Helper()
	h.now = rolloutT0.Add(at)
	require.NoError(h.t, h.r.updateStatus(h.ctx, h.getForm(), nil))
	form := h.getForm()
	require.NotNil(h.t, form.Status.Rollout)
	return form.Status.Rollout
}

// hubSync runs the LynqHub's per-node sync (syncExistingNode) over the form's nodes with every
// rollout slot held, so no node is updated, and returns how many LynqNode writes it made: the
// completion records.
func (h *rolloutHarness) hubSync() int {
	h.t.Helper()
	form := h.getForm()
	hub := &lynqv1.LynqHub{}
	require.NoError(h.t, h.client.Get(h.ctx, types.NamespacedName{Namespace: "default", Name: "test-hub"}, hub))
	nodes := &lynqv1.LynqNodeList{}
	require.NoError(h.t, h.client.List(h.ctx, nodes, client.InNamespace(h.form.Namespace)))
	templateMap := map[string]*lynqv1.LynqForm{form.Name: form}
	before := h.nodeWrites
	for i := range nodes.Items {
		node := &nodes.Items[i]
		row := datasource.NodeRow{UID: node.Spec.UID, Activate: "true", Extra: map[string]string{}}
		outcome := h.hub.syncExistingNode(h.ctx, hub, form, node, row, templateMap, func() bool { return false })
		require.NotEqual(h.t, existingNodeUpdated, outcome)
	}
	return h.nodeWrites - before
}

// hubUpdate runs the LynqHub's update of one node to the form's current generation and the
// given row, and returns the node as written.
func (h *rolloutHarness) hubUpdate(name string, row datasource.NodeRow) *lynqv1.LynqNode {
	h.t.Helper()
	hub := &lynqv1.LynqHub{}
	require.NoError(h.t, h.client.Get(h.ctx, types.NamespacedName{Namespace: "default", Name: "test-hub"}, hub))
	require.NoError(h.t, h.hub.updateLynqNode(h.ctx, hub, h.getForm(), h.getNode(name), row))
	return h.getNode(name)
}

// setFormGeneration moves the LynqForm to a new generation, as a template edit does.
func (h *rolloutHarness) setFormGeneration(generation int64) {
	h.t.Helper()
	form := h.getForm()
	form.Generation = generation
	require.NoError(h.t, h.client.Update(h.ctx, form))
}

// setNode rewrites a node as the LynqNode controller (status) or a test step would.
func (h *rolloutHarness) setNode(name string, mutate func(*lynqv1.LynqNode)) *lynqv1.LynqNode {
	h.t.Helper()
	node := h.getNode(name)
	mutate(node)
	require.NoError(h.t, h.client.Update(h.ctx, node))
	return node
}

func (h *rolloutHarness) getNode(name string) *lynqv1.LynqNode {
	h.t.Helper()
	node := &lynqv1.LynqNode{}
	require.NoError(h.t, h.client.Get(h.ctx, types.NamespacedName{Namespace: "default", Name: name}, node))
	return node
}

func (h *rolloutHarness) events() []string {
	var out []string
	for {
		select {
		case e := <-h.recorder.Events:
			out = append(out, e)
		default:
			return out
		}
	}
}

func (h *rolloutHarness) phaseMetric() float64 {
	return testutil.ToFloat64(metrics.FormRolloutPhase.WithLabelValues(h.form.Name, h.form.Namespace))
}

// updatedNode is a LynqNode the hub updated to the harness form's generation at updateStart and
// has not yet seen complete it.
func updatedNode(name string, updateStart time.Time) *lynqv1.LynqNode {
	return &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  "default",
			UID:        types.UID(name + "-uid"),
			Generation: 5,
			Annotations: map[string]string{
				"lynq.sh/activate":                          "true",
				"lynq.sh/extra":                             "{}",
				lynqv1.AnnotationTemplateGeneration:         fmt.Sprintf("%d", rolloutTestFormGeneration),
				lynqv1.AnnotationRolloutUpdateStartTime:     updateStart.Format(time.RFC3339Nano),
				lynqv1.AnnotationRolloutCompletedGeneration: "",
			},
		},
		Spec:   lynqv1.LynqNodeSpec{UID: name},
		Status: lynqv1.LynqNodeStatus{ObservedGeneration: 5},
	}
}

// outdatedNode is a Ready LynqNode the hub has not updated yet.
func outdatedNode(name string) *lynqv1.LynqNode {
	n := updatedNode(name, rolloutT0.Add(-24*time.Hour))
	n.Annotations[lynqv1.AnnotationTemplateGeneration] = "1"
	n.Annotations[lynqv1.AnnotationRolloutCompletedGeneration] = "1"
	markReady(n)
	return n
}

func markReady(n *lynqv1.LynqNode) {
	n.Status.FailedResources = 0
	n.Status.Conditions = []metav1.Condition{
		{Type: ConditionTypeReady, Status: metav1.ConditionTrue, Reason: "Reconciled"},
		{Type: ConditionTypeDegraded, Status: metav1.ConditionFalse, Reason: "Healthy"},
	}
}

func markNotReady(n *lynqv1.LynqNode) {
	n.Status.FailedResources = 0
	n.Status.Conditions = []metav1.Condition{
		{Type: ConditionTypeReady, Status: metav1.ConditionFalse, Reason: ReasonNotAllResourcesReady},
		{Type: ConditionTypeDegraded, Status: metav1.ConditionFalse, Reason: "Healthy"},
	}
}

func markFailed(n *lynqv1.LynqNode) {
	n.Status.FailedResources = 1
	n.Status.Conditions = []metav1.Condition{
		{Type: ConditionTypeReady, Status: metav1.ConditionFalse, Reason: ReasonResourcesFailed},
		{Type: ConditionTypeDegraded, Status: metav1.ConditionTrue, Reason: ReasonResourceFailures},
	}
}

func withStatus(n *lynqv1.LynqNode, mark func(*lynqv1.LynqNode)) *lynqv1.LynqNode {
	mark(n)
	return n
}

func TestRolloutDeadline_StuckUpdateFailsRolloutAndResumesWhenFixed(t *testing.T) {
	// Given a serial rollout (maxSkew=1) whose first updated node never becomes Ready
	stuck := withStatus(updatedNode("stuck-web", rolloutT0), markNotReady)
	waiting := outdatedNode("waiting-web")
	h := newRolloutHarness(t, "stuck-form", 1, stuck, waiting)

	// Then the rollout is in progress while the deadline has not passed
	assert.Equal(t, lynqv1.RolloutPhaseInProgress, h.reconcile(0).Phase)
	rollout := h.reconcile(600 * time.Second)
	assert.Equal(t, lynqv1.RolloutPhaseInProgress, rollout.Phase, "exactly at the deadline is not past it")
	assert.Empty(t, h.events())

	// When progressDeadlineSeconds elapses without the node reaching Ready
	rollout = h.reconcile(601 * time.Second)

	// Then the rollout is Failed, names the node, and announces it once
	assert.Equal(t, lynqv1.RolloutPhaseFailed, rollout.Phase)
	assert.Contains(t, rollout.Message, "stuck-web (not Ready within 600s)")
	assert.NotContains(t, rollout.Message, "waiting-web")
	assert.NotNil(t, rollout.CompletionTime)
	events := h.events()
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "RolloutFailed")
	assert.Contains(t, events[0], "stuck-web")
	assert.Equal(t, metrics.RolloutPhaseToMetric(string(lynqv1.RolloutPhaseFailed)), h.phaseMetric())

	// And an unchanged stall neither rewrites the status nor repeats the event
	writes := h.writes
	h.reconcile(20 * time.Minute)
	assert.Equal(t, writes, h.writes, "an unchanged stall must not write the LynqForm status")
	assert.Empty(t, h.events())

	// When the cause is fixed and the node becomes Ready
	h.setNode("stuck-web", markReady)
	rollout = h.reconcile(21 * time.Minute)

	// Then the rollout leaves Failed and resumes
	assert.Equal(t, lynqv1.RolloutPhaseInProgress, rollout.Phase)
	assert.Nil(t, rollout.CompletionTime, "the failure's completion time must not survive the recovery")
	events = h.events()
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "RolloutRecovered")
	assert.Equal(t, metrics.RolloutPhaseToMetric(string(lynqv1.RolloutPhaseInProgress)), h.phaseMetric())
}

func TestRolloutDeadline_FailedNodeFailsRolloutWithoutWaitingForTheDeadline(t *testing.T) {
	// Given maxSkew=2 and an updated node whose resources failed (apply error or readiness timeout)
	failed := withStatus(updatedNode("broken-web", rolloutT0), markFailed)
	progressing := withStatus(updatedNode("slow-web", rolloutT0), markNotReady)
	h := newRolloutHarness(t, "failed-form", 2, failed, progressing)

	// When the form is reconciled shortly after the update
	rollout := h.reconcile(10 * time.Second)

	// Then the rollout is Failed because of the failed node only
	assert.Equal(t, lynqv1.RolloutPhaseFailed, rollout.Phase)
	assert.Contains(t, rollout.Message, "broken-web (failed)")
	assert.NotContains(t, rollout.Message, "slow-web")

	// When the failed node is fixed and both finish
	h.setNode("broken-web", markReady)
	h.setNode("slow-web", markReady)
	rollout = h.reconcile(time.Minute)

	// Then the rollout completes
	assert.Equal(t, lynqv1.RolloutPhaseComplete, rollout.Phase)
}

// The SW-12741 trap applied to rollouts: a node that completed its update and regressed later
// (spot drain, `kubectl rollout restart`) carries an old rollout-update-start. Measuring it
// against the deadline would fail the rollout the moment the regression is seen.
func TestRolloutDeadline_RegressionAfterCompletedUpdateNeverFailsRollout(t *testing.T) {
	// Given a node that completed its update and the hub recorded it
	node := withStatus(updatedNode("drained-web", rolloutT0), markReady)
	h := newRolloutHarness(t, "regress-form", 1, node)
	require.Equal(t, 1, h.hubSync())
	assert.Equal(t, lynqv1.RolloutPhaseComplete, h.reconcile(time.Minute).Phase)

	// When it regresses long after the update stamp
	h.setNode("drained-web", markNotReady)

	// Then the rollout stays Complete, however long the recovery takes
	for _, at := range []time.Duration{2 * time.Hour, 2*time.Hour + 11*time.Minute, 5 * time.Hour} {
		assert.Equal(t, lynqv1.RolloutPhaseComplete, h.reconcile(at).Phase, "at %s", at)
	}
	for _, e := range h.events() {
		assert.NotContains(t, e, "RolloutFailed")
	}
}

// A node that completed the update and regressed later is recovering, not rolling out. Counting
// it as updating flipped a finished rollout to InProgress on every lost pod and restamped its
// completionTime once the pod came back.
func TestRolloutStatus_RecoveringNodeKeepsTheRolloutComplete(t *testing.T) {
	// Given a complete rollout over two nodes whose completion the hub recorded
	h := newRolloutHarness(t, "recovering-form", 1,
		withStatus(updatedNode("drained-web", rolloutT0), markReady),
		withStatus(updatedNode("steady-web", rolloutT0), markReady))
	require.Equal(t, 2, h.hubSync())
	completed := h.reconcile(time.Minute)
	require.Equal(t, lynqv1.RolloutPhaseComplete, completed.Phase)
	require.NotNil(t, completed.CompletionTime)
	completionTime := *completed.CompletionTime
	assert.Equal(t, "All 2 nodes updated and ready", completed.Message)
	h.events()

	// When one node loses a pod and is not Ready for a while
	h.setNode("drained-web", markNotReady)
	writes := h.writes
	rollout := h.reconcile(2 * time.Hour)

	// Then the rollout stays Complete with its completionTime, and says a node is recovering
	assert.Equal(t, lynqv1.RolloutPhaseComplete, rollout.Phase)
	require.NotNil(t, rollout.CompletionTime)
	assert.True(t, completionTime.Equal(rollout.CompletionTime), "a recovery is not a new rollout completion")
	assert.Equal(t, "All 2 nodes updated (1 recovering)", rollout.Message)
	assert.Equal(t, int32(0), rollout.UpdatingNodes, "a recovering node is not updating")
	assert.Equal(t, int32(1), rollout.ReadyUpdatedNodes, "only the Ready node is ready-updated")
	assert.Equal(t, int32(2), rollout.UpdatedNodes)
	assert.Equal(t, 1, h.writes-writes, "the recovering count is a status change: one write")
	assert.Equal(t, metrics.RolloutPhaseToMetric(string(lynqv1.RolloutPhaseComplete)), h.phaseMetric())
	assert.Equal(t, 0.0, testutil.ToFloat64(metrics.FormRolloutUpdatingNodes.WithLabelValues(h.form.Name, h.form.Namespace)))
	assert.Equal(t, 100.0, testutil.ToFloat64(metrics.FormRolloutProgress.WithLabelValues(h.form.Name, h.form.Namespace)))
	assert.Empty(t, h.events(), "no rollout transition to announce")

	// And an unchanged recovery does not rewrite the status
	writes = h.writes
	h.reconcile(3 * time.Hour)
	h.reconcile(4 * time.Hour)
	assert.Equal(t, writes, h.writes, "an unchanged recovering count must not write")

	// When a second node regresses too, the count changes and is written once
	h.setNode("steady-web", markNotReady)
	rollout = h.reconcile(5 * time.Hour)
	assert.Equal(t, "All 2 nodes updated (2 recovering)", rollout.Message)
	assert.Equal(t, writes+1, h.writes)

	// When both recover, the rollout reads as it did before the pod was lost
	h.setNode("drained-web", markReady)
	h.setNode("steady-web", markReady)
	rollout = h.reconcile(6 * time.Hour)
	assert.Equal(t, lynqv1.RolloutPhaseComplete, rollout.Phase)
	assert.Equal(t, "All 2 nodes updated and ready", rollout.Message)
	assert.True(t, completionTime.Equal(rollout.CompletionTime), "the completionTime is the rollout's, not the recovery's")
}

// A recovering node that the LynqNode reports failed needs an operator: it stalls the rollout at
// once, completion marker or not.
func TestRolloutStatus_RecoveringNodeThatFailsStallsTheRollout(t *testing.T) {
	h := newRolloutHarness(t, "recovering-failed-form", 1, withStatus(updatedNode("drained-web", rolloutT0), markReady))
	require.Equal(t, 1, h.hubSync())
	h.setNode("drained-web", markNotReady)
	require.Equal(t, lynqv1.RolloutPhaseComplete, h.reconcile(time.Hour).Phase)
	h.events()

	// When the recovery fails (for example ProgressDeadlineExceeded on its Deployment)
	h.setNode("drained-web", markFailed)
	rollout := h.reconcile(time.Hour + time.Minute)

	// Then the rollout is Failed and says why
	assert.Equal(t, lynqv1.RolloutPhaseFailed, rollout.Phase)
	assert.Contains(t, rollout.Message, "drained-web (failed)")
	events := h.events()
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "RolloutFailed")
}

// Completion is durable: a restart or leader change neither forgets that an update completed nor
// that it did not.
func TestRolloutDeadline_RestartKeepsTheVerdict(t *testing.T) {
	t.Run("a node that completed and regressed before the restart is never failed after it", func(t *testing.T) {
		node := withStatus(updatedNode("regressed-web", rolloutT0), markReady)
		h := newRolloutHarness(t, "restart-regress-form", 1, node)
		require.Equal(t, 1, h.hubSync())
		h.reconcile(time.Minute)
		h.setNode("regressed-web", markNotReady)

		h.restart()

		for _, at := range []time.Duration{2 * time.Minute, 601 * time.Second, 10*time.Minute + time.Second, 3 * time.Hour} {
			assert.NotEqual(t, lynqv1.RolloutPhaseFailed, h.reconcile(at).Phase, "at %s", at)
		}
		assert.Equal(t, 0, h.hubSync(), "the restarted hub must not record the completion again")
	})

	t.Run("a node stuck across the restart is failed from its update stamp", func(t *testing.T) {
		node := withStatus(updatedNode("stuck-web", rolloutT0), markNotReady)
		h := newRolloutHarness(t, "restart-stuck-form", 1, node)
		assert.Equal(t, lynqv1.RolloutPhaseInProgress, h.reconcile(5*time.Minute).Phase)

		h.restart()

		assert.Equal(t, lynqv1.RolloutPhaseInProgress, h.reconcile(600*time.Second).Phase)
		rollout := h.reconcile(601 * time.Second)
		assert.Equal(t, lynqv1.RolloutPhaseFailed, rollout.Phase, "the restart must not delay the verdict")
		assert.Contains(t, rollout.Message, "stuck-web (not Ready within 600s)")
	})

	t.Run("a failed node is failed immediately after the restart", func(t *testing.T) {
		node := withStatus(updatedNode("failed-web", rolloutT0), markFailed)
		h := newRolloutHarness(t, "restart-failed-form", 1, node)

		h.restart()

		assert.Equal(t, lynqv1.RolloutPhaseFailed, h.reconcile(10*time.Second).Phase)
	})
}

// Right after the hub updates a node its Ready condition still describes the previous spec.
// That is neither progress nor completion of the update, however long it lasts.
func TestRolloutDeadline_ReadyForThePreviousSpecIsNotCompletion(t *testing.T) {
	// Given an updated node whose Ready=True predates the spec the hub just wrote
	node := withStatus(updatedNode("stale-web", rolloutT0), markReady)
	node.Generation = 6 // the LynqNode controller has only observed generation 5
	h := newRolloutHarness(t, "stale-form", 1, node)

	// Then it is updating, not ready-updated, and the hub does not record a completion
	rollout := h.reconcile(time.Minute)
	assert.Equal(t, lynqv1.RolloutPhaseInProgress, rollout.Phase)
	assert.Equal(t, int32(0), rollout.ReadyUpdatedNodes)
	assert.Equal(t, int32(1), rollout.UpdatingNodes)
	assert.Equal(t, 0, h.hubSync())

	// When the LynqNode controller never catches up past the deadline
	rollout = h.reconcile(601 * time.Second)

	// Then the rollout is Failed although the node still says Ready=True
	assert.Equal(t, lynqv1.RolloutPhaseFailed, rollout.Phase)
	assert.Contains(t, rollout.Message, "stale-web (not Ready within 600s)")
}

// The completion record is one write per node per update, and none when nothing changed.
func TestRolloutCompletion_IsRecordedOncePerNodePerUpdate(t *testing.T) {
	h := newRolloutHarness(t, "record-form", 2,
		withStatus(updatedNode("done-a", rolloutT0), markReady),
		withStatus(updatedNode("done-b", rolloutT0), markReady),
		withStatus(updatedNode("slow-c", rolloutT0), markNotReady),
		outdatedNode("old-d"))

	// When the hub syncs after two nodes completed
	assert.Equal(t, 2, h.hubSync(), "one write per completed node")
	assert.Equal(t, "2", h.getNode("done-a").Annotations[lynqv1.AnnotationRolloutCompletedGeneration])
	assert.Equal(t, "", h.getNode("slow-c").Annotations[lynqv1.AnnotationRolloutCompletedGeneration])

	// Then syncs that observe nothing new write nothing — also after a regression
	assert.Equal(t, 0, h.hubSync())
	h.setNode("done-a", markNotReady)
	h.setNode("done-a", markReady)
	assert.Equal(t, 0, h.hubSync())

	// And the node that completes later is recorded exactly once
	h.setNode("slow-c", markReady)
	assert.Equal(t, 1, h.hubSync())
	assert.Equal(t, 0, h.hubSync())
}

// A completion is not recorded against an update it did not observe.
func TestRolloutCompletion_IsNotRecordedForANewerUpdate(t *testing.T) {
	h := newRolloutHarness(t, "race-form", 1, withStatus(updatedNode("raced-web", rolloutT0), markReady))
	observed := h.getNode("raced-web")

	// When the node is updated again between the observation and the write
	h.setNode("raced-web", func(n *lynqv1.LynqNode) {
		n.Annotations[lynqv1.AnnotationRolloutUpdateStartTime] = rolloutT0.Add(time.Minute).Format(time.RFC3339Nano)
	})
	require.NoError(t, h.hub.recordRolloutCompletion(h.ctx, observed))

	// Then the newer update stays pending
	assert.Equal(t, "", h.getNode("raced-web").Annotations[lynqv1.AnnotationRolloutCompletedGeneration])
}

// A hub update starts a new, uncompleted update: the previous generation's completion does not
// exempt it from the deadline.
func TestRolloutDeadline_NewUpdateResetsTheCompletion(t *testing.T) {
	// Given a node that completed generation 2 and the hub recorded it
	h := newRolloutHarness(t, "again-form", 1, withStatus(updatedNode("again-web", rolloutT0), markReady))
	require.Equal(t, 1, h.hubSync())

	// When the LynqForm moves to generation 3 and the hub updates the node to it
	h.setFormGeneration(3)
	updated := h.hubUpdate("again-web", datasource.NodeRow{UID: "again-web", Activate: "true", Extra: map[string]string{}})

	// Then the completion is reset in that same write
	assert.Equal(t, "3", updated.Annotations[lynqv1.AnnotationTemplateGeneration])
	assert.Equal(t, "", updated.Annotations[lynqv1.AnnotationRolloutCompletedGeneration])

	// And a new spec that never becomes Ready stalls the rollout one deadline after that update
	stamp, err := time.Parse(time.RFC3339Nano, updated.Annotations[lynqv1.AnnotationRolloutUpdateStartTime])
	require.NoError(t, err)
	h.setNode("again-web", func(n *lynqv1.LynqNode) {
		markNotReady(n)
		n.Status.ObservedGeneration = n.Generation
	})
	h.now = stamp.Add(time.Minute)
	require.NoError(t, h.r.updateStatus(h.ctx, h.getForm(), nil))
	rollout := h.getForm().Status.Rollout
	assert.Equal(t, lynqv1.RolloutPhaseInProgress, rollout.Phase, "the generation 2 completion does not complete generation 3")
	assert.Equal(t, int32(1), rollout.UpdatingNodes)
	h.now = stamp.Add(601 * time.Second)
	require.NoError(t, h.r.updateStatus(h.ctx, h.getForm(), nil))
	assert.Equal(t, lynqv1.RolloutPhaseFailed, h.getForm().Status.Rollout.Phase)
}

// progressDeadlineSeconds measures LynqForm rollouts only. A data-only update — row values
// changed under the same template generation — is not one: it keeps the completion marker and is
// not timed against the deadline. Its failures still stall the rollout at once, because the
// LynqNode reports them as failed resources (readiness timeout or apply error).
func TestRolloutDeadline_DataOnlyUpdateIsNotARollout(t *testing.T) {
	t.Run("a completed node keeps its completion through a data-only update", func(t *testing.T) {
		// Given a node that completed generation 2 and the hub recorded it
		h := newRolloutHarness(t, "data-form", 1, withStatus(updatedNode("data-web", rolloutT0), markReady))
		require.Equal(t, 1, h.hubSync())

		// When the hub applies a changed row under the same template generation
		row := datasource.NodeRow{UID: "data-web", HostOrURL: "https://new.example.com", Activate: "true",
			Extra: map[string]string{"plan": "pro"}}
		updated := h.hubUpdate("data-web", row)

		// Then the update is written, but the completion stays recorded
		assert.Equal(t, "https://new.example.com", updated.Annotations["lynq.sh/hostOrUrl"])
		assert.Equal(t, `{"plan":"pro"}`, updated.Annotations["lynq.sh/extra"])
		assert.NotEqual(t, rolloutT0.Format(time.RFC3339Nano), updated.Annotations[lynqv1.AnnotationRolloutUpdateStartTime],
			"the update stamp still moves: the maxSkew safety margin reads it")
		assert.Equal(t, "2", updated.Annotations[lynqv1.AnnotationRolloutCompletedGeneration])

		// And a node that stays not Ready on the new data is never timed against the deadline
		h.setNode("data-web", func(n *lynqv1.LynqNode) {
			markNotReady(n)
			n.Status.ObservedGeneration = n.Generation
		})
		for _, at := range []time.Duration{601 * time.Second, 3 * time.Hour} {
			rollout := h.reconcile(at)
			assert.Equal(t, lynqv1.RolloutPhaseComplete, rollout.Phase, "at %s: a data-only update is not a rollout", at)
			assert.Equal(t, "All 1 nodes updated (1 recovering)", rollout.Message)
		}

		// But once the LynqNode reports the data update failed, the rollout stalls at once
		h.setNode("data-web", markFailed)
		rollout := h.reconcile(3*time.Hour + time.Minute)
		assert.Equal(t, lynqv1.RolloutPhaseFailed, rollout.Phase)
		assert.Contains(t, rollout.Message, "data-web (failed)")
	})

	t.Run("a pending rollout stays pending through a data-only update", func(t *testing.T) {
		// Given a node whose generation 2 rollout has not completed
		h := newRolloutHarness(t, "data-pending-form", 1, withStatus(updatedNode("pending-web", rolloutT0), markNotReady))

		// When a row change is applied before it completes
		updated := h.hubUpdate("pending-web", datasource.NodeRow{UID: "pending-web", Activate: "true",
			Extra: map[string]string{"plan": "pro"}})

		// Then the rollout is still pending, not completed by the data update
		assert.Equal(t, "", updated.Annotations[lynqv1.AnnotationRolloutCompletedGeneration])
		assert.False(t, updateCompleted(updated))
	})
}

// A rollback to a version without the completion marker leaves the marker as that version found
// it: it neither resets nor records it. A rollout it started then carries a marker for an older
// generation, which is pending, not completed; a data-only update it made is not a rollout.
func TestRolloutDeadline_MarkerLeftByARollbackVersion(t *testing.T) {
	t.Run("a template generation change made during the rollback is still timed", func(t *testing.T) {
		// Given the rolled-back version updated the node to generation 2 without resetting the
		// marker of the generation 1 rollout it had completed
		node := withStatus(updatedNode("rolledback-web", rolloutT0), markNotReady)
		node.Annotations[lynqv1.AnnotationRolloutCompletedGeneration] = "1"
		h := newRolloutHarness(t, "rollback-gen-form", 1, node)

		// Then after the re-upgrade the generation 2 rollout is pending and timed from its stamp
		assert.False(t, updateCompleted(h.getNode("rolledback-web")))
		assert.Equal(t, lynqv1.RolloutPhaseInProgress, h.reconcile(600*time.Second).Phase)
		rollout := h.reconcile(601 * time.Second)
		assert.Equal(t, lynqv1.RolloutPhaseFailed, rollout.Phase)
		assert.Contains(t, rollout.Message, "rolledback-web (not Ready within 600s)")

		// And its completion is recorded once it becomes Ready
		h.setNode("rolledback-web", markReady)
		assert.Equal(t, 1, h.hubSync())
		assert.Equal(t, "2", h.getNode("rolledback-web").Annotations[lynqv1.AnnotationRolloutCompletedGeneration])
	})

	t.Run("a data-only update made during the rollback is not timed", func(t *testing.T) {
		// Given the node completed generation 2, and during the rollback a row change was applied
		// to it under the same generation, which the node has not become Ready for
		node := withStatus(updatedNode("rolledback-data-web", rolloutT0), markNotReady)
		node.Annotations[lynqv1.AnnotationRolloutCompletedGeneration] = "2"
		h := newRolloutHarness(t, "rollback-data-form", 1, node)

		// Then it is not a stalled rollout, however long it lasts
		assert.Equal(t, lynqv1.RolloutPhaseComplete, h.reconcile(3*time.Hour).Phase)

		// But it stalls the rollout as soon as the LynqNode reports it failed
		h.setNode("rolledback-data-web", markFailed)
		assert.Equal(t, lynqv1.RolloutPhaseFailed, h.reconcile(3*time.Hour+time.Minute).Phase)
	})
}

// A node being deleted has no rollout left to complete: the hub does not write to it.
func TestRolloutCompletion_IsNotRecordedOnANodeBeingDeleted(t *testing.T) {
	deleting := withStatus(updatedNode("deleting-web", rolloutT0), markReady)
	deleting.Finalizers = []string{LynqNodeFinalizer}
	deleting.DeletionTimestamp = &metav1.Time{Time: rolloutT0}
	h := newRolloutHarness(t, "deleting-form", 1, deleting)

	assert.False(t, needsRolloutCompletionMark(h.getNode("deleting-web"), h.getForm()),
		"a sync must not even read the node back")
	assert.Equal(t, 0, h.hubSync())
	require.NoError(t, h.hub.recordRolloutCompletion(h.ctx, h.getNode("deleting-web")))
	assert.Equal(t, 0, h.nodeWrites)
	assert.Equal(t, "", h.getNode("deleting-web").Annotations[lynqv1.AnnotationRolloutCompletedGeneration])
}

// Rollout stats are computed for one LynqForm generation. If the form moves on before they are
// written, they describe the previous rollout and must not be published under the new one.
func TestRolloutDeadline_StatsForAnOlderFormGenerationAreNotPublished(t *testing.T) {
	// Given generation 2's rollout is stalled on a node past its deadline
	h := newRolloutHarness(t, "moved-form", 1, withStatus(updatedNode("stuck-web", rolloutT0), markNotReady))
	require.Equal(t, lynqv1.RolloutPhaseInProgress, h.reconcile(time.Minute).Phase)
	h.events()
	writes := h.writes
	h.now = rolloutT0.Add(601 * time.Second)

	// When the form moves to generation 3 while that stall is being computed
	h.afterNodeList = func() { h.setFormGeneration(3) }
	result, err := h.r.Reconcile(h.ctx, reconcile.Request{NamespacedName: h.form})

	// Then nothing is published and the reconcile is retried
	require.NoError(t, err)
	assert.True(t, result.Requeue, "the new generation's status must be computed again")
	assert.Equal(t, writes, h.writes, "no status write for the previous generation's stats")
	assert.Empty(t, h.events(), "no RolloutFailed for a rollout that has just started")
	assert.NotEqual(t, lynqv1.RolloutPhaseFailed, h.getForm().Status.Rollout.Phase)

	// And the retry reports generation 3's rollout, which the stuck node has not joined yet
	rollout := h.reconcile(602 * time.Second)
	assert.Equal(t, int64(3), rollout.TargetGeneration)
	assert.NotEqual(t, lynqv1.RolloutPhaseFailed, rollout.Phase)
	for _, e := range h.events() {
		assert.NotContains(t, e, "RolloutFailed")
	}
}

// A node the hub creates starts its first update pending.
func TestRolloutCompletion_NewNodeStartsPending(t *testing.T) {
	h := newRolloutHarness(t, "create-form", 1)
	hub := &lynqv1.LynqHub{}
	require.NoError(t, h.client.Get(h.ctx, types.NamespacedName{Namespace: "default", Name: "test-hub"}, hub))

	row := datasource.NodeRow{UID: "fresh", Activate: "true", Extra: map[string]string{}}
	require.NoError(t, h.hub.createLynqNode(h.ctx, hub, h.getForm(), row))

	created := h.getNode("fresh-create-form")
	completed, ok := created.Annotations[lynqv1.AnnotationRolloutCompletedGeneration]
	assert.True(t, ok, "a new node must carry the marker, or it would count as a pre-marker node")
	assert.Equal(t, "", completed)
	assert.False(t, updateCompleted(created))
}

// A node created before the completion marker existed has no record either way. It must not
// fail a rollout over a regression on upgrade, and the hub does not backfill it.
func TestRolloutDeadline_NodeWithoutCompletionMarkerIsNotTimed(t *testing.T) {
	legacy := withStatus(updatedNode("legacy-web", rolloutT0), markNotReady)
	delete(legacy.Annotations, lynqv1.AnnotationRolloutCompletedGeneration)
	h := newRolloutHarness(t, "legacy-form", 1, legacy)

	rollout := h.reconcile(3 * time.Hour)
	assert.Equal(t, lynqv1.RolloutPhaseComplete, rollout.Phase, "a node that predates the marker completed its update")
	assert.Equal(t, int32(0), rollout.UpdatingNodes)
	assert.Equal(t, "All 1 nodes updated (1 recovering)", rollout.Message)
	h.setNode("legacy-web", markReady)
	assert.Equal(t, 0, h.hubSync(), "no backfill write for nodes that predate the marker")

	// Its failures still stall the rollout
	h.setNode("legacy-web", markFailed)
	assert.Equal(t, lynqv1.RolloutPhaseFailed, h.reconcile(4*time.Hour).Phase)
}

// A steady rollout reconciled again with nothing changed must not write.
func TestRolloutDeadline_NoOpReconcileDoesNotWrite(t *testing.T) {
	h := newRolloutHarness(t, "noop-form", 1,
		withStatus(updatedNode("done-web", rolloutT0), markReady),
		withStatus(updatedNode("slow-web", rolloutT0), markNotReady))
	h.reconcile(time.Minute)

	writes := h.writes
	h.reconcile(2 * time.Minute)
	h.reconcile(3 * time.Minute)
	assert.Equal(t, writes, h.writes)
}

// A node list that cannot be read is not "no nodes": it must not turn a stalled rollout into a
// recovery and back.
func TestRolloutDeadline_ListErrorLeavesAFailedRolloutUntouched(t *testing.T) {
	h := newRolloutHarness(t, "listerr-form", 1, withStatus(updatedNode("broken-web", rolloutT0), markFailed))
	require.Equal(t, lynqv1.RolloutPhaseFailed, h.reconcile(10*time.Second).Phase)
	h.events()
	writes := h.writes

	// When the node list fails
	h.listErr = fmt.Errorf("cache unavailable")
	_, err := h.r.Reconcile(h.ctx, reconcile.Request{NamespacedName: h.form})

	// Then the reconcile fails for a retry and nothing is published
	require.Error(t, err)
	assert.Equal(t, writes, h.writes, "no status write")
	for _, e := range h.events() {
		assert.NotContains(t, e, "RolloutRecovered")
	}
	assert.Equal(t, lynqv1.RolloutPhaseFailed, h.getForm().Status.Rollout.Phase)
	assert.Equal(t, metrics.RolloutPhaseToMetric(string(lynqv1.RolloutPhaseFailed)), h.phaseMetric())
}

// A deleted LynqForm leaves no rollout series behind, or a form deleted while Failed keeps
// LynqFormRolloutFailed firing.
func TestRolloutDeadline_DeletedFormDropsItsRolloutMetrics(t *testing.T) {
	h := newRolloutHarness(t, "deleted-form", 1, withStatus(updatedNode("broken-web", rolloutT0), markFailed))
	require.Equal(t, lynqv1.RolloutPhaseFailed, h.reconcile(10*time.Second).Phase)
	require.Equal(t, metrics.RolloutPhaseToMetric(string(lynqv1.RolloutPhaseFailed)), h.phaseMetric())

	// When the form is deleted and its NotFound reconcile runs
	require.NoError(t, h.client.Delete(h.ctx, h.getForm()))
	_, err := h.r.Reconcile(h.ctx, reconcile.Request{NamespacedName: h.form})
	require.NoError(t, err)

	// Then none of its rollout series exist any more
	assert.False(t, metrics.FormRolloutPhase.DeleteLabelValues(h.form.Name, h.form.Namespace), "phase series left behind")
	assert.False(t, metrics.FormRolloutUpdatingNodes.DeleteLabelValues(h.form.Name, h.form.Namespace), "updating series left behind")
	assert.False(t, metrics.FormRolloutProgress.DeleteLabelValues(h.form.Name, h.form.Namespace), "progress series left behind")
}

func TestFormatStalledMessage_IsBoundedAndOrderIndependent(t *testing.T) {
	var stalled []stalledNode
	for i := 7; i >= 1; i-- {
		stalled = append(stalled, stalledNode{name: fmt.Sprintf("node-%d", i), reason: "failed"})
	}
	reversed := make([]stalledNode, len(stalled))
	for i := range stalled {
		reversed[len(stalled)-1-i] = stalled[i]
	}

	msg := formatStalledMessage(stalled, 600*time.Second)

	assert.Equal(t, msg, formatStalledMessage(reversed, 600*time.Second))
	assert.Contains(t, msg, "7 updated node(s)")
	assert.Contains(t, msg, "node-1 (failed), node-2 (failed), node-3 (failed), node-4 (failed), node-5 (failed), and 2 more")
	assert.False(t, strings.Contains(msg, "node-6"))
}
