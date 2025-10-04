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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SlurmJobSpec defines the desired state of SlurmJob
type SlurmJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	UserName string `json:"userName,omitempty"`
	Command  string `json:"command,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

// SlurmJobStatus defines the observed state of SlurmJob
type SlurmJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	RestartCount int `json:"restartCount,omitempty"`

	SlurmJobRunCurrentStatus SlurmJobRunStatus   `json:"slurmJobRunCurrentStatus,omitempty"`
	SlurmJobRunStatusHistory []SlurmJobRunStatus `json:"slurmJobRunStatusHistory,omitempty"`
}

type SlurmJobRunStatus struct {
	// Incremented after each restart.
	RunID int `json:"runID,omitempty"`

	// Used to determine if the job has been removed from the slurm cluster.
	// In this case it's hard to determine the end state, so use a separate field.
	Removed bool `json:"removed,omitempty"`

	Priority              int                              `json:"priority,omitempty"`
	State                 string                           `json:"state,omitempty"`
	PhysicalNodeSnapshots map[string]*PhysicalNodeSnapshot `json:"physicalNodeSnapshots,omitempty"`

	SubmitTimestamp   metav1.Time `json:"submitTimestamp,omitempty"`
	StartTimestamp    metav1.Time `json:"startTimestamp,omitempty"`
	LastSyncTimestamp metav1.Time `json:"lastSyncTimestamp,omitempty"`
}

func (s *SlurmJobRunStatus) IsEqual(s2 SlurmJobRunStatus) bool {
	if len(s.PhysicalNodeSnapshots) != len(s2.PhysicalNodeSnapshots) {
		return false
	}
	for k, v := range s.PhysicalNodeSnapshots {
		if v2, ok := s2.PhysicalNodeSnapshots[k]; !ok ||
			v.PhysicalNodeName != v2.PhysicalNodeName || v.K8sNodeName != v2.K8sNodeName || v.SlurmNodeName != v2.SlurmNodeName {
			return false
		}
	}

	return s.RunID == s2.RunID && s.Removed == s2.Removed && s.Priority == s2.Priority && s.State == s2.State
}

type PhysicalNodeSnapshot struct {
	PhysicalNodeName string `json:"physicalNodeName,omitempty"`
	K8sNodeName      string `json:"k8sNodeName,omitempty"`
	SlurmNodeName    string `json:"slurmNodeName,omitempty"`

	// Accumulated runtime of the job on this physical node across runs.
	AccumulatedRuntime metav1.Duration `json:"accumulatedRuntime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SlurmJob is the Schema for the slurmjobs API
type SlurmJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SlurmJobSpec   `json:"spec,omitempty"`
	Status SlurmJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SlurmJobList contains a list of SlurmJob
type SlurmJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SlurmJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SlurmJob{}, &SlurmJobList{})
}
