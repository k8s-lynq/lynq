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

package apply

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
	"github.com/k8s-lynq/lynq/internal/fieldfilter"
)

const (
	// FieldManager is the name used for Server-Side Apply
	FieldManager = "lynq-operator"

	// AnnotationAppliedHash stores the desired spec hash from the last successful apply.
	// Used for cross-restart cache restoration: a new Applier (after controller restart)
	// can read this annotation to skip re-applying unchanged resources.
	AnnotationAppliedHash = "lynq.sh/applied-hash"

	// AnnotationApplyStartTime records when the operator last successfully applied this resource.
	// Used as a stable reference for readiness-timeout calculations across reconcile loops.
	// Without this, applyStartTime resets every reconcile so resources that need multiple
	// reconcile loops to time out (e.g., Deployments with bad images) would never time out.
	AnnotationApplyStartTime = "lynq.sh/apply-start-time"

	// Labels for cross-namespace resource tracking
	LabelNodeName      = "lynq.sh/node"
	LabelNodeNamespace = "lynq.sh/node-namespace"

	// Label for orphaned resources (DeletionPolicy=Retain) - used for selectors
	LabelOrphaned = "lynq.sh/orphaned"

	// Annotations for orphaned resources - detailed information
	AnnotationOrphanedAt     = "lynq.sh/orphaned-at"
	AnnotationOrphanedReason = "lynq.sh/orphaned-reason"

	// Annotation for storing DeletionPolicy on resources
	AnnotationDeletionPolicy = "lynq.sh/deletion-policy"

	// OrphanedLabelValue is the value for orphaned label
	OrphanedLabelValue = "true"
)

// ConflictError represents a resource conflict error
type ConflictError struct {
	ResourceName string
	Namespace    string
	Kind         string
	Err          error
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("resource conflict for %s/%s (%s): %v", e.Namespace, e.ResourceName, e.Kind, e.Err)
}

func (e *ConflictError) Unwrap() error {
	return e.Err
}

// appliedState tracks the last successful apply for a resource to skip no-op PATCHes.
type appliedState struct {
	desiredHash     string // hash of the desired spec we last applied
	resourceVersion string // post-apply resourceVersion from API server
	// generation is the spec-version-counter the API server assigned after our last apply.
	// It only increments on spec changes (status/finalizer/annotation-only writes don't bump
	// it), so it is the right signal to detect external drift while ignoring benign RV bumps.
	// We compare cached generation against the live generation; mismatch ⇒ external spec
	// change ⇒ must re-apply.
	generation int64
}

// Applier handles Server-Side Apply operations
type Applier struct {
	client    client.Client
	scheme    *runtime.Scheme
	appliedRV sync.Map // key: "kind/ns/name" → *appliedState
}

// NewApplier creates a new Applier
func NewApplier(c client.Client, scheme *runtime.Scheme) *Applier {
	return &Applier{
		client: c,
		scheme: scheme,
	}
}

