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

package controller

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/graph"
	"github.com/k8s-lynq/lynq/internal/metrics"
)

const (
	// ConditionTypeValid is the condition type for LynqForm validation status
	ConditionTypeValid = "Valid"
	// ConditionTypeApplied is the condition type for LynqForm applied status
	ConditionTypeApplied = "Applied"
)

// LynqFormReconciler reconciles a LynqForm object
type LynqFormReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// now is the clock rollout.progressDeadlineSeconds is measured against. nil means time.Now.
	now func() time.Time
}

func (r *LynqFormReconciler) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqforms,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqforms/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqforms/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqhubs,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqnodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile validates a LynqForm and checks node statuses
func (r *LynqFormReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch LynqForm
	tmpl := &lynqv1.LynqForm{}
	if err := r.Get(ctx, req.NamespacedName, tmpl); err != nil {
		if errors.IsNotFound(err) {
			// The form is gone: drop its rollout series, or a deleted form stalled in Failed keeps
			// LynqFormRolloutFailed firing for the life of the process.
			deleteRolloutMetrics(req.Name, req.Namespace)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get LynqForm")
		return ctrl.Result{}, err
	}

	// Validate
	validationErrors := r.validate(ctx, tmpl)

	// Check previous validation state to avoid duplicate events
	wasValid := false
	for _, cond := range tmpl.Status.Conditions {
		if cond.Type == ConditionTypeValid && cond.Status == metav1.ConditionTrue {
			wasValid = true
			break
		}
	}

	if len(validationErrors) > 0 {
		logger.Info("LynqForm validation failed", "errors", validationErrors)
		// Update status with validation errors
		if err := r.updateStatus(ctx, tmpl, validationErrors); err != nil {
			if stderrors.Is(err, errFormChangedDuringStatus) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		// Emit warning event for validation failure (only if state changed)
		if wasValid {
			r.Recorder.Eventf(tmpl, corev1.EventTypeWarning, "ValidationFailed",
				"Template validation failed: %v", validationErrors)
		}
		return ctrl.Result{}, nil
	}

	// Emit normal event for validation success only if state changed (was invalid before)
	if !wasValid {
		r.Recorder.Event(tmpl, corev1.EventTypeNormal, "ValidationPassed",
			"Template validation passed successfully")
	}

	// Check node statuses and update status
	if err := r.updateStatus(ctx, tmpl, validationErrors); err != nil {
		if stderrors.Is(err, errFormChangedDuringStatus) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

// validate validates a LynqForm
func (r *LynqFormReconciler) validate(ctx context.Context, tmpl *lynqv1.LynqForm) []string {
	var validationErrors []string

	// 1. Check if LynqHub exists
	if err := r.validateHubExists(ctx, tmpl); err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("Hub validation failed: %v", err))
		// Emit specific event for hub not found
		r.Recorder.Eventf(tmpl, corev1.EventTypeWarning, "HubNotFound",
			"Referenced LynqHub '%s' not found in namespace '%s'",
			tmpl.Spec.HubID, tmpl.Namespace)
	}

	// 2. Check for duplicate resource IDs
	if dupes := r.findDuplicateIDs(tmpl); len(dupes) > 0 {
		validationErrors = append(validationErrors, fmt.Sprintf("Duplicate resource IDs: %v", dupes))
		r.Recorder.Eventf(tmpl, corev1.EventTypeWarning, "DuplicateResourceIDs",
			"Found duplicate resource IDs: %v", dupes)
	}

	// 3. Validate dependency graph
	if err := r.validateDependencies(tmpl); err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("Dependency validation failed: %v", err))
		r.Recorder.Eventf(tmpl, corev1.EventTypeWarning, "DependencyValidationFailed",
			"Dependency graph validation failed: %v", err)
	}

	return validationErrors
}

// validateHubExists checks if the referenced LynqHub exists
func (r *LynqFormReconciler) validateHubExists(ctx context.Context, tmpl *lynqv1.LynqForm) error {
	hub := &lynqv1.LynqHub{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      tmpl.Spec.HubID,
		Namespace: tmpl.Namespace,
	}, hub); err != nil {
		return fmt.Errorf("hub '%s' not found: %w", tmpl.Spec.HubID, err)
	}
	return nil
}

