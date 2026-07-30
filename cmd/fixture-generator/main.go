package main

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"

	"github.com/bupt20220407/accelerator-fabric-scheduler/pkg/fixtures"
)

func main() {
	for index, topology := range fixtures.ThreeNodeTopologies() {
		encoded, err := yaml.Marshal(topology)
		if err != nil {
			fmt.Fprintf(os.Stderr, "marshal topology %s: %v\n", topology.Name, err)
			os.Exit(1)
		}
		if index > 0 {
			fmt.Println("---")
		}
		fmt.Print(string(encoded))
	}
}
