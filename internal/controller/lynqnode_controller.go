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
	"encoding/json"
	errorsStd "errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/client-go/tools/record"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/apply"
	"github.com/k8s-lynq/lynq/internal/graph"
	"github.com/k8s-lynq/lynq/internal/metrics"
	"github.com/k8s-lynq/lynq/internal/readiness"
	"github.com/k8s-lynq/lynq/internal/status"
	"github.com/k8s-lynq/lynq/internal/template"
)

// LynqNodeReconciler reconciles a LynqNode object
type LynqNodeReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	StatusManager *status.Manager
}

const (
	// Annotation key for tracking Once creation policy
	AnnotationCreatedOnce = "lynq.sh/created-once"
	// Annotation value for created resources
	AnnotationValueTrue = "true"

	// Finalizer for node cleanup
	LynqNodeFinalizer = "lynqnode.operator.lynq.sh/finalizer"

	// Condition types
	ConditionTypeReady       = "Ready"
	ConditionTypeProgressing = "Progressing"
	ConditionTypeConflicted  = "Conflicted"
	ConditionTypeDegraded    = "Degraded"

	// Resource formatting
	NoResourcesMessage = "no resources"

	// Ready reasons
	ReasonResourcesFailedAndConflicted = "ResourcesFailedAndConflicted"
	ReasonResourcesConflicted          = "ResourcesConflicted"
	ReasonResourcesFailed              = "ResourcesFailed"
	ReasonNotAllResourcesReady         = "NotAllResourcesReady"

	// Degraded reasons
	ReasonResourceFailuresAndConflicts = "ResourceFailuresAndConflicts"
	ReasonResourceFailures             = "ResourceFailures"
	ReasonResourceConflicts            = "ResourceConflicts"
	ReasonResourcesNotReady            = "ResourcesNotReady"

	// Reconcile results
	ResultSuccess        = "success"
	ResultPartialFailure = "partial_failure"
)

// ReconcileType defines the type of reconciliation to perform
type ReconcileType int

const (
	ReconcileTypeUnknown ReconcileType = iota
	ReconcileTypeInit                  // Finalizer needs to be added
	ReconcileTypeCleanup               // Handle deletion
	ReconcileTypeSpec                  // Spec changed (full reconcile with apply)
	ReconcileTypeStatus                // Status-only (fast path, no apply)
)

// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqnodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqnodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqnodes/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqforms,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.lynq.sh,resources=lynqhubs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts;services;configmaps;secrets;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets;daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// NOTE: Cross-namespace resource support requires cluster-wide permissions for resource types
// The above RBAC rules allow the operator to create resources in any namespace when targetNamespace is specified

// Reconcile applies all resources for a node
func (r *LynqNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	startTime := time.Now()

	// Fetch LynqNode
	node := &lynqv1.LynqNode{}
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get LynqNode")
		metrics.LynqNodeReconcileDuration.WithLabelValues("error").Observe(time.Since(startTime).Seconds())
		return ctrl.Result{}, err
	}

	// Determine reconcile type and use appropriate reconciliation path
	reconcileType := r.determineReconcileType(node)

	switch reconcileType {
	case ReconcileTypeCleanup:
		// LynqNode being deleted - handle cleanup
		return r.reconcileCleanup(ctx, node, startTime)

	case ReconcileTypeInit:
		// First time setup - add finalizer
		return r.reconcileInit(ctx, node)

	case ReconcileTypeStatus:
		// Only status changed - fast path, no apply
		logger.V(1).Info("Using fast status reconcile path", "node", node.Name, "generation", node.Generation, "observedGeneration", node.Status.ObservedGeneration)
		return r.reconcileStatus(ctx, node, startTime)

	case ReconcileTypeSpec:
		// Spec changed or template updated - full reconcile with apply
		logger.V(1).Info("Using full reconcile path", "node", node.Name, "generation", node.Generation, "observedGeneration", node.Status.ObservedGeneration)
		return r.reconcileSpec(ctx, node, startTime)

	default:
		logger.Error(nil, "Unknown reconcile type", "type", reconcileType)
		metrics.LynqNodeReconcileDuration.WithLabelValues("error").Observe(time.Since(startTime).Seconds())
		return ctrl.Result{}, fmt.Errorf("unknown reconcile type: %v", reconcileType)
	}
}

