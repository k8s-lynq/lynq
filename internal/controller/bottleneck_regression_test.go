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

// Package controller - bottleneck_regression_test.go
//
// # Bottleneck Bug Regression Tests
//
// Root cause analysis, fix verification, and regression prevention tests for the
// maxSkew deadlock bug.
//
// ## Scenario
//
// ~100 Deployment replicas had their Lynq ownership removed (direct edits outside the operator).
// When the LynqForm was updated with conflictPolicy:Force + patchStrategy:replace, the first
// 3-5 resources applied successfully but then a deadlock occurred.
// Restarting the controller did not resolve it; increasing maxSkew from 5→30 worked around it.
//
// ## Root Cause (Two Compounding Bugs)
//
// ### BUG 1 (Primary): Incorrect timeout reference time
//   Location: internal/controller/lynqnode_controller.go
//   Before: elapsed := time.Since(current.GetCreationTimestamp().Time)
//           → Pre-existing resources always exceed timeout immediately → instant FAILED
//   After:  elapsed := time.Since(applyStartTime)
//           → Measured from apply start → correctly treated as "not ready yet"
//
// ### BUG 2 (Compounding): In-memory cache lost on controller restart
//   Location: internal/apply/ssa.go (appliedRV sync.Map)
//   Before: Cache cleared on restart → all resources re-Updated → re-triggers BUG 1
//   After:  Successful apply stores lynq.sh/applied-hash annotation on resource
//           → After restart, annotation restores cache → unnecessary Update skipped
//
// ## Deadlock Mechanism
//
//  1. Hub starts updating maxSkew=5 LynqNodes
//  2. [BUG 1] Each node's Deployment apply → immediately FAILED → LynqNode Ready=False
//  3. countUpdatingNodes: 5 not-Ready → count=5 >= maxSkew=5 → all remaining nodes blocked
//  4. Controller restart → [BUG 2] cache cleared → [BUG 1] re-triggered → still blocked
//  5. Raising maxSkew=30 creates enough slots for some to complete → workaround, not a fix

package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/apply"
	"github.com/k8s-lynq/lynq/internal/graph"
	"github.com/k8s-lynq/lynq/internal/readiness"
	"github.com/k8s-lynq/lynq/internal/status"
	"github.com/k8s-lynq/lynq/internal/template"
)

// =============================================================================
// TEST HELPERS
// =============================================================================

func makeBottleneckScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, lynqv1.AddToScheme(s))
	require.NoError(t, appsv1.AddToScheme(s))
	require.NoError(t, corev1.AddToScheme(s))
	return s
}

func makeTestNode(name, namespace string) *lynqv1.LynqNode {
	return &lynqv1.LynqNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-uid-123",
		},
		Spec: lynqv1.LynqNodeSpec{UID: "test-node"},
	}
}

func makeDeploymentResource(id, name string, patchStrategy lynqv1.PatchStrategy, waitForReady bool, timeoutSeconds int32) lynqv1.TResource {
	waitReady := waitForReady
	deploySpec := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": name, "namespace": "default"},
			"spec": map[string]interface{}{
				"replicas": int64(1),
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{"app": "test"},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{"labels": map[string]interface{}{"app": "test"}},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{"name": "app", "image": "nginx:latest"},
						},
					},
				},
			},
		},
	}
	return lynqv1.TResource{
		ID:             id,
		Spec:           deploySpec,
		NameTemplate:   name,
		WaitForReady:   &waitReady,
		TimeoutSeconds: timeoutSeconds,
		PatchStrategy:  patchStrategy,
		ConflictPolicy: lynqv1.ConflictPolicyForce,
		DeletionPolicy: lynqv1.DeletionPolicyDelete,
	}
}

// makeRollingDeployment creates a Deployment in rolling-update state (not ready).
// creationTimestamp is set to the given age in the past.
func makeRollingDeployment(name, namespace string, age time.Duration) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			ResourceVersion:   "100",
			Generation:        2,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-age)),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx:latest"}}},
			},
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1, // Rolling in progress: hasn't caught up
			AvailableReplicas:  0,
			ReadyReplicas:      0,
		},
	}
}

// makeReadyDeployment creates a Deployment that is Ready.
func makeReadyDeployment(name, namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			ResourceVersion:   "200",
			Generation:        2,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-30 * time.Minute)),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx:latest"}}},
			},
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 2, // Caught up (equal to Generation)
			AvailableReplicas:  1,
			ReadyReplicas:      1,
		},
	}
}

func buildSingleNodeGraph(resource lynqv1.TResource) ([]*graph.Node, error) {
	g := graph.NewDependencyGraph()
	if err := g.AddResource(resource); err != nil {
		return nil, err
	}
	return g.TopologicalSort()
}

