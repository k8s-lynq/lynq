package api

import (
	"strconv"

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

// groupNodes lists every LynqNode under key(node). Nodes with an empty key are skipped.
func groupNodes(nodes *unstructured.UnstructuredList, key func(unstructured.Unstructured) string) map[string][]unstructured.Unstructured {
	groups := make(map[string][]unstructured.Unstructured)
	if nodes == nil {
		return groups
	}
	for _, node := range nodes.Items {
		if k := key(node); k != "" {
			groups[k] = append(groups[k], node)
		}
	}
	return groups
}

// rolloutView is a LynqForm rollout as the dashboard shows it, counted from the form's
// LynqNodes. It mirrors the UI's rolloutView (ui/src/lib/health.ts); health_test.go and
// health.test.ts both run the "rollout" cases in dashboard/testdata/health-cases.json.
//
// An updated node (template generation == the form's generation) is ReadyUpdated when it is
// Ready for its current spec, Recovering when it is not but completed this rollout (completion
// marker equal to its template generation, or no marker) and is not failed, and Updating
// otherwise. A failed node stays Updating: it keeps its maxSkew slot and the operator reports
// the rollout Failed over it either way.
//
// A node without the marker counts as completed only when another updated node of the form
// carries it. When none does, the operator predates the marker (Lynq v1.1.23-alpha.1 and
// earlier) and a node without it cannot be told apart from one still rolling out, so every
// updated node that is not Ready is Updating and nothing is Recovering.
//
// Completed counts the updated nodes that completed this rollout, the operator's rollout progress
// (lynqform_rollout_progress = Completed / Total): ReadyUpdated plus every other updated node that
// is not rolloutPending, failed ones included, since the operator counts those too. Without the
// marker no node that is not ReadyUpdated is completed, so Completed is ReadyUpdated: an older
// operator's own progress.
//
// Phase is the operator's status.rollout.phase (ReportedPhase), except that an operator that
// writes the marker but counts a recovering node as updating reports InProgress for a lost pod.
// When every node is updated and the only nodes not Ready are recovering, Phase is Complete.
// Failed is never overridden.
type rolloutView struct {
	Phase, ReportedPhase                               string
	Total, Updated, ReadyUpdated, Updating, Recovering int64
	Completed                                          int64
}

const (
	annotationTemplateGeneration         = "lynq.sh/template-generation"
	annotationRolloutCompletedGeneration = "lynq.sh/rollout-completed-generation"
)

// rolloutPending mirrors the operator's updateCompleted, negated: a node is still in its
// rollout while its completion marker is present and does not name its template generation.
// A node without the marker is pending only when markerSupported is false (see rolloutView).
func rolloutPending(node unstructured.Unstructured, markerSupported bool) bool {
	annotations := node.GetAnnotations()
	completed, ok := annotations[annotationRolloutCompletedGeneration]
	if !ok {
		return !markerSupported
	}
	return completed == "" || completed != annotations[annotationTemplateGeneration]
}

// newRolloutView returns false when the form has no rollout phase.
func newRolloutView(form unstructured.Unstructured, nodes []unstructured.Unstructured) (rolloutView, bool) {
	reported, _, _ := unstructured.NestedString(form.Object, "status", "rollout", "phase")
	if reported == "" {
		return rolloutView{}, false
	}
	view := rolloutView{Phase: reported, ReportedPhase: reported, Total: int64(len(nodes))}

	generation := form.GetGeneration()
	if generation == 0 {
		generation, _ = nestedInt64(form.Object, "status", "rollout", "targetGeneration")
	}
	target := strconv.FormatInt(generation, 10)
	var updated []unstructured.Unstructured
	markerSupported := false
	for _, node := range nodes {
		annotations := node.GetAnnotations()
		if annotations[annotationTemplateGeneration] != target {
			continue
		}
		updated = append(updated, node)
		if _, ok := annotations[annotationRolloutCompletedGeneration]; ok {
			markerSupported = true
		}
	}
	for _, node := range updated {
		view.Updated++
		health := classifyNode(node)
		observed, _ := nestedInt64(node.Object, "status", "observedGeneration")
		pending := rolloutPending(node, markerSupported)
		if health == statusReady && observed == node.GetGeneration() {
			view.ReadyUpdated++
			view.Completed++
			continue
		}
		if !pending {
			view.Completed++
		}
		if health != statusFailed && !pending {
			view.Recovering++
		} else {
			view.Updating++
		}
	}
	if reported == "InProgress" && view.Total > 0 && view.Updated == view.Total &&
		view.Updating == 0 && view.Recovering > 0 {
		view.Phase = "Complete"
	}
	return view, true
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
