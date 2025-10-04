/*
Copyright 2024.

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
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PhysicalNodeSpec defines the desired state of PhysicalNode
type PhysicalNodeSpec struct {
	// Desired state of cluster.
	// Important: Run "make" to regenerate code after modifying this files
	SlurmNodeSpec SlurmNodeSpec `json:"slurmNodeSpec"`
	K8sNodeSpec   K8sNodeSpec   `json:"k8sNodeSpec"`
	Manual        bool          `json:"manual,omitempty"`
}

// PhysicalNodeStatus defines the observed state of PhysicalNode
type PhysicalNodeStatus struct {
	// Observed state of cluster.
	// Important: Run "make" to regenerate code after modifying this file
	SlurmNodeStatus        SlurmNodeStatus   `json:"slurmNodeStatus,omitempty"`
	SlurmNodeStatusHistory []SlurmNodeStatus `json:"slurmNodeStatusHistory,omitempty"`
	K8sNodeStatus          K8sNodeStatus     `json:"k8sNodeStatus,omitempty"`
	K8sNodeStatusHistory   []K8sNodeStatus   `json:"k8sNodeStatusHistory,omitempty"`
}

type SlurmNodeSpec struct {
	GoalState string `json:"goalState"`

	Reason    string      `json:"reason,omitempty"`
	Timestamp metav1.Time `json:"timestamp,omitempty"`
}

func (s *SlurmNodeSpec) IsEqual(s2 SlurmNodeSpec) bool {
	return s.GoalState == s2.GoalState && s.Reason == s2.Reason
}

type SlurmNodeStatus struct {
	Name     string   `json:"name,omitempty"`
	State    []string `json:"state,omitempty"`
	Features []string `json:"features,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Comment  string   `json:"comment,omitempty"`

	Removed   bool        `json:"removed,omitempty"`
	Timestamp metav1.Time `json:"timestamp,omitempty"`
}

func (s *SlurmNodeStatus) IsEqual(s2 SlurmNodeStatus) bool {
	if s.Name != s2.Name {
		return false
	}

	if len(s.State) != len(s2.State) {
		return false
	}
	for i := range s.State {
		if s.State[i] != s2.State[i] {
			return false
		}
	}

	if len(s.Features) != len(s2.Features) {
		return false
	}
	for i := range s.Features {
		if s.Features[i] != s2.Features[i] {
			return false
		}
	}

	if s.Reason != s2.Reason {
		return false
	}

	if s.Comment != s2.Comment {
		return false
	}

	if s.Removed != s2.Removed {
		return false
	}

	return true
}

type K8sNodeSpec struct {
	GoalState string `json:"goalState"`

	Reason    string      `json:"reason,omitempty"`
	Timestamp metav1.Time `json:"timestamp,omitempty"`
}

func (s *K8sNodeSpec) IsEqual(s2 K8sNodeSpec) bool {
	return s.GoalState == s2.GoalState && s.Reason == s2.Reason
}

type K8sNodeStatus struct {
	Name          string         `json:"name,omitempty"`
	Unschedulable bool           `json:"unschedulable,omitempty"`
	Taints        []corev1.Taint `json:"taints,omitempty"`

	Removed   bool        `json:"removed,omitempty"`
	Timestamp metav1.Time `json:"timestamp,omitempty"`
}

func (s *K8sNodeStatus) IsEqual(s2 K8sNodeStatus) bool {
	if s.Name != s2.Name {
		return false
	}

	if s.Unschedulable != s2.Unschedulable {
		return false
	}

	if len(s.Taints) != len(s2.Taints) {
		return false
	}
	taintMap := map[string]corev1.Taint{}
	for _, t := range s.Taints {
		taintMap[t.Key] = t
	}
	for _, t := range s2.Taints {
		if t2, ok := taintMap[t.Key]; !ok || t2 != t {
			return false
		}
	}

	if s.Removed != s2.Removed {
		return false
	}

	return true
}

// EventRecord is a record of an event that's processed or emitted by the controller.
type EventRecord struct {
	Event corev1.Event `json:"event,omitempty"`

	// Timestamp when the event was acknowledged.
	AckTimestamp metav1.Time `json:"acktimestamp,omitempty"`
}

func (e *EventRecord) IsFromEvent(event v1.Event) bool {
	return e.Event.ObjectMeta.Name == event.ObjectMeta.Name
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PhysicalNode is the Schema for the physicalnodes API
type PhysicalNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PhysicalNodeSpec   `json:"spec,omitempty"`
	Status PhysicalNodeStatus `json:"status,omitempty"`

	EventRecords []EventRecord `json:"eventRecords,omitempty"`
}

//+kubebuilder:object:root=true

// PhysicalNodeList contains a list of PhysicalNode
type PhysicalNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PhysicalNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PhysicalNode{}, &PhysicalNodeList{})
}