// ApplyResource applies a resource using the specified patch strategy
// Returns true if the resource was changed, false if no change was needed
// ignoreFields: JSONPath expressions for fields to ignore (only effective on existing resources, not initial creation)
func (a *Applier) ApplyResource(
	ctx context.Context,
	obj *unstructured.Unstructured,
	owner *lynqv1.LynqNode,
	conflictPolicy lynqv1.ConflictPolicy,
	patchStrategy lynqv1.PatchStrategy,
	deletionPolicy lynqv1.DeletionPolicy,
	ignoreFields []string,
) (bool, error) {
	// Set owner reference or tracking labels based on namespace and deletion policy
	if owner != nil {
		isCrossNamespace := obj.GetNamespace() != owner.Namespace
		isNamespaceResource := obj.GetKind() == "Namespace"
		isRetainPolicy := deletionPolicy == lynqv1.DeletionPolicyRetain

		// Use label-based tracking for:
		// 1. Cross-namespace resources (ownerReferences don't work across namespaces)
		// 2. Namespace resources (cannot have ownerReferences)
		// 3. Retain policy resources (to prevent automatic deletion by garbage collector)
		if isCrossNamespace || isNamespaceResource || isRetainPolicy {
			// Use label-based tracking instead of ownerReference
			labels := obj.GetLabels()
			if labels == nil {
				labels = make(map[string]string)
			}
			labels[LabelNodeName] = owner.Name
			labels[LabelNodeNamespace] = owner.Namespace
			obj.SetLabels(labels)
		} else {
			// For same-namespace resources with Delete policy, use traditional ownerReference
			// This enables automatic garbage collection when LynqNode is deleted
			if err := controllerutil.SetControllerReference(owner, obj, a.scheme); err != nil {
				return false, fmt.Errorf("failed to set owner reference: %w", err)
			}
		}
	}

	// Get the existing resource to check for changes
	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(obj.GroupVersionKind())
	existsBeforeApply := true
	beforeResourceVersion := ""

	// orphanReadopted is true when we just stripped orphan markers off a retained resource
	// that is re-entering management. In that case ALL skip paths must be bypassed so we
	// re-stamp ownerReferences/tracking labels/applied-hash/applied-generation correctly,
	// even if the desired spec happens to hash to the same value as the resource carries.
	orphanReadopted := false
	if err := a.client.Get(ctx, key, existing); err != nil {
		if errors.IsNotFound(err) {
			existsBeforeApply = false
		} else {
			return false, fmt.Errorf("failed to get existing resource: %w", err)
		}
	} else {
		beforeResourceVersion = existing.GetResourceVersion()

		// Remove orphan markers if present (resource is being re-added to management)
		// This must be done on the actual cluster resource, not just the in-memory object
		removed, err := a.removeOrphanMarkersFromCluster(ctx, existing)
		if err != nil {
			// Log but don't fail - orphan markers are metadata, not critical
			// The resource will still be applied correctly
			logger := log.FromContext(ctx)
			logger.V(1).Info("Failed to remove orphan markers, continuing anyway", "error", err)
		}

		// If orphan markers were removed, the cluster resource's RV and managedFields
		// have changed. We need to re-Get `existing` so its resourceVersion is current
		// — otherwise the subsequent Replace path (which sets obj.RV from existing.RV)
		// would fail with a 409 Conflict against the API server. We also force-apply
		// (orphanReadopted) so the re-adoption is properly recorded even when the
		// desired hash happens to match what was on the orphaned resource.
		if removed {
			refreshed := &unstructured.Unstructured{}
			refreshed.SetGroupVersionKind(obj.GroupVersionKind())
			if err := a.client.Get(ctx, key, refreshed); err != nil {
				return false, fmt.Errorf("failed to re-get resource after orphan-marker removal: %w", err)
			}
			existing = refreshed
			beforeResourceVersion = existing.GetResourceVersion()
			orphanReadopted = true
		}
	}

	// Apply ignoreFields filtering if resource already exists
	// Instead of removing ignored fields, we COPY values from the existing resource.
	// This ensures SSA doesn't delete fields that should be preserved.
	if existsBeforeApply && len(ignoreFields) > 0 {
		filter, err := fieldfilter.NewFilter(ignoreFields)
		if err != nil {
			return false, fmt.Errorf("failed to create field filter: %w", err)
		}

		// Preserve ignored fields by copying values from existing resource
		// This ensures SSA doesn't delete fields that are externally controlled
		if err := filter.PreserveIgnoredFields(obj, existing); err != nil {
			return false, fmt.Errorf("failed to preserve ignored fields: %w", err)
		}

		logger := log.FromContext(ctx)
		logger.V(1).Info("Preserved ignored fields from existing resource",
			"resource", fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName()),
			"kind", obj.GetKind(),
			"ignoreFields", ignoreFields)
	}

	// Check if we can skip the apply (desired spec unchanged AND resource not externally modified).
	// The hash is computed from the PRE-patch desired object (before server mutations).
	rvKey := fmt.Sprintf("%s/%s/%s", obj.GetKind(), obj.GetNamespace(), obj.GetName())
	preApplyDesiredHash := a.computeDesiredHash(obj)
	if a.shouldSkipApply(existsBeforeApply, orphanReadopted, preApplyDesiredHash, beforeResourceVersion, rvKey, existing) {
		return false, nil
	}

	// Decide once whether this apply should reset apply-start-time. We're past the
	// skip checks above, so we're definitely going to write — the question is only
	// whether this is a real spec change (reset the readiness-timeout clock) or a
	// re-stamp of a spec already carried by the resource (preserve the clock so a
	// resource that's been progressing through a rolling update doesn't get its
	// timeout reset every reconcile).
	specChanged := true
	if existsBeforeApply {
		if stored, ok := existing.GetAnnotations()[AnnotationAppliedHash]; ok && stored == preApplyDesiredHash {
			specChanged = false
		}
	}

	// Bundle the tracking annotations INTO the desired object so the spec and the
	// applied-hash + apply-start-time annotations land atomically in one API call.
	// Doing this avoids the race that caused the maxSkew cascade: previously a
	// separate persistAppliedHash MergePatch followed the SSA, and the SSA's watch
	// event could be processed by the informer before the MergePatch's. A second
	// reconcile would then see new spec but OLD applied-hash, falsely conclude
	// the spec was unapplied, and re-issue the SSA — bumping generation an
	// extra time and starving the hub's maxSkew enforcement.
	setLynqTrackingAnnotations(obj, preApplyDesiredHash, specChanged, existing.GetAnnotations())

	// Apply (per strategy) is delegated to keep this function under the gocyclo limit.
	// applyByStrategy may early-return when it creates a brand-new resource; in that case
	// it returns earlyReturn=true and the caller propagates the result.
	earlyReturn, changed, err := a.applyByStrategy(ctx, applyArgs{
		obj:                 obj,
		existing:            existing,
		patchStrategy:       patchStrategy,
		conflictPolicy:      conflictPolicy,
		existsBeforeApply:   existsBeforeApply,
		rvKey:               rvKey,
		preApplyDesiredHash: preApplyDesiredHash,
	})
	if err != nil || earlyReturn {
		return changed, err
	}

	// Check if resource was actually changed by comparing resourceVersion.
	// Patch/Update/Create responses update obj in-place with the server response,
	// so obj.GetResourceVersion() reflects the post-apply state without an extra GET.
	afterResourceVersion := obj.GetResourceVersion()
	changed = beforeResourceVersion != afterResourceVersion

	// Store applied state in-memory for fast skip detection within this process.
	// Use preApplyDesiredHash (computed before Patch mutates obj with server response)
	// so that the next reconcile's skip check compares like-with-like.
	// generation is tracked in-memory only: it lets the fast-path skip survive benign
	// status/finalizer RV bumps without an extra post-apply API write (which would
	// re-introduce the race that commit 7b629e4 fixed).
	a.appliedRV.Store(rvKey, &appliedState{
		desiredHash:     preApplyDesiredHash,
		resourceVersion: afterResourceVersion,
		generation:      obj.GetGeneration(),
	})

	return changed, nil
}