// findDuplicateIDs finds duplicate resource IDs
func (r *LynqFormReconciler) findDuplicateIDs(tmpl *lynqv1.LynqForm) []string {
	seen := make(map[string]bool)
	var duplicates []string

	allResources := r.collectAllResources(tmpl)

	for _, resource := range allResources {
		if resource.ID == "" {
			continue
		}
		if seen[resource.ID] {
			duplicates = append(duplicates, resource.ID)
		}
		seen[resource.ID] = true
	}

	return duplicates
}

// validateDependencies validates the dependency graph
func (r *LynqFormReconciler) validateDependencies(tmpl *lynqv1.LynqForm) error {
	allResources := r.collectAllResources(tmpl)

	// Build dependency graph
	depGraph, err := graph.BuildGraph(allResources)
	if err != nil {
		return err
	}

	// Validate (checks for cycles and missing dependencies)
	if err := depGraph.Validate(); err != nil {
		return err
	}

	return nil
}

// collectAllResources collects all resources from the template
func (r *LynqFormReconciler) collectAllResources(tmpl *lynqv1.LynqForm) []lynqv1.TResource {
	var resources []lynqv1.TResource

	resources = append(resources, tmpl.Spec.ServiceAccounts...)
	resources = append(resources, tmpl.Spec.Deployments...)
	resources = append(resources, tmpl.Spec.StatefulSets...)
	resources = append(resources, tmpl.Spec.DaemonSets...)
	resources = append(resources, tmpl.Spec.Services...)
	resources = append(resources, tmpl.Spec.Ingresses...)
	resources = append(resources, tmpl.Spec.ConfigMaps...)
	resources = append(resources, tmpl.Spec.Secrets...)
	resources = append(resources, tmpl.Spec.PersistentVolumeClaims...)
	resources = append(resources, tmpl.Spec.Jobs...)
	resources = append(resources, tmpl.Spec.CronJobs...)
	resources = append(resources, tmpl.Spec.PodDisruptionBudgets...)
	resources = append(resources, tmpl.Spec.NetworkPolicies...)
	resources = append(resources, tmpl.Spec.HorizontalPodAutoscalers...)
	resources = append(resources, tmpl.Spec.Namespaces...)
	resources = append(resources, tmpl.Spec.Manifests...)

	return resources
}

// rolloutStats holds rollout statistics for a template
type rolloutStats struct {
	totalNodes        int32
	readyNodes        int32
	updatedNodes      int32 // Nodes updated to target generation
	updatingNodes     int32 // Updated but not Ready for the updated spec yet
	readyUpdatedNodes int32 // Updated AND Ready for the updated spec
	// stalledNodes are updated nodes that failed, or never became Ready within
	// progressDeadlineSeconds of their update. Any entry puts the rollout in Phase=Failed.
	stalledNodes []stalledNode
}

// checkLynqNodeStatuses checks the status of all nodes using this template
func (r *LynqFormReconciler) checkLynqNodeStatuses(ctx context.Context, tmpl *lynqv1.LynqForm) (totalLynqNodes, readyLynqNodes int32, err error) {
	stats, err := r.calculateRolloutStats(ctx, tmpl)
	return stats.totalNodes, stats.readyNodes, err
}

