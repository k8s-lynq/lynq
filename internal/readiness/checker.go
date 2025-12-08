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

package readiness

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ConditionStatusTrue represents a true condition status
	ConditionStatusTrue = "True"
)

// Checker checks if resources are ready
type Checker struct {
	client client.Client
}

// NewChecker creates a new readiness checker
func NewChecker(c client.Client) *Checker {
	return &Checker{client: c}
}

// WaitForReady waits for a resource to become ready
func (c *Checker) WaitForReady(
	ctx context.Context,
	name, namespace string,
	obj *unstructured.Unstructured,
	timeout time.Duration,
) error {
	deadline := time.Now().Add(timeout)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for resource to be ready")
			}

			// Get current state
			key := types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			}

			current := obj.DeepCopy()
			if err := c.client.Get(ctx, key, current); err != nil {
				if errors.IsNotFound(err) {
					continue
				}
				return fmt.Errorf("failed to get resource: %w", err)
			}

			// Check readiness
			if c.IsReady(current) {
				return nil
			}
		}
	}
}

// IsReady checks if a resource is ready based on its type
func (c *Checker) IsReady(obj *unstructured.Unstructured) bool {
	gvk := obj.GroupVersionKind()

	switch gvk.Kind {
	case "Namespace":
		return c.isNamespaceReady(obj)
	case "ConfigMap", "Secret", "ServiceAccount":
		return true // These are ready immediately
	case "Service":
		return c.isServiceReady(obj)
	case "Deployment":
		return c.isDeploymentReady(obj)
	case "StatefulSet":
		return c.isStatefulSetReady(obj)
	case "DaemonSet":
		return c.isDaemonSetReady(obj)
	case "Job":
		return c.isJobReady(obj)
	case "CronJob":
		return true // CronJobs are ready when created
	case "Ingress":
		return c.isIngressReady(obj)
	case "PersistentVolumeClaim":
		return c.isPVCReady(obj)
	case "PodDisruptionBudget":
		return true // PDBs are ready immediately after creation
	case "NetworkPolicy":
		return true // NetworkPolicies are ready immediately after creation
	case "HorizontalPodAutoscaler":
		return c.isHPAReady(obj)
	default:
		// For custom resources, check status.conditions
		return c.hasReadyCondition(obj)
	}
}

// isNamespaceReady checks if a namespace is ready
func (c *Checker) isNamespaceReady(obj *unstructured.Unstructured) bool {
	phase, found, _ := unstructured.NestedString(obj.Object, "status", "phase")
	if !found {
		return false
	}
	return phase == "Active"
}

// isServiceReady checks if a service is ready
func (c *Checker) isServiceReady(obj *unstructured.Unstructured) bool {
	// Services are generally ready immediately
	// For LoadBalancer type, we could check for ingress IP
	serviceType, _, _ := unstructured.NestedString(obj.Object, "spec", "type")
	if serviceType == "LoadBalancer" {
		ingress, found, _ := unstructured.NestedSlice(obj.Object, "status", "loadBalancer", "ingress")
		return found && len(ingress) > 0
	}
	return true
}

// isDeploymentReady checks if a deployment is ready
func (c *Checker) isDeploymentReady(obj *unstructured.Unstructured) bool {
	// Check observed generation
	generation, _, _ := unstructured.NestedInt64(obj.Object, "metadata", "generation")
	observedGeneration, _, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")

	if generation != observedGeneration {
		return false
	}

	// Check replicas
	replicas, found, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if !found {
		replicas = 1 // Default replicas when not specified
	}

	// Special case: deployment scaled to 0 is considered not ready
	// (no pods serving traffic, even though it's at desired state)
	if replicas == 0 {
		return false
	}

	availableReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "availableReplicas")
	updatedReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "updatedReplicas")

	// Deployment is ready when actual replicas match desired replicas
	return availableReplicas >= replicas && updatedReplicas >= replicas
}

// isStatefulSetReady checks if a statefulset is ready
func (c *Checker) isStatefulSetReady(obj *unstructured.Unstructured) bool {
	// Check observed generation
	generation, _, _ := unstructured.NestedInt64(obj.Object, "metadata", "generation")
	observedGeneration, _, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")

	if generation != observedGeneration {
		return false
	}

	// Check replicas
	replicas, found, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if !found {
		replicas = 1 // Default replicas when not specified
	}

	// Special case: statefulset scaled to 0 is considered not ready
	// (no pods serving traffic, even though it's at desired state)
	if replicas == 0 {
		return false
	}

	readyReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
	updatedReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "updatedReplicas")

	return readyReplicas >= replicas && updatedReplicas >= replicas
}

