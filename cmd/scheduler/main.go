package main

import (
	"os"

	"k8s.io/component-base/cli"
	_ "k8s.io/component-base/metrics/prometheus/clientgo"
	_ "k8s.io/component-base/metrics/prometheus/version"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
	controllerruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	_ "sigs.k8s.io/scheduler-plugins/apis/config/scheme"
	"sigs.k8s.io/scheduler-plugins/pkg/coscheduling"

	"github.com/bupt20220407/accelerator-fabric-scheduler/pkg/observability"
	"github.com/bupt20220407/accelerator-fabric-scheduler/pkg/plugin/topologyfit"
)

func main() {
	observability.RegisterSchedulerMetrics()
	controllerruntimelog.SetLogger(klog.NewKlogr())
	command := app.NewSchedulerCommand(
		app.WithPlugin(topologyfit.Name, topologyfit.New),
		app.WithPlugin(coscheduling.Name, coscheduling.New),
	)
	os.Exit(cli.Run(command))
}