// applyResources applies all resources and returns counts for ready, failed, changed, conflicted, and skipped resources
// skippedIds contains the IDs of resources that were skipped due to dependency failures
func (r *LynqNodeReconciler) applyResources(ctx context.Context, node *lynqv1.LynqNode, sortedNodes []*graph.Node, vars template.Variables) (readyCount, failedCount, changedCount, conflictedCount, skippedCount int32, skippedIds []string) {
	logger := log.FromContext(ctx)
	applier := apply.NewApplier(r.Client, r.Scheme)
	checker := readiness.NewChecker(r.Client)
	templateEngine := template.NewEngine()

	totalResources := int32(len(sortedNodes))
	progressingSet := false
	templateAppliedEventEmitted := false

	// Track failed resource IDs to skip dependent resources (actual failures)
	failedResourceIds := make(map[string]bool)
	// Track not-ready resource IDs to block dependent resources (still progressing, not failed)
	notReadyResourceIds := make(map[string]bool)

	for _, graphNode := range sortedNodes {
		resource := graphNode.Resource

		// Check if any dependency has failed (actual failure)
		var failedDepId string
		for _, depId := range graphNode.DependsOn {
			if failedResourceIds[depId] {
				failedDepId = depId
				break
			}
		}

		// Check if any dependency is not ready yet (still progressing)
		var notReadyDepId string
		if failedDepId == "" {
			for _, depId := range graphNode.DependsOn {
				if notReadyResourceIds[depId] {
					notReadyDepId = depId
					break
				}
			}
		}

		// If dependency failed, check skipOnDependencyFailure flag
		if failedDepId != "" {
			// Default is true (skip when dependency fails)
			skipOnFailure := resource.SkipOnDependencyFailure == nil || *resource.SkipOnDependencyFailure

			if skipOnFailure {
				// Mark as failed so dependents of this resource will also be skipped
				failedResourceIds[resource.ID] = true
				skippedCount++
				skippedIds = append(skippedIds, resource.ID)

				logger.Info("Skipping resource due to failed dependency",
					"id", resource.ID,
					"failedDependency", failedDepId,
					"skipOnDependencyFailure", skipOnFailure)

				r.Recorder.Eventf(node, corev1.EventTypeWarning, "DependencySkipped",
					"Resource '%s' skipped because dependency '%s' failed. Set skipOnDependencyFailure=false to create anyway.",
					resource.ID, failedDepId)
				continue
			} else {
				// skipOnDependencyFailure=false: proceed with creation despite failed dependency
				logger.Info("Creating resource despite failed dependency (skipOnDependencyFailure=false)",
					"id", resource.ID,
					"failedDependency", failedDepId)

				r.Recorder.Eventf(node, corev1.EventTypeWarning, "DependencyFailedButProceeding",
					"Dependency '%s' failed, but creating resource '%s' anyway (skipOnDependencyFailure=false)",
					failedDepId, resource.ID)
				// Don't continue - proceed with creation
			}
		}

		// If dependency is not ready yet, block this resource silently (no skip event)
		// This ensures proper ordering: dependents wait for dependencies to be ready
		// Unlike failed dependencies, this doesn't trigger skipOnDependencyFailure logic
		if notReadyDepId != "" {
			// Mark as not-ready so dependents of this resource will also be blocked
			notReadyResourceIds[resource.ID] = true

			logger.V(1).Info("Blocking resource until dependency is ready",
				"id", resource.ID,
				"notReadyDependency", notReadyDepId)
			continue
		}

		// Check if node is being deleted before processing each resource
		// This allows quick exit when node is deleted during reconciliation
		currentLynqNode := &lynqv1.LynqNode{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(node), currentLynqNode); err != nil {
			if errors.IsNotFound(err) {
				// LynqNode was deleted, stop processing
				logger.Info("LynqNode deleted during reconciliation, stopping resource application")
				return readyCount, failedCount, changedCount, conflictedCount, skippedCount, skippedIds
			}
			// Continue on other errors
		} else if !currentLynqNode.DeletionTimestamp.IsZero() {
			// LynqNode is being deleted, stop processing immediately
			logger.Info("LynqNode deletion in progress, stopping resource application",
				"node", node.Name,
				"processedResources", readyCount+failedCount)
			return readyCount, failedCount, changedCount, conflictedCount, skippedCount, skippedIds
		}

		// Render templates
		obj, err := r.renderResource(ctx, templateEngine, resource, vars, node)
		if err != nil {
			logger.Error(err, "Failed to render resource", "id", resource.ID)
			r.Recorder.Eventf(node, corev1.EventTypeWarning, "TemplateRenderError",
				"Failed to render resource %s: %v", resource.ID, err)
			failedResourceIds[resource.ID] = true
			failedCount++
			continue
		}

		// Handle CreationPolicy.Once
		if resource.CreationPolicy == lynqv1.CreationPolicyOnce {
			// Check if resource already exists and has the "created-once" annotation
			exists, hasAnnotation, err := r.checkOnceCreated(ctx, obj)
			if err != nil {
				logger.Error(err, "Failed to check Once policy", "id", resource.ID)
				failedResourceIds[resource.ID] = true
				failedCount++
				continue
			}

			if exists && hasAnnotation {
				// Resource already created with Once policy, skip
				logger.V(1).Info("Skipping resource (CreationPolicy=Once, already created)", "id", resource.ID, "name", obj.GetName())
				readyCount++ // Count as ready since it exists
				continue
			}

			// Add annotation to track that this was created with Once policy
			annotations := obj.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			annotations[AnnotationCreatedOnce] = AnnotationValueTrue
			obj.SetAnnotations(annotations)
		}

		// Apply resource with specified patch strategy and track changes
		// Pass deletionPolicy to prevent ownerReference for Retain policy resources
		// ignoreFields are handled inside ApplyResource to avoid duplicate API calls
		deletionPolicy := resource.DeletionPolicy
		if deletionPolicy == "" {
			deletionPolicy = lynqv1.DeletionPolicyDelete // Default
		}

		// Pass ignoreFields to ApplyResource
		// Only effective for WhenNeeded policy; Once policy ignores this parameter
		ignoreFields := resource.IgnoreFields
		if resource.CreationPolicy == lynqv1.CreationPolicyOnce {
			// For Once policy, ignoreFields has no effect
			ignoreFields = nil
		}

		changed, applyErr := applier.ApplyResource(ctx, obj, node, resource.ConflictPolicy, resource.PatchStrategy, deletionPolicy, ignoreFields)

		// Track changes and emit events on first change
		if changed {
			changedCount++

			// On first change, update Progressing condition and emit event
			if !progressingSet {
				r.StatusManager.PublishProgressingCondition(node, true, "Reconciling", "Reconciling changed resources")
				progressingSet = true

				// Emit detailed template applied event on first resource change
				if !templateAppliedEventEmitted {
					r.emitTemplateAppliedEvent(ctx, node, totalResources)
					templateAppliedEventEmitted = true
				}
			}
		}

		// Record apply metrics
		kind := obj.GetKind()
		if kind == "" {
			kind = "Unknown"
		}
		applyResult := "success"
		if applyErr != nil {
			applyResult = "error"
		}
		metrics.ApplyAttemptsTotal.WithLabelValues(kind, applyResult, string(resource.ConflictPolicy)).Inc()

		if applyErr != nil {
			logger.Error(applyErr, "Failed to apply resource", "id", resource.ID)

			// Check if this is a ConflictError
			var conflictErr *apply.ConflictError
			if errorsStd.As(applyErr, &conflictErr) {
				// Resource conflict detected
				conflictedCount++
				// Increment conflict counter metric
				metrics.LynqNodeConflictsTotal.WithLabelValues(node.Name, node.Namespace, kind, string(resource.ConflictPolicy)).Inc()
				r.Recorder.Eventf(node, corev1.EventTypeWarning, "ResourceConflict",
					"Resource conflict detected for %s/%s (Kind: %s, Policy: %s). "+
						"Another controller or user may be managing this resource. "+
						"Consider using ConflictPolicy=Force to take ownership or resolve the conflict manually. Error: %v",
					conflictErr.Namespace, conflictErr.ResourceName, conflictErr.Kind, resource.ConflictPolicy, conflictErr.Err)
			} else {
				// Other apply error
				r.Recorder.Eventf(node, corev1.EventTypeWarning, "ApplyFailed",
					"Failed to apply resource %s: %v", resource.ID, applyErr)
			}

			failedResourceIds[resource.ID] = true
			failedCount++
			continue
		}

		// Check readiness immediately after apply (non-blocking)
		// Fast status reconcile will continue checking every 30 seconds
		if resource.WaitForReady != nil && *resource.WaitForReady {
			// Get current state from cluster to check readiness
			current := &unstructured.Unstructured{}
			current.SetGroupVersionKind(obj.GroupVersionKind())
			err := r.Get(ctx, client.ObjectKey{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			}, current)
			if err != nil {
				logger.Error(err, "Failed to get resource for readiness check", "id", resource.ID, "name", obj.GetName())
				failedResourceIds[resource.ID] = true
				failedCount++
				continue
			}

			// Check if ready NOW (non-blocking check)
			if checker.IsReady(current) {
				logger.V(1).Info("Resource is ready", "id", resource.ID, "name", obj.GetName())
				readyCount++
			} else {
				// Not ready yet - check if timeout has expired
				timeoutSeconds := resource.TimeoutSeconds
				if timeoutSeconds <= 0 {
					timeoutSeconds = 300 // Default 5 minutes
				}

				// Check resource creation time to determine if timeout expired
				creationTime := current.GetCreationTimestamp().Time
				elapsed := time.Since(creationTime)
				timeoutDuration := time.Duration(timeoutSeconds) * time.Second

				if elapsed >= timeoutDuration {
					// Timeout expired - mark as FAILED (triggers DependencySkipped)
					logger.Info("Resource not ready after timeout, marking as failed",
						"id", resource.ID, "name", obj.GetName(),
						"elapsed", elapsed.String(), "timeout", timeoutDuration.String())
					r.Recorder.Eventf(node, corev1.EventTypeWarning, "ReadinessTimeout",
						"Resource '%s' not ready after %s (timeout: %s)", resource.ID, elapsed.Round(time.Second), timeoutDuration)
					failedResourceIds[resource.ID] = true
					failedCount++
				} else {
					// Still within timeout - mark as "not ready" to block dependents silently
					logger.V(1).Info("Resource not ready yet, will check again in next reconcile",
						"id", resource.ID, "name", obj.GetName(),
						"elapsed", elapsed.String(), "timeout", timeoutDuration.String())
					// Mark as "not ready" to block dependents, but NOT as failed
					// This ensures proper ordering without triggering DependencySkipped events
					notReadyResourceIds[resource.ID] = true
				}
			}
		} else {
			// No readiness check required, count as ready
			readyCount++
		}
	}

	return readyCount, failedCount, changedCount, conflictedCount, skippedCount, skippedIds
}

// emitTemplateAppliedEvent emits a detailed event when template changes are being applied
func (r *LynqNodeReconciler) emitTemplateAppliedEvent(ctx context.Context, node *lynqv1.LynqNode, totalResources int32) {
	logger := log.FromContext(ctx)

	// Get template information from node
	templateName := node.Spec.TemplateRef
	templateGeneration := node.Annotations["lynq.sh/template-generation"]

	// Count resources by type
	resourceCounts := r.countLynqNodeResourcesByType(node)
	resourceDetails := r.formatLynqNodeResourceDetails(resourceCounts)

	// Get registry name from labels
	registryName := node.Labels["lynq.sh/hub"]
	if registryName == "" {
		registryName = "unknown"
	}

	// Emit detailed event
	r.Recorder.Eventf(node, corev1.EventTypeNormal, "TemplateResourcesApplying",
		"Applying resources from LynqForm '%s' (generation: %s). "+
			"Reconciling %d total resources: %s. "+
			"Hub: %s, LynqNode UID: %s, Namespace: %s. "+
			"Resources will be applied in dependency order with readiness checks.",
		templateName, templateGeneration,
		totalResources, resourceDetails,
		registryName, node.Spec.UID, node.Namespace)

	logger.Info("Applying template resources to cluster",
		"node", node.Name,
		"template", templateName,
		"generation", templateGeneration,
		"totalResources", totalResources,
		"hub", registryName)
}

