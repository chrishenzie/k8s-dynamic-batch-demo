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

package controllers

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	taskv1alpha1 "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/apis/taskcontroller/v1alpha1"
	clientset "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/clientset/versioned"
	taskscheme "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/clientset/versioned/scheme"
	informers "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/informers/externalversions/taskcontroller/v1alpha1"
	listers "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/listers/taskcontroller/v1alpha1"
)

const controllerAgentName = "task-controller"

const (
	SuccessSynced     = "Synced"
	ErrResourceExists = "ErrResourceExists"

	MessageResourceExists = "Resource %q already exists and is not managed by TaskCluster"
	MessageResourceSynced = "TaskCluster synced successfully"
	FieldManager          = controllerAgentName
)

/*
This is a copy of the Kubernetes task-controller, with minor changes to
instead reconcile TaskCluster objects.

For the original controller implementation, please see:
https://github.com/kubernetes/task-controller/blob/v0.31.1/controller.go
*/
type TaskClusterController struct {
	kubeclientset   kubernetes.Interface
	taskclientset clientset.Interface

	deploymentsLister  appslisters.DeploymentLister
	deploymentsSynced  cache.InformerSynced
	taskClustersLister listers.TaskClusterLister
	taskClustersSynced cache.InformerSynced

	workqueue workqueue.TypedRateLimitingInterface[cache.ObjectName]
	recorder  record.EventRecorder
}

func NewTaskClusterController(
	ctx context.Context,
	kubeclientset kubernetes.Interface,
	taskclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	taskClusterInformer informers.TaskClusterInformer) *TaskClusterController {
	logger := klog.FromContext(ctx)

	utilruntime.Must(taskscheme.AddToScheme(scheme.Scheme))
	logger.V(4).Info("Creating event broadcaster")

	eventBroadcaster := record.NewBroadcaster(record.WithContext(ctx))
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	ratelimiter := workqueue.NewTypedMaxOfRateLimiter(
		workqueue.NewTypedItemExponentialFailureRateLimiter[cache.ObjectName](5*time.Millisecond, 1000*time.Second),
		&workqueue.TypedBucketRateLimiter[cache.ObjectName]{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	controller := &TaskClusterController{
		kubeclientset:      kubeclientset,
		taskclientset:    taskclientset,
		deploymentsLister:  deploymentInformer.Lister(),
		deploymentsSynced:  deploymentInformer.Informer().HasSynced,
		taskClustersLister: taskClusterInformer.Lister(),
		taskClustersSynced: taskClusterInformer.Informer().HasSynced,
		workqueue:          workqueue.NewTypedRateLimitingQueue(ratelimiter),
		recorder:           recorder,
	}

	logger.Info("Setting up event handlers")
	taskClusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueTaskCluster,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueTaskCluster(new)
		},
	})
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

