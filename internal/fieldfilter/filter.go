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

package fieldfilter

import (
	"fmt"

	"github.com/ohler55/ojg/jp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Filter handles JSONPath-based field filtering for Kubernetes resources
// Uses the ojg/jp library for complete JSONPath standard support
type Filter struct {
	ignoreFields []string
	parsedPaths  []jp.Expr
}

// NewFilter creates a new field filter with JSONPath expressions
// JSONPath expressions follow the standard JSONPath format:
// - Simple paths: $.spec.replicas
// - Nested paths: $.spec.template.spec.containers[0].image
// - Map keys: $.metadata.annotations['app.kubernetes.io/name']
// - Wildcards: $.spec.containers[*].image
// - Filters: $.items[?(@.status == 'active')]
func NewFilter(ignoreFields []string) (*Filter, error) {
	if len(ignoreFields) == 0 {
		return &Filter{
			ignoreFields: []string{},
			parsedPaths:  []jp.Expr{},
		}, nil
	}

	f := &Filter{
		ignoreFields: ignoreFields,
		parsedPaths:  make([]jp.Expr, 0, len(ignoreFields)),
	}

	// Parse and validate all paths using ojg/jp
	for _, pathStr := range ignoreFields {
		path, err := jp.ParseString(pathStr)
		if err != nil {
			return nil, fmt.Errorf("invalid JSONPath %q: %w", pathStr, err)
		}
		f.parsedPaths = append(f.parsedPaths, path)
	}

	return f, nil
}

// RemoveIgnoredFields removes fields matching ignoreFields from obj
// This modifies the object in-place using JSONPath deletion
func (f *Filter) RemoveIgnoredFields(obj *unstructured.Unstructured) error {
	if obj == nil {
		return fmt.Errorf("object cannot be nil")
	}

	// No-op if no ignore fields
	if len(f.parsedPaths) == 0 {
		return nil
	}

	// Get the underlying map from unstructured object
	data := obj.Object

	// Remove each ignored field using JSONPath
	for _, path := range f.parsedPaths {
		// Use Remove() which handles non-existent paths gracefully
		// Returns modified data and error
		_, err := path.Remove(data)
		if err != nil {
			// Silently continue - some paths may not exist and that's OK
			// In production, this could be logged at debug level
			continue
		}
	}

	return nil
}

// ValidateJSONPath validates a single JSONPath expression
// This is a convenience function for validation without creating a Filter
func ValidateJSONPath(pathStr string) error {
	_, err := jp.ParseString(pathStr)
	if err != nil {
		return fmt.Errorf("invalid JSONPath %q: %w", pathStr, err)
	}
	return nil
}

// GetMatchingFields returns the values at the specified JSONPath
// This is useful for debugging or inspecting what fields would be removed
func (f *Filter) GetMatchingFields(obj *unstructured.Unstructured) (map[string][]interface{}, error) {
	if obj == nil {
		return nil, fmt.Errorf("object cannot be nil")
	}

	results := make(map[string][]interface{})
	data := obj.Object

	for i, path := range f.parsedPaths {
		matches := path.Get(data)
		if len(matches) > 0 {
			results[f.ignoreFields[i]] = matches
		}
	}

	return results, nil
}

// PreserveIgnoredFields copies values from existing object to desired object for ignored fields
// This ensures that SSA doesn't remove fields that are being preserved.
// For each ignoreFields path:
// - If the path exists in 'existing', copy its value to 'desired'
// - If the path doesn't exist in 'existing', remove it from 'desired'
// This modifies the desired object in-place.
func (f *Filter) PreserveIgnoredFields(desired, existing *unstructured.Unstructured) error {
	if desired == nil {
		return fmt.Errorf("desired object cannot be nil")
	}

	// No-op if no ignore fields
	if len(f.parsedPaths) == 0 {
		return nil
	}

	desiredData := desired.Object

	// If existing is nil, just remove the ignored fields from desired
	// (this handles initial creation case)
	if existing == nil {
		for _, path := range f.parsedPaths {
			_, _ = path.Remove(desiredData)
		}
		return nil
	}

	existingData := existing.Object

	// For each ignored field, copy value from existing to desired
	for _, path := range f.parsedPaths {
		// Get value from existing resource
		existingValues := path.Get(existingData)

		if len(existingValues) > 0 {
			// Value exists in existing resource - copy it to desired
			// Use Set() to overwrite the value in desired
			err := path.Set(desiredData, existingValues[0])
			if err != nil {
				// If Set fails, try to at least preserve by not changing
				// This can happen with complex paths
				continue
			}
		} else {
			// Value doesn't exist in existing resource - remove from desired
			// This ensures we don't accidentally set a new value for an ignored field
			_, _ = path.Remove(desiredData)
		}
	}

	return nil
}