// emitTemplateAppliedCompleteEvent emits a detailed completion event after template resources are applied
func (r *LynqNodeReconciler) emitTemplateAppliedCompleteEvent(ctx context.Context, node *lynqv1.LynqNode, totalResources, readyCount, failedCount, changedCount int32) {
	logger := log.FromContext(ctx)

	// Get template information
	templateName := node.Spec.TemplateRef
	templateGeneration := node.Annotations["lynq.sh/template-generation"]

	// Get registry name from labels
	registryName := node.Labels["lynq.sh/hub"]
	if registryName == "" {
		registryName = "unknown"
	}

	// Determine event type and message based on results
	if failedCount > 0 {
		// Partial failure
		r.Recorder.Eventf(node, corev1.EventTypeWarning, "TemplateAppliedPartial",
			"Applied LynqForm '%s' (generation: %s) with partial success. "+
				"Changed: %d, Ready: %d, Failed: %d out of %d total resources. "+
				"Hub: %s, LynqNode UID: %s. "+
				"Failed resources require attention.",
			templateName, templateGeneration,
			changedCount, readyCount, failedCount, totalResources,
			registryName, node.Spec.UID)

		logger.Error(nil, "Template application completed with failures",
			"node", node.Name,
			"template", templateName,
			"generation", templateGeneration,
			"changed", changedCount,
			"ready", readyCount,
			"failed", failedCount,
			"total", totalResources)
	} else {
		// Success
		r.Recorder.Eventf(node, corev1.EventTypeNormal, "TemplateAppliedSuccess",
			"Successfully applied LynqForm '%s' (generation: %s). "+
				"All %d resources reconciled successfully (%d changed, %d ready). "+
				"Hub: %s, LynqNode UID: %s, Namespace: %s. "+
				"All resources are now in desired state.",
			templateName, templateGeneration,
			totalResources, changedCount, readyCount,
			registryName, node.Spec.UID, node.Namespace)

		logger.Info("Template application completed successfully",
			"node", node.Name,
			"template", templateName,
			"generation", templateGeneration,
			"changed", changedCount,
			"ready", readyCount,
			"total", totalResources)
	}
}

// countLynqNodeResourcesByType counts resources by type in a LynqNode
func (r *LynqNodeReconciler) countLynqNodeResourcesByType(node *lynqv1.LynqNode) map[string]int {
	counts := make(map[string]int)
	spec := &node.Spec

	if len(spec.ServiceAccounts) > 0 {
		counts["ServiceAccounts"] = len(spec.ServiceAccounts)
	}
	if len(spec.Deployments) > 0 {
		counts["Deployments"] = len(spec.Deployments)
	}
	if len(spec.StatefulSets) > 0 {
		counts["StatefulSets"] = len(spec.StatefulSets)
	}
	if len(spec.Services) > 0 {
		counts["Services"] = len(spec.Services)
	}
	if len(spec.Ingresses) > 0 {
		counts["Ingresses"] = len(spec.Ingresses)
	}
	if len(spec.ConfigMaps) > 0 {
		counts["ConfigMaps"] = len(spec.ConfigMaps)
	}
	if len(spec.Secrets) > 0 {
		counts["Secrets"] = len(spec.Secrets)
	}
	if len(spec.PersistentVolumeClaims) > 0 {
		counts["PVCs"] = len(spec.PersistentVolumeClaims)
	}
	if len(spec.Jobs) > 0 {
		counts["Jobs"] = len(spec.Jobs)
	}
	if len(spec.CronJobs) > 0 {
		counts["CronJobs"] = len(spec.CronJobs)
	}
	if len(spec.Manifests) > 0 {
		counts["Manifests"] = len(spec.Manifests)
	}

	return counts
}

// formatLynqNodeResourceDetails formats resource counts into a readable string
func (r *LynqNodeReconciler) formatLynqNodeResourceDetails(counts map[string]int) string {
	var details []string

	if count, ok := counts["ServiceAccounts"]; ok {
		details = append(details, fmt.Sprintf("%d ServiceAccount(s)", count))
	}
	if count, ok := counts["Deployments"]; ok {
		details = append(details, fmt.Sprintf("%d Deployment(s)", count))
	}
	if count, ok := counts["StatefulSets"]; ok {
		details = append(details, fmt.Sprintf("%d StatefulSet(s)", count))
	}
	if count, ok := counts["Services"]; ok {
		details = append(details, fmt.Sprintf("%d Service(s)", count))
	}
	if count, ok := counts["Ingresses"]; ok {
		details = append(details, fmt.Sprintf("%d Ingress(es)", count))
	}
	if count, ok := counts["ConfigMaps"]; ok {
		details = append(details, fmt.Sprintf("%d ConfigMap(s)", count))
	}
	if count, ok := counts["Secrets"]; ok {
		details = append(details, fmt.Sprintf("%d Secret(s)", count))
	}
	if count, ok := counts["PVCs"]; ok {
		details = append(details, fmt.Sprintf("%d PVC(s)", count))
	}
	if count, ok := counts["Jobs"]; ok {
		details = append(details, fmt.Sprintf("%d Job(s)", count))
	}
	if count, ok := counts["CronJobs"]; ok {
		details = append(details, fmt.Sprintf("%d CronJob(s)", count))
	}
	if count, ok := counts["Manifests"]; ok {
		details = append(details, fmt.Sprintf("%d Manifest(s)", count))
	}

	if len(details) == 0 {
		return NoResourcesMessage
	}

	// Join all details with commas
	result := ""
	for i, detail := range details {
		if i > 0 {
			result += ", "
		}
		result += detail
	}
	return result
}

// checkOnceCreated checks if a resource exists and has the "created-once" annotation
func (r *LynqNodeReconciler) checkOnceCreated(ctx context.Context, obj *unstructured.Unstructured) (exists bool, hasAnnotation bool, err error) {
	// Try to get the resource
	current := obj.DeepCopy()
	key := client.ObjectKey{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	err = r.Get(ctx, key, current)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, err
	}

	// Resource exists, check for annotation
	annotations := current.GetAnnotations()
	if annotations != nil && annotations[AnnotationCreatedOnce] == AnnotationValueTrue {
		return true, true, nil
	}

	return true, false, nil
}

// buildTemplateVariablesFromAnnotations builds template variables from LynqNode annotations
func (r *LynqNodeReconciler) buildTemplateVariablesFromAnnotations(node *lynqv1.LynqNode) (template.Variables, error) {
	// Get required values from annotations
	hostOrURL := node.Annotations["lynq.sh/hostOrUrl"]
	if hostOrURL == "" {
		hostOrURL = node.Spec.UID
	}

	activate := node.Annotations["lynq.sh/activate"]
	if activate == "" {
		activate = AnnotationValueTrue
	}

	// Parse extra values from JSON
	extraJSON := node.Annotations["lynq.sh/extra"]
	extraValues := make(map[string]string)
	if extraJSON != "" {
		if err := json.Unmarshal([]byte(extraJSON), &extraValues); err != nil {
			return nil, fmt.Errorf("failed to unmarshal extra values: %w", err)
		}
	}

	vars := template.BuildVariables(node.Spec.UID, hostOrURL, activate, extraValues)

	// Add context variables: hubId and templateRef
	// hubId is from annotation (set by LynqHub controller)
	if hubId := node.Annotations["lynq.sh/hubId"]; hubId != "" {
		vars["hubId"] = hubId
	}
	// templateRef is from spec
	if node.Spec.TemplateRef != "" {
		vars["templateRef"] = node.Spec.TemplateRef
	}

	return vars, nil
}

// collectResourcesFromLynqNode collects all resources from LynqNode.Spec
func (r *LynqNodeReconciler) collectResourcesFromLynqNode(node *lynqv1.LynqNode) []lynqv1.TResource {
	var resources []lynqv1.TResource

	resources = append(resources, node.Spec.ServiceAccounts...)
	resources = append(resources, node.Spec.Deployments...)
	resources = append(resources, node.Spec.StatefulSets...)
	resources = append(resources, node.Spec.DaemonSets...)
	resources = append(resources, node.Spec.Services...)
	resources = append(resources, node.Spec.Ingresses...)
	resources = append(resources, node.Spec.ConfigMaps...)
	resources = append(resources, node.Spec.Secrets...)
	resources = append(resources, node.Spec.PersistentVolumeClaims...)
	resources = append(resources, node.Spec.Jobs...)
	resources = append(resources, node.Spec.CronJobs...)
	resources = append(resources, node.Spec.PodDisruptionBudgets...)
	resources = append(resources, node.Spec.NetworkPolicies...)
	resources = append(resources, node.Spec.HorizontalPodAutoscalers...)
	resources = append(resources, node.Spec.Namespaces...)
	resources = append(resources, node.Spec.Manifests...)

	return resources
}