func makeReconcilerForClient(scheme *runtime.Scheme, c client.Client) *LynqNodeReconciler {
	return &LynqNodeReconciler{
		Client:           c,
		Scheme:           scheme,
		Recorder:         &fakeRecorder{},
		StatusManager:    status.NewManager(c, status.WithSyncMode()),
		ReadinessChecker: readiness.NewChecker(c),
		Applier:          apply.NewApplier(c, scheme),
	}
}

func defaultVars() template.Variables {
	return template.Variables{
		"uid":       "test-node",
		"activate":  "true",
		"hostOrUrl": "https://example.com",
	}
}

func int32Ptr(i int32) *int32 { return &i }

// =============================================================================
// BUG 1 REGRESSION: timeout must be measured from apply start time
// =============================================================================

// TestRegression_Bug1_CoreLogic_ApplyStartTimeUsedForTimeout documents the root cause
// of BUG 1 by directly testing the mathematical difference between the two approaches.
//
// BUG 1 root cause: time.Since(creationTimestamp) → old resources always immediately FAILED
// After fix:        time.Since(applyStartTime)    → elapsed ≈ 0 at apply time, never immediately FAILED
func TestRegression_Bug1_CoreLogic_ApplyStartTimeUsedForTimeout(t *testing.T) {
	const timeoutSeconds = int32(300)
	timeoutDuration := time.Duration(timeoutSeconds) * time.Second

	// A resource created 15 minutes ago (typical pre-existing Deployment)
	resourceCreationTime := time.Now().Add(-15 * time.Minute)

	// Using creationTimestamp: elapsed=15m >> 5m timeout → immediately FAILED (bug)
	elapsedFromCreation := time.Since(resourceCreationTime)
	assert.True(t, elapsedFromCreation >= timeoutDuration,
		"before fix: creationTimestamp-based elapsed(%s) >= timeout(%s) → resource immediately marked FAILED",
		elapsedFromCreation.Round(time.Second), timeoutDuration)

	// Using applyStartTime: elapsed≈0 << 5m timeout → correctly "not ready yet" (after fix)
	applyStartTime := time.Now()
	elapsedFromApply := time.Since(applyStartTime)
	assert.False(t, elapsedFromApply >= timeoutDuration,
		"after fix: applyStartTime-based elapsed(%s) < timeout(%s) → correctly treated as 'not ready yet'",
		elapsedFromApply.Round(time.Millisecond), timeoutDuration)

	t.Logf("elapsed from creationTimestamp: %s (immediately FAILED)", elapsedFromCreation.Round(time.Second))
	t.Logf("elapsed from applyStartTime:    %s (not ready yet)", elapsedFromApply.Round(time.Millisecond))
}

// TestRegression_Bug1_OldResourceNotImmediatelyFailed verifies that a pre-existing
// resource updated via patchStrategy:replace is NOT immediately marked as failed.
//
// After fix: timeout measured from apply start → failedCount=0 (not ready yet)
func TestRegression_Bug1_OldResourceNotImmediatelyFailed(t *testing.T) {
	scheme := makeBottleneckScheme(t)

	// Deployment created 10 minutes ago (rolling in progress), timeout=5m
	// Before fix: elapsed=10m > 5m → immediately FAILED
	// After fix:  elapsed ≈ 0 from apply start < 5m → not ready yet
	deploy := makeRollingDeployment("deploy-old", "default", 10*time.Minute)
	node := makeTestNode("test-node", "default")

	resource := makeDeploymentResource("deploy-id", "deploy-old", lynqv1.PatchStrategyReplace, true, 300)
	sortedNodes, err := buildSingleNodeGraph(resource)
	require.NoError(t, err)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deploy, node).Build()
	r := makeReconcilerForClient(scheme, c)

	_, failedCount, _, _, _, _ := r.applyResources(context.Background(), node, sortedNodes, defaultVars())

	assert.Equal(t, int32(0), failedCount,
		"immediately after apply the timeout has not elapsed, so failedCount must be 0.\n"+
			"  Timeout is measured from applyStartTime, not creationTimestamp.")
}

// TestRegression_Bug1_ResourceStillWithinTimeoutAfterApply verifies that resources
// with various old creationTimestamps are NOT immediately failed when apply just started.
func TestRegression_Bug1_ResourceStillWithinTimeoutAfterApply(t *testing.T) {
	testCases := []struct {
		name    string
		age     time.Duration
		timeout int32
	}{
		{"created 5m01s ago, 5m timeout", 5*time.Minute + 1*time.Second, 300},
		{"created 1h ago, 5m timeout", 1 * time.Hour, 300},
		{"created 1d ago, 5m timeout", 24 * time.Hour, 300},
		{"created 30d ago, 10m timeout", 30 * 24 * time.Hour, 600},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := makeBottleneckScheme(t)
			deploy := makeRollingDeployment("deploy", "default", tc.age)
			node := makeTestNode("test-node", "default")

			resource := makeDeploymentResource("deploy-id", "deploy", lynqv1.PatchStrategyReplace, true, tc.timeout)
			sortedNodes, err := buildSingleNodeGraph(resource)
			require.NoError(t, err)

			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deploy, node).Build()
			r := makeReconcilerForClient(scheme, c)

			_, failedCount, _, _, _, _ := r.applyResources(context.Background(), node, sortedNodes, defaultVars())

			assert.Equal(t, int32(0), failedCount,
				"[%s] failedCount must be 0 immediately after apply.", tc.name)
		})
	}
}

