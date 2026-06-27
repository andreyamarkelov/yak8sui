package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ListNamespaces() ([]string, error) {
	clientset, err := newClientset()
	if err != nil {
		return nil, err
	}

	nsList, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	result := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		result = append(result, ns.Name)
	}

	return result, nil
}
