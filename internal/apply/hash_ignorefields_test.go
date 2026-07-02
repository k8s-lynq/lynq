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

package apply

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/k8s-lynq/lynq/internal/fieldfilter"
)

// TestComputeDesiredHash_IgnoreFields is the M3 regression guard: a change to
// an ignored field (e.g. HPA-owned spec.replicas) must NOT change the desired
// hash, so it triggers no re-apply and no apply-start-time reset. A change to a
// NON-ignored field must still change the hash.
func TestComputeDesiredHash_IgnoreFields(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	applier := NewApplier(client, scheme)

	deploy := func(replicas int64, image string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "app", "namespace": "default"},
			"spec": map[string]interface{}{
				"replicas": replicas,
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{"name": "app", "image": image},
						},
					},
				},
			},
		}}
	}

	filter, err := fieldfilter.NewFilter([]string{"$.spec.replicas"})
	if err != nil {
		t.Fatalf("NewFilter: %v", err)
	}

	base := deploy(3, "nginx:1.25")
	scaled := deploy(10, "nginx:1.25")  // only the ignored field (replicas) differs
	reimaged := deploy(3, "nginx:1.26") // a managed (non-ignored) field differs

	t.Run("ignored-field change leaves hash stable", func(t *testing.T) {
		h1 := applier.computeDesiredHash(base, filter)
		h2 := applier.computeDesiredHash(scaled, filter)
		if h1 == "" || h2 == "" {
			t.Fatal("hash should be non-empty")
		}
		if h1 != h2 {
			t.Errorf("hash changed on ignored-field-only change (%s != %s) — HPA scaling would churn re-applies", h1, h2)
		}
	})

	t.Run("managed-field change still changes hash", func(t *testing.T) {
		h1 := applier.computeDesiredHash(base, filter)
		h3 := applier.computeDesiredHash(reimaged, filter)
		if h1 == h3 {
			t.Errorf("hash unchanged on image change (%s) — real drift would be missed", h1)
		}
	})

	t.Run("computeDesiredHash does not mutate the apply payload", func(t *testing.T) {
		obj := deploy(7, "nginx:1.25")
		_ = applier.computeDesiredHash(obj, filter)
		// The ignored field must still be present in obj (only the hash-input
		// copy had it removed) so SSA still sends the preserved value.
		replicas, found, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
		if !found || replicas != 7 {
			t.Errorf("apply payload was mutated: spec.replicas found=%v val=%d, want 7", found, replicas)
		}
	})

	t.Run("nil filter hashes the whole object (replicas participates)", func(t *testing.T) {
		h1 := applier.computeDesiredHash(base, nil)
		h2 := applier.computeDesiredHash(scaled, nil)
		if h1 == h2 {
			t.Error("with no ignoreFields, a replicas change should change the hash")
		}
	})
}
