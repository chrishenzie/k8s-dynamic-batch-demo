/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"

	v1alpha1 "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/apis/taskcontroller/v1alpha1"
	scheme "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// TaskObjectsGetter has a method to return a TaskObjectInterface.
// A group's client should implement this interface.
type TaskObjectsGetter interface {
	TaskObjects(namespace string) TaskObjectInterface
}

// TaskObjectInterface has methods to work with TaskObject resources.
type TaskObjectInterface interface {
	Create(ctx context.Context, taskObject *v1alpha1.TaskObject, opts v1.CreateOptions) (*v1alpha1.TaskObject, error)
	Update(ctx context.Context, taskObject *v1alpha1.TaskObject, opts v1.UpdateOptions) (*v1alpha1.TaskObject, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.TaskObject, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.TaskObjectList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.TaskObject, err error)
	TaskObjectExpansion
}

// taskObjects implements TaskObjectInterface
type taskObjects struct {
	*gentype.ClientWithList[*v1alpha1.TaskObject, *v1alpha1.TaskObjectList]
}

// newTaskObjects returns a TaskObjects
func newTaskObjects(c *TaskcontrollerV1alpha1Client, namespace string) *taskObjects {
	return &taskObjects{
		gentype.NewClientWithList[*v1alpha1.TaskObject, *v1alpha1.TaskObjectList](
			"taskobjects",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *v1alpha1.TaskObject { return &v1alpha1.TaskObject{} },
			func() *v1alpha1.TaskObjectList { return &v1alpha1.TaskObjectList{} }),
	}
}
