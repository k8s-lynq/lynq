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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	errorsStd "errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
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
	Scheme           *runtime.Scheme
	Recorder         record.EventRecorder
	StatusManager    *status.Manager
	TemplateEngine   *template.Engine
	Applier          *apply.Applier
	ReadinessChecker *readiness.Checker
	renderCache      sync.Map // key: "nodeName/resourceID" → *renderCacheEntry

	// LegacyReadinessStrict reverts the readiness check to the pre-phase-model
	// behavior: strict equality (every replica available) plus the rollout
	// timeout, no per-phase classification, no WorkloadDegraded events. Wired
	// from the --legacy-readiness-strict flag for emergency rollback. Default
	// false — leave the new phase-model path on. Slated for removal after one
	// release cycle if no users opt in.
	LegacyReadinessStrict bool
}

// renderCacheEntry holds a cached rendered resource to skip expensive re-rendering
// when inputs (node spec + template variables) haven't changed.
type renderCacheEntry struct {
	inputKey string                     // cache key incorporating all render inputs
	rendered *unstructured.Unstructured // the fully rendered resource
}

// computeVariablesHash hashes the template-variable annotations the Hub
// rewrites for database-driven updates. These changes do NOT bump
// metadata.generation, so determineReconcileType compares this hash against
// status.observedVariablesHash to detect them and route to a full reconcile
// (M2). Uses the same annotation set that feeds computeRenderCacheKey, minus
// the identity/generation fields that are compared separately.
func computeVariablesHash(node *lynqv1.LynqNode) string {
	a := node.Annotations
	// Key-prefixed and newline-separated so a value ending where the next
	// begins cannot collide with a different split.
	raw := strings.Join([]string{
		"uid=" + a["lynq.sh/uid"],
		"activate=" + a["lynq.sh/activate"],
		"hostOrUrl=" + a["lynq.sh/hostOrUrl"],
		"extra=" + a["lynq.sh/extra"],
		"hubId=" + a["lynq.sh/hubId"],
		"template-generation=" + a["lynq.sh/template-generation"],
	}, "\n")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:8]) // 16-char hex, sufficient for change detection
}

// computeRenderCacheKey builds a cache key from all inputs that affect renderResource output.
// This includes node UID (changes on delete/recreate), generation, template variable annotations,
// and resource ID. The UID ensures cache invalidation when a LynqNode is deleted and recreated
// with the same name but different content.
func computeRenderCacheKey(node *lynqv1.LynqNode, resourceID string) string {
	return fmt.Sprintf("%s/%s/%s/%d/%s/%s/%s/%s/%s/%s",
		node.Name,
		node.Namespace,
		string(node.UID),
		node.Generation,
		node.Annotations["lynq.sh/hostOrUrl"],
		node.Annotations["lynq.sh/activate"],
		node.Annotations["lynq.sh/extra"],
		node.Annotations["lynq.sh/hubId"],
		node.Annotations["lynq.sh/template-generation"],
		resourceID,
	)
}

// Getters with nil-safe fallback (for tests that don't set these fields)
func (r *LynqNodeReconciler) getTemplateEngine() *template.Engine {
	if r.TemplateEngine != nil {
		return r.TemplateEngine
	}
	return template.NewEngine()
}

func (r *LynqNodeReconciler) getApplier() *apply.Applier {
	if r.Applier != nil {
		return r.Applier
	}
	return apply.NewApplier(r.Client, r.Scheme)
}