// renderResource renders a resource template
// Note: NameTemplate, LabelsTemplate, AnnotationsTemplate, TargetNamespace are already rendered by Hub controller
// We only need to render the spec (unstructured.Unstructured) contents which may contain template variables
func (r *LynqNodeReconciler) renderResource(ctx context.Context, engine *template.Engine, resource lynqv1.TResource, vars template.Variables, node *lynqv1.LynqNode) (*unstructured.Unstructured, error) {
	// Get spec (already an unstructured.Unstructured)
	obj := resource.Spec.DeepCopy()

	// Set metadata (use already-rendered values from resource)
	if resource.NameTemplate != "" {
		obj.SetName(resource.NameTemplate)
	}

	// Set namespace: use TargetNamespace if specified, otherwise use LynqNode CR's namespace
	targetNamespace := node.Namespace
	if resource.TargetNamespace != "" {
		targetNamespace = resource.TargetNamespace
	}
	obj.SetNamespace(targetNamespace)

	// Set labels
	labels := resource.LabelsTemplate
	if labels == nil {
		labels = make(map[string]string)
	}

	// For cross-namespace resources or Namespaces, add tracking labels
	// since they cannot have ownerReferences
	isCrossNamespace := targetNamespace != node.Namespace
	isNamespaceResource := obj.GetKind() == "Namespace"
	if isCrossNamespace || isNamespaceResource {
		labels["lynq.sh/node"] = node.Name
		labels["lynq.sh/node-namespace"] = node.Namespace
	}

	if len(labels) > 0 {
		obj.SetLabels(labels)
	}

	// Set annotations (including DeletionPolicy for orphan cleanup)
	annotations := resource.AnnotationsTemplate
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Add DeletionPolicy annotation to enable correct orphan cleanup
	// This is critical because orphaned resources no longer exist in the template
	deletionPolicy := string(resource.DeletionPolicy)
	if deletionPolicy == "" {
		deletionPolicy = string(lynqv1.DeletionPolicyDelete) // Default
	}
	annotations[apply.AnnotationDeletionPolicy] = deletionPolicy

	if len(annotations) > 0 {
		obj.SetAnnotations(annotations)
	}

	// Render spec recursively (for template variables inside the unstructured object)
	renderedSpec, err := r.renderUnstructured(ctx, obj.Object, engine, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render spec: %w", err)
	}
	obj.Object = renderedSpec

	return obj, nil
}

// renderUnstructured recursively renders template variables in unstructured data
// Returns an error if any template rendering fails (e.g., missing variable references)
//
//nolint:unparam // ctx is passed through for recursive calls
func (r *LynqNodeReconciler) renderUnstructured(ctx context.Context, data map[string]interface{}, engine *template.Engine, vars template.Variables) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for k, v := range data {
		switch val := v.(type) {
		case string:
			// Try to render as template
			rendered, err := engine.Render(val, vars)
			if err != nil {
				// Return error to mark resource as failed (e.g., missing variable reference)
				return nil, fmt.Errorf("template rendering failed for field %q: %w", k, err)
			}
			result[k] = rendered
		case map[string]interface{}:
			// Recurse into nested maps
			rendered, err := r.renderUnstructured(ctx, val, engine, vars)
			if err != nil {
				return nil, fmt.Errorf("template rendering failed in nested field %q: %w", k, err)
			}
			result[k] = rendered
		case []interface{}:
			// Recurse into arrays
			renderedArray := make([]interface{}, len(val))
			for i, item := range val {
				if itemMap, ok := item.(map[string]interface{}); ok {
					rendered, err := r.renderUnstructured(ctx, itemMap, engine, vars)
					if err != nil {
						return nil, fmt.Errorf("template rendering failed for array item %q[%d]: %w", k, i, err)
					}
					renderedArray[i] = rendered
				} else if itemStr, ok := item.(string); ok {
					rendered, err := engine.Render(itemStr, vars)
					if err != nil {
						return nil, fmt.Errorf("template rendering failed for array string %q[%d]: %w", k, i, err)
					}
					renderedArray[i] = rendered
				} else {
					renderedArray[i] = item
				}
			}
			result[k] = renderedArray
		default:
			result[k] = v
		}
	}

	return result, nil
}

// LynqNodeStatusUpdate contains all calculated status fields for a LynqNode
// This structure consolidates status calculation logic for better testability and maintainability
type LynqNodeStatusUpdate struct {
	// Resource counts
	DesiredResources    int32
	ReadyResources      int32
	FailedResources     int32
	ConflictedResources int32

	// Applied resource tracking
	AppliedResources []string

	// Computed conditions
	Conditions []metav1.Condition

	// Flags for internal use
	IsReady    bool
	IsDegraded bool
}

// calculateLynqNodeStatus computes all LynqNode status fields based on resource counts and applied resources.
// This centralizes all status calculation logic for better testability and maintainability.
//
// Parameters:
//   - readyCount: Number of resources that are ready
//   - failedCount: Number of resources that failed
//   - conflictedCount: Number of resources in conflict
//   - totalResources: Total number of desired resources
//   - appliedResourceKeys: Keys of successfully applied resources
//   - isProgressing: Whether reconciliation is currently in progress
//
// Returns:
//   - *LynqNodeStatusUpdate: Complete status update with all fields calculated
func (r *LynqNodeReconciler) calculateLynqNodeStatus(
	readyCount, failedCount, conflictedCount, totalResources int32,
	appliedResourceKeys []string,
	isProgressing bool,
) *LynqNodeStatusUpdate {
	// Initialize status update
	update := &LynqNodeStatusUpdate{
		DesiredResources:    totalResources,
		ReadyResources:      readyCount,
		FailedResources:     failedCount,
		ConflictedResources: conflictedCount,
		AppliedResources:    appliedResourceKeys,
		Conditions:          []metav1.Condition{},
	}

	// Calculate overall health flags
	hasConflict := conflictedCount > 0
	isFullyReady := failedCount == 0 && conflictedCount == 0 && readyCount == totalResources
	isDegraded := failedCount > 0 || hasConflict

	update.IsReady = isFullyReady
	update.IsDegraded = isDegraded || (readyCount != totalResources)

	// 1. Ready Condition
	readyCond := metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "Successfully reconciled all resources",
		LastTransitionTime: metav1.Now(),
	}
	if !isFullyReady {
		readyCond.Status = metav1.ConditionFalse
		// Prioritize conflict and failure reasons for better visibility
		if failedCount > 0 && conflictedCount > 0 {
			readyCond.Reason = ReasonResourcesFailedAndConflicted
			readyCond.Message = fmt.Sprintf("%d resources failed and %d resources in conflict", failedCount, conflictedCount)
		} else if conflictedCount > 0 {
			readyCond.Reason = ReasonResourcesConflicted
			readyCond.Message = fmt.Sprintf("%d resources in conflict", conflictedCount)
		} else if failedCount > 0 {
			readyCond.Reason = ReasonResourcesFailed
			readyCond.Message = fmt.Sprintf("%d resources failed", failedCount)
		} else if readyCount != totalResources {
			readyCond.Reason = ReasonNotAllResourcesReady
			readyCond.Message = fmt.Sprintf("Not all resources are ready: %d/%d ready", readyCount, totalResources)
		}
	}

	// 2. Progressing Condition
	progressingCond := metav1.Condition{
		Type:               ConditionTypeProgressing,
		Status:             metav1.ConditionFalse,
		Reason:             "ReconcileComplete",
		Message:            "Reconciliation completed",
		LastTransitionTime: metav1.Now(),
	}
	if isProgressing {
		progressingCond.Status = metav1.ConditionTrue
		progressingCond.Reason = "Reconciling"
		progressingCond.Message = "Reconciling changed resources"
	}

	// 3. Conflicted Condition
	conflictedCond := metav1.Condition{
		Type:               ConditionTypeConflicted,
		Status:             metav1.ConditionFalse,
		Reason:             "NoConflict",
		Message:            "No resource conflicts detected",
		LastTransitionTime: metav1.Now(),
	}
	if hasConflict {
		conflictedCond.Status = metav1.ConditionTrue
		conflictedCond.Reason = "ResourceConflict"
		conflictedCond.Message = "One or more resources are in conflict. Check events for details."
	}

	// 4. Degraded Condition
	degradedCond := metav1.Condition{
		Type:               ConditionTypeDegraded,
		Status:             metav1.ConditionFalse,
		Reason:             "Healthy",
		Message:            "All resources are healthy",
		LastTransitionTime: metav1.Now(),
	}

	// Determine degraded state (includes Ready != Desired check)
	isDegradedForCondition := isDegraded || (readyCount != totalResources)
	if isDegradedForCondition {
		degradedCond.Status = metav1.ConditionTrue
		if failedCount > 0 && hasConflict {
			degradedCond.Reason = ReasonResourceFailuresAndConflicts
			degradedCond.Message = fmt.Sprintf("LynqNode has %d failed and %d conflicted resources", failedCount, conflictedCount)
		} else if failedCount > 0 {
			degradedCond.Reason = ReasonResourceFailures
			degradedCond.Message = fmt.Sprintf("LynqNode has %d failed resources", failedCount)
		} else if hasConflict {
			degradedCond.Reason = ReasonResourceConflicts
			degradedCond.Message = fmt.Sprintf("LynqNode has %d conflicted resources", conflictedCount)
		} else if readyCount != totalResources {
			degradedCond.Reason = ReasonResourcesNotReady
			degradedCond.Message = fmt.Sprintf("Not all resources are ready: %d/%d ready", readyCount, totalResources)
		}
	}

	// Assemble all conditions
	update.Conditions = []metav1.Condition{
		readyCond,
		progressingCond,
		conflictedCond,
		degradedCond,
	}

	return update
}

