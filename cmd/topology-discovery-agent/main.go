package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/bupt/accelerator-fabric-scheduler/pkg/discovery"
	clientset "github.com/bupt/accelerator-fabric-scheduler/pkg/generated/clientset/versioned"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/observability"
)

func main() {
	kubeconfig := flag.String("kubeconfig", "", "optional path to a kubeconfig; defaults to in-cluster config")
	nodeName := flag.String("node-name", os.Getenv("NODE_NAME"), "node to discover")
	providerName := flag.String("provider", "fixture", "discovery provider: fixture or nvidia-smi")
	interval := flag.Duration("interval", 20*time.Second, "discovery refresh interval")
	metricsAddress := flag.String("metrics-bind-address", ":8080", "address for metrics and health; empty disables it")
	flag.Parse()
	if *interval <= 0 {
		log.Fatal("interval must be positive")
	}
	provider, err := buildProvider(*providerName)
	if err != nil {
		log.Fatal(err)
	}
	config, err := buildConfig(*kubeconfig)
	if err != nil {
		log.Fatal(err)
	}
	client, err := clientset.NewForConfig(config)
	if err != nil {
		log.Fatal(fmt.Errorf("create scheduling client: %w", err))
	}
	agent, err := discovery.NewAgent(client, provider, *nodeName)
	if err != nil {
		log.Fatal(err)
	}

	observability.RegisterDiscoveryMetrics()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	go func() {
		if err := observability.Serve(ctx, *metricsAddress); err != nil {
			log.Printf("metrics server stopped: %v", err)
			cancel()
		}
	}()
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		started := time.Now()
		err := agent.Reconcile(ctx)
		observability.ObserveDiscovery(provider.Name(), err == nil, time.Since(started))
		if err != nil {
			log.Printf("topology discovery failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func buildProvider(name string) (discovery.Provider, error) {
	switch name {
	case "fixture":
		return discovery.FixtureProvider{}, nil
	case "nvidia-smi":
		return discovery.NewNVIDIAProvider(nil), nil
	default:
		return nil, fmt.Errorf("unsupported discovery provider %q", name)
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
