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
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"time"

	"golang.org/x/time/rate"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"

	taskcontrollerv1alpha1 "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/apis/taskcontroller/v1alpha1"
	clientset "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/clientset/versioned"
	taskscheme "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/clientset/versioned/scheme"
	informers "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/informers/externalversions/taskcontroller/v1alpha1"
	listers "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/listers/taskcontroller/v1alpha1"
)

const (
	taskControllerAgentName   = "task-controller"
	taskMessageResourceSynced = "Task synced successfully"
)

type TaskController struct {
	kubeclientset kubernetes.Interface
	taskclientset clientset.Interface

	taskClustersLister listers.TaskClusterLister
	taskClustersSynced cache.InformerSynced
	podsLister         corelisters.PodLister
	podsSynced         cache.InformerSynced
	taskObjectsLister  listers.TaskObjectLister
	taskObjectsSynced  cache.InformerSynced
	tasksLister        listers.TaskLister
	tasksSynced        cache.InformerSynced

	workqueue workqueue.TypedRateLimitingInterface[cache.ObjectName]
	recorder  record.EventRecorder
}

func NewTaskController(
	ctx context.Context,
	kubeclientset kubernetes.Interface,
	taskclientset clientset.Interface,
	taskClusterInformer informers.TaskClusterInformer,
	podInformer coreinformers.PodInformer,
	taskObjectInformer informers.TaskObjectInformer,
	taskInformer informers.TaskInformer) *TaskController {
	logger := klog.FromContext(ctx)
	utilruntime.Must(taskscheme.AddToScheme(scheme.Scheme))
	logger.V(4).Info("Creating event broadcaster")

	eventBroadcaster := record.NewBroadcaster(record.WithContext(ctx))
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: taskControllerAgentName})
	ratelimiter := workqueue.NewTypedMaxOfRateLimiter(
		workqueue.NewTypedItemExponentialFailureRateLimiter[cache.ObjectName](5*time.Millisecond, 1000*time.Second),
		&workqueue.TypedBucketRateLimiter[cache.ObjectName]{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	controller := &TaskController{
		kubeclientset:      kubeclientset,
		taskclientset:      taskclientset,
		taskClustersLister: taskClusterInformer.Lister(),
		taskClustersSynced: taskClusterInformer.Informer().HasSynced,
		podsLister:         podInformer.Lister(),
		podsSynced:         podInformer.Informer().HasSynced,
		taskObjectsLister:  taskObjectInformer.Lister(),
		taskObjectsSynced:  taskObjectInformer.Informer().HasSynced,
		tasksLister:        taskInformer.Lister(),
		tasksSynced:        taskInformer.Informer().HasSynced,
		workqueue:          workqueue.NewTypedRateLimitingQueue(ratelimiter),
		recorder:           recorder,
	}

	logger.Info("Setting up event handlers")
	taskInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueTask,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueTask(new)
		},
	})
	// Watch for changes to the Pods running Task containers and enqueue
	// the corresponding Task.
	// TODO: Optimize by only enqueueing Tasks for newly terminated containers.
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newPod := new.(*corev1.Pod)
			oldPod := old.(*corev1.Pod)
			if newPod.ResourceVersion == oldPod.ResourceVersion {
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})
	// Also watch for changes to TaskObjects, and enqueue the Tasks that
	// reference them as input.
	taskObjectInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newTaskObject := new.(*taskcontrollerv1alpha1.TaskObject)
			oldTaskObject := old.(*taskcontrollerv1alpha1.TaskObject)
			if newTaskObject.ResourceVersion == oldTaskObject.ResourceVersion {
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

func (c *TaskController) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()
	logger := klog.FromContext(ctx)

	logger.Info("Starting Task controller")
	logger.Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(ctx.Done(), c.taskClustersSynced, c.tasksSynced, c.podsSynced); !ok {
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

func (c *TaskController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *TaskController) processNextWorkItem(ctx context.Context) bool {
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

func (c *TaskController) syncHandler(ctx context.Context, objectRef cache.ObjectName) error {
	// Look up the Task to sync.
	task, err := c.tasksLister.Tasks(objectRef.Namespace).Get(objectRef.Name)
	if errors.IsNotFound(err) {
		utilruntime.HandleErrorWithContext(ctx, err, "Task referenced by item in work queue no longer exists", "objectReference", objectRef)
		return nil
	}
	if err != nil {
		return err
	}

	// If the Task is already finished, then there's nothing to do.
	if task.Status.State == taskcontrollerv1alpha1.TaskStateFinished {
		return nil
	}

	taskCluster, err := c.taskClustersLister.TaskClusters(task.Namespace).Get(task.Spec.ClusterName)
	if errors.IsNotFound(err) {
		utilruntime.HandleErrorWithContext(ctx, err, "TaskCluster doesn't exist", "objectReference", objectRef, "taskCluster", task.Spec.ClusterName)
		return nil
	}
	if err != nil {
		return err
	}

	if task.Spec.InputObject != "" {
		_, err := c.taskObjectsLister.TaskObjects(task.Namespace).Get(task.Spec.InputObject)
		if errors.IsNotFound(err) {
			utilruntime.HandleErrorWithContext(ctx, err, "Input TaskObject doesn't exist", "objectReference", objectRef, "inputObject", task.Spec.InputObject)
			return nil
		}
		if err != nil {
			return err
		}
	}

	// If the Task is already assigned to a Pod, update its status if it
	// terminated.
	if task.Status.PodName != "" {
		pod, err := c.podsLister.Pods(taskCluster.Namespace).Get(task.Status.PodName)
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		if isTaskTerminated(pod, task.Name) {
			// TODO: Removing the terminated ephemeral container
			// from the Pod spec is unsupported by kube-apiserver.
			//
			// updatedPod := removeTaskContainer(pod, task.Name)
			// if err := c.patchPodEphemeralContainers(ctx, pod, updatedPod); err != nil {
			// return err
			// }

			return c.markTaskFinished(ctx, task)
		}
		return nil
	}

	// Find Pods in the TaskCluster to schedule on.
	selector := labels.Set{"task-cluster": taskCluster.GetName()}.AsSelector()
	pods, err := c.podsLister.Pods(taskCluster.Namespace).List(selector)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return fmt.Errorf("No Pods exist for TaskCluster: %s", taskCluster.GetName())
	}

	// Pick a random pod to load balance.
	pod := pods[rand.Intn(len(pods))]

	// Only schedule Tasks on ready Pods, otherwise you may encounter race
	// conditions with the Pod's ReplicaSet controller.
	if !podutil.IsPodReadyConditionTrue(pod.Status) {
		return fmt.Errorf("Pod is not ready yet")
	}

	updatedPod, err := appendTaskContainer(task, pod)
	if err != nil {
		return fmt.Errorf("Error appending task container: %v", err)
	}
	if err := c.patchPodEphemeralContainers(ctx, pod, updatedPod); err != nil {
		return err
	}

	// Update the Task's status to indicate the Pod name and that it's running.
	err = c.markTaskRunning(ctx, task, pod.Name)
	if err != nil {
		return err
	}

	c.recorder.Event(taskCluster, corev1.EventTypeNormal, SuccessSynced, taskMessageResourceSynced)
	return nil
}

func (c *TaskController) enqueueTask(obj interface{}) {
	if objectRef, err := cache.ObjectToName(obj); err != nil {
		utilruntime.HandleError(err)
		return
	} else {
		c.workqueue.Add(objectRef)
	}
}

func (c *TaskController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	logger := klog.FromContext(context.Background())
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			// If the object value is not too big and does not contain sensitive information then
			// it may be useful to include it.
			utilruntime.HandleErrorWithContext(context.Background(), nil, "Error decoding object, invalid type", "type", fmt.Sprintf("%T", obj))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			// If the object value is not too big and does not contain sensitive information then
			// it may be useful to include it.
			utilruntime.HandleErrorWithContext(context.Background(), nil, "Error decoding object tombstone, invalid type", "type", fmt.Sprintf("%T", tombstone.Obj))
			return
		}
		logger.V(4).Info("Recovered deleted object", "resourceName", object.GetName())
	}
	logger.V(4).Info("Processing object", "object", klog.KObj(object))

	switch obj := obj.(type) {
	case *corev1.Pod:
		c.handlePod(obj)
	case *taskcontrollerv1alpha1.TaskObject:
		c.handleTaskObject(obj)
	default:
		utilruntime.HandleErrorWithContext(context.Background(), nil, "Error decoding object, invalid type", "type", fmt.Sprintf("%T", obj))
	}
}

