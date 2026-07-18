package main

import (
	"os"

	"k8s.io/component-base/cli"
	_ "k8s.io/component-base/metrics/prometheus/clientgo"
	_ "k8s.io/component-base/metrics/prometheus/version"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	"github.com/bupt/accelerator-fabric-scheduler/pkg/plugin/topologyfit"
)

func main() {
	command := app.NewSchedulerCommand(
		app.WithPlugin(topologyfit.Name, topologyfit.New),
	)
	os.Exit(cli.Run(command))
}