// shouldSkipApply decides whether the desired spec is already on the resource (so the
// apply call can be elided). It returns true only when we can prove the live spec
// matches what we previously applied:
//
//   - In-memory fast path: the cache says we wrote this hash AND (the RV hasn't been
//     bumped at all, or the metadata.generation hasn't changed since we wrote it).
//     metadata.generation is the drift-resistant signal — it only bumps on real spec
//     changes, so benign status/finalizer/annotation writes don't invalidate the match.
//     This is what protects patchStrategy:replace from the infinite reconcile loop:
//     the K8s deployment controller's status updates bump RV but not generation, so
//     the second OR clause keeps us on the skip path.
//
//   - Annotation slow path (cross-restart cache restoration): applied-hash matches
//     the desired hash. This is hash-only — we accept that an external `kubectl
//     edit` that leaves applied-hash untouched will not be self-healed across a
//     controller restart, until the in-memory generation tracking rebuilds. This
//     trade-off mirrors main's pre-3efafeb behavior; tightening it requires a
//     post-apply write that re-introduces the race fixed by 7b629e4.
//
// Returns false when we MUST apply: the resource was re-adopted from orphaned state,
// the hash couldn't be computed, the resource doesn't exist yet, or any of the proofs
// above failed.
func (a *Applier) shouldSkipApply(
	existsBeforeApply, orphanReadopted bool,
	preApplyDesiredHash, beforeResourceVersion, rvKey string,
	existing *unstructured.Unstructured,
) bool {
	// Bail out of all skip paths when:
	//   - the resource doesn't exist yet (must create),
	//   - we just re-adopted from an orphaned state (must re-stamp owner/labels),
	//   - hash computation failed (force apply; an empty hash would otherwise
	//     spuriously match a missing applied-hash annotation).
	if !existsBeforeApply || orphanReadopted || preApplyDesiredHash == "" {
		return false
	}

	// Fast path: in-memory cache hit. When the cache has an entry whose desiredHash
	// matches, the in-memory generation tracking is AUTHORITATIVE for this process —
	// generation mismatch ⇒ drift ⇒ re-apply. Falling through to the slow path on a
	// cache-hit-with-drift would mask the drift behind the slow path's hash-only
	// annotation check.
	if prev, ok := a.appliedRV.Load(rvKey); ok {
		state := prev.(*appliedState)
		if state.desiredHash != preApplyDesiredHash {
			// Template hash changed since we last applied — must re-apply.
			return false
		}
		// Hash matches. Skip iff RV matches (no external write at all) OR generation
		// matches (RV bumped by benign status/finalizer/annotation writes but spec
		// untouched). The generation comparison is the in-process drift detector.
		return state.resourceVersion == beforeResourceVersion || state.generation == existing.GetGeneration()
	}

	// Slow path: hash-only annotation fallback for cross-restart cache restoration.
	// Refreshes the in-memory cache so subsequent reconciles in this process can take
	// the fast path (and benefit from generation-based drift detection).
	//
	// Trade-off: an external mutation that bumps live spec but leaves applied-hash
	// untouched is NOT detected here. It will be caught by the fast path once the
	// in-memory cache rebuilds (on the next template change or controller activity).
	// Stamping generation into an annotation would close this gap but requires a
	// post-apply MergePatch that re-introduces the race fixed by commit 7b629e4.
	annotations := existing.GetAnnotations()
	if annotations == nil {
		return false
	}
	storedHash, hashOK := annotations[AnnotationAppliedHash]
	if !hashOK || storedHash != preApplyDesiredHash {
		return false
	}
	a.appliedRV.Store(rvKey, &appliedState{
		desiredHash:     preApplyDesiredHash,
		resourceVersion: beforeResourceVersion,
		generation:      existing.GetGeneration(),
	})
	return true
}

