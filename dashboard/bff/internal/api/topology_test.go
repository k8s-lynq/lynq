package api

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func cond(condType, status, reason string) interface{} {
	return map[string]interface{}{"type": condType, "status": status, "reason": reason}
}

func lynqNode(name, form string, status map[string]interface{}) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": "default",
			"labels":    map[string]interface{}{"lynq.sh/hub": "mysql"},
		},
		"spec":   map[string]interface{}{"templateRef": form},
		"status": status,
	}}
}

func findTopologyNode(t *testing.T, topo *TopologyData, id string) TopologyNode {
	t.Helper()
	for _, n := range topo.Nodes {
		if n.ID == id {
			return n
		}
	}
	t.Fatalf("topology node %q not found", id)
	return TopologyNode{}
}

func TestBuildTopology_ThreeStateHealth(t *testing.T) {
	hubs := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "mysql", "namespace": "default"},
		"status": map[string]interface{}{
			"desired": int64(4), "ready": int64(1), "failed": int64(1),
			"conditions": []interface{}{cond("Ready", "True", "DatabaseConnected")},
		},
	}}}}
	forms := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web", "namespace": "default"},
		"spec":     map[string]interface{}{"hubId": "mysql"},
		"status": map[string]interface{}{
			"totalNodes": int64(3), "readyNodes": int64(1),
			"rollout": map[string]interface{}{"phase": "Failed", "message": "stalled nodes: c-web"},
			"conditions": []interface{}{
				cond("Valid", "True", "ValidationPassed"),
				cond("Applied", "False", "NotAllNodesReady"),
			},
		},
	}}}}
	nodes := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{
		lynqNode("a-web", "web", map[string]interface{}{
			"conditions": []interface{}{cond("Ready", "True", "Reconciled")},
		}),
		// Recovering a lost pod: not ready, not degraded
		lynqNode("b-web", "web", map[string]interface{}{
			"conditions": []interface{}{
				cond("Ready", "False", "NotAllResourcesReady"),
				cond("Degraded", "False", "Healthy"),
			},
		}),
		lynqNode("c-web", "web", map[string]interface{}{
			"failedResources": int64(1),
			"conditions": []interface{}{
				cond("Ready", "False", "NotAllResourcesReady"),
				cond("Degraded", "True", "ResourcesFailed"),
				cond("Conflicted", "True", "ResourceConflict"),
			},
		}),
	}}

	topo := buildTopology(hubs, forms, nodes, nil)

	hub := findTopologyNode(t, topo, "hub-default-mysql")
	if hub.Status != statusFailed {
		t.Errorf("hub status = %q, want %q (c-web is failed)", hub.Status, statusFailed)
	}
	// Desired comes from the hub status; ready and failed are counted from its LynqNodes
	if hub.Metrics != (TopologyMetrics{Desired: 4, Ready: 1, Failed: 1}) {
		t.Errorf("hub metrics = %+v, want desired 4, ready 1, failed 1", hub.Metrics)
	}

	form := findTopologyNode(t, topo, "form-default-web")
	if form.Status != statusFailed {
		t.Errorf("form status = %q, want %q (rollout phase Failed)", form.Status, statusFailed)
	}
	if form.Metrics.Failed != 1 {
		t.Errorf("form failed nodes = %d, want 1 (the recovering node is not failed)", form.Metrics.Failed)
	}
	if form.Metadata["rolloutPhase"] != "Failed" || form.Metadata["rolloutMessage"] != "stalled nodes: c-web" {
		t.Errorf("form metadata = %v, want the rollout phase and message", form.Metadata)
	}

	for id, want := range map[string]string{
		"node-default-a-web": statusReady,
		"node-default-b-web": statusPending,
		"node-default-c-web": statusFailed,
	} {
		if got := findTopologyNode(t, topo, id).Status; got != want {
			t.Errorf("%s status = %q, want %q", id, got, want)
		}
	}
	if c := findTopologyNode(t, topo, "node-default-c-web").Metadata["conflicted"]; c != true {
		t.Errorf("c-web conflicted = %v, want true", c)
	}
	if c := findTopologyNode(t, topo, "node-default-b-web").Metadata["conflicted"]; c != false {
		t.Errorf("b-web conflicted = %v, want false", c)
	}
}

