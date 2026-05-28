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

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
)

const (
	// ConditionStatusTrue represents a true condition status
	ConditionStatusTrue = "True"

	// Workload kinds — extracted so the kind switch in IsReady,
	// GetReadinessMessage, and ClassifyPhase all reference the same literals
	// (satisfies the goconst linter).
	kindNamespace               = "Namespace"
	kindConfigMap               = "ConfigMap"
	kindSecret                  = "Secret"
	kindServiceAccount          = "ServiceAccount"
	kindService                 = "Service"
	kindDeployment              = "Deployment"
	kindStatefulSet             = "StatefulSet"
	kindDaemonSet               = "DaemonSet"
	kindJob                     = "Job"
	kindCronJob                 = "CronJob"
	kindIngress                 = "Ingress"
	kindPVC                     = "PersistentVolumeClaim"
	kindPDB                     = "PodDisruptionBudget"
	kindNetworkPolicy           = "NetworkPolicy"
	kindHorizontalPodAutoscaler = "HorizontalPodAutoscaler"
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
	case kindNamespace:
		return c.isNamespaceReady(obj)
	case kindConfigMap, kindSecret, kindServiceAccount:
		return true // These are ready immediately
	case kindService:
		return c.isServiceReady(obj)
	case kindDeployment:
		return c.isDeploymentReady(obj)
	case kindStatefulSet:
		return c.isStatefulSetReady(obj)
	case kindDaemonSet:
		return c.isDaemonSetReady(obj)
	case kindJob:
		return c.isJobReady(obj)
	case kindCronJob:
		return true // CronJobs are ready when created
	case kindIngress:
		return c.isIngressReady(obj)
	case kindPVC:
		return c.isPVCReady(obj)
	case kindPDB:
		return true // PDBs are ready immediately after creation
	case kindNetworkPolicy:
		return true // NetworkPolicies are ready immediately after creation
	case kindHorizontalPodAutoscaler:
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
// A deployment is considered ready when:
// 1. The deployment controller has observed the latest spec (observedGeneration == generation)
// 2. All replicas are updated to the latest spec (updatedReplicas == replicas)
// 3. All replicas are available (availableReplicas == replicas)
// 4. All replicas are ready (readyReplicas == replicas)
// This ensures that during a rolling update, we wait for all NEW pods to be ready,
// not just that the old pods are still available.
func (c *Checker) isDeploymentReady(obj *unstructured.Unstructured) bool {
	// Check observed generation - controller must have processed the latest spec
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
	readyReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")

	// Deployment is ready when ALL conditions are met:
	// - updatedReplicas == replicas: All pods are running the new spec
	// - readyReplicas == replicas: All pods have passed readiness probes
	// - availableReplicas == replicas: All pods are available for traffic
	// Using == instead of >= to ensure the rollout is fully complete
	return updatedReplicas == replicas && readyReplicas == replicas && availableReplicas == replicas
}

// isStatefulSetReady checks if a statefulset is ready
// A statefulset is considered ready when:
// 1. The controller has observed the latest spec (observedGeneration == generation)
// 2. All replicas are updated to the latest spec (updatedReplicas == replicas)
// 3. All replicas are ready (readyReplicas == replicas)
// 4. All replicas are at current revision (currentReplicas == replicas)
// This ensures that during a rolling update, we wait for all pods to be updated and ready.
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
	currentReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "currentReplicas")

	// StatefulSet is ready when ALL conditions are met:
	// - updatedReplicas == replicas: All pods are running the new spec
	// - readyReplicas == replicas: All pods have passed readiness probes
	// - currentReplicas == replicas: All pods are at current revision
	return updatedReplicas == replicas && readyReplicas == replicas && currentReplicas == replicas
}

