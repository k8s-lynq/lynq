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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
		r.updateStatus(ctx, tmpl, validationErrors, 0, 0)
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

	// Check node statuses
	totalLynqNodes, readyLynqNodes, err := r.checkLynqNodeStatuses(ctx, tmpl)
	if err != nil {
		logger.Error(err, "Failed to check node statuses")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Update status with node counts
	r.updateStatus(ctx, tmpl, validationErrors, totalLynqNodes, readyLynqNodes)

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

// checkLynqNodeStatuses checks the status of all nodes using this template
func (r *LynqFormReconciler) checkLynqNodeStatuses(ctx context.Context, tmpl *lynqv1.LynqForm) (totalLynqNodes, readyLynqNodes int32, err error) {
	// List all nodes that reference this template
	nodeList := &lynqv1.LynqNodeList{}
	if err := r.List(ctx, nodeList, client.InNamespace(tmpl.Namespace)); err != nil {
		return 0, 0, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Filter nodes that use this template
	for _, node := range nodeList.Items {
		if node.Spec.TemplateRef == tmpl.Name {
			totalLynqNodes++

			// Check if node is Ready
			for _, condition := range node.Status.Conditions {
				if condition.Type == ConditionTypeReady && condition.Status == metav1.ConditionTrue {
					readyLynqNodes++
					break
				}
			}
		}
	}

	return totalLynqNodes, readyLynqNodes, nil
}

// updateStatus updates LynqForm status with retry on conflict
func (r *LynqFormReconciler) updateStatus(ctx context.Context, tmpl *lynqv1.LynqForm, validationErrors []string, totalLynqNodes, readyLynqNodes int32) {
	logger := log.FromContext(ctx)

	// Retry status update on conflict
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version of the template
		key := client.ObjectKeyFromObject(tmpl)
		latest := &lynqv1.LynqForm{}
		if err := r.Get(ctx, key, latest); err != nil {
			return err
		}

		// Update status fields
		latest.Status.ObservedGeneration = latest.Generation
		latest.Status.TotalNodes = totalLynqNodes
		latest.Status.ReadyNodes = readyLynqNodes

		// Prepare Valid condition
		validCondition := metav1.Condition{
			Type:               ConditionTypeValid,
			Status:             metav1.ConditionTrue,
			Reason:             "ValidationPassed",
			Message:            "Template validation passed",
			LastTransitionTime: metav1.Now(),
		}

		if len(validationErrors) > 0 {
			validCondition.Status = metav1.ConditionFalse
			validCondition.Reason = "ValidationFailed"
			validCondition.Message = fmt.Sprintf("Validation errors: %v", validationErrors)
		}

		// Prepare Applied condition
		appliedCondition := metav1.Condition{
			Type:               "Applied",
			Status:             metav1.ConditionFalse,
			Reason:             "NotAllNodesReady",
			Message:            fmt.Sprintf("%d/%d nodes ready", readyLynqNodes, totalLynqNodes),
			LastTransitionTime: metav1.Now(),
		}

		if totalLynqNodes > 0 && readyLynqNodes == totalLynqNodes {
			appliedCondition.Status = metav1.ConditionTrue
			appliedCondition.Reason = "AllNodesReady"
			appliedCondition.Message = fmt.Sprintf("All %d nodes ready", totalLynqNodes)
		} else if totalLynqNodes == 0 {
			appliedCondition.Reason = "NoNodes"
			appliedCondition.Message = "No nodes using this template"
		}

		// Update or append Valid condition
		foundValid := false
		for i := range latest.Status.Conditions {
			if latest.Status.Conditions[i].Type == ConditionTypeValid {
				latest.Status.Conditions[i] = validCondition
				foundValid = true
				break
			}
		}
		if !foundValid {
			latest.Status.Conditions = append(latest.Status.Conditions, validCondition)
		}

		// Update or append Applied condition
		foundApplied := false
		for i := range latest.Status.Conditions {
			if latest.Status.Conditions[i].Type == ConditionTypeApplied {
				latest.Status.Conditions[i] = appliedCondition
				foundApplied = true
				break
			}
		}
		if !foundApplied {
			latest.Status.Conditions = append(latest.Status.Conditions, appliedCondition)
		}

		// Update status subresource
		return r.Status().Update(ctx, latest)
	})

	if err != nil {
		logger.Error(err, "Failed to update LynqForm status after retries")
	}
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
