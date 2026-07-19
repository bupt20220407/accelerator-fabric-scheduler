package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	policycontroller "github.com/bupt/accelerator-fabric-scheduler/pkg/controller/policy"
	topologycontroller "github.com/bupt/accelerator-fabric-scheduler/pkg/controller/topology"
	clientset "github.com/bupt/accelerator-fabric-scheduler/pkg/generated/clientset/versioned"
	informers "github.com/bupt/accelerator-fabric-scheduler/pkg/generated/informers/externalversions"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/observability"
)

func main() {
	kubeconfig := flag.String("kubeconfig", "", "optional path to a kubeconfig; defaults to in-cluster config")
	workers := flag.Int("workers", 1, "number of topology status workers")
	metricsAddress := flag.String("metrics-bind-address", ":8080", "address for the metrics and health server; empty disables it")
	flag.Parse()
	observability.RegisterControllerMetrics()

	config, err := buildConfig(*kubeconfig)
	if err != nil {
		log.Fatal(err)
	}
	client, err := clientset.NewForConfig(config)
	if err != nil {
		log.Fatal(fmt.Errorf("create scheduling client: %w", err))
	}
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(fmt.Errorf("create Kubernetes client: %w", err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	factory := informers.NewSharedInformerFactory(client, 30*time.Second)
	topologyController := topologycontroller.New(client, factory.Scheduling().V1alpha1().AcceleratorTopologies())
	policyController := policycontroller.New(kubeClient, factory.Scheduling().V1alpha1().AcceleratorPlacementPolicies())
	factory.Start(ctx.Done())
	errCh := make(chan error, 3)
	go func() { errCh <- topologyController.Run(ctx, *workers) }()
	go func() { errCh <- policyController.Run(ctx, *workers) }()
	go func() { errCh <- observability.Serve(ctx, *metricsAddress) }()
	if err := <-errCh; err != nil {
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