// isDaemonSetReady checks if a daemonset is ready
// A daemonset is considered ready when:
// 1. All desired pods are scheduled (currentNumberScheduled == desiredNumberScheduled)
// 2. All pods are updated to the latest spec (updatedNumberScheduled == desiredNumberScheduled)
// 3. All pods are ready (numberReady == desiredNumberScheduled)
// 4. All pods are available (numberAvailable == desiredNumberScheduled)
// This ensures that during a rolling update, we wait for all pods to be updated and ready.
func (c *Checker) isDaemonSetReady(obj *unstructured.Unstructured) bool {
	// Check observed generation
	generation, _, _ := unstructured.NestedInt64(obj.Object, "metadata", "generation")
	observedGeneration, _, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")

	if generation != observedGeneration {
		return false
	}

	desiredNumberScheduled, _, _ := unstructured.NestedInt64(obj.Object, "status", "desiredNumberScheduled")

	// DaemonSet with no nodes to schedule is not ready
	if desiredNumberScheduled == 0 {
		return false
	}

	currentNumberScheduled, _, _ := unstructured.NestedInt64(obj.Object, "status", "currentNumberScheduled")
	updatedNumberScheduled, _, _ := unstructured.NestedInt64(obj.Object, "status", "updatedNumberScheduled")
	numberReady, _, _ := unstructured.NestedInt64(obj.Object, "status", "numberReady")
	numberAvailable, _, _ := unstructured.NestedInt64(obj.Object, "status", "numberAvailable")

	// DaemonSet is ready when ALL conditions are met
	return currentNumberScheduled == desiredNumberScheduled &&
		updatedNumberScheduled == desiredNumberScheduled &&
		numberReady == desiredNumberScheduled &&
		numberAvailable == desiredNumberScheduled
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

// PhaseResult is the output of ClassifyPhase — the observed phase of a child
// resource plus the supporting evidence the controller and metrics layer need.
type PhaseResult struct {
	// Phase is the resource's current classification.
	Phase lynqv1.ResourcePhase

	// Reason is a short human-readable diagnostic, populated for non-Available
	// phases (and surfaced as the per-resource Reason in status.resourcePhases
	// and as part of the WorkloadDegraded / ReadinessTimeout event message).
	Reason string

	// Replicas captures the workload's replica counters. Zero-valued for
	// non-workload kinds (Service, ConfigMap, etc.).
	Replicas ReplicaStatus

	// RolloutTimedOut is true when Phase==Failed because the rollout timeout
	// elapsed while the resource was Progressing. False for Failed-by-other-
	// reasons (ProgressDeadlineExceeded, Job Failed, etc.). Lets the
	// controller emit a precise event reason ("ReadinessTimeout" vs other).
	RolloutTimedOut bool
}

// ReplicaStatus captures the per-resource replica counters used by metrics
// (lynqnode_resource_replicas_*). For DaemonSet, Available holds
// numberAvailable, Ready holds numberReady, Updated holds
// updatedNumberScheduled, Desired holds desiredNumberScheduled.
type ReplicaStatus struct {
	Desired   int64
	Available int64
	Ready     int64
	Updated   int64
}

// ClassifyPhase returns the observed phase of obj using only its live status
// fields and the supplied elapsed/timeout — no annotation writes, no in-memory
// state. Pure function so behavior on controller restart matches warm-loop
// behavior exactly.
//
// elapsedSinceApply should be computed by the caller from the
// lynq.sh/apply-start-time annotation (see controller's elapsedSinceApply
// helper). rolloutTimeout=0 disables the Progressing→Failed transition (used
// by IsReady's strict-mode wrapper which never escalates).
func (c *Checker) ClassifyPhase(
	obj *unstructured.Unstructured,
	elapsedSinceApply time.Duration,
	rolloutTimeout time.Duration,
) PhaseResult {
	gvk := obj.GroupVersionKind()
	switch gvk.Kind {
	case kindNamespace:
		return c.classifyNamespacePhase(obj)
	case kindConfigMap, kindSecret, kindServiceAccount, kindCronJob, kindPDB, kindNetworkPolicy:
		return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}
	case kindService:
		return c.classifyServicePhase(obj, elapsedSinceApply, rolloutTimeout)
	case kindDeployment:
		return c.classifyDeploymentPhase(obj, elapsedSinceApply, rolloutTimeout)
	case kindStatefulSet:
		return c.classifyStatefulSetPhase(obj, elapsedSinceApply, rolloutTimeout)
	case kindDaemonSet:
		return c.classifyDaemonSetPhase(obj, elapsedSinceApply, rolloutTimeout)
	case kindJob:
		return c.classifyJobPhase(obj)
	case kindIngress:
		return c.classifyIngressPhase(obj, elapsedSinceApply, rolloutTimeout)
	case kindPVC:
		return c.classifyPVCPhase(obj, elapsedSinceApply, rolloutTimeout)
	case kindHorizontalPodAutoscaler:
		return c.classifyHPAPhase(obj, elapsedSinceApply, rolloutTimeout)
	default:
		return c.classifyCustomResourcePhase(obj, elapsedSinceApply, rolloutTimeout)
	}
}

