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
