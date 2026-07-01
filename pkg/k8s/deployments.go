package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeploymentInfo struct {
	Name      string
	Replicas  int32
	Available int32
}

func ListDeployments(namespace string) ([]DeploymentInfo, error) {
	clientset, err := newClientset()
	if err != nil {
		return nil, err
	}

	deployList, err := clientset.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	var deploys []DeploymentInfo
	for _, deploy := range deployList.Items {
		deploys = append(deploys, DeploymentInfo{
			Name:      deploy.Name,
			Replicas:  deploy.Status.Replicas,
			Available: deploy.Status.AvailableReplicas,
		})
	}

	return deploys, nil
}
