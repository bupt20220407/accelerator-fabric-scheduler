package main

import (
	"fmt"
	"os"
)

func main() {
	resource := os.Getenv("SYNTHETIC_ACCELERATOR_RESOURCE")
	ids := os.Getenv("SYNTHETIC_ACCELERATOR_IDS")
	if resource == "" || ids == "" {
		fmt.Fprintln(os.Stderr, "synthetic accelerator allocation environment is missing")
		os.Exit(1)
	}
	fmt.Printf("resource=%s ids=%s\n", resource, ids)
}
