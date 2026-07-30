package discovery

import (
	"context"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	schedulingv1alpha1 "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	fakeclient "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/clientset/versioned/fake"
)

type staticProvider struct {
	name string
	spec schedulingv1alpha1.AcceleratorTopologySpec
}

func (p staticProvider) Name() string { return p.name }
func (p staticProvider) Discover(context.Context, string) (schedulingv1alpha1.AcceleratorTopologySpec, error) {
	return p.spec, nil
}

func TestAgentCreatesAndRefreshesTopologyWithoutChangingGeneration(t *testing.T) {
	ctx := context.Background()
	provider := staticProvider{name: "test", spec: testSpec()}
	client := fakeclient.NewSimpleClientset()
	agent, err := NewAgent(client, provider, "worker")
	if err != nil {
		t.Fatal(err)
	}
	first := time.Date(2026, 7, 19, 1, 2, 3, 0, time.UTC)
	agent.now = func() time.Time { return first }
	if err := agent.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	object, err := client.SchedulingV1alpha1().AcceleratorTopologies().Get(ctx, "worker", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if object.Spec.ObservedGeneration != 1 || object.Labels[ManagedByLabel] != ManagedByValue || object.Labels[ProviderLabel] != "test" {
		t.Fatalf("unexpected created object: %#v", object)
	}
	if object.Annotations[DiscoveredAtAnnotation] != first.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected discovery timestamp %q", object.Annotations[DiscoveredAtAnnotation])
	}

	second := first.Add(time.Minute)
	agent.now = func() time.Time { return second }
	if err := agent.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	object, _ = client.SchedulingV1alpha1().AcceleratorTopologies().Get(ctx, "worker", metav1.GetOptions{})
	if object.Spec.ObservedGeneration != 1 {
		t.Fatalf("unchanged discovery advanced observed generation to %d", object.Spec.ObservedGeneration)
	}
	if object.Annotations[DiscoveredAtAnnotation] != second.Format(time.RFC3339Nano) {
		t.Fatalf("discovery timestamp was not refreshed")
	}
}

func TestAgentAdvancesGenerationOnlyWhenContentChanges(t *testing.T) {
	ctx := context.Background()
	spec := testSpec()
	object := &schedulingv1alpha1.AcceleratorTopology{
		ObjectMeta: metav1.ObjectMeta{Name: "worker", Labels: map[string]string{ManagedByLabel: ManagedByValue, ProviderLabel: "test"}},
		Spec:       spec,
	}
	object.Spec.ObservedGeneration = 7
	changed := testSpec()
	changed.Devices[0].Health = schedulingv1alpha1.HealthUnhealthy
	client := fakeclient.NewSimpleClientset(object)
	agent, _ := NewAgent(client, staticProvider{name: "test", spec: changed}, "worker")
	if err := agent.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	updated, _ := client.SchedulingV1alpha1().AcceleratorTopologies().Get(ctx, "worker", metav1.GetOptions{})
	if updated.Spec.ObservedGeneration != 8 || updated.Spec.Devices[0].Health != schedulingv1alpha1.HealthUnhealthy {
		t.Fatalf("unexpected updated topology: %#v", updated.Spec)
	}
}

func TestAgentAdoptsOnlyMatchingUnmanagedTopology(t *testing.T) {
	ctx := context.Background()
	spec := testSpec()
	spec.ObservedGeneration = 4
	legacy := &schedulingv1alpha1.AcceleratorTopology{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Spec: spec}
	client := fakeclient.NewSimpleClientset(legacy)
	agent, _ := NewAgent(client, staticProvider{name: "fixture", spec: testSpec()}, "worker")
	if err := agent.Reconcile(ctx); err != nil {
		t.Fatalf("matching legacy topology was not adopted: %v", err)
	}
	adopted, _ := client.SchedulingV1alpha1().AcceleratorTopologies().Get(ctx, "worker", metav1.GetOptions{})
	if adopted.Labels[ManagedByLabel] != ManagedByValue || adopted.Spec.ObservedGeneration != 4 {
		t.Fatalf("unexpected adopted topology: %#v", adopted)
	}

	conflict := legacy.DeepCopy()
	conflict.Spec.Devices[0].Product = "different"
	conflict.ResourceVersion = ""
	other := fakeclient.NewSimpleClientset(conflict)
	agent, _ = NewAgent(other, staticProvider{name: "fixture", spec: testSpec()}, "worker")
	if err := agent.Reconcile(ctx); err == nil || !strings.Contains(err.Error(), "refusing to adopt") {
		t.Fatalf("expected adoption refusal, got %v", err)
	}
}

func TestAgentRejectsOtherManagerOrProvider(t *testing.T) {
	for name, labels := range map[string]map[string]string{
		"manager":  {ManagedByLabel: "vendor-operator"},
		"provider": {ManagedByLabel: ManagedByValue, ProviderLabel: "other-provider"},
	} {
		t.Run(name, func(t *testing.T) {
			object := &schedulingv1alpha1.AcceleratorTopology{
				ObjectMeta: metav1.ObjectMeta{Name: "worker", Labels: labels},
				Spec:       testSpec(),
			}
			client := fakeclient.NewSimpleClientset(object)
			agent, _ := NewAgent(client, staticProvider{name: "fixture", spec: testSpec()}, "worker")
			if err := agent.Reconcile(context.Background()); err == nil {
				t.Fatal("expected ownership conflict")
			}
		})
	}
}

func testSpec() schedulingv1alpha1.AcceleratorTopologySpec {
	numa := int32(0)
	return schedulingv1alpha1.AcceleratorTopologySpec{
		NodeName: "worker", ObservedGeneration: 1, Source: schedulingv1alpha1.TopologySourceFixture,
		Devices: []schedulingv1alpha1.AcceleratorDevice{{
			ID: "gpu0", Vendor: schedulingv1alpha1.VendorNVIDIA, Product: "test",
			ResourceName: "nvidia.com/gpu", NUMANode: &numa, MemoryMiB: 1,
			Health: schedulingv1alpha1.HealthHealthy,
		}},
	}
}