// calculateRolloutStats calculates rollout statistics for a template.
//
// A failed node list is returned as an error, never as zero stats: zero nodes is a real state
// (Phase=Idle), and publishing it for a read failure would turn a stalled rollout into a false
// recovery and back on the next pass — two writes, two events and a flapping alert.
func (r *LynqFormReconciler) calculateRolloutStats(ctx context.Context, tmpl *lynqv1.LynqForm) (rolloutStats, error) {
	stats := rolloutStats{}
	targetGenStr := fmt.Sprintf("%d", tmpl.Generation)

	// List all nodes that reference this template
	nodeList := &lynqv1.LynqNodeList{}
	if err := r.List(ctx, nodeList, client.InNamespace(tmpl.Namespace)); err != nil {
		return stats, fmt.Errorf("failed to list LynqNodes: %w", err)
	}

	now := r.clock()
	deadline := progressDeadline(tmpl)

	// Filter nodes that use this template
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if node.Spec.TemplateRef != tmpl.Name {
			continue
		}

		stats.totalNodes++

		nodeReady := classifyNode(node) == nodeHealthReady
		if nodeReady {
			stats.readyNodes++
		}

		// Check if node has been updated to target generation
		nodeGen := node.Annotations[lynqv1.AnnotationTemplateGeneration]
		if nodeGen == targetGenStr {
			stats.updatedNodes++
			// Ready for the previous spec is not progress on this update: it only counts once
			// the LynqNode controller has observed the updated spec.
			if reachedReadyForCurrentSpec(node) {
				stats.readyUpdatedNodes++
			} else {
				stats.updatingNodes++
			}
			if reason := stallReason(node, deadline, now); reason != "" {
				stats.stalledNodes = append(stats.stalledNodes, stalledNode{name: node.Name, reason: reason})
			}
		}
	}

	return stats, nil
}

// errFormChangedDuringStatus reports that the LynqForm moved to a new generation after its
// rollout stats were computed. The stats describe the previous generation's rollout, so they are
// not published; the reconcile is requeued to compute them for the new one.
var errFormChangedDuringStatus = stderrors.New("LynqForm generation changed while computing its status")

// updateStatus updates LynqForm status with retry on conflict. A node list that cannot be read,
// or a form whose generation moved while its stats were computed, is returned without touching
// the status, metrics or events, so the reconcile retries.
func (r *LynqFormReconciler) updateStatus(ctx context.Context, tmpl *lynqv1.LynqForm, validationErrors []string) error {
	stats, err := r.calculateRolloutStats(ctx, tmpl)
	if err != nil {
		return err
	}
	return r.updateStatusWithRollout(ctx, tmpl, validationErrors, stats)
}