// classifyDeploymentPhase implements the canonical 5-phase model for
// Deployments. The key insight: once updatedReplicas == spec.replicas for the
// current generation, Kubernetes has fully rolled out the new ReplicaSet — any
// subsequent drop in availableReplicas is steady-state disruption, NOT a
// rollout artifact, and is classified Degraded (never Failed).
func (c *Checker) classifyDeploymentPhase(
	obj *unstructured.Unstructured,
	elapsed time.Duration,
	rolloutTimeout time.Duration,
) PhaseResult {
	generation, _, _ := unstructured.NestedInt64(obj.Object, "metadata", "generation")
	observedGeneration, _, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")

	replicas, found, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if !found {
		replicas = 1
	}
	availableReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "availableReplicas")
	updatedReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "updatedReplicas")
	readyReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")

	rs := ReplicaStatus{
		Desired:   replicas,
		Available: availableReplicas,
		Ready:     readyReplicas,
		Updated:   updatedReplicas,
	}

	// 1. Pending — controller hasn't observed the latest spec yet.
	if observedGeneration < generation {
		return PhaseResult{
			Phase:    lynqv1.ResourcePhasePending,
			Reason:   fmt.Sprintf("observedGeneration=%d < generation=%d", observedGeneration, generation),
			Replicas: rs,
		}
	}

	// Special case: scaled to 0. Treat as Pending (matches existing semantics:
	// no pods serving traffic, even though K8s considers the spec converged).
	if replicas == 0 {
		return PhaseResult{Phase: lynqv1.ResourcePhasePending, Reason: "spec.replicas=0", Replicas: rs}
	}

	// 2. ProgressDeadlineExceeded — Kubernetes itself gave up. Lynq follows.
	if reason := readDeploymentProgressingReason(obj); reason == "ProgressDeadlineExceeded" {
		return PhaseResult{
			Phase:    lynqv1.ResourcePhaseFailed,
			Reason:   "ProgressDeadlineExceeded (Kubernetes abandoned the rollout)",
			Replicas: rs,
		}
	}

	// 3. Available — rollout complete and fully healthy.
	if updatedReplicas == replicas && availableReplicas == replicas {
		return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable, Replicas: rs}
	}

	// 4. Degraded — rollout completed for the current generation but
	// availability has since dropped. Kubernetes is converging this; Lynq
	// does NOT mark it Failed.
	if updatedReplicas == replicas && availableReplicas < replicas {
		return PhaseResult{
			Phase:    lynqv1.ResourcePhaseDegraded,
			Reason:   fmt.Sprintf("availableReplicas=%d/%d, observedGeneration matched", availableReplicas, replicas),
			Replicas: rs,
		}
	}

	// 5. Progressing — rollout not yet complete (updatedReplicas < replicas).
	if rolloutTimeout > 0 && elapsed >= rolloutTimeout {
		return PhaseResult{
			Phase:           lynqv1.ResourcePhaseFailed,
			Reason:          fmt.Sprintf("rollout timeout %s elapsed (updatedReplicas=%d/%d)", rolloutTimeout, updatedReplicas, replicas),
			Replicas:        rs,
			RolloutTimedOut: true,
		}
	}
	return PhaseResult{
		Phase:    lynqv1.ResourcePhaseProgressing,
		Reason:   fmt.Sprintf("updatedReplicas=%d/%d", updatedReplicas, replicas),
		Replicas: rs,
	}
}

