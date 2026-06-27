package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodInfo struct {
	Name   string
	Status string
}

func ListPods(namespace string) ([]PodInfo, error) {
	clientset, err := newClientset()
	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	result := make([]PodInfo, 0, len(pods.Items))
	for _, pod := range pods.Items {
		result = append(result, PodInfo{
			Name:   pod.Name,
			Status: string(pod.Status.Phase),
		})
	}

	return result, nil
}
