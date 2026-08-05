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

package graph

import (
	"fmt"
	"sort"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
)

// Node represents a resource in the dependency graph
type Node struct {
	Resource   lynqv1.TResource
	ID         string
	DependsOn  []string
	Level      int // Depth in the graph (0 = no dependencies)
	IsResolved bool
}

// DependencyGraph represents a directed acyclic graph of resource dependencies
type DependencyGraph struct {
	Nodes map[string]*Node
}

// NewDependencyGraph creates a new dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		Nodes: make(map[string]*Node),
	}
}

// AddResource adds a resource to the graph
func (g *DependencyGraph) AddResource(resource lynqv1.TResource) error {
	if resource.ID == "" {
		return fmt.Errorf("resource ID cannot be empty")
	}

	if _, exists := g.Nodes[resource.ID]; exists {
		return fmt.Errorf("duplicate resource ID: %s", resource.ID)
	}

	node := &Node{
		Resource:  resource,
		ID:        resource.ID,
		DependsOn: resource.DependIds,
		Level:     0,
	}

	g.Nodes[resource.ID] = node
	return nil
}

// Validate validates the dependency graph
func (g *DependencyGraph) Validate() error {
	// Check that all dependencies exist
	for id, node := range g.Nodes {
		for _, depID := range node.DependsOn {
			if _, exists := g.Nodes[depID]; !exists {
				return fmt.Errorf("resource %s depends on non-existent resource: %s", id, depID)
			}
		}
	}

	// Check for cycles
	if err := g.detectCycles(); err != nil {
		return err
	}

	return nil
}

// detectCycles detects circular dependencies using DFS
func (g *DependencyGraph) detectCycles() error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for id := range g.Nodes {
		if err := g.dfsCheckCycle(id, visited, recStack); err != nil {
			return err
		}
	}

	return nil
}

// dfsCheckCycle performs depth-first search to detect cycles
func (g *DependencyGraph) dfsCheckCycle(id string, visited, recStack map[string]bool) error {
	visited[id] = true
	recStack[id] = true

	node := g.Nodes[id]
	for _, depID := range node.DependsOn {
		if !visited[depID] {
			if err := g.dfsCheckCycle(depID, visited, recStack); err != nil {
				return err
			}
		} else if recStack[depID] {
			return fmt.Errorf("circular dependency detected: %s -> %s", id, depID)
		}
	}

	recStack[id] = false
	return nil
}

// TopologicalSort returns resources in dependency order
func (g *DependencyGraph) TopologicalSort() ([]*Node, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}

	// Calculate levels (depth in the graph)
	g.calculateLevels()

	// Collect and sort nodes
	var result []*Node
	processed := make(map[string]bool)

	// Iterate IDs in a stable order. g.Nodes is a map and Go randomizes map iteration,
	// so without this the order of resources *within* a level varies between calls. That
	// leaks out through LynqNode.Status.SkippedResourceIds, whose reordering is a real
	// etcd write that re-triggers reconciliation (see the appliedResources sort in
	// reconcileSpec for the same failure mode). Dependency ordering *between* levels is
	// unaffected — only the arbitrary intra-level order becomes deterministic.
	ids := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Process nodes level by level
	maxLevel := g.getMaxLevel()
	for level := 0; level <= maxLevel; level++ {
		for _, id := range ids {
			node := g.Nodes[id]
			if node.Level == level && !processed[node.ID] {
				result = append(result, node)
				processed[node.ID] = true
			}
		}
	}

	return result, nil
}

// calculateLevels assigns levels to nodes based on their dependencies
func (g *DependencyGraph) calculateLevels() {
	// Initialize all levels to 0
	for _, node := range g.Nodes {
		node.Level = 0
		node.IsResolved = false
	}

	// Keep iterating until all nodes are resolved
	maxIterations := len(g.Nodes) + 1
	for iteration := 0; iteration < maxIterations; iteration++ {
		allResolved := true

		for _, node := range g.Nodes {
			if node.IsResolved {
				continue
			}

			// Check if all dependencies are resolved
			maxDepLevel := 0
			allDepsResolved := true

			for _, depID := range node.DependsOn {
				depNode := g.Nodes[depID]
				if !depNode.IsResolved {
					allDepsResolved = false
					break
				}
				if depNode.Level >= maxDepLevel {
					maxDepLevel = depNode.Level + 1
				}
			}

			if allDepsResolved {
				node.Level = maxDepLevel
				node.IsResolved = true
			} else {
				allResolved = false
			}
		}

		if allResolved {
			break
		}
	}
}

// getMaxLevel returns the maximum level in the graph
func (g *DependencyGraph) getMaxLevel() int {
	maxLevel := 0
	for _, node := range g.Nodes {
		if node.Level > maxLevel {
			maxLevel = node.Level
		}
	}
	return maxLevel
}

// GetResourcesByLevel returns resources grouped by level
func (g *DependencyGraph) GetResourcesByLevel() map[int][]*Node {
	result := make(map[int][]*Node)

	for _, node := range g.Nodes {
		result[node.Level] = append(result[node.Level], node)
	}

	return result
}

// BuildGraph builds a dependency graph from a list of resources
func BuildGraph(resources []lynqv1.TResource) (*DependencyGraph, error) {
	graph := NewDependencyGraph()

	for _, resource := range resources {
		if err := graph.AddResource(resource); err != nil {
			return nil, err
		}
	}

	if err := graph.Validate(); err != nil {
		return nil, err
	}

	return graph, nil
}