// updateStatusWithRollout updates LynqForm status including rollout status. It returns only
// errFormChangedDuringStatus; other write failures are logged, as before.
func (r *LynqFormReconciler) updateStatusWithRollout(ctx context.Context, tmpl *lynqv1.LynqForm, validationErrors []string, stats rolloutStats) error {
	logger := log.FromContext(ctx)

	// The phase the stored status had and the one this pass computed, for the transition event.
	var phaseBefore, phaseAfter lynqv1.RolloutPhase
	var rolloutMessage string

	// Retry status update on conflict
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version of the template
		key := client.ObjectKeyFromObject(tmpl)
		latest := &lynqv1.LynqForm{}
		if err := r.Get(ctx, key, latest); err != nil {
			return err
		}

		// stats target tmpl.Generation; publishing them under a newer one would report the previous
		// rollout's stall (and RolloutFailed) against a rollout that has just started.
		if latest.Generation != tmpl.Generation {
			return errFormChangedDuringStatus
		}

		// Snapshot status before modifications to detect no-op writes
		statusBefore := latest.Status.DeepCopy()

		// Update status fields
		latest.Status.ObservedGeneration = latest.Generation
		latest.Status.TotalNodes = stats.totalNodes
		latest.Status.ReadyNodes = stats.readyNodes

		// Update rollout status if maxSkew is configured
		phaseBefore, phaseAfter, rolloutMessage = "", "", ""
		if latest.Status.Rollout != nil {
			phaseBefore = latest.Status.Rollout.Phase
		}
		r.updateRolloutStatus(latest, stats)
		if latest.Status.Rollout != nil {
			phaseAfter = latest.Status.Rollout.Phase
			rolloutMessage = latest.Status.Rollout.Message
		}

		// Prepare Valid condition — use meta.SetStatusCondition to preserve LastTransitionTime
		validCondition := metav1.Condition{
			Type:    ConditionTypeValid,
			Status:  metav1.ConditionTrue,
			Reason:  "ValidationPassed",
			Message: "Template validation passed",
		}
		if len(validationErrors) > 0 {
			validCondition.Status = metav1.ConditionFalse
			validCondition.Reason = "ValidationFailed"
			validCondition.Message = fmt.Sprintf("Validation errors: %v", validationErrors)
		}

		// Prepare Applied condition
		appliedCondition := metav1.Condition{
			Type:    ConditionTypeApplied,
			Status:  metav1.ConditionFalse,
			Reason:  "NotAllNodesReady",
			Message: fmt.Sprintf("%d/%d nodes ready", stats.readyNodes, stats.totalNodes),
		}
		if stats.totalNodes > 0 && stats.readyNodes == stats.totalNodes {
			appliedCondition.Status = metav1.ConditionTrue
			appliedCondition.Reason = "AllNodesReady"
			appliedCondition.Message = fmt.Sprintf("All %d nodes ready", stats.totalNodes)
		} else if stats.totalNodes == 0 {
			appliedCondition.Reason = "NoNodes"
			appliedCondition.Message = "No nodes using this template"
		}

		// Use meta.SetStatusCondition which correctly preserves LastTransitionTime
		meta.SetStatusCondition(&latest.Status.Conditions, validCondition)
		meta.SetStatusCondition(&latest.Status.Conditions, appliedCondition)

		// Skip write if status is unchanged (compare full status including rollout fields)
		if apiequality.Semantic.DeepEqual(statusBefore, &latest.Status) {
			return nil
		}

		// Update status subresource
		return r.Status().Update(ctx, latest)
	})

	if stderrors.Is(err, errFormChangedDuringStatus) {
		logger.V(1).Info("LynqForm changed while computing its status; requeueing",
			"computedGeneration", tmpl.Generation)
		return err
	}
	if err != nil {
		logger.Error(err, "Failed to update LynqForm status after retries")
		return nil
	}

	// Announce stall transitions once. The stored phase only changes through the write above, so
	// a transition is reported exactly when it was persisted, and a restart that finds the phase
	// already Failed does not announce it again.
	switch {
	case phaseAfter == lynqv1.RolloutPhaseFailed && phaseBefore != lynqv1.RolloutPhaseFailed:
		r.Recorder.Event(tmpl, corev1.EventTypeWarning, "RolloutFailed", rolloutMessage)
	case phaseBefore == lynqv1.RolloutPhaseFailed && phaseAfter != lynqv1.RolloutPhaseFailed && phaseAfter != "":
		r.Recorder.Eventf(tmpl, corev1.EventTypeNormal, "RolloutRecovered",
			"Rollout is no longer stalled (phase %s): %s", phaseAfter, rolloutMessage)
	}
	return nil
}

