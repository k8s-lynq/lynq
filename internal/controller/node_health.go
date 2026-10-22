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
	"k8s.io/apimachinery/pkg/api/meta"

	lynqv1 "github.com/k8s-lynq/lynq/api/v1"
)

// nodeHealth is the single classification of a LynqNode that the LynqHub status, the maxSkew
// admission check and the LynqForm rollout status all share. Keeping one definition is the
// point: when they disagreed, a node recovering a lost pod counted as failed in the hub while
// the rollout counted it as merely updating.
type nodeHealth int

const (
	// nodeHealthProgressing is everything that is neither ready nor failed: a node still rolling
	// out an update, recovering from a regression (lost pod, drain, external rollout restart), or
	// not yet reconciled at all (no Ready condition). The workload controllers are expected to
	// make progress on it without an operator.
	nodeHealthProgressing nodeHealth = iota
	// nodeHealthReady is a node whose Ready condition is True.
	nodeHealthReady
	// nodeHealthFailed is a node that needs an operator: failed resources (apply error or
	// readiness timeout), conflicted resources, or a Degraded condition (which the LynqNode
	// controller also sets for template, variables and dependency errors).
	nodeHealthFailed
)

// classifyNode returns the health of a LynqNode from its published status.
//
// Failure signals win over Ready=True: a node that reports both needs an operator, and the
// combination can only come from a status the LynqNode controller has not finished rewriting.
// A node with no Ready condition yet is progressing, not failed: it has simply not been
// reconciled.
func classifyNode(node *lynqv1.LynqNode) nodeHealth {
	conds := node.Status.Conditions
	if node.Status.FailedResources > 0 ||
		meta.IsStatusConditionTrue(conds, ConditionTypeConflicted) ||
		meta.IsStatusConditionTrue(conds, ConditionTypeDegraded) {
		return nodeHealthFailed
	}
	if meta.IsStatusConditionTrue(conds, ConditionTypeReady) {
		return nodeHealthReady
	}
	return nodeHealthProgressing
}