// classifyStatefulSetPhase mirrors classifyDeploymentPhase but uses
// currentRevision == updateRevision as the rollout-complete signal alongside
// updatedReplicas == spec.replicas.
func (c *Checker) classifyStatefulSetPhase(
	obj *unstructured.Unstructured,
	elapsed time.Duration,
	rolloutTimeout time.Duration,
) PhaseResult {
	generation, _, _ := unstructured.NestedInt64(obj.Object, "metadata", "generation")
	observedGeneration, _, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")

	replicas, found, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if !found {
		replicas = 1
	}
	readyReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
	updatedReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "updatedReplicas")
	currentReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "currentReplicas")
	currentRevision, _, _ := unstructured.NestedString(obj.Object, "status", "currentRevision")
	updateRevision, _, _ := unstructured.NestedString(obj.Object, "status", "updateRevision")

	rs := ReplicaStatus{
		Desired:   replicas,
		Available: readyReplicas, // STS has no separate availableReplicas; use ready as the closest analogue
		Ready:     readyReplicas,
		Updated:   updatedReplicas,
	}

	if observedGeneration < generation {
		return PhaseResult{
			Phase:    lynqv1.ResourcePhasePending,
			Reason:   fmt.Sprintf("observedGeneration=%d < generation=%d", observedGeneration, generation),
			Replicas: rs,
		}
	}

	if replicas == 0 {
		return PhaseResult{Phase: lynqv1.ResourcePhasePending, Reason: "spec.replicas=0", Replicas: rs}
	}

	// Rollout complete: updatedReplicas converged AND revisions match. When
	// updateRevision is empty (fresh STS or older API server), fall back to
	// the updatedReplicas check alone.
	revisionsConverged := updateRevision == "" || currentRevision == updateRevision
	rolloutComplete := updatedReplicas == replicas && currentReplicas == replicas && revisionsConverged

	if rolloutComplete && readyReplicas == replicas {
		return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable, Replicas: rs}
	}

	if rolloutComplete && readyReplicas < replicas {
		return PhaseResult{
			Phase:    lynqv1.ResourcePhaseDegraded,
			Reason:   fmt.Sprintf("readyReplicas=%d/%d, revisions converged", readyReplicas, replicas),
			Replicas: rs,
		}
	}

	// Progressing
	if rolloutTimeout > 0 && elapsed >= rolloutTimeout {
		return PhaseResult{
			Phase:           lynqv1.ResourcePhaseFailed,
			Reason:          fmt.Sprintf("rollout timeout %s elapsed (updatedReplicas=%d/%d, currentRevision=%s, updateRevision=%s)", rolloutTimeout, updatedReplicas, replicas, currentRevision, updateRevision),
			Replicas:        rs,
			RolloutTimedOut: true,
		}
	}
	return PhaseResult{
		Phase:    lynqv1.ResourcePhaseProgressing,
		Reason:   fmt.Sprintf("updatedReplicas=%d/%d, currentReplicas=%d/%d", updatedReplicas, replicas, currentReplicas, replicas),
		Replicas: rs,
	}
}

// classifyDaemonSetPhase uses updatedNumberScheduled == desiredNumberScheduled
// as the rollout-complete signal. Once a DS has rolled out across all matching
// nodes, transient drops in numberAvailable (e.g., during node drain) are
// Degraded, not Failed.
func (c *Checker) classifyDaemonSetPhase(
	obj *unstructured.Unstructured,
	elapsed time.Duration,
	rolloutTimeout time.Duration,
) PhaseResult {
	generation, _, _ := unstructured.NestedInt64(obj.Object, "metadata", "generation")
	observedGeneration, _, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")

	desired, _, _ := unstructured.NestedInt64(obj.Object, "status", "desiredNumberScheduled")
	updated, _, _ := unstructured.NestedInt64(obj.Object, "status", "updatedNumberScheduled")
	ready, _, _ := unstructured.NestedInt64(obj.Object, "status", "numberReady")
	available, _, _ := unstructured.NestedInt64(obj.Object, "status", "numberAvailable")

	rs := ReplicaStatus{Desired: desired, Available: available, Ready: ready, Updated: updated}

	if observedGeneration < generation {
		return PhaseResult{
			Phase:    lynqv1.ResourcePhasePending,
			Reason:   fmt.Sprintf("observedGeneration=%d < generation=%d", observedGeneration, generation),
			Replicas: rs,
		}
	}

	// No nodes match the selector — Pending (matches existing behavior).
	if desired == 0 {
		return PhaseResult{Phase: lynqv1.ResourcePhasePending, Reason: "desiredNumberScheduled=0", Replicas: rs}
	}

	if updated == desired && available == desired {
		return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable, Replicas: rs}
	}

	if updated == desired && available < desired {
		return PhaseResult{
			Phase:    lynqv1.ResourcePhaseDegraded,
			Reason:   fmt.Sprintf("numberAvailable=%d/%d, updatedNumberScheduled matched", available, desired),
			Replicas: rs,
		}
	}

	// Progressing
	if rolloutTimeout > 0 && elapsed >= rolloutTimeout {
		return PhaseResult{
			Phase:           lynqv1.ResourcePhaseFailed,
			Reason:          fmt.Sprintf("rollout timeout %s elapsed (updatedNumberScheduled=%d/%d)", rolloutTimeout, updated, desired),
			Replicas:        rs,
			RolloutTimedOut: true,
		}
	}
	return PhaseResult{
		Phase:    lynqv1.ResourcePhaseProgressing,
		Reason:   fmt.Sprintf("updatedNumberScheduled=%d/%d", updated, desired),
		Replicas: rs,
	}
}