// cleanupLynqNodeResources handles resource cleanup according to DeletionPolicy
// This function uses best-effort approach: it tries to clean up all resources but won't block deletion
// if some resources fail to clean up. Resources with ownerReferences will be garbage collected by Kubernetes.
func (r *LynqNodeReconciler) cleanupLynqNodeResources(ctx context.Context, node *lynqv1.LynqNode) error {
	logger := log.FromContext(ctx)
	logger.Info("Starting node resource cleanup", "node", node.Name)

	applier := apply.NewApplier(r.Client, r.Scheme)
	templateEngine := template.NewEngine()

	// Build template variables from annotations
	vars, err := r.buildTemplateVariablesFromAnnotations(node)
	if err != nil {
		logger.Error(err, "Failed to build template variables for cleanup, using empty variables")
		// Continue with cleanup even if variables fail
		vars = template.Variables{}
	}

	// Collect all resources
	allResources := r.collectResourcesFromLynqNode(node)
	logger.Info("Collected resources for cleanup", "count", len(allResources))

	// Track cleanup statistics
	var cleanupErrors []string
	successCount := 0
	failedCount := 0
	retainedCount := 0

	// Process each resource according to its DeletionPolicy
	for i, res := range allResources {
		// Check context cancellation (timeout) before processing each resource
		if ctx.Err() != nil {
			logger.Info("Cleanup context cancelled, stopping cleanup",
				"processed", i,
				"total", len(allResources),
				"reason", ctx.Err())
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("cleanup timed out after processing %d/%d resources", i, len(allResources)))
			// Exit loop immediately on timeout
			break
		}

		// Render resource to get actual name/namespace
		rendered, err := r.renderResource(ctx, templateEngine, res, vars, node)
		if err != nil {
			logger.Error(err, "Failed to render resource for cleanup, skipping",
				"resource", res.ID,
				"kind", res.Spec.GetKind())
			failedCount++
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("render failed for %s: %v", res.ID, err))
			// Continue with other resources
			continue
		}

		resourceName := rendered.GetName()
		resourceKind := rendered.GetKind()
		orphanReason := "LynqNodeDeleted"

		// Apply deletion policy
		switch res.DeletionPolicy {
		case lynqv1.DeletionPolicyRetain:
			// Remove ownerReferences but keep resource
			logger.Info("Retaining resource (removing ownerReferences and adding orphan labels)",
				"resource", resourceName,
				"kind", resourceKind,
				"namespace", rendered.GetNamespace())

			if err := applier.DeleteResource(ctx, rendered, lynqv1.DeletionPolicyRetain, orphanReason); err != nil {
				logger.Error(err, "Failed to retain resource, continuing",
					"resource", resourceName,
					"kind", resourceKind)
				failedCount++
				cleanupErrors = append(cleanupErrors, fmt.Sprintf("retain failed for %s/%s: %v", resourceKind, resourceName, err))
				// Continue with other resources
			} else {
				retainedCount++
				r.Recorder.Eventf(node, corev1.EventTypeNormal, "ResourceRetained",
					"Resource %s/%s retained with orphan labels (ownerReferences removed)", resourceKind, resourceName)
			}

		case lynqv1.DeletionPolicyDelete, "":
			// Delete resource (default behavior)
			// Most resources with ownerReferences will be garbage collected automatically
			logger.V(1).Info("Processing resource deletion",
				"resource", resourceName,
				"kind", resourceKind,
				"namespace", rendered.GetNamespace())

			if err := applier.DeleteResource(ctx, rendered, lynqv1.DeletionPolicyDelete, orphanReason); err != nil {
				// Not a fatal error - ownerReferences will handle cleanup
				logger.V(1).Info("Resource deletion delegated to ownerReference garbage collection",
					"resource", resourceName,
					"kind", resourceKind,
					"error", err.Error())
				// Don't count as failure since GC will handle it
			}
			successCount++
		}
	}

	// Log cleanup summary
	logger.Info("LynqNode resource cleanup completed",
		"node", node.Name,
		"total", len(allResources),
		"successful", successCount,
		"retained", retainedCount,
		"failed", failedCount)

	// Return error only if there were significant failures
	// This allows cleanup to proceed even with partial failures
	if len(cleanupErrors) > 0 {
		logger.Info("Cleanup completed with some errors", "errorCount", len(cleanupErrors))
		// Return first few errors for visibility
		maxErrors := 3
		if len(cleanupErrors) > maxErrors {
			return fmt.Errorf("cleanup had %d errors, first %d: %v", len(cleanupErrors), maxErrors, cleanupErrors[:maxErrors])
		}
		return fmt.Errorf("cleanup had errors: %v", cleanupErrors)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LynqNodeReconciler) SetupWithManager(mgr ctrl.Manager, concurrency int) error {
	// Create smart predicates that react to both spec AND status changes
	// This enables real-time status propagation from child resources
	ownedResourcePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj := e.ObjectOld
			newObj := e.ObjectNew

			// Always reconcile on generation change (spec change)
			if oldObj.GetGeneration() != newObj.GetGeneration() {
				return true
			}

			// Always reconcile on annotation change
			if !reflect.DeepEqual(oldObj.GetAnnotations(), newObj.GetAnnotations()) {
				return true
			}

			// Reconcile on status change for specific resource types
			// This enables real-time status propagation
			switch obj := newObj.(type) {
			case *appsv1.Deployment:
				oldDep := e.ObjectOld.(*appsv1.Deployment)
				// Check if ready replicas or conditions changed
				if obj.Status.ReadyReplicas != oldDep.Status.ReadyReplicas ||
					obj.Status.AvailableReplicas != oldDep.Status.AvailableReplicas ||
					!reflect.DeepEqual(obj.Status.Conditions, oldDep.Status.Conditions) {
					return true
				}
			case *appsv1.StatefulSet:
				oldSts := e.ObjectOld.(*appsv1.StatefulSet)
				if obj.Status.ReadyReplicas != oldSts.Status.ReadyReplicas ||
					obj.Status.CurrentReplicas != oldSts.Status.CurrentReplicas ||
					!reflect.DeepEqual(obj.Status.Conditions, oldSts.Status.Conditions) {
					return true
				}
			case *appsv1.DaemonSet:
				oldDs := e.ObjectOld.(*appsv1.DaemonSet)
				if obj.Status.NumberReady != oldDs.Status.NumberReady ||
					obj.Status.DesiredNumberScheduled != oldDs.Status.DesiredNumberScheduled ||
					!reflect.DeepEqual(obj.Status.Conditions, oldDs.Status.Conditions) {
					return true
				}
			case *batchv1.Job:
				oldJob := e.ObjectOld.(*batchv1.Job)
				if obj.Status.Succeeded != oldJob.Status.Succeeded ||
					obj.Status.Failed != oldJob.Status.Failed ||
					!reflect.DeepEqual(obj.Status.Conditions, oldJob.Status.Conditions) {
					return true
				}
			case *networkingv1.Ingress:
				oldIng := e.ObjectOld.(*networkingv1.Ingress)
				if !reflect.DeepEqual(obj.Status.LoadBalancer, oldIng.Status.LoadBalancer) {
					return true
				}
			case *autoscalingv2.HorizontalPodAutoscaler:
				oldHPA := e.ObjectOld.(*autoscalingv2.HorizontalPodAutoscaler)
				if obj.Status.CurrentReplicas != oldHPA.Status.CurrentReplicas ||
					obj.Status.DesiredReplicas != oldHPA.Status.DesiredReplicas ||
					!reflect.DeepEqual(obj.Status.Conditions, oldHPA.Status.Conditions) {
					return true
				}
			}

			// Don't reconcile for other status-only changes
			return false
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&lynqv1.LynqNode{}).
		Named("lynqnode").
		// Watch owned resources for drift detection with predicates (same-namespace with ownerReference)
		// When these resources are modified, the parent LynqNode will be reconciled
		// Predicates ensure we only react to meaningful changes (generation/annotations)
		Owns(&corev1.ServiceAccount{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&corev1.Service{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&corev1.ConfigMap{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&corev1.Secret{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&corev1.PersistentVolumeClaim{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&appsv1.StatefulSet{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&appsv1.DaemonSet{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&batchv1.Job{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&batchv1.CronJob{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&networkingv1.Ingress{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&policyv1.PodDisruptionBudget{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&networkingv1.NetworkPolicy{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}, builder.WithPredicates(ownedResourcePredicate)).
		// Watch resources with label-based tracking (cross-namespace or resources without ownerReference support)
		// These use labels for tracking: lynq.sh/node and lynq.sh/node-namespace
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&corev1.ServiceAccount{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&corev1.PersistentVolumeClaim{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&appsv1.StatefulSet{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&appsv1.DaemonSet{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&batchv1.Job{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&batchv1.CronJob{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&networkingv1.Ingress{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&policyv1.PodDisruptionBudget{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&networkingv1.NetworkPolicy{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		Watches(
			&autoscalingv2.HorizontalPodAutoscaler{},
			handler.EnqueueRequestsFromMapFunc(r.findNodeForLabeledResource),
			builder.WithPredicates(ownedResourcePredicate),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: concurrency,
		}).
		Complete(r)
}

// findNodeForLabeledResource maps any resource to its LynqNode using tracking labels
// This supports cross-namespace resources and resources without ownerReference support (like Namespaces)
func (r *LynqNodeReconciler) findNodeForLabeledResource(ctx context.Context, obj client.Object) []ctrl.Request {
	// Check if this resource has our tracking labels
	labels := obj.GetLabels()
	if labels == nil {
		return nil
	}

	nodeName := labels["lynq.sh/node"]
	nodeNamespace := labels["lynq.sh/node-namespace"]

	if nodeName == "" || nodeNamespace == "" {
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{
				Name:      nodeName,
				Namespace: nodeNamespace,
			},
		},
	}
}

// buildResourceKey generates a unique key for a resource
// Format: "kind/namespace/name@id"
// Example: "Deployment/default/myapp@app-deployment"
func buildResourceKey(obj *unstructured.Unstructured, resourceID string) string {
	kind := obj.GetKind()
	namespace := obj.GetNamespace()
	name := obj.GetName()
	return fmt.Sprintf("%s/%s/%s@%s", kind, namespace, name, resourceID)
}

// parseResourceKey parses a resource key into its components
// Returns: kind, namespace, name, resourceID, error
func parseResourceKey(key string) (string, string, string, string, error) {
	// Split by '@' first to separate resource ID
	parts := strings.Split(key, "@")
	if len(parts) != 2 {
		return "", "", "", "", fmt.Errorf("invalid resource key format: %s (expected format: kind/namespace/name@id)", key)
	}
	resourceID := parts[1]

	// Split the first part by '/' to get kind/namespace/name
	resourceParts := strings.Split(parts[0], "/")
	if len(resourceParts) != 3 {
		return "", "", "", "", fmt.Errorf("invalid resource key format: %s (expected format: kind/namespace/name@id)", key)
	}

	kind := resourceParts[0]
	namespace := resourceParts[1]
	name := resourceParts[2]

	return kind, namespace, name, resourceID, nil
}

// buildAppliedResourceKeys builds a set of resource keys from current LynqNode.Spec
func (r *LynqNodeReconciler) buildAppliedResourceKeys(ctx context.Context, node *lynqv1.LynqNode) (map[string]bool, error) {
	keys := make(map[string]bool)
	templateEngine := template.NewEngine()

	// Build template variables
	vars, err := r.buildTemplateVariablesFromAnnotations(node)
	if err != nil {
		return nil, fmt.Errorf("failed to build template variables: %w", err)
	}

	// Collect all resources
	allResources := r.collectResourcesFromLynqNode(node)

	// Render each resource and build key
	for _, res := range allResources {
		rendered, err := r.renderResource(ctx, templateEngine, res, vars, node)
		if err != nil {
			// Skip resources that fail to render (they won't be applied)
			continue
		}

		key := buildResourceKey(rendered, res.ID)
		keys[key] = true
	}

	return keys, nil
}

// findOrphanedResources finds resources that were previously applied but are no longer in the spec
func (r *LynqNodeReconciler) findOrphanedResources(previousKeys []string, currentKeys map[string]bool) []string {
	var orphans []string

	for _, prevKey := range previousKeys {
		if !currentKeys[prevKey] {
			orphans = append(orphans, prevKey)
		}
	}

	return orphans
}

// deleteOrphanedResource deletes a resource identified by its key
func (r *LynqNodeReconciler) deleteOrphanedResource(ctx context.Context, node *lynqv1.LynqNode, key string) error {
	logger := log.FromContext(ctx)

	// Parse the key
	kind, namespace, name, resourceID, err := parseResourceKey(key)
	if err != nil {
		logger.Error(err, "Failed to parse resource key", "key", key)
		return err
	}

	// Get DeletionPolicy from the resource's annotation
	// This is necessary because orphaned resources are no longer in the template
	// We stored DeletionPolicy as an annotation during resource creation
	deletionPolicy := lynqv1.DeletionPolicyDelete // Default

	// Create an unstructured object to represent the resource
	obj := &unstructured.Unstructured{}
	obj.SetKind(kind)
	obj.SetNamespace(namespace)
	obj.SetName(name)

	// Set appropriate API version based on kind
	apiVersion := r.getAPIVersionForKind(kind)
	obj.SetAPIVersion(apiVersion)

	// Try to get the resource to read DeletionPolicy from annotation
	existingObj := obj.DeepCopy()
	if err := r.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, existingObj); err == nil {
		// Resource exists, read DeletionPolicy from annotation
		if annotations := existingObj.GetAnnotations(); annotations != nil {
			if policyStr, ok := annotations[apply.AnnotationDeletionPolicy]; ok {
				deletionPolicy = lynqv1.DeletionPolicy(policyStr)
				logger.V(1).Info("Read DeletionPolicy from resource annotation",
					"resource", name,
					"deletionPolicy", policyStr)
			}
		}
	} else if !errors.IsNotFound(err) {
		logger.Error(err, "Failed to get resource for DeletionPolicy check", "resource", name)
		// Continue with default policy if we can't read the resource
	}

	// Delete or retain the resource based on DeletionPolicy
	applier := apply.NewApplier(r.Client, r.Scheme)
	orphanReason := "RemovedFromTemplate"

	if err := applier.DeleteResource(ctx, obj, deletionPolicy, orphanReason); err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "Failed to handle orphaned resource",
				"key", key,
				"kind", kind,
				"namespace", namespace,
				"name", name,
				"deletionPolicy", deletionPolicy)
			return err
		}
		// Resource already gone, treat as success
	}

	if deletionPolicy == lynqv1.DeletionPolicyRetain {
		logger.Info("Retained orphaned resource with orphan labels",
			"key", key,
			"kind", kind,
			"namespace", namespace,
			"name", name,
			"resourceID", resourceID)
		r.Recorder.Eventf(node, corev1.EventTypeNormal, "OrphanedResourceRetained",
			"Retained orphaned resource %s/%s (ID: %s) - removed from template, marked with orphan labels", kind, name, resourceID)
	} else {
		logger.Info("Deleted orphaned resource",
			"key", key,
			"kind", kind,
			"namespace", namespace,
			"name", name,
			"resourceID", resourceID)
		r.Recorder.Eventf(node, corev1.EventTypeNormal, "OrphanedResourceDeleted",
			"Deleted orphaned resource %s/%s (ID: %s) - removed from template", kind, name, resourceID)
	}

	return nil
}

// getAPIVersionForKind returns the API version for a given kind string
func (r *LynqNodeReconciler) getAPIVersionForKind(kind string) string {
	switch kind {
	case "Namespace", "ServiceAccount", "Service", "ConfigMap", "Secret", "PersistentVolumeClaim":
		return "v1"
	case "Deployment", "StatefulSet", "DaemonSet":
		return "apps/v1"
	case "Job", "CronJob":
		return "batch/v1"
	case "Ingress":
		return "networking.k8s.io/v1"
	default:
		// For unknown kinds, return v1 as default
		return "v1"
	}
}

// determineReconcileType determines what type of reconciliation is needed
func (r *LynqNodeReconciler) determineReconcileType(node *lynqv1.LynqNode) ReconcileType {
	// 1. Check deletion
	if !node.DeletionTimestamp.IsZero() {
		return ReconcileTypeCleanup
	}

	// 2. Check finalizer
	if !controllerutil.ContainsFinalizer(node, LynqNodeFinalizer) {
		return ReconcileTypeInit
	}

	// 3. Check if node is in Degraded state or has failed resources
	// In these cases, we need full reconcile to retry failed resources
	if node.Status.FailedResources > 0 {
		return ReconcileTypeSpec
	}

	// Check for Degraded condition (includes conflicts)
	for _, cond := range node.Status.Conditions {
		if cond.Type == ConditionTypeDegraded && cond.Status == metav1.ConditionTrue {
			return ReconcileTypeSpec
		}
	}

	// 4. Check if this was triggered by owned resource status change
	// We can infer this by checking if the node's generation matches status.observedGeneration
	if node.Generation == node.Status.ObservedGeneration {
		// Generation hasn't changed, likely triggered by child resource status change
		return ReconcileTypeStatus
	}

	// 5. Default to full reconcile for spec changes
	return ReconcileTypeSpec
}

// hasOwnershipConflict checks if a resource has an ownership conflict with the node
// Returns true if the resource is managed by a different controller or has conflicting ownerReferences
func (r *LynqNodeReconciler) hasOwnershipConflict(obj *unstructured.Unstructured, node *lynqv1.LynqNode) bool {
	// Check ownerReferences
	ownerRefs := obj.GetOwnerReferences()
	if len(ownerRefs) == 0 {
		// No owner - check tracking labels for cross-namespace resources
		labels := obj.GetLabels()
		if labels != nil {
			labelLynqNode := labels["lynq.sh/node"]
			labelNamespace := labels["lynq.sh/node-namespace"]

			// If it has our tracking labels, verify they match
			if labelLynqNode != "" || labelNamespace != "" {
				return labelLynqNode != node.Name || labelNamespace != node.Namespace
			}
		}
		// No owner and no tracking labels - not a conflict, just unmanaged
		return false
	}

	// Check if any owner is this node
	for _, ref := range ownerRefs {
		if ref.UID == node.UID {
			return false // We own it
		}
	}

	// Owned by someone else
	return true
}

// reconcileCleanup handles node deletion with finalizer
func (r *LynqNodeReconciler) reconcileCleanup(ctx context.Context, node *lynqv1.LynqNode, startTime time.Time) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(node, LynqNodeFinalizer) {
		logger.Info("LynqNode deletion requested, starting cleanup", "node", node.Name)

		// Create a timeout context for cleanup (30 seconds max)
		cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Perform best-effort cleanup with deletion policies
		if err := r.cleanupLynqNodeResources(cleanupCtx, node); err != nil {
			logger.Error(err, "Cleanup encountered errors (will proceed with deletion)",
				"node", node.Name)
			r.Recorder.Eventf(node, corev1.EventTypeWarning, "CleanupPartialFailure",
				"Some resources could not be cleaned up: %v. Kubernetes garbage collector will handle remaining resources with ownerReferences.", err)
		}

		// ALWAYS remove finalizer after cleanup attempt
		controllerutil.RemoveFinalizer(node, LynqNodeFinalizer)
		if err := r.Update(ctx, node); err != nil {
			logger.Error(err, "Failed to remove finalizer", "node", node.Name)
			return ctrl.Result{}, err
		}

		logger.Info("LynqNode deletion completed, finalizer removed", "node", node.Name)
		r.Recorder.Eventf(node, corev1.EventTypeNormal, "LynqNodeDeleted",
			"LynqNode %s deleted successfully. Resources will be cleaned up by Kubernetes garbage collector.", node.Name)
		metrics.LynqNodeReconcileDuration.WithLabelValues("success").Observe(time.Since(startTime).Seconds())
	}
	return ctrl.Result{}, nil
}

// reconcileInit handles finalizer initialization
func (r *LynqNodeReconciler) reconcileInit(ctx context.Context, node *lynqv1.LynqNode) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	controllerutil.AddFinalizer(node, LynqNodeFinalizer)
	if err := r.Update(ctx, node); err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, err
	}
	logger.Info("Finalizer added to LynqNode", "node", node.Name)
	// Requeue to continue with reconciliation
	return ctrl.Result{Requeue: true}, nil
}

// reconcileSpec handles full reconciliation with resource application
// This is triggered when spec changes or template updates
func (r *LynqNodeReconciler) reconcileSpec(ctx context.Context, node *lynqv1.LynqNode, startTime time.Time) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Running full reconcile with resource application", "node", node.Name)

	// Build template variables from annotations
	vars, err := r.buildTemplateVariablesFromAnnotations(node)
	if err != nil {
		logger.Error(err, "Failed to build template variables")
		r.StatusManager.PublishReadyCondition(node, false, "VariablesBuildError", err.Error())
		r.StatusManager.PublishDegradedCondition(node, true, "VariablesBuildError", err.Error())
		// Publish metrics to ensure degraded status is tracked
		r.StatusManager.PublishMetrics(node, 0, 0, 0, 0, []metav1.Condition{
			{Type: "Degraded", Status: metav1.ConditionTrue, Reason: "VariablesBuildError"},
		}, true, "VariablesBuildError")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Collect all resources from LynqNode.Spec
	allResources := r.collectResourcesFromLynqNode(node)

	// Build dependency graph
	depGraph, err := graph.BuildGraph(allResources)
	if err != nil {
		logger.Error(err, "Failed to build dependency graph")
		r.StatusManager.PublishReadyCondition(node, false, "DependencyError", err.Error())
		r.StatusManager.PublishDegradedCondition(node, true, "DependencyCycle", "Dependency cycle detected in resource graph")
		// Publish metrics to ensure degraded status is tracked
		r.StatusManager.PublishMetrics(node, 0, 0, 0, 0, []metav1.Condition{
			{Type: "Degraded", Status: metav1.ConditionTrue, Reason: "DependencyCycle"},
		}, true, "DependencyCycle")
		return ctrl.Result{}, err
	}

	// Get sorted resources
	sortedNodes, err := depGraph.TopologicalSort()
	if err != nil {
		logger.Error(err, "Failed to sort resources")
		r.StatusManager.PublishReadyCondition(node, false, "SortError", err.Error())
		r.StatusManager.PublishDegradedCondition(node, true, "DependencyCycle", err.Error())
		// Publish metrics to ensure degraded status is tracked
		r.StatusManager.PublishMetrics(node, 0, 0, 0, 0, []metav1.Condition{
			{Type: "Degraded", Status: metav1.ConditionTrue, Reason: "DependencyCycle"},
		}, true, "DependencyCycle")
		return ctrl.Result{}, err
	}

	// Detect and cleanup orphaned resources
	currentKeys, err := r.buildAppliedResourceKeys(ctx, node)
	if err != nil {
		logger.Error(err, "Failed to build applied resource keys")
		currentKeys = make(map[string]bool)
	}

	previousKeys := node.Status.AppliedResources
	orphanedKeys := r.findOrphanedResources(previousKeys, currentKeys)

	if len(orphanedKeys) > 0 {
		logger.Info("Found orphaned resources", "count", len(orphanedKeys))
		for _, orphanKey := range orphanedKeys {
			if err := r.deleteOrphanedResource(ctx, node, orphanKey); err != nil {
				logger.Error(err, "Failed to delete orphaned resource", "key", orphanKey)
			}
		}
	}

	// Apply resources and track changes
	readyCount, failedCount, changedCount, conflictedCount, skippedCount, skippedIds := r.applyResources(ctx, node, sortedNodes, vars)
	totalResources := int32(len(sortedNodes))

	// Build applied resource keys
	appliedResourceKeys := make([]string, 0, len(currentKeys))
	for key := range currentKeys {
		appliedResourceKeys = append(appliedResourceKeys, key)
	}

	// Calculate complete status using centralized logic
	statusUpdate := r.calculateLynqNodeStatus(
		readyCount,
		failedCount,
		conflictedCount,
		totalResources,
		appliedResourceKeys,
		false, // not progressing after reconciliation completes
	)

	// Update ObservedGeneration to mark this generation as reconciled
	// This is critical to prevent repeated full reconciles
	r.StatusManager.PublishObservedGeneration(node, node.Generation)

	// Publish all status fields at once through StatusManager
	r.StatusManager.PublishResourceCounts(node, statusUpdate.ReadyResources, statusUpdate.FailedResources, statusUpdate.DesiredResources, statusUpdate.ConflictedResources)
	r.StatusManager.PublishAppliedResources(node, statusUpdate.AppliedResources)

	// Publish skipped resources (due to dependency failures)
	r.StatusManager.PublishSkippedResources(node, skippedCount, skippedIds)
	for _, cond := range statusUpdate.Conditions {
		switch cond.Type {
		case ConditionTypeReady:
			r.StatusManager.PublishReadyCondition(node, statusUpdate.IsReady, cond.Reason, cond.Message)
		case ConditionTypeProgressing:
			r.StatusManager.PublishProgressingCondition(node, cond.Status == metav1.ConditionTrue, cond.Reason, cond.Message)
		case ConditionTypeConflicted:
			r.StatusManager.PublishConflictedCondition(node, cond.Status == metav1.ConditionTrue)
		case ConditionTypeDegraded:
			r.StatusManager.PublishDegradedCondition(node, statusUpdate.IsDegraded, cond.Reason, cond.Message)
		}
	}

	// Find degraded reason for metrics
	var degradedReason string
	for _, cond := range statusUpdate.Conditions {
		if cond.Type == "Degraded" {
			degradedReason = cond.Reason
			break
		}
	}

	// Publish metrics
	r.StatusManager.PublishMetrics(node, readyCount, failedCount, totalResources, conflictedCount, statusUpdate.Conditions, statusUpdate.IsDegraded, degradedReason)

	// Emit completion event if resources were changed
	if changedCount > 0 {
		r.emitTemplateAppliedCompleteEvent(ctx, node, totalResources, readyCount, failedCount, changedCount)
		logger.Info("Reconciliation completed with changes", "changed", changedCount, "ready", readyCount, "failed", failedCount, "conflicted", conflictedCount)
	} else {
		logger.V(1).Info("Reconciliation completed without changes", "ready", readyCount, "failed", failedCount, "conflicted", conflictedCount)
	}

	// Record metrics
	result := ResultSuccess
	if failedCount > 0 {
		result = ResultPartialFailure
	}
	metrics.LynqNodeReconcileDuration.WithLabelValues(result).Observe(time.Since(startTime).Seconds())

	// Requeue after 30 seconds for faster resource status reflection
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// reconcileStatus handles status-only reconciliation (fast path)
// This is triggered when child resources change their status (e.g., Deployment becomes ready)
// It does NOT apply resources, only checks their current status
func (r *LynqNodeReconciler) reconcileStatus(ctx context.Context, node *lynqv1.LynqNode, startTime time.Time) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Running status-only reconcile (fast path)", "node", node.Name)

	// Build template variables
	vars, err := r.buildTemplateVariablesFromAnnotations(node)
	if err != nil {
		logger.Error(err, "Failed to build template variables for status check")
		// Fall back to full reconcile on variable errors
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Collect resources
	allResources := r.collectResourcesFromLynqNode(node)
	totalResources := int32(len(allResources))

	// Check readiness WITHOUT applying (just check status)
	readyCount, failedCount, conflictedCount := r.checkResourcesReadiness(ctx, node, allResources, vars)

	// Calculate complete status using centralized logic
	// Note: We don't have appliedResourceKeys here since this is status-only reconcile
	// Use existing status.appliedResources
	statusUpdate := r.calculateLynqNodeStatus(
		readyCount,
		failedCount,
		conflictedCount,
		totalResources,
		node.Status.AppliedResources, // Keep existing applied resources
		false,                        // not progressing
	)

	// Update ObservedGeneration to match current Generation
	r.StatusManager.PublishObservedGeneration(node, node.Generation)

	// Publish all status fields through StatusManager
	r.StatusManager.PublishResourceCounts(node, statusUpdate.ReadyResources, statusUpdate.FailedResources, statusUpdate.DesiredResources, statusUpdate.ConflictedResources)
	for _, cond := range statusUpdate.Conditions {
		switch cond.Type {
		case ConditionTypeReady:
			r.StatusManager.PublishReadyCondition(node, statusUpdate.IsReady, cond.Reason, cond.Message)
		case ConditionTypeProgressing:
			r.StatusManager.PublishProgressingCondition(node, cond.Status == metav1.ConditionTrue, cond.Reason, cond.Message)
		case ConditionTypeConflicted:
			r.StatusManager.PublishConflictedCondition(node, cond.Status == metav1.ConditionTrue)
		case ConditionTypeDegraded:
			r.StatusManager.PublishDegradedCondition(node, statusUpdate.IsDegraded, cond.Reason, cond.Message)
		}
	}

	// Find degraded reason for metrics
	var degradedReason string
	for _, cond := range statusUpdate.Conditions {
		if cond.Type == ConditionTypeDegraded {
			degradedReason = cond.Reason
			break
		}
	}

	// Publish metrics - critical for Prometheus scraping even during status-only reconciles
	r.StatusManager.PublishMetrics(node, readyCount, failedCount, totalResources, conflictedCount, statusUpdate.Conditions, statusUpdate.IsDegraded, degradedReason)

	// Record metrics
	metrics.LynqNodeReconcileDuration.WithLabelValues("status_only").Observe(time.Since(startTime).Seconds())

	logger.V(1).Info("Status-only reconcile completed",
		"node", node.Name,
		"ready", readyCount,
		"failed", failedCount,
		"conflicted", conflictedCount,
		"duration", time.Since(startTime).String())

	// Requeue after 5 minutes for periodic health check
	// Next change will trigger immediate reconcile via watch
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// checkResourcesReadiness checks the readiness of resources WITHOUT applying them
// This is much faster than applyResources as it only reads status
// Returns: readyCount, failedCount, conflictedCount
func (r *LynqNodeReconciler) checkResourcesReadiness(
	ctx context.Context,
	node *lynqv1.LynqNode,
	resources []lynqv1.TResource,
	vars template.Variables,
) (readyCount, failedCount, conflictedCount int32) {
	logger := log.FromContext(ctx)
	checker := readiness.NewChecker(r.Client)
	templateEngine := template.NewEngine()

	for _, resource := range resources {
		// Render resource (just to get name/namespace)
		obj, err := r.renderResource(ctx, templateEngine, resource, vars, node)
		if err != nil {
			logger.V(1).Info("Failed to render resource for status check", "id", resource.ID, "error", err)
			failedCount++
			continue
		}

		// Get current resource from cluster
		current := obj.DeepCopy()
		err = r.Get(ctx, client.ObjectKey{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		}, current)

		if err != nil {
			if errors.IsNotFound(err) {
				// Resource doesn't exist - count as failed
				logger.V(1).Info("Resource not found in cluster", "id", resource.ID, "name", obj.GetName())
				failedCount++
				continue
			}
			logger.Error(err, "Failed to get resource for status check", "id", resource.ID, "name", obj.GetName())
			failedCount++
			continue
		}

		// Check ownership conflict
		if r.hasOwnershipConflict(current, node) {
			logger.V(1).Info("Resource has ownership conflict", "id", resource.ID, "name", obj.GetName())
			conflictedCount++
			failedCount++
			continue
		}

		// Check readiness
		if resource.WaitForReady != nil && *resource.WaitForReady {
			if !checker.IsReady(current) {
				logger.V(1).Info("Resource not ready", "id", resource.ID, "name", obj.GetName())
				failedCount++
				continue
			}
		}

		// Resource is ready
		readyCount++
	}

	return readyCount, failedCount, conflictedCount
}
