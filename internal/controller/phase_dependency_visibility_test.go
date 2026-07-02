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
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/graph"
)

// Dependency-visibility guard: resources that are NOT applied this reconcile
// because of a dependency (blocked while the dependency is still progressing,
// or skipped because the dependency failed) must still appear in
// status.resourcePhases as Pending with an explanatory reason. Without this,
// an operator looking at a node stuck at "2/5 ready, 0 failed" has no way to
// tell WHICH resources are waiting and WHY.

func makeConfigMapResource(id, name string, dependIds []string) lynqv1.TResource {
	wait := false
	return lynqv1.TResource{
		ID: id,
		Spec: unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]interface{}{"name": name, "namespace": "default"},
			"data":       map[string]interface{}{"k": "v"},
		}},
		NameTemplate:   name,
		DependIds:      dependIds,
		WaitForReady:   &wait,
		ConflictPolicy: lynqv1.ConflictPolicyForce,
		DeletionPolicy: lynqv1.DeletionPolicyDelete,
	}
}

func buildTwoNodeGraph(t *testing.T, a, b lynqv1.TResource) []*graph.Node {
	t.Helper()
	g := graph.NewDependencyGraph()
	if err := g.AddResource(a); err != nil {
		t.Fatalf("AddResource(a): %v", err)
	}
	if err := g.AddResource(b); err != nil {
		t.Fatalf("AddResource(b): %v", err)
	}
	sorted, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	return sorted
}

func findEntry(entries []lynqv1.ResourcePhaseEntry, id string) *lynqv1.ResourcePhaseEntry {
	for i := range entries {
		if entries[i].ID == id {
			return &entries[i]
		}
	}
	return nil
}

// A dependent whose dependency is still rolling out (not ready, within
// timeout) is blocked — it must surface as Pending with a "blocked: waiting
// for dependency" reason and be counted in pendingCount.
func TestApplyResources_DependencyBlocked_VisibleAsPending(t *testing.T) {
	scheme := makeBottleneckScheme(t)
	node := makeTestNode("test-node", "default")

	// dep-a: a mid-rollout Deployment (observedGeneration lags generation) —
	// not ready but within timeout → blocks dependents without failing.
	// Pre-created + PatchStrategyReplace because the fake client does not
	// support SSA apply patches (same pattern as the bottleneck regression
	// tests).
	deploy := makeRollingDeployment("deploy-a", "default", 0)
	resA := makeDeploymentResource("dep-a", "deploy-a", lynqv1.PatchStrategyReplace, true, 300)
	resB := makeConfigMapResource("dep-b", "cm-b", []string{"dep-a"})
	sorted := buildTwoNodeGraph(t, resA, resB)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deploy, node).Build()
	r := makeReconcilerForClient(scheme, c)

	ready, failed, _, _, skipped, _, _, _, pending, _, phases, _ :=
		r.applyResources(context.Background(), node, sorted, defaultVars(), false)

	if failed != 0 || skipped != 0 {
		t.Fatalf("failed=%d skipped=%d, want 0/0 (nothing failed — dependency is just progressing)", failed, skipped)
	}
	if ready != 0 {
		t.Fatalf("ready=%d, want 0", ready)
	}
	// dep-b blocked must be counted as pending. (dep-a itself classifies as
	// Pending or Progressing depending on how the fake client mutates
	// generation/status on update — not the contract under test here.)
	if pending < 1 {
		t.Errorf("pendingCount=%d, want >=1 (dep-b blocked on its dependency)", pending)
	}
	if entryA := findEntry(phases, "dep-a"); entryA == nil {
		t.Errorf("dep-a missing from resourcePhases")
	} else if entryA.Phase != lynqv1.ResourcePhasePending && entryA.Phase != lynqv1.ResourcePhaseProgressing {
		t.Errorf("dep-a phase=%q, want Pending or Progressing (mid-rollout)", entryA.Phase)
	}

	entry := findEntry(phases, "dep-b")
	if entry == nil {
		t.Fatalf("blocked resource dep-b missing from resourcePhases (%d entries) — operator cannot see why the node is not ready", len(phases))
	}
	if entry.Phase != lynqv1.ResourcePhasePending {
		t.Errorf("dep-b phase=%q, want Pending", entry.Phase)
	}
	if !strings.Contains(entry.Reason, "waiting for dependency 'dep-a'") {
		t.Errorf("dep-b reason=%q, want mention of the blocking dependency", entry.Reason)
	}
}

// A dependent whose dependency FAILED (apply error) is skipped — it must
// surface as Pending with a "skipped: dependency ... failed" reason, be listed
// in skippedIds, and NOT inflate pendingCount (skippedResources counts it).
func TestApplyResources_DependencySkipped_VisibleAsPending(t *testing.T) {
	scheme := makeBottleneckScheme(t)
	node := makeTestNode("test-node", "default")

	// dep-a: a kind not registered in the scheme — apply fails deterministically.
	wait := true
	resA := lynqv1.TResource{
		ID: "dep-a",
		Spec: unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata":   map[string]interface{}{"name": "widget-a", "namespace": "default"},
		}},
		NameTemplate:   "widget-a",
		WaitForReady:   &wait,
		TimeoutSeconds: 300,
		ConflictPolicy: lynqv1.ConflictPolicyForce,
		DeletionPolicy: lynqv1.DeletionPolicyDelete,
	}
	resB := makeConfigMapResource("dep-b", "cm-b", []string{"dep-a"})
	sorted := buildTwoNodeGraph(t, resA, resB)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()
	r := makeReconcilerForClient(scheme, c)

	_, failed, _, _, skipped, skippedIds, _, _, pending, _, phases, _ :=
		r.applyResources(context.Background(), node, sorted, defaultVars(), false)

	if failed != 1 {
		t.Fatalf("failedCount=%d, want 1 (dep-a apply error)", failed)
	}
	if skipped != 1 || len(skippedIds) != 1 || skippedIds[0] != "dep-b" {
		t.Fatalf("skipped=%d ids=%v, want dep-b skipped", skipped, skippedIds)
	}
	if pending != 0 {
		t.Errorf("pendingCount=%d, want 0 (skipped resources are counted in skippedResources, not pending)", pending)
	}

	// dep-a: Failed with an apply-error reason.
	entryA := findEntry(phases, "dep-a")
	if entryA == nil || entryA.Phase != lynqv1.ResourcePhaseFailed {
		t.Fatalf("dep-a entry=%+v, want Failed", entryA)
	}
	if !strings.Contains(entryA.Reason, "apply error") {
		t.Errorf("dep-a reason=%q, want an apply-error diagnostic", entryA.Reason)
	}

	// dep-b: Pending with a skipped-because-dependency-failed reason.
	entryB := findEntry(phases, "dep-b")
	if entryB == nil {
		t.Fatalf("skipped resource dep-b missing from resourcePhases (%d entries)", len(phases))
	}
	if entryB.Phase != lynqv1.ResourcePhasePending {
		t.Errorf("dep-b phase=%q, want Pending", entryB.Phase)
	}
	if !strings.Contains(entryB.Reason, "skipped: dependency 'dep-a' failed") {
		t.Errorf("dep-b reason=%q, want a skipped-due-to-failed-dependency diagnostic", entryB.Reason)
	}
}
