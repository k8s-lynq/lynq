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
	"fmt"
	"sort"
	"strings"
	"time"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
)

// defaultProgressDeadlineSeconds mirrors the CRD default of rollout.progressDeadlineSeconds for
// LynqForms that reach the controller without API-server defaulting.
const defaultProgressDeadlineSeconds = 600

// maxStalledNodesInMessage caps how many node names the rollout message lists, so a rollout
// that stalls across hundreds of nodes does not grow the LynqForm status without bound.
const maxStalledNodesInMessage = 5

// The rollout progress of an updated LynqNode is read from durable state only, so a controller
// restart or leader change sees exactly what the previous process saw:
//
//   - lynq.sh/rollout-update-start: when the hub last updated the node.
//   - lynq.sh/rollout-completed-generation: whether that update completed. The hub clears it in
//     the write that updates the node and sets it to the template generation once it observes
//     the node Ready for its current spec (recordRolloutCompletion).
//
// progressDeadlineSeconds measures only "the update never completed". A node that completed and
// later regressed (spot drain, external `kubectl rollout restart`) carries an update stamp as old
// as the update, and timing the regression against it would fail the rollout the instant the
// node was seen not ready. The completion marker is what exempts it.

// updateCompleted reports whether the node's current update has completed, according to the
// completion marker the hub keeps. A node without the marker predates it: it is treated as
// completed, so an upgrade cannot fail a rollout over a node that regressed before it. Its
// stuck updates still stall the rollout once the LynqNode reports them failed.
func updateCompleted(node *lynqv1.LynqNode) bool {
	completed, ok := node.Annotations[lynqv1.AnnotationRolloutCompletedGeneration]
	if !ok {
		return true
	}
	return completed != "" && completed == node.Annotations[lynqv1.AnnotationTemplateGeneration]
}

// reachedReadyForCurrentSpec reports whether the node is Ready for the spec it carries now.
//
// The LynqNode controller must have observed the current generation: right after the hub
// updates a node, its Ready condition still describes the previous spec, and counting that as
// completion would exempt a stuck update from the deadline. A spec-identical update (for example
// a LynqForm change that only touches rollout settings) does not move the generation, and the
// node rightly counts as complete at once.
func reachedReadyForCurrentSpec(node *lynqv1.LynqNode) bool {
	return classifyNode(node) == nodeHealthReady && node.Status.ObservedGeneration == node.Generation
}

// stallReason says why an updated node stalls the rollout, or "" when it does not.
//
// A failed node stalls it at once: it keeps its maxSkew slot and needs an operator. Otherwise a
// node stalls it only when its current update has not completed and was started longer than the
// deadline ago. Ready for the previous spec is not completion, so a node the LynqNode controller
// never catches up on is timed like any other.
func stallReason(node *lynqv1.LynqNode, deadline time.Duration, now time.Time) string {
	if classifyNode(node) == nodeHealthFailed {
		return "failed"
	}
	if reachedReadyForCurrentSpec(node) || updateCompleted(node) {
		return ""
	}
	stamp, err := time.Parse(time.RFC3339Nano, node.Annotations[lynqv1.AnnotationRolloutUpdateStartTime])
	if err != nil || now.Sub(stamp) <= deadline {
		return ""
	}
	return fmt.Sprintf("not Ready within %ds", int64(deadline/time.Second))
}

// progressDeadline returns the form's rollout.progressDeadlineSeconds as a duration.
func progressDeadline(tmpl *lynqv1.LynqForm) time.Duration {
	seconds := int32(defaultProgressDeadlineSeconds)
	if tmpl.Spec.Rollout != nil && tmpl.Spec.Rollout.ProgressDeadlineSeconds > 0 {
		seconds = tmpl.Spec.Rollout.ProgressDeadlineSeconds
	}
	return time.Duration(seconds) * time.Second
}

// stalledNode is an updated node that stalls the rollout.
type stalledNode struct {
	name   string
	reason string
}

// formatStalledMessage renders the Failed-phase rollout message. The order is sorted and the
// text carries no elapsed times, so an unchanged stall renders byte-identically and does not
// rewrite the LynqForm status on every reconcile.
func formatStalledMessage(stalled []stalledNode, deadline time.Duration) string {
	sorted := make([]stalledNode, len(stalled))
	copy(sorted, stalled)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].name < sorted[j].name })

	parts := make([]string, 0, maxStalledNodesInMessage+1)
	for i, s := range sorted {
		if i == maxStalledNodesInMessage {
			parts = append(parts, fmt.Sprintf("and %d more", len(sorted)-maxStalledNodesInMessage))
			break
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", s.name, s.reason))
	}
	return fmt.Sprintf("Rollout stalled: %d updated node(s) failed or did not become Ready within progressDeadlineSeconds=%d: %s",
		len(sorted), int64(deadline/time.Second), strings.Join(parts, ", "))
}