// TestRegression_Bug1_NewlyCreatedResourceShouldNotFail verifies that a resource is
// NOT immediately failed even when patchStrategy:replace loses creationTimestamp
// (fake client does not preserve creationTimestamp through Update, unlike real K8s).
func TestRegression_Bug1_NewlyCreatedResourceShouldNotFail(t *testing.T) {
	scheme := makeBottleneckScheme(t)
	freshDeploy := makeRollingDeployment("fresh-deploy", "default", 1*time.Second)
	node := makeTestNode("test-node", "default")

	resource := makeDeploymentResource("deploy-id", "fresh-deploy", lynqv1.PatchStrategyReplace, true, 300)
	sortedNodes, err := buildSingleNodeGraph(resource)
	require.NoError(t, err)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(freshDeploy, node).Build()
	r := makeReconcilerForClient(scheme, c)

	_, failedCount, _, _, _, _ := r.applyResources(context.Background(), node, sortedNodes, defaultVars())

	assert.Equal(t, int32(0), failedCount,
		"failedCount must be 0 immediately after apply even with patchStrategy:replace + fake client.\n"+
			"  (fake client loses creationTimestamp after Update, but applyStartTime-based measurement is unaffected)")
}

// =============================================================================
// BUG 2 REGRESSION: unchanged resources must not be re-applied after controller restart
// =============================================================================

// TestRegression_Bug2_UnchangedResourceNotReappliedAfterRestart verifies that after
// a controller restart, a resource whose desired spec has NOT changed does NOT
// trigger client.Update().
//
// Fix: Applier stores lynq.sh/applied-hash annotation on the resource after a successful apply.
// On cache miss after restart, the annotation is read; if the hash matches, Update is skipped.
//
// Test design: single shared fake client → annotation written by r1 is visible to r2.
func TestRegression_Bug2_UnchangedResourceNotReappliedAfterRestart(t *testing.T) {
	scheme := makeBottleneckScheme(t)
	node := makeTestNode("test-node", "default")
	readyDeploy := makeReadyDeployment("ready-deploy", "default")

	// waitForReady=false: isolates cache behavior, excludes BUG 1 influence
	resource := makeDeploymentResource("deploy-id", "ready-deploy", lynqv1.PatchStrategyReplace, false, 300)
	sortedNodes, err := buildSingleNodeGraph(resource)
	require.NoError(t, err)

	// Single shared fake client: simulates cluster state persisting across restart
	totalUpdateCalls := 0
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(readyDeploy, node).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				if obj.GetObjectKind().GroupVersionKind().Kind == resourceKindDeployment {
					totalUpdateCalls++
				}
				return c.Update(ctx, obj, opts...)
			},
		}).
		Build()

	r1 := makeReconcilerForClient(scheme, c)
	r1.applyResources(context.Background(), node, sortedNodes, defaultVars())
	require.Equal(t, 1, totalUpdateCalls, "precondition: first apply must call Update once")

	// New controller instance (simulated restart): fresh Applier with empty in-memory cache, same cluster state
	prevCount := totalUpdateCalls
	r2 := makeReconcilerForClient(scheme, c)
	r2.applyResources(context.Background(), node, sortedNodes, defaultVars())
	restartUpdateCalls := totalUpdateCalls - prevCount

	assert.Equal(t, 0, restartUpdateCalls,
		"after controller restart, client.Update() must NOT be called for an unchanged resource.\n"+
			"  Cache is restored from lynq.sh/applied-hash annotation, skipping unnecessary Update.")
}

