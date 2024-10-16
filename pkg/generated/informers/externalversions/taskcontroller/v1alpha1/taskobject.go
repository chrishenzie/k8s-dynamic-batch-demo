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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	taskcontrollerv1alpha1 "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/apis/taskcontroller/v1alpha1"
	versioned "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/clientset/versioned"
	internalinterfaces "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/listers/taskcontroller/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// TaskObjectInformer provides access to a shared informer and lister for
// TaskObjects.
type TaskObjectInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.TaskObjectLister
}

type taskObjectInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewTaskObjectInformer constructs a new informer for TaskObject type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewTaskObjectInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredTaskObjectInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredTaskObjectInformer constructs a new informer for TaskObject type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredTaskObjectInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.TaskcontrollerV1alpha1().TaskObjects(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.TaskcontrollerV1alpha1().TaskObjects(namespace).Watch(context.TODO(), options)
			},
		},
		&taskcontrollerv1alpha1.TaskObject{},
		resyncPeriod,
		indexers,
	)
}

func (f *taskObjectInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredTaskObjectInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *taskObjectInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&taskcontrollerv1alpha1.TaskObject{}, f.defaultInformer)
}

func (f *taskObjectInformer) Lister() v1alpha1.TaskObjectLister {
	return v1alpha1.NewTaskObjectLister(f.Informer().GetIndexer())
}
