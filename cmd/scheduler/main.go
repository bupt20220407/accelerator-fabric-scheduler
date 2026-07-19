package main

import (
	"os"

	"k8s.io/component-base/cli"
	_ "k8s.io/component-base/metrics/prometheus/clientgo"
	_ "k8s.io/component-base/metrics/prometheus/version"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	"github.com/bupt/accelerator-fabric-scheduler/pkg/observability"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/plugin/topologyfit"
)

func main() {
	observability.RegisterSchedulerMetrics()
	command := app.NewSchedulerCommand(
		app.WithPlugin(topologyfit.Name, topologyfit.New),
	)
	os.Exit(cli.Run(command))
}
