// package k8s holds all logic for talking to the Kubernetes API.
// Keeping it separate from main lets us reuse and test it independently.
package k8s

import (
	"context"       // carries deadlines/cancellation into API calls
	"fmt"           // formatted printing and error wrapping
	"os"            // access to the operating system, e.g. the home directory
	"path/filepath" // builds file paths in an OS-independent way

	// metav1 holds the common "meta" types shared by all Kubernetes objects,
	// such as the options struct used when listing resources.
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// kubernetes is the typed client ("clientset") for the core K8s API.
	"k8s.io/client-go/kubernetes"
	// clientcmd knows how to read a kubeconfig file and build a connection config.
	"k8s.io/client-go/tools/clientcmd"
)

type PodInfo struct {
	Name   string
	Status string
}

// GetPodNames connects to the cluster and returns the names of the pods
// in the given namespace. It returns an error instead of crashing, so the
// caller decides how to handle failures.
func GetPodNames(namespace string) ([]PodInfo, error) {
	// Ask the OS for the current user's home directory (e.g. /Users/you).
	home, err := os.UserHomeDir()
	if err != nil {
		// %w wraps the original error so the caller can inspect it later.
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}
	// Build the standard kubeconfig path: ~/.kube/config
	kubeconfig := filepath.Join(home, ".kube", "config")

	// Read the kubeconfig file and turn it into a connection config.
	// The empty first argument means "no explicit master URL override".
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	// Create a clientset: the object through which we call the K8s API.
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// CoreV1() selects the core API group, Pods(namespace) scopes to one
	// namespace, and List sends the actual request to the cluster.
	// context.Background() is an empty, never-cancelled context.
	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Pre-allocate a slice with length 0 but capacity for every pod,
	// which avoids re-allocating as we append.
	names := make([]PodInfo, 0, len(pods.Items))

	// Loop over each pod; "_" discards the index since we only need the value.
	for _, pod := range pods.Items {
		// Collect just the pod's name into our result slice.
		names = append(names, PodInfo{Name: pod.Name, Status: string(pod.Status.Phase)})
	}

	// Return the names and a nil error to signal success.
	return names, nil
}
