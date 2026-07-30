package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/bupt20220407/accelerator-fabric-scheduler/pkg/deviceplugin"
)

func main() {
	var config deviceplugin.Config
	flag.StringVar(&config.ResourceName, "resource-name", "", "qualified extended resource name")
	flag.IntVar(&config.Count, "count", 8, "number of synthetic devices")
	flag.IntVar(&config.DevicesPerNUMA, "devices-per-numa", 4, "number of consecutive devices assigned to each synthetic NUMA node")
	flag.StringVar(&config.SocketDir, "socket-dir", pluginapi.DevicePluginPath, "kubelet device plugin socket directory")
	flag.Parse()

	plugin, err := deviceplugin.New(config)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	log.Printf("registering %s with %d synthetic devices", config.ResourceName, config.Count)
	if err := plugin.Run(ctx); err != nil {
		log.Fatal(fmt.Errorf("run synthetic device plugin: %w", err))
	}
}
