package dra

import (
	"fmt"
	"strings"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/dynamic-resource-allocation/resourceslice"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/fixtures"
)

const DriverName = "accelerator.scheduling.bupt.dev"

func ResourcesForNode(nodeName string) (resourceslice.DriverResources, error) {
	var topology *schedulingv1alpha1.AcceleratorTopology
	for _, candidate := range fixtures.ThreeNodeTopologies() {
		if candidate.Spec.NodeName == nodeName {
			topology = candidate
			break
		}
	}
	if topology == nil {
		return resourceslice.DriverResources{}, fmt.Errorf("no synthetic topology fixture for node %q", nodeName)
	}

	devices := make([]resourceapi.Device, 0, len(topology.Spec.Devices))
	for _, source := range topology.Spec.Devices {
		attributes := map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
			"vendor":       stringAttribute(string(source.Vendor)),
			"product":      stringAttribute(source.Product),
			"resourceName": stringAttribute(source.ResourceName),
			"memoryMiB":    intAttribute(source.MemoryMiB),
			"health":       stringAttribute(string(source.Health)),
		}
		if source.NUMANode != nil {
			attributes["numaNode"] = intAttribute(int64(*source.NUMANode))
		}
		if source.PCIeRoot != "" {
			attributes["pcieRoot"] = stringAttribute(source.PCIeRoot)
		}
		if source.FabricGroup != "" {
			attributes["fabricGroup"] = stringAttribute(source.FabricGroup)
			attributes["fabricType"] = stringAttribute(strings.SplitN(source.FabricGroup, "-", 2)[0])
		}
		devices = append(devices, resourceapi.Device{Name: source.ID, Attributes: attributes})
	}

	return resourceslice.DriverResources{Pools: map[string]resourceslice.Pool{
		nodeName: {Slices: []resourceslice.Slice{{Devices: devices}}},
	}}, nil
}

func stringAttribute(value string) resourceapi.DeviceAttribute {
	return resourceapi.DeviceAttribute{StringValue: &value}
}

func intAttribute(value int64) resourceapi.DeviceAttribute {
	return resourceapi.DeviceAttribute{IntValue: &value}
}