// applyArgs bundles ApplyResource's per-call state so applyByStrategy can keep a small
// signature without losing context. Used purely as a parameter object.
type applyArgs struct {
	obj                 *unstructured.Unstructured
	existing            *unstructured.Unstructured
	patchStrategy       lynqv1.PatchStrategy
	conflictPolicy      lynqv1.ConflictPolicy
	existsBeforeApply   bool
	rvKey               string
	preApplyDesiredHash string
}

// applyByStrategy dispatches the actual API write based on patchStrategy.
// Returns (earlyReturn, changed, error):
//   - earlyReturn=true when the strategy did a Create (resource didn't exist before).
//     The caller should propagate (changed, error) without computing RV diff or storing
//     the cache twice — applyByStrategy already populated the cache in that path.
//   - earlyReturn=false on Patch/Update; the caller computes RV diff and stores cache.
func (a *Applier) applyByStrategy(ctx context.Context, args applyArgs) (bool, bool, error) {
	switch args.patchStrategy {
	case lynqv1.PatchStrategyApply, "":
		force := args.conflictPolicy == lynqv1.ConflictPolicyForce
		if err := a.client.Patch(ctx, args.obj, client.Apply, &client.PatchOptions{
			FieldManager: FieldManager,
			Force:        &force,
		}); err != nil {
			if errors.IsConflict(err) && args.conflictPolicy == lynqv1.ConflictPolicyStuck {
				return false, false, &ConflictError{
					ResourceName: args.obj.GetName(),
					Namespace:    args.obj.GetNamespace(),
					Kind:         args.obj.GetKind(),
					Err:          err,
				}
			}
			return false, false, fmt.Errorf("failed to apply resource: %w", err)
		}
		return false, false, nil

	case lynqv1.PatchStrategyMerge:
		if !args.existsBeforeApply {
			if err := a.client.Create(ctx, args.obj); err != nil {
				return true, false, fmt.Errorf("failed to create resource for merge: %w", err)
			}
			a.appliedRV.Store(args.rvKey, &appliedState{
				desiredHash:     args.preApplyDesiredHash,
				resourceVersion: args.obj.GetResourceVersion(),
				generation:      args.obj.GetGeneration(),
			})
			return true, true, nil
		}
		if err := a.client.Patch(ctx, args.obj, client.Merge); err != nil {
			return false, false, fmt.Errorf("failed to merge resource: %w", err)
		}
		return false, false, nil

	case lynqv1.PatchStrategyReplace:
		if !args.existsBeforeApply {
			if err := a.client.Create(ctx, args.obj); err != nil {
				return true, false, fmt.Errorf("failed to create resource: %w", err)
			}
			a.appliedRV.Store(args.rvKey, &appliedState{
				desiredHash:     args.preApplyDesiredHash,
				resourceVersion: args.obj.GetResourceVersion(),
				generation:      args.obj.GetGeneration(),
			})
			return true, true, nil
		}
		args.obj.SetResourceVersion(args.existing.GetResourceVersion())
		if err := a.client.Update(ctx, args.obj); err != nil {
			return false, false, fmt.Errorf("failed to replace resource: %w", err)
		}
		return false, false, nil

	default:
		return false, false, fmt.Errorf("unsupported patch strategy: %s", args.patchStrategy)
	}
}

