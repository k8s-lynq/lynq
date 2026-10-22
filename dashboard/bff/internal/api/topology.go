package api

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TopologyNode represents a node in the topology graph
type TopologyNode struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // hub, form, node, resource
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	Status    string                 `json:"status"` // ready, pending (progressing), failed, skipped
	Children  []string               `json:"children,omitempty"`
	Metrics   TopologyMetrics        `json:"metrics"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // Extra info like creationPolicy
}

// TopologyMetrics contains resource metrics
type TopologyMetrics struct {
	Desired int64 `json:"desired"`
	Ready   int64 `json:"ready"`
	Failed  int64 `json:"failed"`
}

// TopologyEdge represents a connection between nodes
type TopologyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// TopologyData contains the complete topology
type TopologyData struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// AppliedResource represents a Kubernetes resource managed by a LynqNode
type AppliedResource struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	ID        string `json:"id"`
}

// parseAppliedResource parses "Kind/Namespace/Name@ID" format
func parseAppliedResource(s string) *AppliedResource {
	// Format: "Kind/Namespace/Name@ID"
	// Example: "Deployment/haulla-tenant/haulla-api-1497849876@core-api"
	atIdx := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '@' {
			atIdx = i
			break
		}
	}

	var id, rest string
	if atIdx >= 0 {
		rest = s[:atIdx]
		id = s[atIdx+1:]
	} else {
		rest = s
		id = ""
	}

	parts := make([]string, 0, 3)
	start := 0
	for i := 0; i < len(rest); i++ {
		if rest[i] == '/' {
			parts = append(parts, rest[start:i])
			start = i + 1
		}
	}
	parts = append(parts, rest[start:])

	if len(parts) >= 3 {
		return &AppliedResource{
			Kind:      parts[0],
			Namespace: parts[1],
			Name:      parts[2],
			ID:        id,
		}
	}
	return nil
}

// ResourcePolicy contains policy information for a TResource
type ResourcePolicy struct {
	CreationPolicy string
	DeletionPolicy string
	ConflictPolicy string
}

// buildTopology creates topology data from Kubernetes resources
func buildTopology(hubs, forms, nodes, orphanedResources *unstructured.UnstructuredList) *TopologyData {
	topology := &TopologyData{
		Nodes: []TopologyNode{},
		Edges: []TopologyEdge{},
	}

	// Map form hubId to form name for edge creation
	formsByHub := make(map[string][]string)

	// Build resource policy map: formName -> resourceID -> ResourcePolicy
	formResourcePolicies := buildFormResourcePolicies(forms)

	// LynqFormStatus has no failed-node count, and LynqHub status.failed counts every not-Ready
	// node in Lynq v1.1.23-alpha.1 and earlier, so both are counted from the LynqNodes themselves
	nodesByForm := countNodes(nodes, formKey)
	nodesByHub := countNodes(nodes, hubKey)

	// Process hubs
	for _, hub := range hubs.Items {
		name, _, _ := unstructured.NestedString(hub.Object, "metadata", "name")
		namespace, _, _ := unstructured.NestedString(hub.Object, "metadata", "namespace")

		health := newHubHealth(hub, nodesByHub[namespace+"/"+name])
		status := classifyHub(hub, health)
		nodeID := "hub-" + namespace + "-" + name

		topology.Nodes = append(topology.Nodes, TopologyNode{
			ID:        nodeID,
			Type:      "hub",
			Name:      name,
			Namespace: namespace,
			Status:    status,
			Children:  []string{},
			Metrics: TopologyMetrics{
				Desired: health.Desired,
				Ready:   health.Ready,
				Failed:  health.Failed,
			},
		})
	}

	// Process forms
	for _, form := range forms.Items {
		name, _, _ := unstructured.NestedString(form.Object, "metadata", "name")
		namespace, _, _ := unstructured.NestedString(form.Object, "metadata", "namespace")
		hubId, _, _ := unstructured.NestedString(form.Object, "spec", "hubId")

		totalNodes, _, _ := unstructured.NestedInt64(form.Object, "status", "totalNodes")
		readyNodes, _, _ := unstructured.NestedInt64(form.Object, "status", "readyNodes")
		formNodes := nodesByForm[namespace+"/"+name]

		status := classifyForm(form, formNodes)
		nodeID := "form-" + namespace + "-" + name

		var formMetadata map[string]interface{}
		if phase, _, _ := unstructured.NestedString(form.Object, "status", "rollout", "phase"); phase != "" {
			message, _, _ := unstructured.NestedString(form.Object, "status", "rollout", "message")
			formMetadata = map[string]interface{}{
				"rolloutPhase":   phase,
				"rolloutMessage": message,
			}
		}

		topology.Nodes = append(topology.Nodes, TopologyNode{
			ID:        nodeID,
			Type:      "form",
			Name:      name,
			Namespace: namespace,
			Status:    status,
			Children:  []string{},
			Metrics: TopologyMetrics{
				Desired: totalNodes,
				Ready:   readyNodes,
				Failed:  formNodes.Failed,
			},
			Metadata: formMetadata,
		})

		// Track form by hub for edge creation
		if hubId != "" {
			formsByHub[hubId] = append(formsByHub[hubId], nodeID)
			// Create edge from hub to form
			hubNodeID := "hub-" + namespace + "-" + hubId
			topology.Edges = append(topology.Edges, TopologyEdge{
				Source: hubNodeID,
				Target: nodeID,
			})
		}
	}

	// Process nodes
	for _, node := range nodes.Items {
		name, _, _ := unstructured.NestedString(node.Object, "metadata", "name")
		namespace, _, _ := unstructured.NestedString(node.Object, "metadata", "namespace")
		hubRef := node.GetLabels()["lynq.sh/hub"]
		templateRef, _, _ := unstructured.NestedString(node.Object, "spec", "templateRef")

		desired, _, _ := unstructured.NestedInt64(node.Object, "status", "desiredResources")
		ready, _, _ := unstructured.NestedInt64(node.Object, "status", "readyResources")
		failed, _, _ := unstructured.NestedInt64(node.Object, "status", "failedResources")

		// Get skipped resource count and IDs for individual resource status
		// Only trust skippedResourceIds if skippedResources count is actually set and > 0
		// This works around a controller bug where Service/Ingress IDs are incorrectly added to skippedResourceIds
		skippedResourceIds := make(map[string]bool)
		skippedCount, _, _ := unstructured.NestedInt64(node.Object, "status", "skippedResources")
		if skippedCount > 0 {
			skippedIdsRaw, found, _ := unstructured.NestedSlice(node.Object, "status", "skippedResourceIds")
			if found {
				for _, idRaw := range skippedIdsRaw {
					if id, ok := idRaw.(string); ok {
						skippedResourceIds[id] = true
					}
				}
			}
		}

		status := classifyNode(node)
		nodeID := "node-" + namespace + "-" + name

		topology.Nodes = append(topology.Nodes, TopologyNode{
			ID:        nodeID,
			Type:      "node",
			Name:      name,
			Namespace: namespace,
			Status:    status,
			Metrics: TopologyMetrics{
				Desired: desired,
				Ready:   ready,
				Failed:  failed,
			},
			// The drawer shows past ResourceConflict events only while the conflict is live
			Metadata: map[string]interface{}{
				"conflicted": conditionStatus(node, "Conflicted") == "True",
			},
		})

		// Create edge from form to node
		if templateRef != "" {
			formNodeID := "form-" + namespace + "-" + templateRef
			topology.Edges = append(topology.Edges, TopologyEdge{
				Source: formNodeID,
				Target: nodeID,
			})
		}

		// Alternative: create edge from hub to node if no templateRef
		if templateRef == "" && hubRef != "" {
			hubNodeID := "hub-" + namespace + "-" + hubRef
			topology.Edges = append(topology.Edges, TopologyEdge{
				Source: hubNodeID,
				Target: nodeID,
			})
		}

		// Process applied resources for this node
		// Use NestedSlice instead of NestedStringSlice because JSON unmarshals as []interface{}
		appliedResourcesRaw, found, _ := unstructured.NestedSlice(node.Object, "status", "appliedResources")
		if found {
			// Get resource policies from the form
			var resourcePolicies map[string]ResourcePolicy
			if templateRef != "" {
				resourcePolicies = formResourcePolicies[templateRef]
			}

			for _, resRaw := range appliedResourcesRaw {
				resStr, ok := resRaw.(string)
				if !ok {
					continue
				}
				res := parseAppliedResource(resStr)
				if res == nil {
					continue
				}

				resourceID := "resource-" + namespace + "-" + name + "-" + res.ID
				resourceType := "resource" // Generic type for all K8s resources

				// Determine individual resource status
				// Resources in appliedResources are actively managed (reconciliation targets)
				// - "ready" = successfully applied and being reconciled
				// - "skipped" = skipped due to dependency failure
				// Note: This is NOT the K8s Ready condition, but whether it's being managed
				resourceStatus := statusReady
				if skippedResourceIds[res.ID] {
					resourceStatus = statusSkipped
				}

				// Look up policy for this resource
				var metadata map[string]interface{}
				if policy, ok := resourcePolicies[res.ID]; ok {
					metadata = map[string]interface{}{
						"creationPolicy": policy.CreationPolicy,
						"deletionPolicy": policy.DeletionPolicy,
						"conflictPolicy": policy.ConflictPolicy,
					}
				}

				topology.Nodes = append(topology.Nodes, TopologyNode{
					ID:        resourceID,
					Type:      resourceType,
					Name:      res.Kind + "/" + res.Name,
					Namespace: res.Namespace,
					Status:    resourceStatus,
					Metrics: TopologyMetrics{
						Desired: 1,
						Ready:   1,
						Failed:  0,
					},
					Metadata: metadata,
				})

				// Create edge from node to resource
				topology.Edges = append(topology.Edges, TopologyEdge{
					Source: nodeID,
					Target: resourceID,
				})
			}
		}
	}

	// Process orphaned resources
	if orphanedResources != nil {
		for _, res := range orphanedResources.Items {
			name, _, _ := unstructured.NestedString(res.Object, "metadata", "name")
			namespace, _, _ := unstructured.NestedString(res.Object, "metadata", "namespace")
			kind, _, _ := unstructured.NestedString(res.Object, "kind")

			// Get orphan metadata from labels/annotations
			labels, _, _ := unstructured.NestedStringMap(res.Object, "metadata", "labels")
			annotations, _, _ := unstructured.NestedStringMap(res.Object, "metadata", "annotations")

			orphanedAt := annotations["lynq.sh/orphaned-at"]
			orphanedReason := annotations["lynq.sh/orphaned-reason"]
			originalNode := labels["lynq.sh/node"]
			originalNodeNamespace := labels["lynq.sh/node-namespace"]

			resourceID := "orphan-" + namespace + "-" + kind + "-" + name

			metadata := map[string]interface{}{
				"orphaned":              true,
				"orphanedAt":            orphanedAt,
				"orphanedReason":        orphanedReason,
				"originalNode":          originalNode,
				"originalNodeNamespace": originalNodeNamespace,
			}

			topology.Nodes = append(topology.Nodes, TopologyNode{
				ID:        resourceID,
				Type:      "orphan",
				Name:      kind + "/" + name,
				Namespace: namespace,
				Status:    statusSkipped, // Use skipped status for orphaned resources
				Metrics: TopologyMetrics{
					Desired: 0,
					Ready:   0,
					Failed:  0,
				},
				Metadata: metadata,
			})

			// No edges for orphaned resources - they're disconnected
		}
	}

	// Update children arrays in topology nodes
	for i := range topology.Nodes {
		for _, edge := range topology.Edges {
			if edge.Source == topology.Nodes[i].ID {
				topology.Nodes[i].Children = append(topology.Nodes[i].Children, edge.Target)
			}
		}
	}

	return topology
}

// buildFormResourcePolicies extracts resource policies from all LynqForms
// Returns map: formName -> resourceID -> ResourcePolicy
func buildFormResourcePolicies(forms *unstructured.UnstructuredList) map[string]map[string]ResourcePolicy {
	result := make(map[string]map[string]ResourcePolicy)

	// Resource type fields in LynqForm spec
	resourceTypes := []string{
		"serviceAccounts", "deployments", "statefulSets", "daemonSets",
		"services", "ingresses", "configMaps", "secrets",
		"persistentVolumeClaims", "jobs", "cronJobs",
		"podDisruptionBudgets", "networkPolicies", "horizontalPodAutoscalers",
		"manifests",
	}

	for _, form := range forms.Items {
		formName, _, _ := unstructured.NestedString(form.Object, "metadata", "name")
		if formName == "" {
			continue
		}

		policies := make(map[string]ResourcePolicy)

		for _, resType := range resourceTypes {
			resources, found, _ := unstructured.NestedSlice(form.Object, "spec", resType)
			if !found {
				continue
			}

			for _, res := range resources {
				resMap, ok := res.(map[string]interface{})
				if !ok {
					continue
				}

				id, _, _ := unstructured.NestedString(resMap, "id")
				if id == "" {
					continue
				}

				creationPolicy, _, _ := unstructured.NestedString(resMap, "creationPolicy")
				deletionPolicy, _, _ := unstructured.NestedString(resMap, "deletionPolicy")
				conflictPolicy, _, _ := unstructured.NestedString(resMap, "conflictPolicy")

				// Set defaults if empty
				if creationPolicy == "" {
					creationPolicy = "WhenNeeded"
				}
				if deletionPolicy == "" {
					deletionPolicy = "Delete"
				}
				if conflictPolicy == "" {
					conflictPolicy = "Stuck"
				}

				policies[id] = ResourcePolicy{
					CreationPolicy: creationPolicy,
					DeletionPolicy: deletionPolicy,
					ConflictPolicy: conflictPolicy,
				}
			}
		}

		result[formName] = policies
	}

	return result
}
