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
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// isSemanticallyUnchanged compares a dry-run SSA result with the live object to
// determine if the apply would be a no-op. This follows the Flux SSA manager pattern
// (fluxcd/pkg/ssa/manager_diff.go) of stripping volatile metadata and status before
// comparison, and using apiequality.Semantic.DeepEqual which treats nil and empty
// maps/slices as equivalent.
//
// References:
//   - Flux normalize: https://github.com/fluxcd/pkg/blob/main/ssa/normalize/normalize.go
//   - K8s nil vs empty: https://github.com/kubernetes/kubernetes/pull/125317
func isSemanticallyUnchanged(live, dryRun *unstructured.Unstructured) bool {
	liveN := prepareForDiff(live)
	dryN := prepareForDiff(dryRun)

	equal := apiequality.Semantic.DeepEqual(liveN.Object, dryN.Object)
	if !equal {
		// Debug: find which top-level keys differ
		for k := range liveN.Object {
			if !apiequality.Semantic.DeepEqual(liveN.Object[k], dryN.Object[k]) {
				fmt.Printf("[DEBUG] SSA diff at key %q: live_type=%T dryrun_type=%T\n", k, liveN.Object[k], dryN.Object[k])
			}
		}
		for k := range dryN.Object {
			if _, ok := liveN.Object[k]; !ok {
				fmt.Printf("[DEBUG] SSA diff: key %q only in dryrun\n", k)
			}
		}
	}
	return equal
}

// prepareForDiff strips volatile metadata and status from an object for semantic
// comparison. These fields change independently of the desired state and would
// cause false diffs if included.
func prepareForDiff(obj *unstructured.Unstructured) *unstructured.Unstructured {
	c := obj.DeepCopy()

	// Remove status — exists on Deployments, StatefulSets, etc. and changes
	// independently of spec. Including it would make every comparison fail.
	delete(c.Object, "status")

	// Remove volatile metadata fields that the API server manages
	if metadata, ok := c.Object["metadata"].(map[string]interface{}); ok {
		delete(metadata, "managedFields")
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "creationTimestamp")
		delete(metadata, "generation")
		delete(metadata, "deletionTimestamp")
		delete(metadata, "deletionGracePeriodSeconds")
		delete(metadata, "selfLink")
	}

	return c
}