// setLynqTrackingAnnotations stamps applied-hash and apply-start-time directly onto obj's
// metadata BEFORE the apply call, so the spec and our tracking annotations land atomically
// in a single API request. This eliminates the race that occurred when a separate
// MergePatch followed the apply: the SSA's watch event could be processed by the cache
// informer before the MergePatch's watch event, so a re-triggered reconcile would see
// "new spec but old applied-hash" and falsely re-apply.
//
// applyStart should be true when the spec is actually changing (i.e., stored applied-hash
// is absent or doesn't match the new hash). When false, the existing apply-start-time on
// the resource is preserved so the readiness-timeout clock keeps accumulating elapsed
// time across reconcile loops.
func setLynqTrackingAnnotations(obj *unstructured.Unstructured, hash string, applyStart bool, existingAnnotations map[string]string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[AnnotationAppliedHash] = hash
	if applyStart {
		annotations[AnnotationApplyStartTime] = time.Now().UTC().Format(time.RFC3339Nano)
	} else if existing := existingAnnotations[AnnotationApplyStartTime]; existing != "" {
		// Preserve the prior apply-start-time so the readiness-timeout clock keeps
		// accumulating. Without this, every re-apply of an unchanged spec would
		// reset the clock and prevent the timeout from ever firing.
		annotations[AnnotationApplyStartTime] = existing
	}
	obj.SetAnnotations(annotations)
}

// computeDesiredHash produces a lightweight hash of the desired spec for skip detection.
// The hash MUST exclude any annotations we write ourselves (applied-hash, apply-start-time),
// otherwise the hash would change every time we re-apply (because we set apply-start-time
// to now), defeating the cache check on the next reconcile.
func (a *Applier) computeDesiredHash(obj *unstructured.Unstructured) string {
	// Use JSON serialization of the spec for a deterministic hash.
	// This is called at most once per resource per reconcile (on apply or skip-check).
	// We marshal a sanitized COPY so the caller's obj is not mutated.
	sanitized := sanitizeObjectForHashing(obj.Object)
	data, err := json.Marshal(sanitized)
	if err != nil {
		return "" // Force apply on hash failure
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8]) // 16-char hex, sufficient for change detection
}

// sanitizeObjectForHashing returns a shallow-copied view of obj with our tracking
// annotations removed so the hash is stable across reconciles. We only need to copy
// the metadata.annotations map to avoid mutating the caller's data — the rest of obj
// is referenced as-is.
func sanitizeObjectForHashing(obj map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(obj))
	for k, v := range obj {
		out[k] = v
	}
	metaMap, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return out
	}
	annos, ok := metaMap["annotations"].(map[string]interface{})
	if !ok || len(annos) == 0 {
		return out
	}
	// Copy metadata to avoid mutating the original, then strip our keys.
	newMeta := make(map[string]interface{}, len(metaMap))
	for k, v := range metaMap {
		newMeta[k] = v
	}
	newAnnos := make(map[string]interface{}, len(annos))
	for k, v := range annos {
		// Strip every annotation we write ourselves so the desired-spec hash stays
		// stable across reconciles. Without this, our own annotations would be hashed
		// alongside the desired spec, the hash would change every time we re-stamp
		// apply-start-time, and the cache check would never hit.
		if k == AnnotationAppliedHash || k == AnnotationApplyStartTime {
			continue
		}
		newAnnos[k] = v
	}
	if len(newAnnos) == 0 {
		delete(newMeta, "annotations")
	} else {
		newMeta["annotations"] = newAnnos
	}
	out["metadata"] = newMeta
	return out
}