func TestBuildTopology_FailedNodesCountedPerFormAndNamespace(t *testing.T) {
	form := func(name, ns string) unstructured.Unstructured {
		return unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": name, "namespace": ns},
			"spec":     map[string]interface{}{"hubId": "mysql"},
		}}
	}
	forms := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{
		form("web", "default"), form("worker", "default"), form("web", "other"),
	}}
	failed := map[string]interface{}{"failedResources": int64(1)}
	other := lynqNode("x-web", "web", failed)
	other.SetNamespace("other")
	nodes := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{
		lynqNode("a-web", "web", failed),
		lynqNode("b-web", "web", failed),
		lynqNode("a-worker", "worker", map[string]interface{}{}),
		other,
	}}

	topo := buildTopology(&unstructured.UnstructuredList{}, forms, nodes, nil)

	for id, want := range map[string]int64{
		"form-default-web":    2,
		"form-default-worker": 0,
		"form-other-web":      1,
	} {
		if got := findTopologyNode(t, topo, id).Metrics.Failed; got != want {
			t.Errorf("%s failed nodes = %d, want %d", id, got, want)
		}
	}
}

func TestBuildTopology_HubCountedFromItsNodes(t *testing.T) {
	// An operator from before the three-state split counts the recovering node in status.failed
	hubs := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "mysql", "namespace": "default"},
		"status": map[string]interface{}{
			"desired": int64(3), "ready": int64(1), "failed": int64(2),
			"conditions": []interface{}{cond("Ready", "True", "DatabaseConnected")},
		},
	}}}}
	recovering := map[string]interface{}{
		"conditions": []interface{}{
			cond("Ready", "False", "NotAllResourcesReady"),
			cond("Degraded", "False", "Healthy"),
		},
	}
	// Same hub name in another namespace, and a node of another hub: neither is mysql's
	elsewhere := lynqNode("x-web", "web", map[string]interface{}{"failedResources": int64(1)})
	elsewhere.SetNamespace("other")
	otherHub := lynqNode("y-web", "web", map[string]interface{}{"failedResources": int64(1)})
	otherHub.SetLabels(map[string]string{"lynq.sh/hub": "postgres"})
	nodes := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{
		lynqNode("a-web", "web", map[string]interface{}{
			"conditions": []interface{}{cond("Ready", "True", "Reconciled")},
		}),
		lynqNode("b-web", "web", recovering),
		lynqNode("c-web", "web", recovering),
		elsewhere,
		otherHub,
	}}

	hub := findTopologyNode(t, buildTopology(hubs, &unstructured.UnstructuredList{}, nodes, nil), "hub-default-mysql")
	if hub.Status != statusPending {
		t.Errorf("hub status = %q, want %q (its nodes are recovering, not failed)", hub.Status, statusPending)
	}
	if hub.Metrics != (TopologyMetrics{Desired: 3, Ready: 1, Failed: 0}) {
		t.Errorf("hub metrics = %+v, want desired 3, ready 1, failed 0", hub.Metrics)
	}
}

func TestBuildTopology_RolloutRecovering(t *testing.T) {
	// An operator that writes the completion marker but counts a recovering node as updating
	// reports InProgress when a node that completed the rollout loses a pod
	forms := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web", "namespace": "default", "generation": int64(3)},
		"spec":     map[string]interface{}{"hubId": "mysql"},
		"status": map[string]interface{}{
			"rollout": map[string]interface{}{
				"phase":   "InProgress",
				"message": "Updating: 2/2 nodes updated (1 updating, 1 ready)",
			},
		},
	}}}}
	updated := func(name string, ready bool) unstructured.Unstructured {
		status := map[string]interface{}{"conditions": []interface{}{cond("Ready", "True", "Reconciled")}}
		if !ready {
			status = map[string]interface{}{"conditions": []interface{}{
				cond("Ready", "False", "NotAllResourcesReady"),
				cond("Degraded", "False", "Healthy"),
			}}
		}
		node := lynqNode(name, "web", status)
		node.SetAnnotations(map[string]string{
			"lynq.sh/template-generation":          "3",
			"lynq.sh/rollout-completed-generation": "3",
		})
		return node
	}
	nodes := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{
		updated("a-web", true), updated("b-web", false),
	}}

	form := findTopologyNode(t, buildTopology(&unstructured.UnstructuredList{}, forms, nodes, nil), "form-default-web")
	if form.Metadata["rolloutPhase"] != "Complete" || form.Metadata["rolloutReportedPhase"] != "InProgress" ||
		form.Metadata["rolloutRecovering"] != int64(1) {
		t.Errorf("form metadata = %v, want phase Complete, reported InProgress, 1 recovering", form.Metadata)
	}
}