// classifyServicePhase: ClusterIP/NodePort/Headless services are Available
// immediately; LoadBalancer services Progress until status.loadBalancer.ingress
// is populated.
func (c *Checker) classifyServicePhase(
	obj *unstructured.Unstructured,
	elapsed time.Duration,
	rolloutTimeout time.Duration,
) PhaseResult {
	serviceType, _, _ := unstructured.NestedString(obj.Object, "spec", "type")
	if serviceType != "LoadBalancer" {
		return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}
	}
	ingress, found, _ := unstructured.NestedSlice(obj.Object, "status", "loadBalancer", "ingress")
	if found && len(ingress) > 0 {
		return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}
	}
	if rolloutTimeout > 0 && elapsed >= rolloutTimeout {
		return PhaseResult{
			Phase:           lynqv1.ResourcePhaseFailed,
			Reason:          "LoadBalancer ingress not provisioned within rollout timeout",
			RolloutTimedOut: true,
		}
	}
	return PhaseResult{Phase: lynqv1.ResourcePhaseProgressing, Reason: "waiting for LoadBalancer ingress"}
}

// classifyJobPhase: Job is Available on Complete=True or succeeded>0;
// Failed=True is a hard failure (not a rollout timeout — no RolloutTimedOut).
// Otherwise Progressing.
func (c *Checker) classifyJobPhase(obj *unstructured.Unstructured) PhaseResult {
	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, cond := range conditions {
		cm, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		ctype, _, _ := unstructured.NestedString(cm, "type")
		cstatus, _, _ := unstructured.NestedString(cm, "status")
		if ctype == "Complete" && cstatus == ConditionStatusTrue {
			return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}
		}
		if ctype == "Failed" && cstatus == ConditionStatusTrue {
			return PhaseResult{Phase: lynqv1.ResourcePhaseFailed, Reason: "Job Failed condition True"}
		}
	}
	succeeded, _, _ := unstructured.NestedInt64(obj.Object, "status", "succeeded")
	if succeeded > 0 {
		return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}
	}
	return PhaseResult{Phase: lynqv1.ResourcePhaseProgressing, Reason: "Job not yet complete"}
}

// classifyNamespacePhase: Active → Available; everything else → Pending.
func (c *Checker) classifyNamespacePhase(obj *unstructured.Unstructured) PhaseResult {
	phase, found, _ := unstructured.NestedString(obj.Object, "status", "phase")
	if found && phase == "Active" {
		return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}
	}
	return PhaseResult{Phase: lynqv1.ResourcePhasePending, Reason: fmt.Sprintf("namespace phase=%q", phase)}
}

// classifyIngressPhase: Available once the controller populates
// status.loadBalancer.ingress (or, for controllers that don't populate status,
// once spec.rules exists — matches existing isIngressReady semantics).
func (c *Checker) classifyIngressPhase(
	obj *unstructured.Unstructured,
	elapsed time.Duration,
	rolloutTimeout time.Duration,
) PhaseResult {
	ingress, found, _ := unstructured.NestedSlice(obj.Object, "status", "loadBalancer", "ingress")
	if found && len(ingress) > 0 {
		return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}
	}
	rules, found, _ := unstructured.NestedSlice(obj.Object, "spec", "rules")
	if found && len(rules) > 0 {
		return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}
	}
	if rolloutTimeout > 0 && elapsed >= rolloutTimeout {
		return PhaseResult{
			Phase:           lynqv1.ResourcePhaseFailed,
			Reason:          "Ingress neither populated status nor declared rules within rollout timeout",
			RolloutTimedOut: true,
		}
	}
	return PhaseResult{Phase: lynqv1.ResourcePhaseProgressing, Reason: "waiting for Ingress provisioning"}
}

// classifyPVCPhase: Bound → Available; otherwise Progressing/Failed.
func (c *Checker) classifyPVCPhase(
	obj *unstructured.Unstructured,
	elapsed time.Duration,
	rolloutTimeout time.Duration,
) PhaseResult {
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
	if phase == "Bound" {
		return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}
	}
	if rolloutTimeout > 0 && elapsed >= rolloutTimeout {
		return PhaseResult{
			Phase:           lynqv1.ResourcePhaseFailed,
			Reason:          fmt.Sprintf("PVC phase=%q, expected Bound", phase),
			RolloutTimedOut: true,
		}
	}
	return PhaseResult{Phase: lynqv1.ResourcePhaseProgressing, Reason: fmt.Sprintf("PVC phase=%q", phase)}
}