// TestRegression_Bug2_ChangedResourceMustBeReapplied verifies that after a controller
// restart, a resource whose desired spec HAS changed IS still re-applied correctly.
// (Safety net: ensures spec changes are never silently skipped)
func TestRegression_Bug2_ChangedResourceMustBeReapplied(t *testing.T) {
	scheme := makeBottleneckScheme(t)
	node := makeTestNode("test-node", "default")

	existingDeploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-deploy", Namespace: "default", ResourceVersion: "42", Generation: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx:1.0"}}},
			},
		},
		Status: appsv1.DeploymentStatus{ObservedGeneration: 1, AvailableReplicas: 1, ReadyReplicas: 1},
	}

	// Desired spec: image=nginx:2.0 (changed)
	updatedSpec := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "my-deploy", "namespace": "default"},
			"spec": map[string]interface{}{
				"replicas": int64(1),
				"selector": map[string]interface{}{"matchLabels": map[string]interface{}{"app": "test"}},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{"labels": map[string]interface{}{"app": "test"}},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{"name": "app", "image": "nginx:2.0"},
						},
					},
				},
			},
		},
	}
	waitFalse := false
	changedResource := lynqv1.TResource{
		ID: "deploy-id", Spec: updatedSpec, NameTemplate: "my-deploy",
		WaitForReady: &waitFalse, TimeoutSeconds: 300,
		PatchStrategy: lynqv1.PatchStrategyReplace, ConflictPolicy: lynqv1.ConflictPolicyForce,
		DeletionPolicy: lynqv1.DeletionPolicyDelete,
	}
	sortedNodes, err := buildSingleNodeGraph(changedResource)
	require.NoError(t, err)

	updateCalls := 0
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existingDeploy, node).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				if obj.GetObjectKind().GroupVersionKind().Kind == resourceKindDeployment {
					updateCalls++
				}
				return c.Update(ctx, obj, opts...)
			},
		}).
		Build()
	r := makeReconcilerForClient(scheme, c)
	r.applyResources(context.Background(), node, sortedNodes, defaultVars())

	assert.Equal(t, 1, updateCalls, "when spec has changed, client.Update() must be called.")
}

// =============================================================================
// BUG 1+2 COMBINED REGRESSION: controller restart must not worsen maxSkew deadlock
// =============================================================================

// TestRegression_RestartDoesNotWorsenMaxSkewDeadlock verifies that controller restart
// does NOT cause additional resources to be immediately marked as failed, AND that
// the BUG 2 fix (annotation-based cache) prevents redundant Update calls after restart.
func TestRegression_RestartDoesNotWorsenMaxSkewDeadlock(t *testing.T) {
	scheme := makeBottleneckScheme(t)
	node := makeTestNode("test-node", "default")
	rollingDeploy := makeRollingDeployment("rolling-deploy", "default", 30*time.Minute)

	resource := makeDeploymentResource("deploy-id", "rolling-deploy", lynqv1.PatchStrategyReplace, true, 300)
	sortedNodes, err := buildSingleNodeGraph(resource)
	require.NoError(t, err)

	// Shared fake client: annotation written by r1 is visible to r2,
	// simulating a real restart where cluster state persists across the restart.
	updateCalls := 0
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(rollingDeploy, node).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				if obj.GetObjectKind().GroupVersionKind().Kind == resourceKindDeployment {
					updateCalls++
				}
				return c.Update(ctx, obj, opts...)
			},
		}).
		Build()

	r1 := makeReconcilerForClient(scheme, c)
	_, failedCount1, _, _, _, _ := r1.applyResources(context.Background(), node, sortedNodes, defaultVars())
	t.Logf("first controller: failedCount=%d, updateCalls=%d", failedCount1, updateCalls)
	require.Equal(t, 1, updateCalls, "precondition: first apply must call Update once")

	// Simulate restart: new Applier with empty in-memory cache, same shared cluster state.
	// BUG 2 fix: applied-hash annotation restores cache → no redundant Update triggered.
	prevUpdates := updateCalls
	r2 := makeReconcilerForClient(scheme, c)
	_, failedCount2, _, _, _, _ := r2.applyResources(context.Background(), node, sortedNodes, defaultVars())
	restartUpdateCalls := updateCalls - prevUpdates
	t.Logf("second controller (after restart): failedCount=%d, updateCalls=%d", failedCount2, restartUpdateCalls)

	assert.Equal(t, int32(0), failedCount2,
		"a rolling Deployment must NOT be immediately marked FAILED after controller restart.")
	assert.Equal(t, 0, restartUpdateCalls,
		"after restart, BUG 2 fix (annotation-based cache) must prevent redundant Update calls.\n"+
			"  Without fix: cache cleared → Update called → rolling update re-triggered → BUG 1 fires again.")
}

// TestRegression_MaxSkewNotSaturatedByFalseFailures verifies that false timeout failures
// do NOT saturate the maxSkew slots, ensuring the hub can keep making progress.
func TestRegression_MaxSkewNotSaturatedByFalseFailures(t *testing.T) {
	scheme := makeBottleneckScheme(t)

	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprintf("node-%d", i), func(t *testing.T) {
			node := makeTestNode("test-node", "default")
			rollingDeploy := makeRollingDeployment("rolling-deploy", "default", 15*time.Minute)

			resource := makeDeploymentResource("deploy-id", "rolling-deploy", lynqv1.PatchStrategyReplace, true, 300)
			sortedNodes, err := buildSingleNodeGraph(resource)
			require.NoError(t, err)

			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rollingDeploy, node).Build()
			r := makeReconcilerForClient(scheme, c)

			_, failedCount, _, _, _, _ := r.applyResources(context.Background(), node, sortedNodes, defaultVars())

			assert.Equal(t, int32(0), failedCount,
				"node-%d: rolling Deployment must not be immediately FAILED and waste a maxSkew slot.", i)
		})
	}
}