// DeleteResource deletes a resource respecting deletion policy
func (a *Applier) DeleteResource(
	ctx context.Context,
	obj *unstructured.Unstructured,
	policy lynqv1.DeletionPolicy,
	orphanReason string,
) error {
	if policy == lynqv1.DeletionPolicyRetain {
		// Remove owner references and tracking labels but keep the resource
		// Add orphan labels to mark it as retained orphan
		return a.removeOwnerReferencesAndLabels(ctx, obj, orphanReason)
	}

	// Delete the resource
	if err := a.client.Delete(ctx, obj); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete resource: %w", err)
	}

	return nil
}

// GetResource retrieves a resource from the cluster
func (a *Applier) GetResource(
	ctx context.Context,
	name, namespace string,
	obj *unstructured.Unstructured,
) error {
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	if err := a.client.Get(ctx, key, obj); err != nil {
		return err
	}

	return nil
}

// removeOwnerReferencesAndLabels removes all owner references and tracking labels from the resource
// and adds orphan labels to mark it as a retained orphan resource
func (a *Applier) removeOwnerReferencesAndLabels(ctx context.Context, obj *unstructured.Unstructured, orphanReason string) error {
	// Get current resource
	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	current := obj.DeepCopy()
	if err := a.client.Get(ctx, key, current); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// Remove owner references
	current.SetOwnerReferences(nil)

	// Update labels: remove tracking labels and add orphan label
	labels := current.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	// Remove tracking labels
	delete(labels, LabelNodeName)
	delete(labels, LabelNodeNamespace)

	// Add orphan label (for selector queries)
	labels[LabelOrphaned] = OrphanedLabelValue

	current.SetLabels(labels)

	// Update annotations: add orphan metadata
	annotations := current.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Add orphan annotations with timestamp and reason
	annotations[AnnotationOrphanedAt] = metav1.Now().Format(time.RFC3339)
	if orphanReason != "" {
		annotations[AnnotationOrphanedReason] = orphanReason
	}

	current.SetAnnotations(annotations)

	// Update the resource
	if err := a.client.Update(ctx, current); err != nil {
		return fmt.Errorf("failed to remove owner references and labels: %w", err)
	}

	// Log the orphaning
	logger := log.FromContext(ctx)
	logger.Info("Orphan markers added - resource retained",
		"kind", current.GetKind(),
		"name", current.GetName(),
		"namespace", current.GetNamespace(),
		"reason", orphanReason)

	// Create event on the resource
	message := fmt.Sprintf("Resource retained with orphan markers (reason: %s)", orphanReason)
	a.createEventForResource(ctx, current, corev1.EventTypeNormal, "OrphanMarkersAdded", message)

	return nil
}

// removeOrphanMarkersFromCluster removes orphan label and annotations from a cluster resource
// This is called when a previously orphaned resource is being re-added to management
// Returns true if markers were removed and resource was updated
func (a *Applier) removeOrphanMarkersFromCluster(ctx context.Context, obj *unstructured.Unstructured) (bool, error) {
	// Check if orphan markers are present
	labels := obj.GetLabels()
	annotations := obj.GetAnnotations()

	hasOrphanLabel := labels != nil && labels[LabelOrphaned] == OrphanedLabelValue
	hasOrphanAnnotations := annotations != nil && (annotations[AnnotationOrphanedAt] != "" || annotations[AnnotationOrphanedReason] != "")

	// If no orphan markers, nothing to do
	if !hasOrphanLabel && !hasOrphanAnnotations {
		return false, nil
	}

	// Get the current resource from cluster
	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
	current := obj.DeepCopy()
	if err := a.client.Get(ctx, key, current); err != nil {
		if errors.IsNotFound(err) {
			// Resource doesn't exist, nothing to clean
			return false, nil
		}
		return false, fmt.Errorf("failed to get resource for orphan marker cleanup: %w", err)
	}

	// Track if we made changes
	changed := false

	// Remove orphan label
	labels = current.GetLabels()
	if labels != nil && labels[LabelOrphaned] == OrphanedLabelValue {
		delete(labels, LabelOrphaned)
		current.SetLabels(labels)
		changed = true
	}

	// Remove orphan annotations
	annotations = current.GetAnnotations()
	if annotations != nil {
		if annotations[AnnotationOrphanedAt] != "" || annotations[AnnotationOrphanedReason] != "" {
			delete(annotations, AnnotationOrphanedAt)
			delete(annotations, AnnotationOrphanedReason)
			current.SetAnnotations(annotations)
			changed = true
		}
	}

	// Update the resource if we made changes
	if changed {
		if err := a.client.Update(ctx, current); err != nil {
			return false, fmt.Errorf("failed to remove orphan markers: %w", err)
		}

		// Log the re-adoption
		logger := log.FromContext(ctx)
		logger.Info("Orphan markers removed - resource re-adopted into management",
			"kind", current.GetKind(),
			"name", current.GetName(),
			"namespace", current.GetNamespace())

		// Create event on the resource
		a.createEventForResource(ctx, current, corev1.EventTypeNormal, "OrphanMarkersRemoved",
			"Resource re-adopted into management - orphan markers removed")
	}

	return changed, nil
}

