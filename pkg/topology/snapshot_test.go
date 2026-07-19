package topology

import (
	"testing"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
)

func TestBuildCreatesDeterministicIndexes(t *testing.T) {
	numa0 := int32(0)
	snapshot, err := Build(schedulingv1alpha1.AcceleratorTopologySpec{
		NodeName:           "worker",
		ObservedGeneration: 7,
		Devices: []schedulingv1alpha1.AcceleratorDevice{
			{ID: "gpu1", Vendor: schedulingv1alpha1.VendorNVIDIA, Product: "H100", ResourceName: "nvidia.com/gpu", NUMANode: &numa0, PCIeRoot: "root0", FabricGroup: "clique0", MemoryMiB: 80, Health: schedulingv1alpha1.HealthHealthy},
			{ID: "gpu0", Vendor: schedulingv1alpha1.VendorNVIDIA, Product: "H100", ResourceName: "nvidia.com/gpu", NUMANode: &numa0, PCIeRoot: "root0", FabricGroup: "clique0", MemoryMiB: 80, Health: schedulingv1alpha1.HealthHealthy},
		},
		Links: []schedulingv1alpha1.AcceleratorLink{
			{Source: "gpu1", Target: "gpu0", Type: schedulingv1alpha1.LinkNVLink, BandwidthGBps: 450, Hops: 1, Health: schedulingv1alpha1.HealthHealthy},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got := snapshot.Devices(); got[0].ID != "gpu0" || got[1].ID != "gpu1" {
		t.Fatalf("devices are not deterministically sorted: %v", got)
	}
	if got := snapshot.DevicesByNUMA(0); len(got) != 2 {
		t.Fatalf("DevicesByNUMA() length = %d, want 2", len(got))
	}
	if link, found := snapshot.LinkBetween("gpu0", "gpu1"); !found || link.Source != "gpu0" || link.Target != "gpu1" {
		t.Fatalf("forward link = (%v, %v)", link, found)
	}
	if link, found := snapshot.LinkBetween("gpu1", "gpu0"); !found || link.Source != "gpu1" || link.Target != "gpu0" {
		t.Fatalf("reverse link = (%v, %v)", link, found)
	}
}

func TestBuildRejectsInvalidGraph(t *testing.T) {
	base := schedulingv1alpha1.AcceleratorTopologySpec{
		NodeName:           "worker",
		ObservedGeneration: 1,
		Devices: []schedulingv1alpha1.AcceleratorDevice{
			{ID: "gpu0", Vendor: schedulingv1alpha1.VendorNVIDIA, Product: "H100", ResourceName: "nvidia.com/gpu", Health: schedulingv1alpha1.HealthHealthy},
		},
	}

	tests := map[string]func(*schedulingv1alpha1.AcceleratorTopologySpec){
		"duplicate device": func(spec *schedulingv1alpha1.AcceleratorTopologySpec) {
			spec.Devices = append(spec.Devices, spec.Devices[0])
		},
		"missing endpoint": func(spec *schedulingv1alpha1.AcceleratorTopologySpec) {
			spec.Links = []schedulingv1alpha1.AcceleratorLink{{Source: "gpu0", Target: "gpu9", Type: schedulingv1alpha1.LinkNVLink, Health: schedulingv1alpha1.HealthHealthy}}
		},
		"self link": func(spec *schedulingv1alpha1.AcceleratorTopologySpec) {
			spec.Links = []schedulingv1alpha1.AcceleratorLink{{Source: "gpu0", Target: "gpu0", Type: schedulingv1alpha1.LinkNVLink, Health: schedulingv1alpha1.HealthHealthy}}
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			spec := base
			spec.Devices = append([]schedulingv1alpha1.AcceleratorDevice(nil), base.Devices...)
			mutate(&spec)
			if _, err := Build(spec); err == nil {
				t.Fatal("Build() accepted invalid graph")
			}
		})
	}
}

func TestSnapshotDoesNotExposeMutableNUMAPointer(t *testing.T) {
	numa := int32(0)
	snapshot, err := Build(schedulingv1alpha1.AcceleratorTopologySpec{
		NodeName:           "worker",
		ObservedGeneration: 1,
		Devices: []schedulingv1alpha1.AcceleratorDevice{
			{ID: "gpu0", Vendor: schedulingv1alpha1.VendorNVIDIA, Product: "H100", ResourceName: "nvidia.com/gpu", NUMANode: &numa, Health: schedulingv1alpha1.HealthHealthy},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	device, _ := snapshot.Device("gpu0")
	*device.NUMANode = 9
	device, _ = snapshot.Device("gpu0")
	if *device.NUMANode != 0 {
		t.Fatalf("snapshot NUMA was mutated: %d", *device.NUMANode)
	}
}