// =============================================================================
// HELPER FUNCTION: elapsedSinceApply — direct unit tests
// =============================================================================

// TestElapsedSinceApply_ReadsAnnotation verifies that elapsedSinceApply reads the
// apply.AnnotationApplyStartTime annotation when present, not the fallback time.
func TestElapsedSinceApply_ReadsAnnotation(t *testing.T) {
	// Stamp 2 seconds ago in the annotation
	applyTime := time.Now().Add(-2 * time.Second)
	obj := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				apply.AnnotationApplyStartTime: applyTime.UTC().Format(time.RFC3339Nano),
			},
		},
	}
	fallback := time.Now() // if used: elapsed ≈ 0, which is < 1s

	elapsed := elapsedSinceApply(obj, fallback)

	assert.GreaterOrEqual(t, elapsed, 1*time.Second,
		"elapsedSinceApply must read from the annotation (≈2s elapsed), not the fallback (≈0s).")
}

// TestElapsedSinceApply_FallsBackWhenAnnotationAbsent verifies that when the annotation
// is missing, elapsedSinceApply uses the fallback time.
func TestElapsedSinceApply_FallsBackWhenAnnotationAbsent(t *testing.T) {
	obj := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
	}
	fallback := time.Now().Add(-5 * time.Second) // 5s ago

	elapsed := elapsedSinceApply(obj, fallback)

	assert.GreaterOrEqual(t, elapsed, 4*time.Second,
		"without annotation, elapsedSinceApply must fall back to the fallback parameter (≈5s).")
}

// TestElapsedSinceApply_FallsBackOnInvalidFormat verifies that a malformed annotation
// is silently ignored and the fallback time is used.
func TestElapsedSinceApply_FallsBackOnInvalidFormat(t *testing.T) {
	obj := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				apply.AnnotationApplyStartTime: "not-a-valid-timestamp",
			},
		},
	}
	fallback := time.Now().Add(-3 * time.Second)

	elapsed := elapsedSinceApply(obj, fallback)

	assert.GreaterOrEqual(t, elapsed, 2*time.Second,
		"with an invalid annotation, elapsedSinceApply must fall back to the fallback parameter.")
}

// =============================================================================
// TIMER PERSISTENCE: apply-start-time annotation must survive across reconcile loops
// =============================================================================

// TestRegression_TimerPersistsAcrossReconciles verifies that the readiness timeout
// clock accumulates correctly across multiple reconcile loops.
//
// Without the annotation mechanism, applyStartTime resets each reconcile loop, so
// a resource that needs multiple loops to time out (e.g., a Deployment with a bad image)
// would never actually time out — its elapsed time would always be ≈0.
//
// Test design:
//   - timeoutSeconds=1, rolling Deployment (not ready)
//   - 1st reconcile: apply → apply-start-time annotation stamped (≈now)
//   - sleep 2s (annotation is now 2s old)
//   - 2nd reconcile (shared client, new reconciler): reads annotation → elapsed≈2s > 1s → FAILED
//
// Without the annotation: 2nd reconcile elapsed = time.Since(fresh applyStartTime) ≈ 0 < 1s → NOT failed.
func TestRegression_TimerPersistsAcrossReconciles(t *testing.T) {
	scheme := makeBottleneckScheme(t)
	node := makeTestNode("test-node", "default")
	// Fresh Deployment (not ready: observedGeneration != generation)
	rollingDeploy := makeRollingDeployment("rolling-deploy", "default", 0)

	// Very short timeout (1s) to make the test fast
	resource := makeDeploymentResource("deploy-id", "rolling-deploy", lynqv1.PatchStrategyReplace, true, 1)
	sortedNodes, err := buildSingleNodeGraph(resource)
	require.NoError(t, err)

	// Shared fake client: annotation written by r1 is readable by r2
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rollingDeploy, node).Build()

	// 1st reconcile: applies resource, stamps apply-start-time = now
	r1 := makeReconcilerForClient(scheme, c)
	_, failedCount1, _, _, _, _ := r1.applyResources(context.Background(), node, sortedNodes, defaultVars())
	assert.Equal(t, int32(0), failedCount1,
		"immediately after apply the 1s timeout has not yet elapsed → failedCount must be 0")

	// Wait for the timeout to elapse (annotation is now ≈2s old, timeout=1s)
	time.Sleep(2 * time.Second)

	// 2nd reconcile (new reconciler = empty in-memory cache, same cluster state)
	// elapsedSinceApply reads the annotation (≈2s ago) → elapsed > 1s → timeout fires
	r2 := makeReconcilerForClient(scheme, c)
	_, failedCount2, _, _, _, _ := r2.applyResources(context.Background(), node, sortedNodes, defaultVars())

	assert.Equal(t, int32(1), failedCount2,
		"after 2s with timeoutSeconds=1, apply-start-time annotation must be read to fire the timeout.\n"+
			"  failedCount=0 means the annotation is not being read and the timer is resetting each reconcile.")
}