func (c *TaskController) handlePod(pod *corev1.Pod) {
	// Verify the Pod belongs to a TaskCluster.
	_, ok := pod.Labels["task-cluster"]
	if !ok {
		return
	}

	// Enqueue the Task's containers.
	for _, container := range pod.Spec.Containers {
		task, err := c.tasksLister.Tasks(pod.GetNamespace()).Get(container.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			utilruntime.HandleErrorWithContext(context.Background(), err, "Error getting Task", "name", container.Name)
			return
		}
		c.enqueueTask(task)
	}
}

func (c *TaskController) handleTaskObject(obj *taskcontrollerv1alpha1.TaskObject) {
	tasks, err := c.tasksLister.Tasks(obj.Namespace).List(labels.Everything())
	if err != nil {
		utilruntime.HandleErrorWithContext(context.Background(), err, "Error listing Tasks")
		return
	}
	for _, task := range tasks {
		if task.Spec.InputObject == obj.Name {
			c.enqueueTask(task)
		}
	}
}

// The only way to update ephemeral containers is by patching, not updating.
// Implementation taken from the "debug" ephemeral container logic in kubelet:
// https://github.com/kubernetes/kubernetes/blob/v1.31.1/staging/src/k8s.io/kubectl/pkg/cmd/debug/debug.go#L482
func (c *TaskController) patchPodEphemeralContainers(ctx context.Context, oldPod *corev1.Pod, newPod *corev1.Pod) error {
	oldPodJS, err := json.Marshal(oldPod)
	if err != nil {
		return fmt.Errorf("Error creating JSON for old pod: %v", err)
	}

	newPodJS, err := json.Marshal(newPod)
	if err != nil {
		return fmt.Errorf("Error creating JSON for new pod: %v", err)
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(oldPodJS, newPodJS, oldPod)
	if err != nil {
		return fmt.Errorf("Error creating patch: %v", err)
	}

	_, err = c.kubeclientset.CoreV1().Pods(oldPod.Namespace).Patch(ctx, oldPod.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{}, "ephemeralcontainers")
	if err != nil {
		// The API server will return a 404 when the EphemeralContainers feature is disabled because the `/ephemeralcontainers` subresource
		// is missing. Unlike the 404 returned by a missing pod, the status details will be empty.
		if serr, ok := err.(*errors.StatusError); ok && serr.Status().Reason == metav1.StatusReasonNotFound && serr.ErrStatus.Details.Name == "" {
			return fmt.Errorf("Ephemeral containers are disabled for this cluster (error from server: %q)", err)
		}
		return err
	}
	return nil
}

func (c *TaskController) markTaskRunning(ctx context.Context, task *taskcontrollerv1alpha1.Task, podName string) error {
	copied := task.DeepCopy()
	copied.Status.PodName = podName
	copied.Status.State = taskcontrollerv1alpha1.TaskStateRunning
	_, err := c.taskclientset.TaskcontrollerV1alpha1().Tasks(copied.Namespace).UpdateStatus(ctx, copied, metav1.UpdateOptions{FieldManager: FieldManager})
	return err
}

func (c *TaskController) markTaskFinished(ctx context.Context, task *taskcontrollerv1alpha1.Task) error {
	copied := task.DeepCopy()
	copied.Status.State = taskcontrollerv1alpha1.TaskStateFinished
	_, err := c.taskclientset.TaskcontrollerV1alpha1().Tasks(copied.Namespace).UpdateStatus(ctx, copied, metav1.UpdateOptions{FieldManager: FieldManager})
	return err
}

func appendTaskContainer(task *taskcontrollerv1alpha1.Task, pod *corev1.Pod) (*corev1.Pod, error) {
	// Find the kube-api-access volume to ensure it is mounted into the
	// ephemeral container so it can call kube-apiserver.
	volumeIndex := slices.IndexFunc(pod.Spec.Volumes, func(v corev1.Volume) bool {
		return strings.HasPrefix(v.Name, "kube-api-access")
	})
	if volumeIndex < 0 {
		return nil, fmt.Errorf("Unable to find kube-api-access volume")
	}
	kubeAPIAccessVol := pod.Spec.Volumes[volumeIndex]

	taskContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:    task.GetName(),
			Image:   task.Spec.Image,
			Command: task.Spec.Command,
			Args:    task.Spec.Args,
			// This is a band-aid to allow locally built images to run
			// (because it tries to pull it from docker remotely).
			ImagePullPolicy: corev1.PullIfNotPresent,
			Env: []corev1.EnvVar{
				// Required for a Task to look up its spec.
				{Name: "TASK_NAME", Value: task.GetName()},
				{Name: "TASK_NAMESPACE", Value: task.GetNamespace()},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      kubeAPIAccessVol.Name,
					MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
					ReadOnly:  true,
				},
			},
		},
	}

	copied := pod.DeepCopy()
	copied.Spec.EphemeralContainers = append(copied.Spec.EphemeralContainers, taskContainer)
	return copied, nil
}