// isDaemonSetReady checks if a daemonset is ready
func (c *Checker) isDaemonSetReady(obj *unstructured.Unstructured) bool {
	desiredNumberScheduled, _, _ := unstructured.NestedInt64(obj.Object, "status", "desiredNumberScheduled")
	numberReady, _, _ := unstructured.NestedInt64(obj.Object, "status", "numberReady")

	return numberReady >= desiredNumberScheduled && desiredNumberScheduled > 0
}

// isJobReady checks if a job is complete
func (c *Checker) isJobReady(obj *unstructured.Unstructured) bool {
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if found {
		for _, cond := range conditions {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _, _ := unstructured.NestedString(condMap, "type")
			condStatus, _, _ := unstructured.NestedString(condMap, "status")

			if condType == "Complete" && condStatus == ConditionStatusTrue {
				return true
			}
			if condType == "Failed" && condStatus == ConditionStatusTrue {
				return false
			}
		}
	}

	succeeded, _, _ := unstructured.NestedInt64(obj.Object, "status", "succeeded")
	return succeeded > 0
}

// isIngressReady checks if an ingress is ready
func (c *Checker) isIngressReady(obj *unstructured.Unstructured) bool {
	// Check for load balancer ingress
	ingress, found, _ := unstructured.NestedSlice(obj.Object, "status", "loadBalancer", "ingress")
	if found && len(ingress) > 0 {
		return true
	}

	// Some ingress controllers don't populate status, so check if rules exist
	rules, found, _ := unstructured.NestedSlice(obj.Object, "spec", "rules")
	return found && len(rules) > 0
}

// isPVCReady checks if a PVC is bound
func (c *Checker) isPVCReady(obj *unstructured.Unstructured) bool {
	phase, found, _ := unstructured.NestedString(obj.Object, "status", "phase")
	if !found {
		return false
	}
	return phase == "Bound"
}

// isHPAReady checks if a HorizontalPodAutoscaler is ready
func (c *Checker) isHPAReady(obj *unstructured.Unstructured) bool {
	// Check for AbleToScale condition
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		return false
	}

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _, _ := unstructured.NestedString(condMap, "type")
		condStatus, _, _ := unstructured.NestedString(condMap, "status")

		// HPA is ready when AbleToScale condition is True
		if condType == "AbleToScale" && condStatus == ConditionStatusTrue {
			return true
		}
	}

	return false
}

// hasReadyCondition checks for a Ready condition in status.conditions
func (c *Checker) hasReadyCondition(obj *unstructured.Unstructured) bool {
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		// No conditions, assume ready if resource exists
		return true
	}

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _, _ := unstructured.NestedString(condMap, "type")
		condStatus, _, _ := unstructured.NestedString(condMap, "status")

		if condType == "Ready" && condStatus == ConditionStatusTrue {
			return true
		}
	}

	return false
}

// GetReadinessMessage returns a human-readable message about resource readiness
func (c *Checker) GetReadinessMessage(obj *unstructured.Unstructured) string {
	if c.IsReady(obj) {
		return "Resource is ready"
	}

	gvk := obj.GroupVersionKind()
	switch gvk.Kind {
	case "Deployment":
		replicas, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
		availableReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "availableReplicas")
		return fmt.Sprintf("Waiting for replicas: %d/%d available", availableReplicas, replicas)
	case "StatefulSet":
		replicas, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
		readyReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
		return fmt.Sprintf("Waiting for replicas: %d/%d ready", readyReplicas, replicas)
	case "Job":
		succeeded, _, _ := unstructured.NestedInt64(obj.Object, "status", "succeeded")
		failed, _, _ := unstructured.NestedInt64(obj.Object, "status", "failed")
		return fmt.Sprintf("Job status: %d succeeded, %d failed", succeeded, failed)
	case "HorizontalPodAutoscaler":
		currentReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "currentReplicas")
		desiredReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "desiredReplicas")
		return fmt.Sprintf("HPA status: %d current, %d desired replicas", currentReplicas, desiredReplicas)
	default:
		return "Waiting for resource to be ready"
	}
}