func (r *LynqNodeReconciler) getReadinessChecker() *readiness.Checker {
	if r.ReadinessChecker != nil {
		return r.ReadinessChecker
	}
	return readiness.NewChecker(r.Client)
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

	// Resource kinds used in template rendering and readiness checks
	resourceKindConfigMap   = "ConfigMap"
	resourceKindSecret      = "Secret"
	resourceKindDeployment  = "Deployment"
	resourceKindStatefulSet = "StatefulSet"
	resourceKindDaemonSet   = "DaemonSet"

	// Degraded reasons
	ReasonResourceFailuresAndConflicts = "ResourceFailuresAndConflicts"
	ReasonResourceFailures             = "ResourceFailures"
	ReasonResourceConflicts            = "ResourceConflicts"
	ReasonResourcesNotReady            = "ResourcesNotReady"
	// ReasonResourcesDegraded fires when at least one resource is in the
	// Degraded phase (steady-state K8s-converged disruption) but no resources
	// have failed and there are no ownership conflicts. LynqNode.Ready stays
	// True in this case — Kubernetes is converging the workload, Lynq is not
	// attributing failure. See ResourcePhase docs.
	ReasonResourcesDegraded = "ResourcesDegraded"
	// ReasonResourceFailuresAndDegraded fires when at least one resource has
	// failed (Lynq-attributed) AND at least one other resource is in
	// steady-state Degraded.
	ReasonResourceFailuresAndDegraded = "ResourceFailuresAndDegraded"

	// Reconcile results
	ResultSuccess        = "success"
	ResultPartialFailure = "partial_failure"

	// ForceReapplyInterval is the cadence of the periodic drift-correction
	// resync: every reconcile after this duration since the last full reapply
	// bypasses the apply-skip check and re-applies every child resource. This
	// is Lynq's external-drift correction mechanism — external edits that
	// preserve the lynq.sh/applied-hash annotation on a child resource will be
	// corrected on the next force-reapply cycle, not within seconds of the
	// edit. The 10-minute default matches Crossplane's periodic resync model.
	ForceReapplyInterval = 10 * time.Minute
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
//
// degradedCount/progressingCount/pendingCount break the not-Ready/not-Failed
// space into native K8s phases — see ClassifyPhase. resourcePhases is the
// per-resource source of truth (one entry per resource attempted, written
// back to LynqNode.status.resourcePhases). replicaMetrics carries the
// per-resource workload counters (drives lynqnode_resource_replicas_* +
// degraded-since-seconds gauges). degradedResourceIds is the array of
// currently-Degraded resource IDs (surfaces in `kubectl get -o wide`).
//
// forceReapply: when true, every ApplyResource call bypasses the annotation-based
// skip check and re-applies unconditionally. This is the periodic drift-correction
// resync gated by LynqNode.Status.LastFullReconcileAt (see ForceReapplyInterval).
func (r *LynqNodeReconciler) applyResources(
	ctx context.Context,
	node *lynqv1.LynqNode,
	sortedNodes []*graph.Node,
	vars template.Variables,
	forceReapply bool,
) (
	readyCount, failedCount, changedCount, conflictedCount, skippedCount int32,
	skippedIds []string,
	degradedCount, progressingCount, pendingCount int32,
	degradedResourceIds []string,
	resourcePhases []lynqv1.ResourcePhaseEntry,
	replicaMetrics map[string]status.ResourceReplicaMetrics,
) {
	// previousEntries lets the new phase-model path detect Available→Degraded,
	// Degraded→Available, and Progressing/Pending→Available transitions for
	// event emission and the phase_transitions_total counter, and carry forward
	// DegradedSince. Reading from node.Status.ResourcePhases means restart
	// behavior is consistent: a resource that was already Degraded at restart
	// does NOT re-emit WorkloadDegraded (no transition observed).
	previousEntries := buildPreviousPhasesMap(node)
	replicaMetrics = make(map[string]status.ResourceReplicaMetrics)
	// Non-nil so the status manager always overwrites status.degradedResourceIds
	// — otherwise a nil slice on recovery leaves the previous IDs stale (F4).
	degradedResourceIds = []string{}

	logger := log.FromContext(ctx)
	applier := r.getApplier()
	checker := r.getReadinessChecker()
	templateEngine := r.getTemplateEngine()

	// Record when this reconcile cycle started applying resources.
	// Timeout for each resource is measured from this point, not from the resource's
	// creationTimestamp. A pre-existing resource being updated should not be immediately
	// timed out just because it was created long before this reconcile cycle started.
	applyStartTime := time.Now()

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
				// Record a Pending phase entry so the resource stays visible in
				// status.resourcePhases (the array is documented as one entry
				// per resource). Pending — not Failed — because the skip is
				// per-reconcile: once the dependency recovers, this resource
				// is applied normally. It is counted in skippedResources /
				// skippedResourceIds, NOT in pendingResources, to keep the
				// existing count semantics unchanged.
				resourcePhases = append(resourcePhases, lynqv1.ResourcePhaseEntry{
					ID: resource.ID, Kind: resource.Spec.GroupVersionKind().Kind, Name: resource.NameTemplate,
					Phase:  lynqv1.ResourcePhasePending,
					Reason: fmt.Sprintf("skipped: dependency '%s' failed (retries next reconcile)", failedDepId),
				})
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
			// Record a Pending phase entry (and count it) so an operator can
			// answer "why is this node at 2/5 ready with nothing failed?"
			// directly from status.resourcePhases: the resource has not been
			// applied yet because it is waiting on a dependency.
			pendingCount++
			resourcePhases = append(resourcePhases, lynqv1.ResourcePhaseEntry{
				ID: resource.ID, Kind: resource.Spec.GroupVersionKind().Kind, Name: resource.NameTemplate,
				Phase:  lynqv1.ResourcePhasePending,
				Reason: fmt.Sprintf("blocked: waiting for dependency '%s' to become ready", notReadyDepId),
			})
			continue
		}

		// Check if node is being deleted before processing each resource
		// This allows quick exit when node is deleted during reconciliation
		currentLynqNode := &lynqv1.LynqNode{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(node), currentLynqNode); err != nil {
			if errors.IsNotFound(err) {
				// LynqNode was deleted, stop processing
				logger.Info("LynqNode deleted during reconciliation, stopping resource application")
				return readyCount, failedCount, changedCount, conflictedCount, skippedCount, skippedIds,
					degradedCount, progressingCount, pendingCount, degradedResourceIds, resourcePhases, replicaMetrics
			}
			// Continue on other errors
		} else if !currentLynqNode.DeletionTimestamp.IsZero() {
			// LynqNode is being deleted, stop processing immediately
			logger.Info("LynqNode deletion in progress, stopping resource application",
				"node", node.Name,
				"processedResources", readyCount+failedCount)
			return readyCount, failedCount, changedCount, conflictedCount, skippedCount, skippedIds,
				degradedCount, progressingCount, pendingCount, degradedResourceIds, resourcePhases, replicaMetrics
		}

		// Render templates (with cache: skip expensive rendering when inputs unchanged)
		obj, err := r.renderResourceCached(ctx, templateEngine, resource, vars, node)
		if err != nil {
			logger.Error(err, "Failed to render resource", "id", resource.ID)
			r.Recorder.Eventf(node, corev1.EventTypeWarning, "TemplateRenderError",
				"Failed to render resource %s: %v", resource.ID, err)
			failedResourceIds[resource.ID] = true
			failedCount++
			// F3: record a Failed phase entry so the resource is reflected in
			// status.resourcePhases and its lynqnode_resource_phase stateset is
			// (re)set to Failed — otherwise a previously-Available series would
			// linger at 1. obj is nil here, so derive kind/name from the spec.
			resourcePhases = append(resourcePhases, lynqv1.ResourcePhaseEntry{
				ID: resource.ID, Kind: resource.Spec.GroupVersionKind().Kind, Name: resource.NameTemplate,
				Phase: lynqv1.ResourcePhaseFailed, Reason: fmt.Sprintf("template render error: %v", err),
			})
			continue
		}

		// Handle CreationPolicy.Once (extracted to keep applyResources's
		// cyclomatic complexity under the gocyclo limit).
		onceOutcome, onceErr := r.handleCreationPolicyOnce(ctx, resource, obj)
		switch onceOutcome {
		case onceCheckFailed:
			logger.Error(onceErr, "Failed to check Once policy", "id", resource.ID)
			failedResourceIds[resource.ID] = true
			failedCount++
			resourcePhases = append(resourcePhases, lynqv1.ResourcePhaseEntry{
				ID: resource.ID, Kind: obj.GetKind(), Name: obj.GetName(),
				Phase: lynqv1.ResourcePhaseFailed, Reason: fmt.Sprintf("CreationPolicy=Once check failed: %v", onceErr),
			})
			continue
		case onceAlreadyExists:
			logger.V(1).Info("Skipping resource (CreationPolicy=Once, already created)", "id", resource.ID, "name", obj.GetName())
			readyCount++
			resourcePhases = append(resourcePhases, lynqv1.ResourcePhaseEntry{
				ID: resource.ID, Kind: obj.GetKind(), Name: obj.GetName(),
				Phase: lynqv1.ResourcePhaseAvailable, Reason: "CreationPolicy=Once, already created",
			})
			continue
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

		changed, applyErr := applier.ApplyResource(ctx, obj, node, resource.ConflictPolicy, resource.PatchStrategy, deletionPolicy, ignoreFields, forceReapply)

		// M1: soft guardrail for multi-manager safety. `replace` (full Update)
		// and `merge` (strategic merge) do NOT do field-level ownership like
		// SSA — on shared workloads they can clobber webhook-injected sidecars,
		// API-defaulted fields, and HPA-owned spec.replicas. Warn (once per real
		// write) when either is used on a pod-based workload; only `apply` (SSA)
		// is multi-manager-safe. No behavior change — visibility only.
		if changed && applyErr == nil {
			r.warnUnsafePatchStrategy(ctx, node, resource, obj.GetKind())
		}

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
			// F3: record a Failed phase entry (covers both conflict and other
			// apply errors) so status.resourcePhases and the phase stateset
			// reflect the failure instead of a stale Available.
			resourcePhases = append(resourcePhases, lynqv1.ResourcePhaseEntry{
				ID: resource.ID, Kind: kind, Name: obj.GetName(),
				Phase: lynqv1.ResourcePhaseFailed, Reason: fmt.Sprintf("apply error: %v", applyErr),
			})
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
				resourcePhases = append(resourcePhases, lynqv1.ResourcePhaseEntry{
					ID: resource.ID, Kind: kind, Name: obj.GetName(),
					Phase: lynqv1.ResourcePhaseFailed, Reason: fmt.Sprintf("failed to read live state: %v", err),
				})
				continue
			}

			timeoutSeconds := resource.TimeoutSeconds
			if timeoutSeconds <= 0 {
				timeoutSeconds = 300 // Default 5 minutes
			}
			timeoutDuration := time.Duration(timeoutSeconds) * time.Second
			// elapsedSinceApply reads lynq.sh/apply-start-time from the
			// resource annotation (set by the Applier, persisted across
			// reconciles). applyStartTime is the fallback for the very first
			// reconcile before the annotation has been written.
			elapsed := elapsedSinceApply(current, applyStartTime)

			if r.LegacyReadinessStrict {
				// LEGACY PATH — preserved verbatim from the pre-phase-model
				// behavior for emergency rollback via --legacy-readiness-strict.
				// Strict equality + rollout timeout; no per-phase classification,
				// no WorkloadDegraded events.
				if checker.IsReady(current) {
					logger.V(1).Info("Resource is ready", "id", resource.ID, "name", obj.GetName())
					readyCount++
				} else if elapsed >= timeoutDuration {
					logger.Info("Resource not ready after timeout, marking as failed",
						"id", resource.ID, "name", obj.GetName(),
						"elapsed", elapsed.String(), "timeout", timeoutDuration.String())
					r.Recorder.Eventf(node, corev1.EventTypeWarning, "ReadinessTimeout",
						"Resource '%s' not ready after %s (timeout: %s)", resource.ID, elapsed.Round(time.Second), timeoutDuration)
					failedResourceIds[resource.ID] = true
					failedCount++
				} else {
					logger.V(1).Info("Resource not ready yet, will check again in next reconcile",
						"id", resource.ID, "name", obj.GetName(),
						"elapsed", elapsed.String(), "timeout", timeoutDuration.String())
					notReadyResourceIds[resource.ID] = true
				}
				continue
			}

			// NEW PATH — phase-driven classification + per-resource phase
			// tracking + transition-based events. Extracted into a helper
			// to keep applyResources's cyclomatic complexity manageable
			// (gocyclo limit is 35).
			entry, replicas, phase := r.evaluateResourcePhase(ctx, node, resource, current, kind, elapsed, timeoutDuration, previousEntries)
			r.aggregatePhaseCounters(phase, resource.ID,
				&readyCount, &degradedCount, &progressingCount, &pendingCount, &failedCount,
				&degradedResourceIds, failedResourceIds, notReadyResourceIds)
			replicaMetrics[resource.ID] = replicas
			resourcePhases = append(resourcePhases, entry)
		} else {
			// No readiness check required, count as ready
			readyCount++
			resourcePhases = append(resourcePhases, lynqv1.ResourcePhaseEntry{
				ID: resource.ID, Kind: kind, Name: obj.GetName(),
				Phase: lynqv1.ResourcePhaseAvailable, Reason: "waitForReady=false",
			})
		}
	}

	return readyCount, failedCount, changedCount, conflictedCount, skippedCount, skippedIds,
		degradedCount, progressingCount, pendingCount, degradedResourceIds, resourcePhases, replicaMetrics
}