func removeTaskContainer(pod *corev1.Pod, name string) *corev1.Pod {
	containers := []corev1.EphemeralContainer{}
	for _, c := range pod.Spec.EphemeralContainers {
		if c.Name != name {
			containers = append(containers, c)
		}
	}

	copied := pod.DeepCopy()
	copied.Spec.EphemeralContainers = containers
	return copied
}

func isTaskTerminated(pod *corev1.Pod, name string) bool {
	return slices.ContainsFunc(pod.Status.EphemeralContainerStatuses, func(s corev1.ContainerStatus) bool {
		return s.Name == name && s.State.Terminated != nil
	})
	// for _, status := range statuses {
	// if status.Name == name && status.State.Terminated != nil {
	// return true
	// }
	// }
	// return false
}

func containsTaskContainer(pod *corev1.Pod, name string) bool {
	return slices.ContainsFunc(pod.Spec.EphemeralContainers, func(c corev1.EphemeralContainer) bool {
		return c.Name == name
	})
}

func newTerminatedTaskContainers(oldPod *corev1.Pod, newPod *corev1.Pod) []string {
	oldNonTerminated := make(map[string]struct{})
	for _, container := range oldPod.Status.EphemeralContainerStatuses {
		if container.State.Terminated == nil {
			oldNonTerminated[container.Name] = struct{}{}
		}
	}

	newTerminated := []string{}
	for _, container := range newPod.Status.EphemeralContainerStatuses {
		if container.State.Terminated != nil {
			if _, exists := oldNonTerminated[container.Name]; !exists {
				newTerminated = append(newTerminated, container.Name)
			}
		}
	}
	return newTerminated
}
