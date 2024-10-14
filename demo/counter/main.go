package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	taskv1alpha1 "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/apis/taskcontroller/v1alpha1"
	clientset "github.com/chrishenzie/k8s-dynamic-batch-demo/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error creating Kubernetes config: %v", err)
	}
	client, err := clientset.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating clientset: %v", err)
	}

	self, err := taskDefinition(client)
	if err != nil {
		log.Fatalf("Error getting task: %v", err)
	}
	input, err := taskInput(client, self)
	if err != nil {
		log.Fatalf("Error getting task input: %v", err)
	}
	count, limit, err := parseInput(input)
	if err != nil {
		log.Fatalf("Error parsing task input: %v", err)
	}

	if count >= limit {
		return
	}
	err = createNextTask(client, self, count+1, limit)
	if err != nil {
		log.Fatalf("Error creating next task: %v", err)
	}
}

func taskDefinition(client clientset.Interface) (*taskv1alpha1.Task, error) {
	name, ok := os.LookupEnv("TASK_NAME")
	if !ok {
		return nil, fmt.Errorf("Error getting task name: $TASK_NAME is not set")
	}
	namespace, ok := os.LookupEnv("TASK_NAMESPACE")
	if !ok {
		return nil, fmt.Errorf("Error getting task namespace: $TASK_NAMESPACE is not set")
	}
	return client.TaskcontrollerV1alpha1().Tasks(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func taskInput(client clientset.Interface, task *taskv1alpha1.Task) (map[string]string, error) {
	input, err := client.TaskcontrollerV1alpha1().TaskObjects(task.Namespace).Get(context.TODO(), task.Spec.InputObject, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return input.Data, nil
}

func parseInput(input map[string]string) (int, int, error) {
	count, ok := input["count"]
	if !ok {
		return -1, -1, fmt.Errorf("count not present in input")
	}
	limit, ok := input["limit"]
	if !ok {
		return -1, -1, fmt.Errorf("limit not present in input")
	}
	parsedCount, err := strconv.Atoi(count)
	if err != nil {
		return -1, -1, err
	}
	parsedLimit, err := strconv.Atoi(limit)
	if err != nil {
		return -1, -1, err
	}
	return parsedCount, parsedLimit, nil
}

func createNextTask(client clientset.Interface, task *taskv1alpha1.Task, count int, limit int) error {
	output := &taskv1alpha1.TaskObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      task.Name + "-output",
			Namespace: task.Namespace,
		},
		Data: map[string]string{
			"count": strconv.Itoa(count),
			"limit": strconv.Itoa(limit),
		},
	}
	_, err := client.TaskcontrollerV1alpha1().TaskObjects(task.Namespace).Create(context.TODO(), output, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	next := &taskv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("task-%v", count),
			Namespace: task.Namespace,
		},
		Spec: taskv1alpha1.TaskSpec{
			ClusterName: task.Spec.ClusterName,
			Image:       task.Spec.Image,
			Command:     task.Spec.Command,
			Args:        task.Spec.Args,
			InputObject: output.Name,
		},
	}
	_, err = client.TaskcontrollerV1alpha1().Tasks(task.Namespace).Create(context.TODO(), next, metav1.CreateOptions{})
	return err
}