// createEventForResource creates a Kubernetes Event for a resource
func (a *Applier) createEventForResource(ctx context.Context, obj *unstructured.Unstructured, eventType, reason, message string) {
	logger := log.FromContext(ctx)

	// Create Event object
	now := metav1.Now()
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s.%x", obj.GetName(), now.Unix()),
			Namespace: obj.GetNamespace(),
		},
		InvolvedObject: corev1.ObjectReference{
			APIVersion: obj.GetAPIVersion(),
			Kind:       obj.GetKind(),
			Name:       obj.GetName(),
			Namespace:  obj.GetNamespace(),
			UID:        obj.GetUID(),
		},
		Reason:  reason,
		Message: message,
		Source: corev1.EventSource{
			Component: "lynq-operator",
		},
		FirstTimestamp: now,
		LastTimestamp:  now,
		Count:          1,
		Type:           eventType,
	}

	// Try to create the event
	if err := a.client.Create(ctx, event); err != nil {
		// Log but don't fail - events are best-effort
		logger.V(1).Info("Failed to create event for resource",
			"kind", obj.GetKind(),
			"name", obj.GetName(),
			"reason", reason,
			"error", err.Error())
	}
}

// ResourceExists checks if a resource exists
func (a *Applier) ResourceExists(ctx context.Context, name, namespace string, obj *unstructured.Unstructured) (bool, error) {
	err := a.GetResource(ctx, name, namespace, obj)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// IsResourceReady checks if a resource is ready (basic check using status.conditions)
func IsResourceReady(obj *unstructured.Unstructured) bool {
	// Try to get status.conditions
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		// No conditions found, check if it's a simple resource type
		return isSimpleResourceReady(obj)
	}

	// Check for Ready condition
	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _, _ := unstructured.NestedString(condMap, "type")
		condStatus, _, _ := unstructured.NestedString(condMap, "status")

		if condType == "Ready" && condStatus == string(metav1.ConditionTrue) {
			return true
		}
	}

	return false
}

// isSimpleResourceReady checks readiness for resources without conditions
func isSimpleResourceReady(obj *unstructured.Unstructured) bool {
	gvk := obj.GroupVersionKind()

	switch gvk.Kind {
	case "Namespace", "ConfigMap", "Secret", "Service", "ServiceAccount":
		// These resources are ready immediately after creation
		return true
	case "Deployment":
		return isDeploymentReady(obj)
	case "StatefulSet":
		return isStatefulSetReady(obj)
	case "Job":
		return isJobReady(obj)
	default:
		// Unknown resource type, assume ready if it exists
		return true
	}
}

// isDeploymentReady checks if a Deployment is ready
func isDeploymentReady(obj *unstructured.Unstructured) bool {
	generation, _, _ := unstructured.NestedInt64(obj.Object, "metadata", "generation")
	observedGeneration, _, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")

	if generation != observedGeneration {
		return false
	}

	replicas, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	availableReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "availableReplicas")

	return availableReplicas >= replicas
}

// isStatefulSetReady checks if a StatefulSet is ready
func isStatefulSetReady(obj *unstructured.Unstructured) bool {
	replicas, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	readyReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")

	return readyReplicas >= replicas
}

// isJobReady checks if a Job is complete
func isJobReady(obj *unstructured.Unstructured) bool {
	succeeded, _, _ := unstructured.NestedInt64(obj.Object, "status", "succeeded")
	return succeeded > 0
}

// GetResourceMetadata extracts metadata from an unstructured object
func GetResourceMetadata(obj *unstructured.Unstructured) (name, namespace, kind string, err error) {
	name = obj.GetName()
	namespace = obj.GetNamespace()

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return "", "", "", err
	}

	gvk := obj.GroupVersionKind()
	kind = gvk.Kind

	_ = accessor // Use accessor if needed

	return name, namespace, kind, nil
}
