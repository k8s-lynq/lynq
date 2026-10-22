package api

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Topology status values. "pending" means progressing: rolling out, recovering, or not
// reconciled yet. The UI keeps the key and labels it "Progressing".
const (
	statusReady   = "ready"
	statusPending = "pending"
	statusFailed  = "failed"
	statusSkipped = "skipped"
)

// classifyNode mirrors the operator's classifyNode (internal/controller/node_health.go) and the
// UI's classifyNode (ui/src/lib/health.ts). Keep the three in step; health_test.go and
// health.test.ts both run the cases in dashboard/testdata/health-cases.json.
//
// Failed resources, a conflict or Degraded=True is failed, and failure wins over a stale
// Ready=True. Ready=True is ready. Everything else, including a node recovering a lost pod
// (Ready=False, Degraded=False) or one with no conditions yet, is progressing.
func classifyNode(obj unstructured.Unstructured) string {
	failed, _ := nestedInt64(obj.Object, "status", "failedResources")
	if failed > 0 ||
		conditionStatus(obj, "Conflicted") == "True" ||
		conditionStatus(obj, "Degraded") == "True" {
		return statusFailed
	}
	if conditionStatus(obj, "Ready") == "True" {
		return statusReady
	}
	return statusPending
}

// nodeCounts is the classifyNode tally of one form's or one hub's LynqNodes.
type nodeCounts struct {
	Ready, Pending, Failed int64
}

func (c *nodeCounts) add(status string) {
	switch status {
	case statusReady:
		c.Ready++
	case statusFailed:
		c.Failed++
	default:
		c.Pending++
	}
}

func (c nodeCounts) total() int64 { return c.Ready + c.Pending + c.Failed }

// countNodes classifies every LynqNode once and tallies it under key(node). Nodes with an
// empty key are skipped.
func countNodes(nodes *unstructured.UnstructuredList, key func(unstructured.Unstructured) string) map[string]nodeCounts {
	counts := make(map[string]nodeCounts)
	if nodes == nil {
		return counts
	}
	for _, node := range nodes.Items {
		k := key(node)
		if k == "" {
			continue
		}
		c := counts[k]
		c.add(classifyNode(node))
		counts[k] = c
	}
	return counts
}

// formKey groups a LynqNode under its form: "namespace/templateRef".
func formKey(node unstructured.Unstructured) string {
	templateRef, _, _ := unstructured.NestedString(node.Object, "spec", "templateRef")
	if templateRef == "" {
		return ""
	}
	return node.GetNamespace() + "/" + templateRef
}

// hubKey groups a LynqNode under its hub: "namespace/<lynq.sh/hub label>".
func hubKey(node unstructured.Unstructured) string {
	hub := node.GetLabels()["lynq.sh/hub"]
	if hub == "" {
		return ""
	}
	return node.GetNamespace() + "/" + hub
}

// classifyForm: an invalid template, a stalled rollout (rollout.phase=Failed) or any failed
// LynqNode of the form is failed — the same rule as the hub, so one failed node turns both red.
// Applied=True (every node Ready) is ready. Anything else, NoNodes included, is progressing.
func classifyForm(obj unstructured.Unstructured, nodes nodeCounts) string {
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "rollout", "phase")
	if conditionStatus(obj, "Valid") == "False" || phase == "Failed" || nodes.Failed > 0 {
		return statusFailed
	}
	if conditionStatus(obj, "Applied") == "True" {
		return statusReady
	}
	return statusPending
}

// hubHealth is a hub's node breakdown, counted from its LynqNodes rather than read from
// status.ready/status.failed: Lynq v1.1.23-alpha.1 and earlier count every Ready!=True node in
// status.failed, so a node recovering a pod would fail the hub there while the Nodes page shows
// it progressing. Desired still comes from status.desired; desired nodes that do not exist yet
// (not created, or creation held back by maxSkew) are NotCreated and count as progressing.
type hubHealth struct {
	Desired, Ready, Progressing, Failed, NotCreated int64
}

func newHubHealth(obj unstructured.Unstructured, nodes nodeCounts) hubHealth {
	desired, _ := nestedInt64(obj.Object, "status", "desired")
	notCreated := desired - nodes.total()
	if notCreated < 0 {
		notCreated = 0
	}
	return hubHealth{
		Desired:     desired,
		Ready:       nodes.Ready,
		Progressing: nodes.Pending + notCreated,
		Failed:      nodes.Failed,
		NotCreated:  notCreated,
	}
}

// classifyHub: the hub Ready condition reports the datasource connection. A lost datasource or
// any failed LynqNode is failed. A connected hub with nothing progressing is ready. Otherwise
// (rolling out, recovering, not created yet, or not reconciled yet) it is progressing.
func classifyHub(obj unstructured.Unstructured, health hubHealth) string {
	connected := conditionStatus(obj, "Ready")
	if connected == "False" || health.Failed > 0 {
		return statusFailed
	}
	if connected == "True" && health.Progressing == 0 {
		return statusReady
	}
	return statusPending
}

// conditionStatus returns the status of the condition of the given type, or "" if absent.
func conditionStatus(obj unstructured.Unstructured, condType string) string {
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		return ""
	}
	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _, _ := unstructured.NestedString(condition, "type"); t == condType {
			s, _, _ := unstructured.NestedString(condition, "status")
			return s
		}
	}
	return ""
}

// nestedInt64 reads an integer field, accepting the float64 that a plain JSON decode produces
// as well as the int64 the Kubernetes decoder does.
func nestedInt64(obj map[string]interface{}, fields ...string) (int64, bool) {
	v, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil {
		return 0, false
	}
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}
