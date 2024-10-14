/*
Copyright 2017 The Kubernetes Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Task is a specification for a Task resource
type Task struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TaskSpec   `json:"spec"`
	Status TaskStatus `json:"status"`
}

// TaskSpec is the spec for a Task resource
type TaskSpec struct {
	ClusterName string `json:"clusterName"`

	Image   string   `json:"image"`
	Command []string `json:"command"`
	Args    []string `json:"args"`

	InputObject string `json:"inputObject"`
}

// +enum
type TaskState string

const (
	TaskStateRunning  = "Running"
	TaskStateFinished = "Finished"
)

// TaskStatus is the status for a Task resource
type TaskStatus struct {
	PodName string    `json:"podName"`
	State   TaskState `json:"state"`
	// TODO: Get rid of this, replace with ownerRef?
	OutputObject string `json:"outputObject"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskList is a list of Task resources
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Task `json:"items"`
}
