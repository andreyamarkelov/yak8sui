package main

import (
	"log"

	"yak8sui/pkg/ui"
)

func main() {
	app := ui.New(ui.Config{Namespace: "kube-system"})

	if err := app.Run(); err != nil {
		log.Fatalf("error running app: %v", err)
	}
}