func (c *TaskClusterController) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()
	logger := klog.FromContext(ctx)

	logger.Info("Starting TaskCluster controller")
	logger.Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(ctx.Done(), c.deploymentsSynced, c.taskClustersSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Info("Starting workers", "count", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	logger.Info("Started workers")
	<-ctx.Done()
	logger.Info("Shutting down workers")

	return nil
}

func (c *TaskClusterController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *TaskClusterController) processNextWorkItem(ctx context.Context) bool {
	objRef, shutdown := c.workqueue.Get()
	logger := klog.FromContext(ctx)

	if shutdown {
		return false
	}

	defer c.workqueue.Done(objRef)

	err := c.syncHandler(ctx, objRef)
	if err == nil {
		c.workqueue.Forget(objRef)
		logger.Info("Successfully synced", "objectName", objRef)
		return true
	}

	utilruntime.HandleErrorWithContext(ctx, err, "Error syncing; requeuing for later retry", "objectReference", objRef)
	c.workqueue.AddRateLimited(objRef)
	return true
}

func (c *TaskClusterController) syncHandler(ctx context.Context, objectRef cache.ObjectName) error {
	logger := klog.LoggerWithValues(klog.FromContext(ctx), "objectRef", objectRef)

	taskCluster, err := c.taskClustersLister.TaskClusters(objectRef.Namespace).Get(objectRef.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleErrorWithContext(ctx, err, "TaskCluster referenced by item in work queue no longer exists", "objectReference", objectRef)
			return nil
		}

		return err
	}

	deploymentName := taskCluster.Spec.DeploymentName
	if deploymentName == "" {
		utilruntime.HandleErrorWithContext(ctx, nil, "Deployment name missing from object reference", "objectReference", objectRef)
		return nil
	}

	serviceAccountName := taskCluster.Spec.ServiceAccountName
	if serviceAccountName == "" {
		utilruntime.HandleErrorWithContext(ctx, nil, "ServiceAccount name missing from object reference", "objectReference", objectRef)
		return nil
	}

	deployment, err := c.deploymentsLister.Deployments(taskCluster.Namespace).Get(deploymentName)
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(taskCluster.Namespace).Create(ctx, newDeployment(taskCluster), metav1.CreateOptions{FieldManager: FieldManager})
	}
	if err != nil {
		return err
	}

	if !metav1.IsControlledBy(deployment, taskCluster) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		c.recorder.Event(taskCluster, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf("%s", msg)
	}

	if taskCluster.Spec.Replicas != nil && *taskCluster.Spec.Replicas != *deployment.Spec.Replicas {
		logger.V(4).Info("Update deployment resource", "currentReplicas", *taskCluster.Spec.Replicas, "desiredReplicas", *deployment.Spec.Replicas)
		deployment, err = c.kubeclientset.AppsV1().Deployments(taskCluster.Namespace).Update(ctx, newDeployment(taskCluster), metav1.UpdateOptions{FieldManager: FieldManager})
	}
	if err != nil {
		return err
	}

	err = c.updateTaskClusterStatus(taskCluster, deployment)
	if err != nil {
		return err
	}

	c.recorder.Event(taskCluster, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *TaskClusterController) updateTaskClusterStatus(taskCluster *taskv1alpha1.TaskCluster, deployment *appsv1.Deployment) error {
	taskClusterCopy := taskCluster.DeepCopy()
	taskClusterCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	_, err := c.taskclientset.TaskcontrollerV1alpha1().TaskClusters(taskCluster.Namespace).UpdateStatus(context.TODO(), taskClusterCopy, metav1.UpdateOptions{FieldManager: FieldManager})
	return err
}

func (c *TaskClusterController) enqueueTaskCluster(obj interface{}) {
	if objectRef, err := cache.ObjectToName(obj); err != nil {
		utilruntime.HandleError(err)
		return
	} else {
		c.workqueue.Add(objectRef)
	}
}

func (c *TaskClusterController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	logger := klog.FromContext(context.Background())
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleErrorWithContext(context.Background(), nil, "Error decoding object, invalid type", "type", fmt.Sprintf("%T", obj))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleErrorWithContext(context.Background(), nil, "Error decoding object tombstone, invalid type", "type", fmt.Sprintf("%T", tombstone.Obj))
			return
		}
		logger.V(4).Info("Recovered deleted object", "resourceName", object.GetName())
	}
	logger.V(4).Info("Processing object", "object", klog.KObj(object))
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		if ownerRef.Kind != "TaskCluster" {
			return
		}

		taskCluster, err := c.taskClustersLister.TaskClusters(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			logger.V(4).Info("Ignore orphaned object", "object", klog.KObj(object), "taskCluster", ownerRef.Name)
			return
		}

		c.enqueueTaskCluster(taskCluster)
		return
	}
}

func newDeployment(taskCluster *taskv1alpha1.TaskCluster) *appsv1.Deployment {
	labels := map[string]string{
		"task-cluster": taskCluster.Name,
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      taskCluster.Spec.DeploymentName,
			Namespace: taskCluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(taskCluster, taskv1alpha1.SchemeGroupVersion.WithKind("TaskCluster")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: taskCluster.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: taskCluster.Spec.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "pause",
							Image: "k8s.gcr.io/pause:3.8",
						},
					},
				},
			},
		},
	}
}
