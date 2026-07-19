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
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/klog/v2"

	"github.com/bupt/accelerator-fabric-scheduler/pkg/dra"
	schedulingclient "github.com/bupt/accelerator-fabric-scheduler/pkg/generated/clientset/versioned"
	schedulinginformers "github.com/bupt/accelerator-fabric-scheduler/pkg/generated/informers/externalversions"
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
	schedulingClient, err := schedulingclient.NewForConfig(config)
	if err != nil {
		log.Fatal(fmt.Errorf("create scheduling client: %w", err))
	}
	driver, err := dra.NewDriver(dra.DriverName, *nodeName, filepath.Join(*stateRoot, "prepared.json"), *cdiDir)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("recovered %d prepared DRA claims on node %s", driver.PreparedCount(), *nodeName)

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
	factory := schedulinginformers.NewSharedInformerFactory(schedulingClient, 30*time.Second)
	topologyInformer := factory.Scheduling().V1alpha1().AcceleratorTopologies()
	if _, err := dra.ObserveTopologies(ctx, topologyInformer, *nodeName, helper.PublishResources, time.Now, func(err error) {
		klog.ErrorS(err, "publish DRA topology resources", "node", *nodeName)
	}); err != nil {
		log.Fatal(err)
	}
	factory.Start(ctx.Done())
	if !toolscache.WaitForCacheSync(ctx.Done(), topologyInformer.Informer().HasSynced) {
		log.Fatal("wait for topology informer cache sync")
	}
	<-ctx.Done()
}
