// package main marks this as an executable program (not a library).
package main

import (
	"fmt" // printing to standard output
	"log" // logging that can also stop the program (log.Fatalf)

	// our own package; the import path is "module name" + folder path.
	"yak8sui/pkg/k8s"
)

// main is the entry point: execution starts here when the binary runs.
func main() {
	// The namespace we want to inspect. Hardcoded for now.
	namespace := "default"

	// Tell the user what we're about to do.
	fmt.Printf("Fetching pods from namespace: [%s]...\n\n", namespace)

	// Call into our k8s package; it returns the names and a possible error.
	pods, err := k8s.GetPodNames(namespace)
	if err != nil {
		// Fatalf prints the message and exits with a non-zero status code.
		log.Fatalf("error: %v", err)
	}

	// An empty (but non-error) result just means nothing is running there.
	if len(pods) == 0 {
		fmt.Println("No pods found in this namespace.")
		return
	}

	// Print each pod on its own numbered line.
	// Here we keep the index i (starting at 0) and add 1 for human counting.
	for i, name := range pods {
		fmt.Printf("%d. %s\n", i+1, name)
	}
}
