package api

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type healthCase struct {
	Name   string                   `json:"name"`
	Status map[string]interface{}   `json:"status"`
	Nodes  []map[string]interface{} `json:"nodes"`
	Want   string                   `json:"want"`
	// Hub cases only
	WantCounts *struct {
		Ready       int64 `json:"ready"`
		Progressing int64 `json:"progressing"`
		Failed      int64 `json:"failed"`
		NotCreated  int64 `json:"notCreated"`
	} `json:"wantCounts"`
}

// ownNodes builds the case's LynqNodes as nodes of the form "web" and the hub "mysql" in
// "default", and tallies them the way buildTopology does.
func (tc healthCase) ownNodes(key func(unstructured.Unstructured) string, k string) nodeCounts {
	list := &unstructured.UnstructuredList{}
	for i, status := range tc.Nodes {
		list.Items = append(list.Items, lynqNode(fmt.Sprintf("n%d-web", i), "web", status))
	}
	return countNodes(list, key)[k]
}

// loadHealthCases reads the case table the UI's health.test.ts also runs, so the Go and
// TypeScript classifiers cannot drift apart unnoticed.
func loadHealthCases(t *testing.T) map[string][]healthCase {
	t.Helper()
	raw, err := os.ReadFile("../../../testdata/health-cases.json")
	if err != nil {
		t.Fatalf("read shared health cases: %v", err)
	}
	var cases map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("parse shared health cases: %v", err)
	}
	out := make(map[string][]healthCase)
	for _, kind := range []string{"node", "form", "hub"} {
		var list []healthCase
		if err := json.Unmarshal(cases[kind], &list); err != nil {
			t.Fatalf("parse %s cases: %v", kind, err)
		}
		if len(list) == 0 {
			t.Fatalf("no %s cases", kind)
		}
		out[kind] = list
	}
	return out
}

func TestClassify_SharedCases(t *testing.T) {
	cases := loadHealthCases(t)
	object := func(tc healthCase) unstructured.Unstructured {
		return unstructured.Unstructured{Object: map[string]interface{}{"status": tc.Status}}
	}
	for _, tc := range cases["node"] {
		t.Run("node/"+tc.Name, func(t *testing.T) {
			if got := classifyNode(object(tc)); got != tc.Want {
				t.Errorf("got %q, want %q", got, tc.Want)
			}
		})
	}
	for _, tc := range cases["form"] {
		t.Run("form/"+tc.Name, func(t *testing.T) {
			if got := classifyForm(object(tc), tc.ownNodes(formKey, "default/web")); got != tc.Want {
				t.Errorf("got %q, want %q", got, tc.Want)
			}
		})
	}
	for _, tc := range cases["hub"] {
		t.Run("hub/"+tc.Name, func(t *testing.T) {
			health := newHubHealth(object(tc), tc.ownNodes(hubKey, "default/mysql"))
			if got := classifyHub(object(tc), health); got != tc.Want {
				t.Errorf("got %q, want %q", got, tc.Want)
			}
			if tc.WantCounts == nil {
				t.Fatal("hub case has no wantCounts")
			}
			w := tc.WantCounts
			if health.Ready != w.Ready || health.Progressing != w.Progressing ||
				health.Failed != w.Failed || health.NotCreated != w.NotCreated {
				t.Errorf("counts = %+v, want %+v", health, *w)
			}
		})
	}
}

type rolloutCase struct {
	Name       string                 `json:"name"`
	Generation int64                  `json:"generation"`
	Status     map[string]interface{} `json:"status"`
	Nodes      []struct {
		Annotations map[string]string      `json:"annotations"`
		Generation  int64                  `json:"generation"`
		Status      map[string]interface{} `json:"status"`
	} `json:"nodes"`
	Want struct {
		Phase        string `json:"phase"`
		Total        int64  `json:"total"`
		Updated      int64  `json:"updated"`
		ReadyUpdated int64  `json:"readyUpdated"`
		Updating     int64  `json:"updating"`
		Recovering   int64  `json:"recovering"`
		Completed    int64  `json:"completed"`
	} `json:"want"`
}

func TestRolloutView_SharedCases(t *testing.T) {
	raw, err := os.ReadFile("../../../testdata/health-cases.json")
	if err != nil {
		t.Fatalf("read shared health cases: %v", err)
	}
	var file struct {
		Rollout []rolloutCase `json:"rollout"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("parse rollout cases: %v", err)
	}
	if len(file.Rollout) == 0 {
		t.Fatal("no rollout cases")
	}
	for _, tc := range file.Rollout {
		t.Run(tc.Name, func(t *testing.T) {
			form := unstructured.Unstructured{Object: map[string]interface{}{"status": tc.Status}}
			form.SetGeneration(tc.Generation)
			var nodes []unstructured.Unstructured
			for i, n := range tc.Nodes {
				node := lynqNode(fmt.Sprintf("n%d-web", i), "web", n.Status)
				node.SetAnnotations(n.Annotations)
				node.SetGeneration(n.Generation)
				nodes = append(nodes, node)
			}
			got, ok := newRolloutView(form, nodes)
			if !ok {
				t.Fatal("no rollout view")
			}
			w := tc.Want
			reported, _, _ := unstructured.NestedString(tc.Status, "rollout", "phase")
			want := rolloutView{
				Phase: w.Phase, ReportedPhase: reported, Total: w.Total, Updated: w.Updated,
				ReadyUpdated: w.ReadyUpdated, Updating: w.Updating, Recovering: w.Recovering,
				Completed: w.Completed,
			}
			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestRolloutView_NoPhase(t *testing.T) {
	form := unstructured.Unstructured{Object: map[string]interface{}{"status": map[string]interface{}{}}}
	if _, ok := newRolloutView(form, nil); ok {
		t.Error("a form without a rollout phase has a rollout view")
	}
}

func TestClassifyNode_Int64Counts(t *testing.T) {
	// The Kubernetes decoder yields int64, not the float64 of a plain JSON decode
	obj := unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"failedResources": int64(2),
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	if got := classifyNode(obj); got != statusFailed {
		t.Errorf("got %q, want %q", got, statusFailed)
	}
}