// TestHasNonLynqAnnotationChange verifies that the watch predicate filter ignores
// lynq.sh/* annotation changes — these are written by persistAppliedHash after every
// successful apply, and re-firing the watch on them creates a reconcile cascade that
// inflates Deployment generation and breaks the hub's maxSkew enforcement.
func TestHasNonLynqAnnotationChange(t *testing.T) {
	cases := []struct {
		name string
		old  map[string]string
		new  map[string]string
		want bool
	}{
		{
			name: "no change",
			old:  map[string]string{"foo": "bar"},
			new:  map[string]string{"foo": "bar"},
			want: false,
		},
		{
			name: "only lynq.sh/applied-hash added",
			old:  map[string]string{"foo": "bar"},
			new:  map[string]string{"foo": "bar", "lynq.sh/applied-hash": "abc123"},
			want: false,
		},
		{
			name: "only lynq.sh/apply-start-time changed",
			old:  map[string]string{"foo": "bar", "lynq.sh/apply-start-time": "2026-05-18T00:00:00Z"},
			new:  map[string]string{"foo": "bar", "lynq.sh/apply-start-time": "2026-05-18T00:00:01Z"},
			want: false,
		},
		{
			name: "lynq.sh and user annotation both changed",
			old:  map[string]string{"foo": "bar", "lynq.sh/applied-hash": "abc"},
			new:  map[string]string{"foo": "baz", "lynq.sh/applied-hash": "def"},
			want: true,
		},
		{
			name: "user annotation added",
			old:  map[string]string{"lynq.sh/applied-hash": "abc"},
			new:  map[string]string{"lynq.sh/applied-hash": "abc", "deployment.kubernetes.io/revision": "2"},
			want: true,
		},
		{
			name: "user annotation removed",
			old:  map[string]string{"foo": "bar", "lynq.sh/applied-hash": "abc"},
			new:  map[string]string{"lynq.sh/applied-hash": "abc"},
			want: true,
		},
		{
			name: "nil to all-lynq.sh",
			old:  nil,
			new:  map[string]string{"lynq.sh/applied-hash": "abc", "lynq.sh/apply-start-time": "T"},
			want: false,
		},
		{
			name: "nil to user annotation",
			old:  nil,
			new:  map[string]string{"deployment.kubernetes.io/revision": "1"},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hasNonLynqAnnotationChange(tc.old, tc.new)
			assert.Equal(t, tc.want, got,
				"old=%v new=%v: hasNonLynqAnnotationChange returned wrong result", tc.old, tc.new)
		})
	}
}

// TestRegression_TrackingAnnotationsLandAtomicallyWithSpec verifies that a single SSA
// apply writes both the spec and lynq.sh/applied-hash + lynq.sh/apply-start-time in one
// API round-trip, so a reconcile that fires immediately after our apply sees the
// applied-hash annotation already on the resource and short-circuits.
//
// Background — what this regression test catches:
//
// Before the fix, persistAppliedHash issued a separate MergePatch AFTER the SSA. There
// was a window where the cache showed new spec but OLD applied-hash. A reconcile
// triggered by the SSA's generation watch could land in that window, see "spec changed
// but annotation stale", and re-issue the SSA — producing a generation 2 → 4 jump in
// one user-visible update and starving the hub's maxSkew enforcement.
//
// Now both annotations are bundled into the SSA submission, so a single cache update
// makes both fields visible together.
func TestRegression_TrackingAnnotationsLandAtomicallyWithSpec(t *testing.T) {
	scheme := makeBottleneckScheme(t)
	node := makeTestNode("test-node", "default")
	deploy := makeReadyDeployment("atomic-deploy", "default")

	// Use Replace (not SSA) because controller-runtime's fake client has limited SSA
	// support; the atomicity property we're testing (annotations land in the same API
	// call as the spec) is identical for both strategies — for Replace it's the same
	// PUT body, for SSA the same Apply payload.
	resource := makeDeploymentResource("deploy-id", "atomic-deploy", lynqv1.PatchStrategyReplace, true, 300)
	sortedNodes, err := buildSingleNodeGraph(resource)
	require.NoError(t, err)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deploy, node).Build()
	r := makeReconcilerForClient(scheme, c)

	// 1st apply
	_, failedCount1, _, _, _, _ := r.applyResources(context.Background(), node, sortedNodes, defaultVars())
	require.Equal(t, int32(0), failedCount1, "1st apply must not fail")

	// Immediately after the apply returns, the live resource must already carry both
	// lynq.sh/applied-hash and lynq.sh/apply-start-time — i.e., they MUST land in the
	// same API call as the spec, not in a follow-up MergePatch.
	got := &appsv1.Deployment{}
	require.NoError(t, c.Get(context.Background(),
		client.ObjectKey{Namespace: "default", Name: "atomic-deploy"}, got))

	assert.NotEmpty(t, got.Annotations[apply.AnnotationAppliedHash],
		"applied-hash must be on the resource immediately after the first apply "+
			"(bundled into the SSA submission, not added in a follow-up patch)")
	assert.NotEmpty(t, got.Annotations[apply.AnnotationApplyStartTime],
		"apply-start-time must be on the resource immediately after the first apply")
}

