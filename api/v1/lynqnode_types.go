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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LynqNodeSpec defines the desired state of LynqNode.
// Resources are created in the same namespace as this LynqNode CR by default.
// Use TResource.targetNamespace to create resources in different namespaces.
type LynqNodeSpec struct {
	// UID is the unique identifier from the hub data source
	// +kubebuilder:validation:Required
	UID string `json:"uid"`

	// TemplateRef references the LynqForm to use
	// +kubebuilder:validation:Required
	TemplateRef string `json:"templateRef"`

	// ServiceAccounts are the resolved ServiceAccount resources
	// +optional
	ServiceAccounts []TResource `json:"serviceAccounts,omitempty"`

	// Deployments are the resolved Deployment resources
	// +optional
	Deployments []TResource `json:"deployments,omitempty"`

	// StatefulSets are the resolved StatefulSet resources
	// +optional
	StatefulSets []TResource `json:"statefulSets,omitempty"`

	// DaemonSets are the resolved DaemonSet resources
	// +optional
	DaemonSets []TResource `json:"daemonSets,omitempty"`

	// Services are the resolved Service resources
	// +optional
	Services []TResource `json:"services,omitempty"`

	// Ingresses are the resolved Ingress resources
	// +optional
	Ingresses []TResource `json:"ingresses,omitempty"`

	// ConfigMaps are the resolved ConfigMap resources
	// +optional
	ConfigMaps []TResource `json:"configMaps,omitempty"`

	// Secrets are the resolved Secret resources
	// +optional
	Secrets []TResource `json:"secrets,omitempty"`

	// PersistentVolumeClaims are the resolved PVC resources
	// +optional
	PersistentVolumeClaims []TResource `json:"persistentVolumeClaims,omitempty"`

	// Jobs are the resolved Job resources
	// +optional
	Jobs []TResource `json:"jobs,omitempty"`

	// CronJobs are the resolved CronJob resources
	// +optional
	CronJobs []TResource `json:"cronJobs,omitempty"`

	// PodDisruptionBudgets are the resolved PDB resources
	// +optional
	PodDisruptionBudgets []TResource `json:"podDisruptionBudgets,omitempty"`

	// NetworkPolicies are the resolved NetworkPolicy resources
	// +optional
	NetworkPolicies []TResource `json:"networkPolicies,omitempty"`

	// HorizontalPodAutoscalers are the resolved HPA resources
	// +optional
	HorizontalPodAutoscalers []TResource `json:"horizontalPodAutoscalers,omitempty"`

	// Namespaces are the resolved Namespace resources
	// +optional
	Namespaces []TResource `json:"namespaces,omitempty"`

	// Manifests are the resolved arbitrary resources
	// +optional
	Manifests []TResource `json:"manifests,omitempty"`
}

// LynqNodeStatus defines the observed state of LynqNode.
type LynqNodeStatus struct {
	// ObservedGeneration is the generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ReadyResources is the number of resources that are ready
	// +optional
	ReadyResources int32 `json:"readyResources,omitempty"`

	// DesiredResources is the total number of resources
	// +optional
	DesiredResources int32 `json:"desiredResources,omitempty"`

	// FailedResources is the number of resources that failed
	// +optional
	FailedResources int32 `json:"failedResources,omitempty"`

	// SkippedResources is the number of resources skipped due to dependency failures
	// +optional
	SkippedResources int32 `json:"skippedResources,omitempty"`

	// SkippedResourceIds lists the IDs of resources that were skipped due to dependency failures
	// +optional
	SkippedResourceIds []string `json:"skippedResourceIds,omitempty"`

	// AppliedResources tracks the keys of resources that were successfully applied
	// Format: "kind/namespace/name@id" (e.g., "Deployment/default/myapp@app-deployment")
	// This enables detection and cleanup of orphaned resources when removed from form
	// +optional
	AppliedResources []string `json:"appliedResources,omitempty"`

	// Conditions represent the latest available observations of the node's state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="UID",type="string",JSONPath=".spec.uid",description="Node unique identifier"
// +kubebuilder:printcolumn:name="Form",type="string",JSONPath=".spec.templateRef",description="LynqForm reference"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyResources",description="Number of ready resources"
// +kubebuilder:printcolumn:name="Desired",type="integer",JSONPath=".status.desiredResources",description="Total number of resources"
// +kubebuilder:printcolumn:name="Skipped",type="integer",JSONPath=".status.skippedResources",description="Resources skipped due to dependency failures"
// +kubebuilder:printcolumn:name="Conflicted",type="string",JSONPath=".status.conditions[?(@.type=='Conflicted')].status",description="Conflict status"
// +kubebuilder:printcolumn:name="Conditions",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason",description="Condition reason"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LynqNode is the Schema for the lynqnodes API.
type LynqNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LynqNodeSpec   `json:"spec,omitempty"`
	Status LynqNodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LynqNodeList contains a list of LynqNode.
type LynqNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LynqNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LynqNode{}, &LynqNodeList{})
}
