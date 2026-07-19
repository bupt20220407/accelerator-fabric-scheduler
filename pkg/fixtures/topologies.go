package fixtures

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
)

type acceleratorFixture struct {
	nodeName     string
	prefix       string
	vendor       schedulingv1alpha1.AcceleratorVendor
	product      string
	resourceName string
	linkType     schedulingv1alpha1.LinkType
	bandwidth    float64
	memoryMiB    int64
}

func ThreeNodeTopologies() []*schedulingv1alpha1.AcceleratorTopology {
	definitions := []acceleratorFixture{
		{nodeName: "accelerator-fabric-worker", prefix: "gpu", vendor: schedulingv1alpha1.VendorNVIDIA, product: "H100-SXM", resourceName: "nvidia.com/gpu", linkType: schedulingv1alpha1.LinkNVLink, bandwidth: 450, memoryMiB: 81920},
		{nodeName: "accelerator-fabric-worker2", prefix: "ascend", vendor: schedulingv1alpha1.VendorHuawei, product: "Ascend-910B", resourceName: "huawei.com/Ascend910", linkType: schedulingv1alpha1.LinkHCCS, bandwidth: 392, memoryMiB: 65536},
		{nodeName: "accelerator-fabric-worker3", prefix: "gpu", vendor: schedulingv1alpha1.VendorAMD, product: "MI300X", resourceName: "amd.com/gpu", linkType: schedulingv1alpha1.LinkXGMI, bandwidth: 400, memoryMiB: 196608},
	}

	topologies := make([]*schedulingv1alpha1.AcceleratorTopology, 0, len(definitions))
	for _, definition := range definitions {
		topologies = append(topologies, buildTopology(definition))
	}
	return topologies
}

func buildTopology(fixture acceleratorFixture) *schedulingv1alpha1.AcceleratorTopology {
	devices := make([]schedulingv1alpha1.AcceleratorDevice, 0, 8)
	for index := 0; index < 8; index++ {
		numa := int32(index / 4)
		devices = append(devices, schedulingv1alpha1.AcceleratorDevice{
			ID:           fmt.Sprintf("%s%d", fixture.prefix, index),
			Vendor:       fixture.vendor,
			Product:      fixture.product,
			ResourceName: fixture.resourceName,
			NUMANode:     &numa,
			PCIeRoot:     fmt.Sprintf("0000:%02x", 0x30+(index/4)*0x50),
			FabricGroup:  fmt.Sprintf("%s-clique-%d", fixture.linkType, index/4),
			MemoryMiB:    fixture.memoryMiB,
			Health:       schedulingv1alpha1.HealthHealthy,
		})
	}

	links := make([]schedulingv1alpha1.AcceleratorLink, 0, 13)
	for group := 0; group < 2; group++ {
		start := group * 4
		for left := start; left < start+4; left++ {
			for right := left + 1; right < start+4; right++ {
				links = append(links, schedulingv1alpha1.AcceleratorLink{
					Source:        devices[left].ID,
					Target:        devices[right].ID,
					Type:          fixture.linkType,
					BandwidthGBps: fixture.bandwidth,
					Hops:          1,
					Health:        schedulingv1alpha1.HealthHealthy,
				})
			}
		}
	}
	links = append(links, schedulingv1alpha1.AcceleratorLink{
		Source:        devices[3].ID,
		Target:        devices[4].ID,
		Type:          schedulingv1alpha1.LinkPCIe,
		BandwidthGBps: 64,
		Hops:          2,
		Health:        schedulingv1alpha1.HealthHealthy,
	})

	return &schedulingv1alpha1.AcceleratorTopology{
		TypeMeta: metav1.TypeMeta{APIVersion: schedulingv1alpha1.SchemeGroupVersion.String(), Kind: "AcceleratorTopology"},
		ObjectMeta: metav1.ObjectMeta{
			Name: fixture.nodeName,
		},
		Spec: schedulingv1alpha1.AcceleratorTopologySpec{
			NodeName:           fixture.nodeName,
			ObservedGeneration: 1,
			Source:             schedulingv1alpha1.TopologySourceFixture,
			Devices:            devices,
			Links:              links,
		},
	}
}