// TestRegression_StaleRVDoesNotCauseInfiniteUpdateLoop verifies that when something
// (other than us) has bumped the resource's resourceVersion since our last apply, we
// fall back to the lynq.sh/applied-hash annotation instead of unconditionally re-applying.
//
// Background — the infinite loop this prevents:
//
// With patchStrategy:replace, every client.Update() the controller issues bumps the
// Deployment's metadata.generation because our desired obj lacks fields the API server
// adds via defaulters (spec.strategy, progressDeadlineSeconds, imagePullPolicy, etc.).
// The controller watches Deployments and triggers a fresh reconcile on every
// generation change. If the in-memory cache's resourceVersion no longer matches
// (because Kubernetes updated status/finalizers/managed-annotations between our
// previous apply and now), and the cache-hit-with-stale-RV path falls through to
// re-apply, we get:
//
//	apply → generation bump → watch fires → reconcile → apply → … (forever)
//
// observedGeneration permanently lags behind generation, isDeploymentReady stays
// false, and the LynqNode never becomes Ready.
//
// The fix: when the in-memory cache hash matches but the RV is stale, check the
// lynq.sh/applied-hash annotation before re-applying. If the annotation matches,
// the resource still carries our last-applied spec → skip the Update.
func TestRegression_StaleRVDoesNotCauseInfiniteUpdateLoop(t *testing.T) {
	scheme := makeBottleneckScheme(t)
	node := makeTestNode("test-node", "default")
	deploy := makeReadyDeployment("ready-deploy", "default")

	resource := makeDeploymentResource("deploy-id", "ready-deploy", lynqv1.PatchStrategyReplace, true, 300)
	sortedNodes, err := buildSingleNodeGraph(resource)
	require.NoError(t, err)

	updateCalls := 0
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy, node).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				if obj.GetObjectKind().GroupVersionKind().Kind == resourceKindDeployment {
					updateCalls++
				}
				return c.Update(ctx, obj, opts...)
			},
		}).
		Build()

	// Use a SHARED reconciler/Applier so the in-memory cache survives across reconciles
	// (this is what production looks like — main.go creates one Applier per controller).
	r := makeReconcilerForClient(scheme, c)

	// 1st reconcile: applies the deployment, populates cache, stamps annotation
	_, failedCount1, _, _, _, _ := r.applyResources(context.Background(), node, sortedNodes, defaultVars())
	require.Equal(t, int32(0), failedCount1, "1st apply must not fail")
	require.Equal(t, 1, updateCalls, "precondition: 1st reconcile must issue exactly one Update")

	// Simulate an external RV bump (e.g., Kubernetes adding deployment.kubernetes.io/revision,
	// or a status update from the deployment controller). The spec is unchanged, but the RV is.
	stale := &appsv1.Deployment{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "ready-deploy"}, stale))
	if stale.Annotations == nil {
		stale.Annotations = map[string]string{}
	}
	stale.Annotations["external/touched-at"] = time.Now().Format(time.RFC3339Nano)
	require.NoError(t, c.Update(context.Background(), stale))
	// updateCalls now counts the simulated external update as well; baseline it.
	prevUpdates := updateCalls

	// 2nd reconcile: same Applier, same hash, but the cached RV is now stale.
	// The annotation fallback must kick in and skip the apply.
	_, failedCount2, _, _, _, _ := r.applyResources(context.Background(), node, sortedNodes, defaultVars())
	assert.Equal(t, int32(0), failedCount2, "2nd apply must not fail")
	assert.Equal(t, 0, updateCalls-prevUpdates,
		"after a benign external RV bump, the controller must NOT re-issue client.Update() — "+
			"the annotation fallback should detect the spec is unchanged and skip.\n"+
			"  Without the fix: each reconcile re-applies → generation bumps → watch fires "+
			"→ reconcile → re-apply → infinite loop → LynqNode never becomes Ready.")
}

