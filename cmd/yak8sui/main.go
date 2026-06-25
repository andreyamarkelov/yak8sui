package main

import (
	"fmt" // printing to standard output
	"log" // logging that can also stop the program (log.Fatalf)

	// our own package; the import path is "module name" + folder path.
	"yak8sui/pkg/k8s"
)

func main() {
	// The namespace we want to inspect. Hardcoded for now.
	namespace := "kube-system"
	fmt.Printf("Fetching pods from namespace: [%s]...\n\n", namespace)

	// Call into our k8s package; it returns the names, status and a possible error.
	pods, err := k8s.GetPodNames(namespace)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	if len(pods) == 0 {
		fmt.Println("No pods found in this namespace.")
		return
	}

	for i, pod := range pods {
		fmt.Printf("%d. %s (%s)\n", i+1, pod.Name, pod.Status)
	}
}
