package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	controller "github.com/bupt/accelerator-fabric-scheduler/pkg/controller/topology"
	clientset "github.com/bupt/accelerator-fabric-scheduler/pkg/generated/clientset/versioned"
	informers "github.com/bupt/accelerator-fabric-scheduler/pkg/generated/informers/externalversions"
)

func main() {
	kubeconfig := flag.String("kubeconfig", "", "optional path to a kubeconfig; defaults to in-cluster config")
	workers := flag.Int("workers", 1, "number of topology status workers")
	flag.Parse()

	config, err := buildConfig(*kubeconfig)
	if err != nil {
		log.Fatal(err)
	}
	client, err := clientset.NewForConfig(config)
	if err != nil {
		log.Fatal(fmt.Errorf("create scheduling client: %w", err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	factory := informers.NewSharedInformerFactory(client, 30*time.Second)
	topologyController := controller.New(client, factory.Scheduling().V1alpha1().AcceleratorTopologies())
	factory.Start(ctx.Done())
	if err := topologyController.Run(ctx, *workers); err != nil {
		log.Fatal(err)
	}
}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("build in-cluster config: %w", err)
	}
	return config, nil
}