// updateRolloutStatus updates the rollout status based on current statistics
func (r *LynqFormReconciler) updateRolloutStatus(tmpl *lynqv1.LynqForm, stats rolloutStats) {
	// Only track rollout status if maxSkew is configured
	if tmpl.Spec.Rollout == nil || tmpl.Spec.Rollout.MaxSkew == 0 {
		// Clear rollout status if maxSkew is not configured
		tmpl.Status.Rollout = nil
		// Clear metrics when rollout is not configured
		deleteRolloutMetrics(tmpl.Name, tmpl.Namespace)
		return
	}

	// Initialize rollout status if needed
	if tmpl.Status.Rollout == nil {
		tmpl.Status.Rollout = &lynqv1.RolloutStatus{}
	}

	rollout := tmpl.Status.Rollout
	// A CompletionTime stamped when the rollout stalled describes the failure, not the success
	// that may follow it.
	if rollout.Phase == lynqv1.RolloutPhaseFailed && len(stats.stalledNodes) == 0 {
		rollout.CompletionTime = nil
	}
	rollout.TargetGeneration = tmpl.Generation
	rollout.TotalNodes = stats.totalNodes
	rollout.UpdatedNodes = stats.updatedNodes
	rollout.UpdatingNodes = stats.updatingNodes
	rollout.ReadyUpdatedNodes = stats.readyUpdatedNodes

	// Determine rollout phase
	if stats.totalNodes == 0 {
		rollout.Phase = lynqv1.RolloutPhaseIdle
		rollout.Message = "No nodes using this template"
	} else if len(stats.stalledNodes) > 0 {
		// An updated node failed or never became Ready within progressDeadlineSeconds. It keeps
		// its maxSkew slot, so the rollout cannot progress past it: report that instead of an
		// InProgress that never ends. Clears by itself once the node recovers or is fixed.
		rollout.Phase = lynqv1.RolloutPhaseFailed
		rollout.Message = formatStalledMessage(stats.stalledNodes, progressDeadline(tmpl))
		if rollout.CompletionTime == nil {
			now := metav1.Now()
			rollout.CompletionTime = &now
		}
		if rollout.StartTime == nil {
			now := metav1.Now()
			rollout.StartTime = &now
		}
	} else if stats.readyUpdatedNodes == stats.totalNodes {
		// All nodes are updated and ready - rollout complete
		rollout.Phase = lynqv1.RolloutPhaseComplete
		rollout.Message = fmt.Sprintf("All %d nodes updated and ready", stats.totalNodes)
		if rollout.CompletionTime == nil {
			now := metav1.Now()
			rollout.CompletionTime = &now
		}
	} else if stats.updatedNodes == 0 {
		// No nodes have been updated yet - rollout not started or idle
		rollout.Phase = lynqv1.RolloutPhaseIdle
		rollout.Message = "Waiting for nodes to be updated"
		// Reset completion time for new rollout
		rollout.CompletionTime = nil
		if rollout.StartTime == nil {
			now := metav1.Now()
			rollout.StartTime = &now
		}
	} else {
		// Some nodes are updating - rollout in progress
		rollout.Phase = lynqv1.RolloutPhaseInProgress
		rollout.Message = fmt.Sprintf("Updating: %d/%d nodes updated (%d updating, %d ready)",
			stats.updatedNodes, stats.totalNodes, stats.updatingNodes, stats.readyUpdatedNodes)
		// Reset completion time while in progress
		rollout.CompletionTime = nil
		if rollout.StartTime == nil {
			now := metav1.Now()
			rollout.StartTime = &now
		}
	}

	// Update metrics
	metrics.FormRolloutUpdatingNodes.WithLabelValues(tmpl.Name, tmpl.Namespace).Set(float64(stats.updatingNodes))
	metrics.FormRolloutPhase.WithLabelValues(tmpl.Name, tmpl.Namespace).Set(metrics.RolloutPhaseToMetric(string(rollout.Phase)))

	// Calculate progress percentage
	var progress float64
	if stats.totalNodes > 0 {
		progress = float64(stats.readyUpdatedNodes) / float64(stats.totalNodes) * 100
	}
	metrics.FormRolloutProgress.WithLabelValues(tmpl.Name, tmpl.Namespace).Set(progress)
}

// deleteRolloutMetrics drops every rollout series of a LynqForm.
func deleteRolloutMetrics(name, namespace string) {
	metrics.FormRolloutUpdatingNodes.DeleteLabelValues(name, namespace)
	metrics.FormRolloutPhase.DeleteLabelValues(name, namespace)
	metrics.FormRolloutProgress.DeleteLabelValues(name, namespace)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LynqFormReconciler) SetupWithManager(mgr ctrl.Manager, concurrency int) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lynqv1.LynqForm{}).
		Named("lynqform").
		// Watch LynqNodes to update template Applied status when node status changes
		Watches(&lynqv1.LynqNode{}, handler.EnqueueRequestsFromMapFunc(r.findTemplateForLynqNode)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: concurrency,
		}).
		Complete(r)
}

// findTemplateForLynqNode maps a LynqNode to its LynqForm for watch events
func (r *LynqFormReconciler) findTemplateForLynqNode(ctx context.Context, node client.Object) []reconcile.Request {
	t := node.(*lynqv1.LynqNode)
	if t.Spec.TemplateRef == "" {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      t.Spec.TemplateRef,
				Namespace: t.Namespace,
			},
		},
	}
}
