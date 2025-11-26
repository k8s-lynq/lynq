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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
)

func TestDependencyGraph(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dependency Graph Behavior Suite")
}

var _ = Describe("Dependency Graph - Core Behaviors", func() {
	Context("Building Resource Dependency Graph", func() {
		Describe("Adding resources without dependencies", func() {
			It("Should build graph with independent resources", func() {
				By("Given an empty dependency graph")
				graph := NewDependencyGraph()

				By("When adding multiple independent resources")
				resources := []lynqv1.TResource{
					{ID: "configmap", NameTemplate: "app-config"},
					{ID: "secret", NameTemplate: "app-secret"},
					{ID: "pvc", NameTemplate: "app-data"},
				}

				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Then all resources should be in the graph")
				Expect(graph.Nodes).To(HaveLen(3))
				Expect(graph.Nodes).To(HaveKey("configmap"))
				Expect(graph.Nodes).To(HaveKey("secret"))
				Expect(graph.Nodes).To(HaveKey("pvc"))

				By("And resources should have no dependencies")
				for id, node := range graph.Nodes {
					Expect(node.DependsOn).To(BeEmpty(), "Resource %s should have no dependencies", id)
				}
			})
		})

		Describe("Adding resources with linear dependencies", func() {
			It("Should track dependency chain correctly", func() {
				By("Given resources with dependencies: C -> B -> A")
				graph := NewDependencyGraph()

				resources := []lynqv1.TResource{
					{ID: "namespace", NameTemplate: "app-namespace"},
					{ID: "configmap", NameTemplate: "app-config", DependIds: []string{"namespace"}},
					{ID: "deployment", NameTemplate: "app", DependIds: []string{"configmap"}},
				}

				By("When adding resources in order")
				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Then dependencies should be correctly tracked")
				Expect(graph.Nodes["namespace"].DependsOn).To(BeEmpty())
				Expect(graph.Nodes["configmap"].DependsOn).To(Equal([]string{"namespace"}))
				Expect(graph.Nodes["deployment"].DependsOn).To(Equal([]string{"configmap"}))
			})
		})

		Describe("Adding resources with multiple dependencies", func() {
			It("Should handle resource depending on multiple others", func() {
				By("Given a deployment depending on ConfigMap, Secret, and PVC")
				graph := NewDependencyGraph()

				resources := []lynqv1.TResource{
					{ID: "configmap", NameTemplate: "app-config"},
					{ID: "secret", NameTemplate: "app-secret"},
					{ID: "pvc", NameTemplate: "app-data"},
					{
						ID:           "deployment",
						NameTemplate: "app",
						DependIds:    []string{"configmap", "secret", "pvc"},
					},
				}

				By("When building the graph")
				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Then deployment should depend on all three resources")
				deployNode := graph.Nodes["deployment"]
				Expect(deployNode.DependsOn).To(HaveLen(3))
				Expect(deployNode.DependsOn).To(ContainElements("configmap", "secret", "pvc"))
			})
		})

		Describe("Adding resources with diamond dependencies", func() {
			It("Should handle diamond dependency pattern", func() {
				By("Given a diamond pattern: Backend depends on Service and Headless Service, both depend on StatefulSet")
				graph := NewDependencyGraph()

				By("When building the graph")
				//       StatefulSet (Database)
				//           /            \
				//    DB Service    Headless Service
				//           \            /
				//         Backend Deployment
				resources := []lynqv1.TResource{
					{ID: "db-statefulset", NameTemplate: "db"},
					{ID: "db-service", NameTemplate: "db-svc", DependIds: []string{"db-statefulset"}},
					{ID: "db-headless", NameTemplate: "db-headless", DependIds: []string{"db-statefulset"}},
					{
						ID:           "backend-deployment",
						NameTemplate: "backend",
						DependIds:    []string{"db-service", "db-headless"},
					},
				}

				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Then the diamond structure should be correctly represented")
				Expect(graph.Nodes).To(HaveLen(4))
				Expect(graph.Nodes["db-service"].DependsOn).To(Equal([]string{"db-statefulset"}))
				Expect(graph.Nodes["db-headless"].DependsOn).To(Equal([]string{"db-statefulset"}))
				Expect(graph.Nodes["backend-deployment"].DependsOn).To(ContainElements("db-service", "db-headless"))
			})
		})
	})

	Context("Graph Validation", func() {
		Describe("Detecting missing dependencies", func() {
			It("Should reject resource depending on non-existent resource", func() {
				By("Given a resource depending on non-existent resource")
				graph := NewDependencyGraph()

				resources := []lynqv1.TResource{
					{ID: "deployment", DependIds: []string{"missing-configmap"}},
				}

				By("When adding the resource")
				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Then validation should fail")
				err := graph.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("non-existent resource"))
			})
		})

		Describe("Detecting duplicate resource IDs", func() {
			It("Should reject duplicate resource identifiers", func() {
				By("Given a graph with one resource")
				graph := NewDependencyGraph()
				err := graph.AddResource(lynqv1.TResource{ID: "app"})
				Expect(err).ToNot(HaveOccurred())

				By("When adding another resource with same ID")
				err = graph.AddResource(lynqv1.TResource{ID: "app"})

				By("Then it should be rejected")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("duplicate"))
			})
		})

		Describe("Detecting empty resource IDs", func() {
			It("Should reject resources without ID", func() {
				By("Given a resource with empty ID")
				graph := NewDependencyGraph()

				By("When adding the resource")
				err := graph.AddResource(lynqv1.TResource{ID: "", NameTemplate: "test"})

				By("Then it should be rejected")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot be empty"))
			})
		})
	})

	Context("Cycle Detection", func() {
		Describe("Detecting direct circular dependency", func() {
			It("Should detect two resources depending on each other", func() {
				By("Given two resources with circular dependency")
				graph := NewDependencyGraph()

				By("When building a circular graph: A -> B -> A")
				err := graph.AddResource(lynqv1.TResource{
					ID:        "resource-a",
					DependIds: []string{"resource-b"},
				})
				Expect(err).ToNot(HaveOccurred())

				err = graph.AddResource(lynqv1.TResource{
					ID:        "resource-b",
					DependIds: []string{"resource-a"},
				})
				Expect(err).ToNot(HaveOccurred())

				By("Then validation should detect the cycle")
				err = graph.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("circular dependency"))
			})
		})

		Describe("Detecting indirect circular dependency", func() {
			It("Should detect cycle in longer dependency chain", func() {
				By("Given a longer dependency chain with cycle")
				graph := NewDependencyGraph()

				By("When building: A -> B -> C -> D -> B (cycle)")
				resources := []lynqv1.TResource{
					{ID: "b", DependIds: []string{"d"}},
					{ID: "c", DependIds: []string{"b"}},
					{ID: "d", DependIds: []string{"c"}},
					{ID: "a", DependIds: []string{"b"}},
				}

				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Then validation should detect the cycle")
				err := graph.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("circular dependency"))
			})
		})

		Describe("Confirming acyclic graph", func() {
			It("Should pass validation for valid DAG", func() {
				By("Given a valid directed acyclic graph")
				graph := NewDependencyGraph()

				By("When building a proper DAG")
				//     A
				//    / \
				//   B   C
				//    \ /
				//     D
				resources := []lynqv1.TResource{
					{ID: "a"},
					{ID: "b", DependIds: []string{"a"}},
					{ID: "c", DependIds: []string{"a"}},
					{ID: "d", DependIds: []string{"b", "c"}},
				}

				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}

				By("Then validation should succeed")
				err := graph.Validate()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("Topological Sorting for Application Order", func() {
		Describe("Sorting independent resources", func() {
			It("Should return resources in any valid order", func() {
				By("Given independent resources")
				graph := NewDependencyGraph()

				resources := []lynqv1.TResource{
					{ID: "configmap"},
					{ID: "secret"},
					{ID: "pvc"},
				}

				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}
				err := graph.Validate()
				Expect(err).ToNot(HaveOccurred())

				By("When performing topological sort")
				sorted, err := graph.TopologicalSort()

				By("Then all resources should be in result")
				Expect(err).ToNot(HaveOccurred())
				Expect(sorted).To(HaveLen(3))

				// Extract IDs from sorted nodes
				ids := make([]string, len(sorted))
				for i, node := range sorted {
					ids[i] = node.ID
				}
				Expect(ids).To(ConsistOf("configmap", "secret", "pvc"))
			})
		})

		Describe("Sorting linear dependency chain", func() {
			It("Should order resources respecting dependencies", func() {
				By("Given a linear dependency chain")
				graph := NewDependencyGraph()

				By("When dependencies are: namespace -> configmap -> deployment")
				resources := []lynqv1.TResource{
					{ID: "namespace"},
					{ID: "configmap", DependIds: []string{"namespace"}},
					{ID: "deployment", DependIds: []string{"configmap"}},
				}

				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}
				err := graph.Validate()
				Expect(err).ToNot(HaveOccurred())

				By("When sorting the graph")
				sorted, err := graph.TopologicalSort()

				By("Then resources should be in dependency order")
				Expect(err).ToNot(HaveOccurred())

				// Extract IDs
				ids := make([]string, len(sorted))
				for i, node := range sorted {
					ids[i] = node.ID
				}
				Expect(ids).To(Equal([]string{"namespace", "configmap", "deployment"}))
			})
		})

		Describe("Sorting complex application stack", func() {
			It("Should ensure correct deployment order for full stack", func() {
				By("Given a complete application stack")
				graph := NewDependencyGraph()

				By("When defining typical stack dependencies")
				// Namespace -> ConfigMap/Secret/PVC -> Deployment -> Service -> Ingress
				resources := []lynqv1.TResource{
					{ID: "namespace"},
					{ID: "configmap", DependIds: []string{"namespace"}},
					{ID: "secret", DependIds: []string{"namespace"}},
					{ID: "pvc", DependIds: []string{"namespace"}},
					{
						ID:        "deployment",
						DependIds: []string{"configmap", "secret", "pvc"},
					},
					{ID: "service", DependIds: []string{"deployment"}},
					{ID: "ingress", DependIds: []string{"service"}},
				}

				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}
				err := graph.Validate()
				Expect(err).ToNot(HaveOccurred())

				By("When determining application order")
				sorted, err := graph.TopologicalSort()

				By("Then critical order constraints should be satisfied")
				Expect(err).ToNot(HaveOccurred())
				Expect(sorted).To(HaveLen(7))

				// Helper to find index
				indexOf := func(id string) int {
					for i, node := range sorted {
						if node.ID == id {
							return i
						}
					}
					return -1
				}

				// Namespace must come first
				nsIdx := indexOf("namespace")
				Expect(nsIdx).To(Equal(0))

				// ConfigMap, Secret, PVC must come after Namespace
				Expect(indexOf("configmap")).To(BeNumerically(">", nsIdx))
				Expect(indexOf("secret")).To(BeNumerically(">", nsIdx))
				Expect(indexOf("pvc")).To(BeNumerically(">", nsIdx))

				// Deployment must come after all its dependencies
				deployIdx := indexOf("deployment")
				Expect(indexOf("configmap")).To(BeNumerically("<", deployIdx))
				Expect(indexOf("secret")).To(BeNumerically("<", deployIdx))
				Expect(indexOf("pvc")).To(BeNumerically("<", deployIdx))

				// Service must come after Deployment
				svcIdx := indexOf("service")
				Expect(deployIdx).To(BeNumerically("<", svcIdx))

				// Ingress must come last
				ingressIdx := indexOf("ingress")
				Expect(svcIdx).To(BeNumerically("<", ingressIdx))
			})
		})

		Describe("Handling diamond dependencies in sort", func() {
			It("Should respect all dependency constraints", func() {
				By("Given a diamond dependency structure")
				graph := NewDependencyGraph()

				By("When sorting diamond pattern")
				resources := []lynqv1.TResource{
					{ID: "a"},
					{ID: "b", DependIds: []string{"a"}},
					{ID: "c", DependIds: []string{"a"}},
					{ID: "d", DependIds: []string{"b", "c"}},
				}

				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}
				err := graph.Validate()
				Expect(err).ToNot(HaveOccurred())

				sorted, err := graph.TopologicalSort()

				By("Then A should be first and D should be last")
				Expect(err).ToNot(HaveOccurred())
				Expect(sorted[0].ID).To(Equal("a"))
				Expect(sorted[3].ID).To(Equal("d"))

				By("And B,C should be in middle in any order")
				middleIDs := []string{sorted[1].ID, sorted[2].ID}
				Expect(middleIDs).To(ConsistOf("b", "c"))
			})
		})

		Describe("Failing on cyclic graph", func() {
			It("Should refuse to sort graph with cycles", func() {
				By("Given a graph with circular dependencies")
				graph := NewDependencyGraph()

				resources := []lynqv1.TResource{
					{ID: "a", DependIds: []string{"b"}},
					{ID: "b", DependIds: []string{"a"}},
				}

				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}

				By("When attempting to sort")
				_, err := graph.TopologicalSort()

				By("Then sort should fail with cycle error")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("Real-world Application Scenarios", func() {
		Describe("Multi-tier web application", func() {
			It("Should correctly order frontend, backend, and database tiers", func() {
				By("Given a multi-tier application")
				graph := NewDependencyGraph()

				By("When defining tier dependencies")
				resources := []lynqv1.TResource{
					// Database tier
					{ID: "db-pvc"},
					{ID: "db-secret"},
					{ID: "db-statefulset", DependIds: []string{"db-pvc", "db-secret"}},
					{ID: "db-service", DependIds: []string{"db-statefulset"}},

					// Backend tier
					{ID: "backend-configmap"},
					{
						ID:        "backend-deployment",
						DependIds: []string{"backend-configmap", "db-service"},
					},
					{ID: "backend-service", DependIds: []string{"backend-deployment"}},

					// Frontend tier
					{ID: "frontend-deployment", DependIds: []string{"backend-service"}},
					{ID: "frontend-service", DependIds: []string{"frontend-deployment"}},
					{ID: "ingress", DependIds: []string{"frontend-service"}},
				}

				for _, res := range resources {
					err := graph.AddResource(res)
					Expect(err).ToNot(HaveOccurred())
				}
				err := graph.Validate()
				Expect(err).ToNot(HaveOccurred())

				sorted, err := graph.TopologicalSort()

				By("Then tiers should be deployed in correct order")
				Expect(err).ToNot(HaveOccurred())

				// Helper
				indexOf := func(id string) int {
					for i, node := range sorted {
						if node.ID == id {
							return i
						}
					}
					return -1
				}

				// Database tier comes before backend
				Expect(indexOf("db-service")).To(BeNumerically("<", indexOf("backend-deployment")))

				// Backend tier comes before frontend
				Expect(indexOf("backend-service")).To(BeNumerically("<", indexOf("frontend-deployment")))

				// Ingress comes last
				Expect(indexOf("ingress")).To(Equal(len(sorted) - 1))
			})
		})
	})
})
