/*
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

package v1beta2

import (
	fluxksv1beta2 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status conditions
const (
	ReadyCondition = "Ready"
)

// Status reasons
const (
	KustomizationAlreadyExistsReason = "KustomizationAlreadyExists"
	FailedDeleteKustomizationReason  = "FailedDeleteKustomization"
	FailedCreateKustomizationReason  = "FailedCreateKustomization"
	KustomizationExpiredReason       = "KustomizationExpired"
	KustomizationCreatedReason       = "KustomizationCreated"
)

// Finalizer
const (
	FinalizerName = "finalizer.kustomize.raffis.github.io"
)

// CeasingKustomizationSpec defines the desired state of CeasingKustomization
type CeasingKustomizationSpec struct {
	// TTL defines the duration of the kustomizations lifetime.
	// The kustomizaton will be removed once the TTL is expired.
	// +required
	TTL metav1.Duration `json:"ttl"`

	// KustomizationTemplate is a templated flux kustomization
	// +required
	KustomizationTemplate KustomizationTemplate `json:"kustomizationTemplate"`
}

// KustomizationTemplate define kustomization metadata and spec
// TypeObject and Status is excluded and not part of the spec template
type KustomizationTemplate struct {
	metav1.ObjectMeta `json:",inline"`
	Spec              fluxksv1beta2.KustomizationSpec `json:"spec"`
}

// CeasingKustomizationStatus defines the observed state of CeasingKustomization
type CeasingKustomizationStatus struct {
	// LastReconciliationRunTime records the last time the controller ran
	// this automation through to completion (even if no updates were
	// made).
	// +optional
	LastReconciliationRunTime *metav1.Time `json:"lastReconciliationRunTime,omitempty"`
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// CeasingKustomizationNotBound de
func NotReady(ck CeasingKustomization, reason, message string) CeasingKustomization {
	setResourceCondition(&ck, ReadyCondition, metav1.ConditionFalse, reason, message)
	return ck
}

// CeasingKustomizationBound de
func Ready(ck CeasingKustomization, reason, message string) CeasingKustomization {
	setResourceCondition(&ck, ReadyCondition, metav1.ConditionTrue, reason, message)
	return ck
}

// GetStatusConditions returns a pointer to the Status.Conditions slice
func (in *CeasingKustomization) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=ck
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// CeasingKustomization is the Schema for the CeasingKustomizations API
type CeasingKustomization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CeasingKustomizationSpec   `json:"spec,omitempty"`
	Status CeasingKustomizationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CeasingKustomizationList contains a list of CeasingKustomization
type CeasingKustomizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CeasingKustomization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CeasingKustomization{}, &CeasingKustomizationList{})
}

// ConditionalResource is a resource with conditions
type conditionalResource interface {
	GetStatusConditions() *[]metav1.Condition
}

// setResourceCondition sets the given condition with the given status,
// reason and message on a resource.
func setResourceCondition(resource conditionalResource, condition string, status metav1.ConditionStatus, reason, message string) {
	conditions := resource.GetStatusConditions()

	newCondition := metav1.Condition{
		Type:    condition,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	apimeta.SetStatusCondition(conditions, newCondition)
}
