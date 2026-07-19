package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"

	"github.com/bupt/accelerator-fabric-scheduler/pkg/dra"
)

func main() {
	nodeName := flag.String("node-name", os.Getenv("NODE_NAME"), "node where this DRA plugin runs")
	stateRoot := flag.String("state-root", filepath.Join(kubeletplugin.KubeletPluginsDir, dra.DriverName), "persistent plugin state directory")
	cdiDir := flag.String("cdi-dir", "/var/run/cdi", "CDI specification directory shared with the container runtime")
	flag.Parse()
	if *nodeName == "" {
		log.Fatal("node-name is required")
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(fmt.Errorf("build in-cluster config: %w", err))
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(fmt.Errorf("create Kubernetes client: %w", err))
	}
	driver, err := dra.NewDriver(dra.DriverName, *nodeName, filepath.Join(*stateRoot, "prepared.json"), *cdiDir)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("recovered %d prepared DRA claims on node %s", driver.PreparedCount(), *nodeName)
	resources, err := dra.ResourcesForNode(*nodeName)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	helper, err := kubeletplugin.Start(ctx, driver,
		kubeletplugin.DriverName(dra.DriverName),
		kubeletplugin.NodeName(*nodeName),
		kubeletplugin.KubeClient(client),
		kubeletplugin.NodeV1(true),
		kubeletplugin.NodeV1beta1(false),
	)
	if err != nil {
		log.Fatal(fmt.Errorf("start DRA kubelet plugin: %w", err))
	}
	defer helper.Stop()
	if err := helper.PublishResources(ctx, resources); err != nil {
		log.Fatal(fmt.Errorf("publish DRA resources: %w", err))
	}
	<-ctx.Done()
}
