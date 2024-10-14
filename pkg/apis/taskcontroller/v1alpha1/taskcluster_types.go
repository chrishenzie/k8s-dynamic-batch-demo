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

// TaskCluster is a specification for a TaskCluster resource
type TaskCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TaskClusterSpec   `json:"spec"`
	Status TaskClusterStatus `json:"status"`
}

// TaskClusterSpec is the spec for a TaskCluster resource
type TaskClusterSpec struct {
	// TODO: Get rid of this too.
	DeploymentName     string `json:"deploymentName"`
	Replicas           *int32 `json:"replicas"`
	ServiceAccountName string `json:"serviceAccountName"`
}

// TaskClusterStatus is the status for a TaskCluster resource
type TaskClusterStatus struct {
	AvailableReplicas int32 `json:"availableReplicas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskClusterList is a list of TaskCluster resources
type TaskClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TaskCluster `json:"items"`
}