// onceOutcome describes what handleCreationPolicyOnce decided. Lets the
// caller decide whether to continue, abort with failure, or proceed.
type onceOutcome int

const (
	// onceProceed: resource is NOT Once policy, or Once but never created
	// before — caller should continue with apply.
	onceProceed onceOutcome = iota
	// onceCheckFailed: checkOnceCreated returned an error — caller should
	// mark the resource Failed and continue to next resource.
	onceCheckFailed
	// onceAlreadyExists: resource is Once policy and the cluster already
	// has it with the created-once annotation — caller should count Ready
	// and continue to next resource (skip apply).
	onceAlreadyExists
)

// handleCreationPolicyOnce checks whether a CreationPolicy.Once resource
// has already been created. For non-Once resources it returns onceProceed
// immediately. For Once resources that have not been created, it stamps
// the AnnotationCreatedOnce annotation on obj so the apply pass writes it.
//
// Extracted from applyResources to keep that function within the gocyclo
// budget (the inner if/else chain contributed 4 branches).
func (r *LynqNodeReconciler) handleCreationPolicyOnce(
	ctx context.Context,
	resource lynqv1.TResource,
	obj *unstructured.Unstructured,
) (onceOutcome, error) {
	if resource.CreationPolicy != lynqv1.CreationPolicyOnce {
		return onceProceed, nil
	}
	exists, hasAnnotation, err := r.checkOnceCreated(ctx, obj)
	if err != nil {
		return onceCheckFailed, err
	}
	if exists && hasAnnotation {
		return onceAlreadyExists, nil
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[AnnotationCreatedOnce] = AnnotationValueTrue
	obj.SetAnnotations(annotations)
	return onceProceed, nil
}

// evaluateResourcePhase runs the new phase-driven readiness gate for one
// applied resource. Extracted from applyResources to keep cyclomatic
// complexity within the gocyclo budget. Returns the per-resource state the
// caller aggregates into the result.
//
// Called from the new (phase-model) path only — the legacy path stays
// inline in applyResources for backwards-compat clarity.
func (r *LynqNodeReconciler) evaluateResourcePhase(
	ctx context.Context,
	node *lynqv1.LynqNode,
	resource lynqv1.TResource,
	current *unstructured.Unstructured,
	kind string,
	elapsed, timeoutDuration time.Duration,
	previousEntries map[string]lynqv1.ResourcePhaseEntry,
) (lynqv1.ResourcePhaseEntry, status.ResourceReplicaMetrics, lynqv1.ResourcePhase) {
	checker := r.getReadinessChecker()
	phaseResult := checker.ClassifyPhase(current, elapsed, timeoutDuration)
	prevEntry := previousEntries[resource.ID]
	entry := r.processResourcePhase(ctx, node, resource, current.GetName(), kind, phaseResult, prevEntry, elapsed)

	replicas := status.ResourceReplicaMetrics{
		Kind:                 kind,
		Desired:              phaseResult.Replicas.Desired,
		Available:            phaseResult.Replicas.Available,
		Ready:                phaseResult.Replicas.Ready,
		Updated:              phaseResult.Replicas.Updated,
		DegradedSinceSeconds: entry.SinceSeconds,
	}
	return entry, replicas, phaseResult.Phase
}

// aggregatePhaseCounters maps a resource's phase to the caller's aggregate
// counters and dependency-tracking maps. Available AND Degraded both bump
// readyCount — the workload is serving traffic either way. This is the
// headline semantic change: steady-state pod-level disruption no longer
// flips LynqNode.Ready=False.
func (r *LynqNodeReconciler) aggregatePhaseCounters(
	phase lynqv1.ResourcePhase,
	resourceID string,
	readyCount, degradedCount, progressingCount, pendingCount, failedCount *int32,
	degradedResourceIds *[]string,
	failedResourceIds, notReadyResourceIds map[string]bool,
) {
	switch phase {
	case lynqv1.ResourcePhaseAvailable:
		*readyCount++
	case lynqv1.ResourcePhaseDegraded:
		*readyCount++
		*degradedCount++
		*degradedResourceIds = append(*degradedResourceIds, resourceID)
	case lynqv1.ResourcePhaseProgressing:
		*progressingCount++
		notReadyResourceIds[resourceID] = true
	case lynqv1.ResourcePhasePending:
		*pendingCount++
		notReadyResourceIds[resourceID] = true
	case lynqv1.ResourcePhaseFailed:
		failedResourceIds[resourceID] = true
		*failedCount++
	}
}

// processResourcePhase emits transition-based metrics and events for a single
// resource and returns the ResourcePhaseEntry to record in status. It does
// NOT update the aggregate counters — the caller does that after deciding
// which counter to bump for the phase.
//
// Transition rules:
//   - Available → Degraded:        emit WorkloadDegraded event
//   - Degraded → Available:        emit WorkloadRecovered event
//   - Progressing|Pending → Available:
//     emit RolloutComplete event + observe
//     rollout_duration_seconds histogram
//   - Progressing|Pending → Failed (RolloutTimedOut):
//     emit ReadinessTimeout event + observe
//     histogram with result="timeout"
//   - Progressing|Pending → Failed (other):
//     observe histogram with result="aborted"
//
// Every transition (previousPhase != current && previousPhase != "") also
// increments lynqnode_resource_phase_transitions_total{kind,from,to}.
func (r *LynqNodeReconciler) processResourcePhase(
	ctx context.Context,
	node *lynqv1.LynqNode,
	resource lynqv1.TResource,
	resourceName string,
	kind string,
	phaseResult readiness.PhaseResult,
	prevEntry lynqv1.ResourcePhaseEntry,
	elapsed time.Duration,
) lynqv1.ResourcePhaseEntry {
	logger := log.FromContext(ctx)
	currentPhase := phaseResult.Phase
	previousPhase := prevEntry.Phase

	entry := lynqv1.ResourcePhaseEntry{
		ID:     resource.ID,
		Kind:   kind,
		Name:   resourceName,
		Phase:  currentPhase,
		Reason: phaseResult.Reason,
	}

	// degraded-since tracks true time-in-Degraded via a persisted timestamp.
	// On entry into Degraded (previous phase was not Degraded) stamp now; while
	// still Degraded, carry forward the previous entry's DegradedSince. This is
	// anchored to the transition, NOT to lynq.sh/apply-start-time — otherwise a
	// workload applied days ago that degrades now would report days of
	// "degraded" on its second reconcile and misfire the >30m severe alert.
	//
	// Carry-forward additionally requires the previous entry to describe the
	// SAME concrete object (kind+name). If a template edit reuses a TResource
	// ID for a different kind or a re-rendered name, the old object's clock
	// must not leak onto the new one — the new object starts its own window.
	if currentPhase == lynqv1.ResourcePhaseDegraded {
		sameObject := prevEntry.Kind == kind && prevEntry.Name == resourceName
		if previousPhase == lynqv1.ResourcePhaseDegraded && sameObject && prevEntry.DegradedSince != nil {
			entry.DegradedSince = prevEntry.DegradedSince
		} else {
			now := metav1.Now()
			entry.DegradedSince = &now
		}
		entry.SinceSeconds = int64(time.Since(entry.DegradedSince.Time).Seconds())
		if entry.SinceSeconds < 0 {
			entry.SinceSeconds = 0
		}
	}

	// Skip transition handling when we have no prior phase (first reconcile,
	// fresh LynqNode). Prevents spurious WorkloadDegraded events at startup.
	if previousPhase == "" || previousPhase == currentPhase {
		return entry
	}

	// Increment transition counter ONCE per real transition.
	metrics.LynqNodeResourcePhaseTransitionsTotal.WithLabelValues(
		kind,
		string(previousPhase),
		string(currentPhase),
	).Inc()

	switch {
	case previousPhase == lynqv1.ResourcePhaseAvailable && currentPhase == lynqv1.ResourcePhaseDegraded:
		r.Recorder.Eventf(node, corev1.EventTypeWarning, "WorkloadDegraded",
			"Resource '%s' (%s/%s) is degraded: %s. Kubernetes is converging — Lynq is NOT marking this as Failed.",
			resource.ID, kind, resourceName, phaseResult.Reason)

	case previousPhase == lynqv1.ResourcePhaseDegraded && currentPhase == lynqv1.ResourcePhaseAvailable:
		r.Recorder.Eventf(node, corev1.EventTypeNormal, "WorkloadRecovered",
			"Resource '%s' (%s/%s) recovered to Available.",
			resource.ID, kind, resourceName)

	case (previousPhase == lynqv1.ResourcePhaseProgressing || previousPhase == lynqv1.ResourcePhasePending) &&
		currentPhase == lynqv1.ResourcePhaseAvailable:
		r.Recorder.Eventf(node, corev1.EventTypeNormal, "RolloutComplete",
			"Resource '%s' (%s/%s) rollout complete in %s.",
			resource.ID, kind, resourceName, elapsed.Round(time.Second))
		metrics.LynqNodeResourceRolloutDurationSeconds.WithLabelValues(kind, "complete").Observe(elapsed.Seconds())

	case (previousPhase == lynqv1.ResourcePhaseProgressing || previousPhase == lynqv1.ResourcePhasePending) &&
		currentPhase == lynqv1.ResourcePhaseFailed:
		if phaseResult.RolloutTimedOut {
			logger.Info("Resource not ready after timeout, marking as failed",
				"id", resource.ID, "name", resourceName,
				"elapsed", elapsed.String())
			r.Recorder.Eventf(node, corev1.EventTypeWarning, "ReadinessTimeout",
				"Resource '%s' not ready after %s (rollout timeout exceeded): %s",
				resource.ID, elapsed.Round(time.Second), phaseResult.Reason)
			metrics.LynqNodeResourceRolloutDurationSeconds.WithLabelValues(kind, "timeout").Observe(elapsed.Seconds())
		} else {
			r.Recorder.Eventf(node, corev1.EventTypeWarning, "RolloutAborted",
				"Resource '%s' (%s/%s) rollout aborted: %s",
				resource.ID, kind, resourceName, phaseResult.Reason)
			metrics.LynqNodeResourceRolloutDurationSeconds.WithLabelValues(kind, "aborted").Observe(elapsed.Seconds())
		}
	}

	return entry
}

// buildPreviousPhasesMap indexes node.Status.ResourcePhases by resource ID so
// per-resource transition detection has O(1) lookup. Returns the full previous
// entry (not just the phase) so processResourcePhase can carry forward
// DegradedSince to compute true time-in-Degraded. Returns an empty map (not
// nil) so lookups on missing IDs return the zero-value entry (Phase == "")
// cleanly.
func buildPreviousPhasesMap(node *lynqv1.LynqNode) map[string]lynqv1.ResourcePhaseEntry {
	out := make(map[string]lynqv1.ResourcePhaseEntry, len(node.Status.ResourcePhases))
	for _, entry := range node.Status.ResourcePhases {
		out[entry.ID] = entry
	}
	return out
}

// warnUnsafePatchStrategy emits a Warning event + log when a workload kind is
// applied with a non-SSA patch strategy (replace/merge), which can clobber
// fields owned by other managers (webhook sidecars, API defaults, HPA
// spec.replicas). Visibility only — no behavior change (M1). Kubernetes dedups
// events by reason+message so this doesn't spam despite firing per write.
func (r *LynqNodeReconciler) warnUnsafePatchStrategy(ctx context.Context, node *lynqv1.LynqNode, resource lynqv1.TResource, kind string) {
	if resource.PatchStrategy != lynqv1.PatchStrategyReplace && resource.PatchStrategy != lynqv1.PatchStrategyMerge {
		return
	}
	switch kind {
	case resourceKindDeployment, resourceKindStatefulSet, resourceKindDaemonSet:
	default:
		return
	}
	log.FromContext(ctx).Info("patchStrategy is not multi-manager-safe on a workload",
		"id", resource.ID, "kind", kind, "patchStrategy", resource.PatchStrategy)
	r.Recorder.Eventf(node, corev1.EventTypeWarning, "UnsafePatchStrategy",
		"Resource '%s' (%s) uses patchStrategy=%s, which is NOT field-ownership-aware and can "+
			"overwrite fields managed by other controllers (HPA spec.replicas, admission-webhook "+
			"sidecars, API-server defaults). Prefer patchStrategy=apply (SSA); use ignoreFields for "+
			"externally-owned fields such as $.spec.replicas.",
		resource.ID, kind, resource.PatchStrategy)
}

// clearPhaseStateOnEarlyError zeroes per-resource phase state when a reconcile
// aborts BEFORE evaluating any resource (VariablesBuildError / DependencyCycle)
// (F6). Without it, a node that was healthy keeps its previous
// status.resourcePhases array, status.degradedResourceIds, and per-resource
// metric series (e.g. a lingering lynqnode_resource_phase{phase="Available"}=1)
// even though nothing could be evaluated. The aggregate per-phase counts and
// prom gauges are zeroed by the caller's existing PublishMetrics(0,0,0,0,…).
func (r *LynqNodeReconciler) clearPhaseStateOnEarlyError(node *lynqv1.LynqNode) {
	for _, e := range node.Status.ResourcePhases {
		metrics.DeleteResourceSeries(node.Name, node.Namespace, e.ID, e.Kind)
	}
	// Non-nil empty slices so the status manager overwrites (clears) the fields.
	r.StatusManager.PublishResourcePhases(node, []lynqv1.ResourcePhaseEntry{}, []string{}, nil)
	r.StatusManager.PublishResourceCountsWithPhases(node, 0, 0, 0, 0, 0, 0, 0)
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

	logger.V(1).Info("Applying template resources to cluster",
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

		logger.V(1).Info("Template application completed successfully",
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

// renderResourceCached returns a rendered resource, using an in-memory cache to skip
// expensive DeepCopy + renderUnstructured when inputs haven't changed since last render.
// The caller MUST NOT hold onto the returned object across reconciles — ApplyResource mutates it.
func (r *LynqNodeReconciler) renderResourceCached(ctx context.Context, engine *template.Engine, resource lynqv1.TResource, vars template.Variables, node *lynqv1.LynqNode) (*unstructured.Unstructured, error) {
	cacheKey := computeRenderCacheKey(node, resource.ID)
	storageKey := node.Namespace + "/" + node.Name + "/" + resource.ID

	// Check cache
	if cached, ok := r.renderCache.Load(storageKey); ok {
		entry := cached.(*renderCacheEntry)
		if entry.inputKey == cacheKey {
			// Cache hit: return a deep copy (ApplyResource mutates the object)
			return entry.rendered.DeepCopy(), nil
		}
	}

	// Cache miss: render from scratch
	rendered, err := r.renderResource(ctx, engine, resource, vars, node)
	if err != nil {
		return nil, err
	}

	// Store in cache (store the original, return a copy for apply)
	r.renderCache.Store(storageKey, &renderCacheEntry{
		inputKey: cacheKey,
		rendered: rendered,
	})
	return rendered.DeepCopy(), nil
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
	renderedSpec, err := r.renderUnstructured(ctx, obj.Object, engine, vars, obj.GetKind(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to render spec: %w", err)
	}
	obj.Object = renderedSpec

	return obj, nil
}

// renderUnstructured recursively renders template variables in unstructured data
// Returns an error if any template rendering fails (e.g., missing variable references)
// This function also parses type markers (from int, float, bool template functions)
// to restore proper Go types for Kubernetes API compatibility. For string-only
// fields (e.g., ConfigMap data, annotations) values remain strings even if typed
// template functions are used.
//
//nolint:unparam // ctx is passed through for recursive calls
func (r *LynqNodeReconciler) renderUnstructured(
	ctx context.Context,
	data map[string]interface{},
	engine *template.Engine,
	vars template.Variables,
	resourceKind string,
	fieldPath []string,
) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(data))

	for k, v := range data {
		currentPath := append(fieldPath, k)

		switch val := v.(type) {
		case string:
			// Try to render as template
			rendered, err := engine.Render(val, vars)
			if err != nil {
				// Return error to mark resource as failed (e.g., missing variable reference)
				return nil, fmt.Errorf("template rendering failed for field %q: %w", k, err)
			}
			if shouldKeepString(resourceKind, currentPath) {
				result[k] = template.StripTypeMarker(rendered)
			} else {
				result[k] = template.ParseTypedValue(rendered)
			}
		case map[string]interface{}:
			// Recurse into nested maps
			rendered, err := r.renderUnstructured(ctx, val, engine, vars, resourceKind, currentPath)
			if err != nil {
				return nil, fmt.Errorf("template rendering failed in nested field %q: %w", k, err)
			}
			result[k] = rendered
		case []interface{}:
			// Recurse into arrays
			renderedArray := make([]interface{}, len(val))
			for i, item := range val {
				if itemMap, ok := item.(map[string]interface{}); ok {
					rendered, err := r.renderUnstructured(ctx, itemMap, engine, vars, resourceKind, currentPath)
					if err != nil {
						return nil, fmt.Errorf("template rendering failed for array item %q[%d]: %w", k, i, err)
					}
					renderedArray[i] = rendered
				} else if itemStr, ok := item.(string); ok {
					rendered, err := engine.Render(itemStr, vars)
					if err != nil {
						return nil, fmt.Errorf("template rendering failed for array string %q[%d]: %w", k, i, err)
					}
					if shouldKeepString(resourceKind, currentPath) {
						renderedArray[i] = template.StripTypeMarker(rendered)
					} else {
						renderedArray[i] = template.ParseTypedValue(rendered)
					}
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

// shouldKeepString returns true when the current field must remain a string even if
// typed template markers are present. This is required for ConfigMap/Secret data
// and for label/annotation-style maps that only accept string values.
func shouldKeepString(resourceKind string, fieldPath []string) bool {
	if len(fieldPath) == 0 {
		return false
	}

	currentKey := fieldPath[len(fieldPath)-1]
	if currentKey == "apiVersion" || currentKey == "kind" {
		return true
	}

	if len(fieldPath) < 2 {
		return false
	}

	parentKey := fieldPath[len(fieldPath)-2]
	switch parentKey {
	case "annotations", "labels", "matchLabels", "nodeSelector":
		return true
	case "data":
		return resourceKind == resourceKindConfigMap || resourceKind == resourceKindSecret
	case "binaryData", "stringData":
		return resourceKind == resourceKindConfigMap || resourceKind == resourceKindSecret
	default:
		return false
	}
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
//   - readyCount: Number of resources that are ready (Available OR Degraded — both serving traffic)
//   - failedCount: Number of resources that failed (Lynq-attributed: rollout timeout, ProgressDeadlineExceeded, apply error)
//   - conflictedCount: Number of resources in ownership conflict
//   - degradedCount: Number of resources in steady-state Degraded phase (K8s converging — NOT a Lynq failure).
//     Note: degradedCount is already INCLUDED in readyCount — Available + Degraded both count as Ready
//     for LynqNode aggregation. The degradedCount surfaces independently so the Degraded condition can
//     fire with reason=ResourcesDegraded even while LynqNode.Ready=True.
//   - totalResources: Total number of desired resources
//   - appliedResourceKeys: Keys of successfully applied resources
//   - isProgressing: Whether reconciliation is currently in progress
//
// Returns:
//   - *LynqNodeStatusUpdate: Complete status update with all fields calculated
func (r *LynqNodeReconciler) calculateLynqNodeStatus(
	readyCount, failedCount, conflictedCount, degradedCount, totalResources int32,
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
	hasDegraded := degradedCount > 0
	// LynqNode is fully Ready when every resource is Available or Degraded
	// (readyCount == totalResources), there are no Lynq-attributed failures,
	// and no ownership conflicts. Steady-state Degraded resources do NOT
	// flip Ready=False — this is the headline semantic of the phase model.
	isFullyReady := failedCount == 0 && conflictedCount == 0 && readyCount == totalResources
	// "Degraded" as a flag covers failures, conflicts, OR steady-state
	// degraded resources — any one of these warrants exposing the
	// LynqNode.Degraded condition (with the appropriate reason).
	isDegraded := failedCount > 0 || hasConflict || hasDegraded

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
		switch {
		case failedCount > 0 && hasConflict:
			degradedCond.Reason = ReasonResourceFailuresAndConflicts
			degradedCond.Message = fmt.Sprintf("LynqNode has %d failed and %d conflicted resources", failedCount, conflictedCount)
		case failedCount > 0 && hasDegraded:
			// Lynq-attributed failure + steady-state degradation. The
			// failure is the higher-severity signal but the operator should
			// see both.
			degradedCond.Reason = ReasonResourceFailuresAndDegraded
			degradedCond.Message = fmt.Sprintf("LynqNode has %d failed and %d degraded resources", failedCount, degradedCount)
		case failedCount > 0:
			degradedCond.Reason = ReasonResourceFailures
			degradedCond.Message = fmt.Sprintf("LynqNode has %d failed resources", failedCount)
		case hasConflict:
			degradedCond.Reason = ReasonResourceConflicts
			degradedCond.Message = fmt.Sprintf("LynqNode has %d conflicted resources", conflictedCount)
		case hasDegraded:
			// Steady-state degradation — Kubernetes is converging some
			// resources. LynqNode.Ready stays True. This is the new
			// lower-severity signal.
			degradedCond.Reason = ReasonResourcesDegraded
			degradedCond.Message = fmt.Sprintf("LynqNode has %d degraded resources (Kubernetes is converging; Lynq is not attributing failure)", degradedCount)
		case readyCount != totalResources:
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

	applier := r.getApplier()
	templateEngine := r.getTemplateEngine()

	// Build template variables from annotations
	vars, err := r.buildTemplateVariablesFromAnnotations(node)
	if err != nil {
		logger.Error(err, "Failed to build template variables for cleanup, using empty variables")
		// Continue with cleanup even if variables fail
		vars = template.Variables{}
	}

	// Collect all resources
	allResources := r.collectResourcesFromLynqNode(node)
	logger.V(1).Info("Collected resources for cleanup", "count", len(allResources))

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
		logger.V(1).Info("Cleanup completed with some errors", "errorCount", len(cleanupErrors))
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

			// Reconcile on annotation change, BUT ignore annotations we manage ourselves
			// (the `lynq.sh/*` prefix is reserved for operator-owned annotation keys —
			// applied-hash, apply-start-time, applied-generation, deletion-policy,
			// orphaned-*). lynq.sh/applied-generation in particular is stamped by a
			// separate MergePatch after each apply and would otherwise re-trigger
			// reconcile cascades that don't reflect any user-visible spec change.
			// Real user annotation changes (e.g., deployment.kubernetes.io/revision
			// written by the Deployment controller) still fire this predicate.
			if hasNonLynqAnnotationChange(oldObj.GetAnnotations(), newObj.GetAnnotations()) {
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

// buildAppliedResourceKeys builds a set of resource keys from current LynqNode.Spec.
// This uses a lightweight metadata extraction instead of full template rendering,
// since the key only needs kind/namespace/name/id — all available without rendering the spec body.
func (r *LynqNodeReconciler) buildAppliedResourceKeys(node *lynqv1.LynqNode) map[string]bool {
	keys := make(map[string]bool)

	allResources := r.collectResourcesFromLynqNode(node)

	for _, res := range allResources {
		key := r.buildResourceKeyLightweight(res, node)
		if key != "" {
			keys[key] = true
		}
	}

	return keys
}

// buildResourceKeyLightweight extracts a resource key without full spec rendering.
// NameTemplate, TargetNamespace, and Kind are already resolved by the Hub controller
// and stored directly in the TResource fields / Spec metadata.
func (r *LynqNodeReconciler) buildResourceKeyLightweight(res lynqv1.TResource, node *lynqv1.LynqNode) string {
	kind := res.Spec.GetKind()
	if kind == "" {
		return ""
	}

	name := res.NameTemplate // Already rendered by Hub controller

	namespace := node.Namespace
	if res.TargetNamespace != "" {
		namespace = res.TargetNamespace
	}

	return fmt.Sprintf("%s/%s/%s@%s", kind, namespace, name, res.ID)
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

	// F5: the resource is orphaned (removed from the template), so Lynq no
	// longer classifies it into a phase. Drop its per-resource metric series
	// now — otherwise lynqnode_resource_phase / replica gauges / degraded-since
	// for this resource_id linger until the whole LynqNode is deleted. Keyed by
	// the LynqNode's name/namespace (the metric labels), not the resource's
	// target namespace. Idempotent, and correct regardless of the delete/retain
	// outcome below since the resource is no longer part of the node.
	metrics.DeleteResourceSeries(node.Name, node.Namespace, resourceID, kind)

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
	applier := r.getApplier()
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
	case "Namespace", "ServiceAccount", "Service", resourceKindConfigMap, resourceKindSecret, "PersistentVolumeClaim":
		return "v1"
	case resourceKindDeployment, resourceKindStatefulSet, resourceKindDaemonSet:
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

	// 3b. Legacy rollback mode: always take the full path, exactly like the
	// pre-phase-model controller did. The lightweight status path's legacy
	// branch marks any not-ready resource as failed immediately (no timeout
	// grace), which would poison FailedResources and defeat the rollback
	// flag's "restore old behavior" contract.
	if r.LegacyReadinessStrict {
		return ReconcileTypeSpec
	}

	// 4. Check if spec changed (generation mismatch)
	// If observedGeneration doesn't match generation, it means spec changed
	// and we need a full reconcile to apply changes.
	if node.Status.ObservedGeneration != node.Generation {
		return ReconcileTypeSpec
	}

	// 4b. Database-driven variable change (M2). The Hub rewrites the
	// template-variable annotations (lynq.sh/uid, activate, extra, hubId,
	// template-generation) WITHOUT bumping metadata.generation. Detect that by
	// comparing the current variable hash against the last-observed one; a
	// mismatch means the rendered spec would change, so full reconcile.
	if computeVariablesHash(node) != node.Status.ObservedVariablesHash {
		return ReconcileTypeSpec
	}

	// 4c. Periodic drift-correction backstop (M2). The ~10-min force-reapply
	// sweep lives in reconcileSpec, gated by LastFullReconcileAt. A nil value
	// means no baseline yet (fresh node / restart) — take the full path so
	// reconcileSpec establishes it (stamp-now, defer-first-force). Otherwise,
	// once the interval elapses, take the full path so drift on child
	// resources whose applied-hash was preserved gets corrected.
	if node.Status.LastFullReconcileAt == nil ||
		time.Since(node.Status.LastFullReconcileAt.Time) >= ForceReapplyInterval {
		return ReconcileTypeSpec
	}

	// 5. Pure status-propagation event (M2): generation matches, variables
	// unchanged, no failures/degradation, and not yet due for a force-reapply.
	// This is an HPA scale / pod restart / rollout-progress status change on a
	// child resource — re-evaluate phases and update status via the lightweight
	// path WITHOUT re-rendering or re-applying. This is what keeps routine
	// third-party status churn from triggering unnecessary full applies.
	return ReconcileTypeStatus
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

		// Drop all Prometheus series for this LynqNode before removing the
		// finalizer — otherwise the deleted node's labels would persist in
		// Prometheus until cardinality rotation or controller restart. Pass
		// the LAST observed ResourcePhases so per-resource series can be
		// enumerated and dropped.
		r.StatusManager.CleanupNodeMetrics(client.ObjectKeyFromObject(node), node.Status.ResourcePhases)

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
		r.clearPhaseStateOnEarlyError(node) // F6
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
		r.clearPhaseStateOnEarlyError(node) // F6
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
		r.clearPhaseStateOnEarlyError(node) // F6
		return ctrl.Result{}, err
	}

	// Detect and cleanup orphaned resources
	currentKeys := r.buildAppliedResourceKeys(node)

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

	// Decide whether this reconcile should force a full reapply (bypassing the
	// annotation-based apply-skip in shouldSkipApply). This is Lynq's drift-correction
	// mechanism: every ForceReapplyInterval (default ~10 min), we re-apply every child
	// resource unconditionally to repair any external mutation that preserved the
	// lynq.sh/applied-hash annotation and would therefore not be detected by the
	// annotation-only skip check.
	//
	// Nil LastFullReconcileAt means we've never recorded one for this node — either it
	// is brand new or the controller just restarted. Treat nil as "defer the first
	// force by one full interval": stamp `now` as the baseline, set forceReapply=false
	// for this reconcile. Without this defer, every controller restart would force-
	// reapply every LynqNode on the next tick, producing a re-apply storm that could
	// regress the post-restart Ready window guarded by Bottleneck BUG 2.
	forceReapply := false
	if node.Status.LastFullReconcileAt == nil {
		now := metav1.Now()
		r.StatusManager.PublishLastFullReconcileAt(node, now)
	} else if time.Since(node.Status.LastFullReconcileAt.Time) >= ForceReapplyInterval {
		forceReapply = true
	}

	// Apply resources and track changes
	readyCount, failedCount, changedCount, conflictedCount, skippedCount, skippedIds,
		degradedCount, progressingCount, pendingCount,
		degradedResourceIds, resourcePhases, replicaMetrics := r.applyResources(ctx, node, sortedNodes, vars, forceReapply)
	totalResources := int32(len(sortedNodes))

	// If we just completed a force-reapply pass, advance the LastFullReconcileAt
	// timestamp so the next force fires one interval later. We update unconditionally
	// (regardless of failedCount) because the point of this timestamp is "we just ran
	// the full reconcile loop with skip disabled" — failure handling is orthogonal.
	if forceReapply {
		now := metav1.Now()
		r.StatusManager.PublishLastFullReconcileAt(node, now)
	}

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
		degradedCount,
		totalResources,
		appliedResourceKeys,
		false, // not progressing after reconciliation completes
	)

	// Update ObservedGeneration + ObservedVariablesHash to mark this generation
	// AND this set of template-variable annotations as reconciled. This is
	// critical to prevent repeated full reconciles: it lets determineReconcileType
	// route subsequent pure child-status events to the lightweight status path
	// (M2) while still catching Hub-driven variable changes.
	r.StatusManager.PublishObservedState(node, node.Generation, computeVariablesHash(node))

	// Publish all status fields at once through StatusManager.
	// PublishResourceCountsWithPhases is the phase-model extension of
	// PublishResourceCounts — same status path, plus the per-phase aggregate
	// counts (degraded, progressing, pending) on LynqNode.status.
	r.StatusManager.PublishResourceCountsWithPhases(node,
		statusUpdate.ReadyResources, statusUpdate.FailedResources, statusUpdate.DesiredResources, statusUpdate.ConflictedResources,
		degradedCount, progressingCount, pendingCount)
	r.StatusManager.PublishAppliedResources(node, statusUpdate.AppliedResources)
	// Publish per-resource phase array (drives status.resourcePhases +
	// status.degradedResourceIds + lynqnode_resource_phase stateset +
	// per-resource replica metrics).
	r.StatusManager.PublishResourcePhases(node, resourcePhases, degradedResourceIds, replicaMetrics)

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

	// Publish metrics — including new per-phase aggregate gauges.
	r.StatusManager.Publish(status.StatusEvent{
		Type:    status.EventMetricsUpdate,
		NodeKey: client.ObjectKeyFromObject(node),
		Payload: status.MetricsPayload{
			Ready:          readyCount,
			Failed:         failedCount,
			Desired:        totalResources,
			Conflicted:     conflictedCount,
			Degraded:       degradedCount,
			Progressing:    progressingCount,
			Pending:        pendingCount,
			Conditions:     statusUpdate.Conditions,
			IsDegraded:     statusUpdate.IsDegraded,
			DegradedReason: degradedReason,
		},
		Timestamp: time.Now(),
	})

	// Emit completion event if resources were changed
	if changedCount > 0 {
		r.emitTemplateAppliedCompleteEvent(ctx, node, totalResources, readyCount, failedCount, changedCount)
		logger.Info("Reconciliation completed with changes", "changed", changedCount, "ready", readyCount, "failed", failedCount, "conflicted", conflictedCount, "degraded", degradedCount, "progressing", progressingCount)
	} else {
		logger.V(1).Info("Reconciliation completed without changes", "ready", readyCount, "failed", failedCount, "conflicted", conflictedCount, "degraded", degradedCount, "progressing", progressingCount)
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
	readyCount, failedCount, conflictedCount,
		degradedCount, progressingCount, pendingCount,
		degradedResourceIds, resourcePhases, replicaMetrics := r.checkResourcesReadiness(ctx, node, allResources, vars)

	// Calculate complete status using centralized logic
	// Note: We don't have appliedResourceKeys here since this is status-only reconcile
	// Use existing status.appliedResources
	statusUpdate := r.calculateLynqNodeStatus(
		readyCount,
		failedCount,
		conflictedCount,
		degradedCount,
		totalResources,
		node.Status.AppliedResources, // Keep existing applied resources
		false,                        // not progressing
	)

	// Update ObservedGeneration to match current Generation
	r.StatusManager.PublishObservedGeneration(node, node.Generation)

	// Publish all status fields through StatusManager
	r.StatusManager.PublishResourceCountsWithPhases(node,
		statusUpdate.ReadyResources, statusUpdate.FailedResources, statusUpdate.DesiredResources, statusUpdate.ConflictedResources,
		degradedCount, progressingCount, pendingCount)
	r.StatusManager.PublishResourcePhases(node, resourcePhases, degradedResourceIds, replicaMetrics)
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

	// Publish metrics — including new per-phase aggregate gauges.
	r.StatusManager.Publish(status.StatusEvent{
		Type:    status.EventMetricsUpdate,
		NodeKey: client.ObjectKeyFromObject(node),
		Payload: status.MetricsPayload{
			Ready:          readyCount,
			Failed:         failedCount,
			Desired:        totalResources,
			Conflicted:     conflictedCount,
			Degraded:       degradedCount,
			Progressing:    progressingCount,
			Pending:        pendingCount,
			Conditions:     statusUpdate.Conditions,
			IsDegraded:     statusUpdate.IsDegraded,
			DegradedReason: degradedReason,
		},
		Timestamp: time.Now(),
	})

	// Record metrics
	metrics.LynqNodeReconcileDuration.WithLabelValues("status_only").Observe(time.Since(startTime).Seconds())

	logger.V(1).Info("Status-only reconcile completed",
		"node", node.Name,
		"ready", readyCount,
		"failed", failedCount,
		"conflicted", conflictedCount,
		"degraded", degradedCount,
		"progressing", progressingCount,
		"duration", time.Since(startTime).String())

	// Requeue after 5 minutes for periodic health check
	// Next change will trigger immediate reconcile via watch
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// checkResourcesReadiness checks the readiness of resources WITHOUT applying them
// This is much faster than applyResources as it only reads status — driven by
// the child-resource status watch (event-driven reconciliation).
//
// Returns aggregate phase counts and the per-resource phase entries / replica
// metrics for status.resourcePhases + per-resource gauges. Mirrors the return
// surface of applyResources except for the apply-specific values (changed,
// skipped).
func (r *LynqNodeReconciler) checkResourcesReadiness(
	ctx context.Context,
	node *lynqv1.LynqNode,
	resources []lynqv1.TResource,
	_ template.Variables,
) (
	readyCount, failedCount, conflictedCount int32,
	degradedCount, progressingCount, pendingCount int32,
	degradedResourceIds []string,
	resourcePhases []lynqv1.ResourcePhaseEntry,
	replicaMetrics map[string]status.ResourceReplicaMetrics,
) {
	logger := log.FromContext(ctx)
	checker := r.getReadinessChecker()
	previousEntries := buildPreviousPhasesMap(node)
	replicaMetrics = make(map[string]status.ResourceReplicaMetrics)
	// Non-nil so the status manager always overwrites status.degradedResourceIds
	// — otherwise a nil slice on recovery leaves the previous IDs stale (F4).
	degradedResourceIds = []string{}

	for _, resource := range resources {
		// Extract name/namespace without full spec rendering (metadata is already resolved by Hub)
		name := resource.NameTemplate
		namespace := node.Namespace
		if resource.TargetNamespace != "" {
			namespace = resource.TargetNamespace
		}
		gvk := resource.Spec.GroupVersionKind()

		if name == "" || gvk.Kind == "" {
			logger.V(1).Info("Resource missing name or kind for status check", "id", resource.ID)
			failedCount++
			continue
		}

		// Get current resource from cluster using a fresh empty object (no DeepCopy needed)
		current := &unstructured.Unstructured{}
		current.SetGroupVersionKind(gvk)
		err := r.Get(ctx, client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}, current)

		if err != nil {
			if errors.IsNotFound(err) {
				// Resource doesn't exist - count as failed
				logger.V(1).Info("Resource not found in cluster", "id", resource.ID, "name", name)
				failedCount++
				resourcePhases = append(resourcePhases, lynqv1.ResourcePhaseEntry{
					ID: resource.ID, Kind: gvk.Kind, Name: name,
					Phase: lynqv1.ResourcePhaseFailed, Reason: "resource not found in cluster",
				})
				continue
			}
			logger.Error(err, "Failed to get resource for status check", "id", resource.ID, "name", name)
			failedCount++
			resourcePhases = append(resourcePhases, lynqv1.ResourcePhaseEntry{
				ID: resource.ID, Kind: gvk.Kind, Name: name,
				Phase: lynqv1.ResourcePhaseFailed, Reason: fmt.Sprintf("failed to read live state: %v", err),
			})
			continue
		}

		// Check ownership conflict
		if r.hasOwnershipConflict(current, node) {
			logger.V(1).Info("Resource has ownership conflict", "id", resource.ID, "name", name)
			conflictedCount++
			failedCount++
			resourcePhases = append(resourcePhases, lynqv1.ResourcePhaseEntry{
				ID: resource.ID, Kind: gvk.Kind, Name: name,
				Phase: lynqv1.ResourcePhaseFailed, Reason: "ownership conflict — managed by another controller",
			})
			continue
		}

		// Skip non-waited resources (count as Ready, matches existing semantics).
		if resource.WaitForReady != nil && !*resource.WaitForReady {
			readyCount++
			resourcePhases = append(resourcePhases, lynqv1.ResourcePhaseEntry{
				ID: resource.ID, Kind: gvk.Kind, Name: name,
				Phase: lynqv1.ResourcePhaseAvailable, Reason: "waitForReady=false",
			})
			continue
		}

		timeoutSeconds := resource.TimeoutSeconds
		if timeoutSeconds <= 0 {
			timeoutSeconds = 300
		}
		timeoutDuration := time.Duration(timeoutSeconds) * time.Second
		elapsed := elapsedSinceApply(current, time.Now())

		if r.LegacyReadinessStrict {
			// LEGACY PATH — strict equality, no phase model.
			if checker.IsReady(current) {
				readyCount++
			} else {
				failedCount++
			}
			continue
		}

		phaseResult := checker.ClassifyPhase(current, elapsed, timeoutDuration)
		prevEntry := previousEntries[resource.ID]
		entry := r.processResourcePhase(ctx, node, resource, name, gvk.Kind, phaseResult, prevEntry, elapsed)

		switch phaseResult.Phase {
		case lynqv1.ResourcePhaseAvailable:
			readyCount++
		case lynqv1.ResourcePhaseDegraded:
			readyCount++
			degradedCount++
			degradedResourceIds = append(degradedResourceIds, resource.ID)
		case lynqv1.ResourcePhaseProgressing:
			progressingCount++
		case lynqv1.ResourcePhasePending:
			pendingCount++
		case lynqv1.ResourcePhaseFailed:
			failedCount++
		}

		replicaMetrics[resource.ID] = status.ResourceReplicaMetrics{
			Kind:                 gvk.Kind,
			Desired:              phaseResult.Replicas.Desired,
			Available:            phaseResult.Replicas.Available,
			Ready:                phaseResult.Replicas.Ready,
			Updated:              phaseResult.Replicas.Updated,
			DegradedSinceSeconds: entry.SinceSeconds,
		}
		resourcePhases = append(resourcePhases, entry)
	}

	return readyCount, failedCount, conflictedCount,
		degradedCount, progressingCount, pendingCount,
		degradedResourceIds, resourcePhases, replicaMetrics
}

// elapsedSinceApply returns how long it has been since the operator last applied obj.
// It reads lynq.sh/apply-start-time from the resource's annotations — set by the Applier
// at apply time and preserved across reconcile loops. This gives a stable reference for
// readiness-timeout calculations that would otherwise reset with applyStartTime every reconcile.
// fallback is used on the very first reconcile of a resource, before the annotation is set.
func elapsedSinceApply(obj client.Object, fallback time.Time) time.Duration {
	if annotations := obj.GetAnnotations(); annotations != nil {
		if raw := annotations[apply.AnnotationApplyStartTime]; raw != "" {
			if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
				return time.Since(t)
			}
		}
	}
	return time.Since(fallback)
}

// hasNonLynqAnnotationChange reports whether the user-visible annotations on a watched
// child resource have changed. It deliberately ignores the `lynq.sh/*` annotation
// namespace, which is reserved for keys the Applier writes (applied-hash,
// apply-start-time, deletion-policy, orphaned-*). Defense-in-depth filter: today
// applied-hash and apply-start-time are bundled into the apply call itself (no
// follow-up MergePatch), so they wouldn't trigger this watch anyway, but the
// filter keeps the contract clear that `lynq.sh/*` is an operator-owned namespace
// not part of user-visible change detection.
func hasNonLynqAnnotationChange(oldAnno, newAnno map[string]string) bool {
	return !reflect.DeepEqual(stripLynqAnnotations(oldAnno), stripLynqAnnotations(newAnno))
}

// stripLynqAnnotations returns a copy of m with all lynq.sh/* keys removed.
// Returns nil for nil input AND for results that are empty after stripping, so
// DeepEqual compares two "no user-visible annotations" cases as equal regardless
// of whether one map was nil and the other contained only lynq.sh keys.
func stripLynqAnnotations(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	var out map[string]string
	for k, v := range m {
		if strings.HasPrefix(k, "lynq.sh/") {
			continue
		}
		if out == nil {
			out = make(map[string]string, len(m))
		}
		out[k] = v
	}
	return out
}
