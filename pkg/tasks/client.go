package tasks

import (
	"context"
	"fmt"
	"os"

	"k8s.io/client-go/rest"

	clientset "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/clientset/versioned"
)

type TaskClient interface {
	InputTaskObject(ctx context.Context) (map[string]string, error)
	CreateTask(ctx context.Context) error
}

type taskClient struct {
	client clientset.Interface
}

func NewTaskClient() (TaskClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("Error creating Kubernetes config: %v", err)
	}
	client, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Error creating clientset: %v", err)
	}
	return &taskClient{client: client}, nil
}

func (t *taskClient) InputTaskObject(ctx context.Context) (map[string]string, error) {
	name, ok := os.LookupEnv("TASK_NAME")
	if !ok {
		return nil, fmt.Errorf("$TASK_NAME is not set")
	}
	namespace, ok := os.LookupEnv("TASK_NAMESPACE")
	if !ok {
		return nil, fmt.Errorf("$TASK_NAMESPACE is not set")
	}

	task, err := t.client.TaskcontrollerV1alpha1().Tasks(t.namespace).Get(ctx, t.name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error getting Task definition: %v", err)
	}

	// If a Task doesn't specify input, return nothing.
	if task.Spec.InputObject == "" {
		return nil, nil
	}

	input, err := t.client.TaskcontrollerV1alpha1().TaskObjects(task.Namespace).Get(ctx, task.Spec.InputObject, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error getting input TaskObject: %v", err)
	}
	return input.Data, nil
}