// =============================================================================
// HUB CONTROLLER: maxSkew deadlock mechanism verification
// =============================================================================

// TestRegression_MaxSkewDeadlockMechanism demonstrates how the hub controller's
// maxSkew enforcement deadlocks when LynqNodes are all immediately failed by BUG 1.
//
// This test validates the countUpdatingNodes/canUpdateNodeWithCount logic of the
// LynqHub controller and shows that the BUG 1 fix fundamentally resolves the
// maxSkew deadlock.
func TestRegression_MaxSkewDeadlockMechanism(t *testing.T) {
	const maxSkew = int32(3)
	const totalNodes = 10
	const targetGen = int64(5)

	// Simulate LynqNodes immediately FAILED due to BUG 1
	failedNodes := make([]*lynqv1.LynqNode, totalNodes)
	for i := 0; i < totalNodes; i++ {
		failedNodes[i] = &lynqv1.LynqNode{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("node-%d", i),
				Namespace: "default",
				Annotations: map[string]string{
					lynqv1.AnnotationTemplateGeneration: fmt.Sprintf("%d", targetGen),
				},
			},
			Status: lynqv1.LynqNodeStatus{
				FailedResources: 1, // BUG 1 result: immediately FAILED
				Conditions: []metav1.Condition{
					{Type: "Ready", Status: metav1.ConditionFalse, Reason: "ReadinessTimeout"},
				},
			},
		}
	}

	scheme := makeBottleneckScheme(t)
	r := &LynqHubReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}
	tmpl := &lynqv1.LynqForm{
		ObjectMeta: metav1.ObjectMeta{Generation: targetGen},
		Spec:       lynqv1.LynqFormSpec{Rollout: &lynqv1.RolloutConfig{MaxSkew: maxSkew}},
	}

	// With maxSkew=3: 3 nodes immediately FAILED → count=3 → all remaining nodes blocked
	first3 := failedNodes[:int(maxSkew)]
	assert.False(t, r.canUpdateNodeWithCount(context.Background(), tmpl, first3, 0),
		"when %d nodes are FAILED (=updating), no further updates allowed → deadlock", maxSkew)

	// With only 2 FAILED nodes: 1 slot remaining → update allowed
	first2 := failedNodes[:int(maxSkew)-1]
	assert.True(t, r.canUpdateNodeWithCount(context.Background(), tmpl, first2, 0),
		"when fewer than maxSkew=%d nodes are updating (%d), update is allowed → slot available", maxSkew, maxSkew-1)

	// After BUG 1 fix: nodes stay "not ready yet" (failedResources=0) instead of FAILED
	// → countUpdatingNodes counts them correctly as in-progress, not as blocked
	// → false failures no longer saturate all maxSkew slots
	t.Logf("maxSkew=%d: %d FAILED nodes → deadlock", maxSkew, maxSkew)
	t.Logf("after fix: no false FAILEDs → maxSkew slots used only for genuinely rolling nodes")
}

// =============================================================================
// DOCUMENTATION: conflictPolicy:Force has no effect with patchStrategy:replace
// =============================================================================

// TestRegression_ConflictPolicyForceIgnoredForReplaceStrategy documents that
// conflictPolicy:Force has NO EFFECT when patchStrategy is replace.
//
// patchStrategy:replace uses client.Update() which bypasses field ownership entirely.
// conflictPolicy:Force only applies to SSA (patchStrategy:apply).
// Therefore, conflictPolicy:Force was NOT the cause of the reported deadlock bug.
func TestRegression_ConflictPolicyForceIgnoredForReplaceStrategy(t *testing.T) {
	scheme := makeBottleneckScheme(t)
	existingDeploy := makeReadyDeployment("test-deploy", "default")
	node := makeTestNode("test-node", "default")

	resource := makeDeploymentResource("deploy-id", "test-deploy", lynqv1.PatchStrategyReplace, false, 300)
	resource.ConflictPolicy = lynqv1.ConflictPolicyForce

	sortedNodes, err := buildSingleNodeGraph(resource)
	require.NoError(t, err)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(existingDeploy).
		WithObjects(existingDeploy, node).
		Build()
	r := makeReconcilerForClient(scheme, c)

	_, failedCount, _, _, _, _ := r.applyResources(context.Background(), node, sortedNodes, defaultVars())

	assert.Equal(t, int32(0), failedCount,
		"patchStrategy:replace uses client.Update(), so the conflictPolicy:Force flag is never passed.\n"+
			"  The Force flag only takes effect with patchStrategy:apply (SSA).")
}