// classifyHPAPhase: AbleToScale=True → Available; otherwise Progressing/Failed.
func (c *Checker) classifyHPAPhase(
	obj *unstructured.Unstructured,
	elapsed time.Duration,
	rolloutTimeout time.Duration,
) PhaseResult {
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		if rolloutTimeout > 0 && elapsed >= rolloutTimeout {
			return PhaseResult{
				Phase:           lynqv1.ResourcePhaseFailed,
				Reason:          "HPA has no status.conditions within rollout timeout",
				RolloutTimedOut: true,
			}
		}
		return PhaseResult{Phase: lynqv1.ResourcePhaseProgressing, Reason: "HPA status.conditions not populated"}
	}
	for _, cond := range conditions {
		cm, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		ctype, _, _ := unstructured.NestedString(cm, "type")
		cstatus, _, _ := unstructured.NestedString(cm, "status")
		if ctype == "AbleToScale" && cstatus == ConditionStatusTrue {
			return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}
		}
	}
	if rolloutTimeout > 0 && elapsed >= rolloutTimeout {
		return PhaseResult{
			Phase:           lynqv1.ResourcePhaseFailed,
			Reason:          "HPA AbleToScale condition not True within rollout timeout",
			RolloutTimedOut: true,
		}
	}
	return PhaseResult{Phase: lynqv1.ResourcePhaseProgressing, Reason: "HPA AbleToScale condition not yet True"}
}

// classifyCustomResourcePhase: CRDs are Available either when they expose no
// status.conditions (matches existing fallback behavior) or when they expose
// a Ready=True condition. Anything else is Progressing.
func (c *Checker) classifyCustomResourcePhase(
	obj *unstructured.Unstructured,
	elapsed time.Duration,
	rolloutTimeout time.Duration,
) PhaseResult {
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		// Match existing hasReadyCondition: no conditions → assume ready.
		return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}
	}
	for _, cond := range conditions {
		cm, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		ctype, _, _ := unstructured.NestedString(cm, "type")
		cstatus, _, _ := unstructured.NestedString(cm, "status")
		if ctype == "Ready" && cstatus == ConditionStatusTrue {
			return PhaseResult{Phase: lynqv1.ResourcePhaseAvailable}
		}
	}
	if rolloutTimeout > 0 && elapsed >= rolloutTimeout {
		return PhaseResult{
			Phase:           lynqv1.ResourcePhaseFailed,
			Reason:          "custom resource Ready condition not True within rollout timeout",
			RolloutTimedOut: true,
		}
	}
	return PhaseResult{Phase: lynqv1.ResourcePhaseProgressing, Reason: "custom resource Ready condition not yet True"}
}

// readDeploymentProgressingReason extracts the reason of the Progressing
// condition (canonical values: NewReplicaSetAvailable, NewReplicaSetCreated,
// ReplicaSetUpdated, ProgressDeadlineExceeded). Empty if not found.
func readDeploymentProgressingReason(obj *unstructured.Unstructured) string {
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		return ""
	}
	for _, cond := range conditions {
		cm, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		ctype, _, _ := unstructured.NestedString(cm, "type")
		if ctype != "Progressing" {
			continue
		}
		reason, _, _ := unstructured.NestedString(cm, "reason")
		return reason
	}
	return ""
}

// GetReadinessMessage returns a human-readable message about resource readiness
func (c *Checker) GetReadinessMessage(obj *unstructured.Unstructured) string {
	if c.IsReady(obj) {
		return "Resource is ready"
	}

	gvk := obj.GroupVersionKind()
	switch gvk.Kind {
	case kindDeployment:
		replicas, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
		availableReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "availableReplicas")
		return fmt.Sprintf("Waiting for replicas: %d/%d available", availableReplicas, replicas)
	case kindStatefulSet:
		replicas, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
		readyReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
		return fmt.Sprintf("Waiting for replicas: %d/%d ready", readyReplicas, replicas)
	case kindJob:
		succeeded, _, _ := unstructured.NestedInt64(obj.Object, "status", "succeeded")
		failed, _, _ := unstructured.NestedInt64(obj.Object, "status", "failed")
		return fmt.Sprintf("Job status: %d succeeded, %d failed", succeeded, failed)
	case kindHorizontalPodAutoscaler:
		currentReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "currentReplicas")
		desiredReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "desiredReplicas")
		return fmt.Sprintf("HPA status: %d current, %d desired replicas", currentReplicas, desiredReplicas)
	default:
		return "Waiting for resource to be ready"
	}
}
