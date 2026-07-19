package dra

import (
	"fmt"
	"math"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/dynamic-resource-allocation/resourceslice"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/topology"
)

const (
	DriverName      = "accelerator.scheduling.bupt.dev"
	DeviceClassName = "bupt-accelerator"
)

type fabricFacts struct {
	found        bool
	linkType     schedulingv1alpha1.LinkType
	minBandwidth float64
	maxHops      int32
}

func ResourcesForTopology(object *schedulingv1alpha1.AcceleratorTopology) (resourceslice.DriverResources, error) {
	if object == nil {
		return resourceslice.DriverResources{}, fmt.Errorf("topology is nil")
	}
	if _, err := topology.Build(object.Spec); err != nil {
		return resourceslice.DriverResources{}, err
	}

	deviceByID := make(map[string]schedulingv1alpha1.AcceleratorDevice, len(object.Spec.Devices))
	fabricByID := make(map[string]fabricFacts, len(object.Spec.Devices))
	for _, device := range object.Spec.Devices {
		deviceByID[device.ID] = device
	}
	for _, link := range object.Spec.Links {
		if link.Type == schedulingv1alpha1.LinkPCIe || link.Health != schedulingv1alpha1.HealthHealthy {
			continue
		}
		left, right := deviceByID[link.Source], deviceByID[link.Target]
		if left.FabricGroup == "" || left.FabricGroup != right.FabricGroup {
			continue
		}
		for _, deviceID := range []string{link.Source, link.Target} {
			facts := fabricByID[deviceID]
			if !facts.found || link.BandwidthGBps < facts.minBandwidth {
				facts.minBandwidth = link.BandwidthGBps
			}
			if link.Hops > facts.maxHops {
				facts.maxHops = link.Hops
			}
			facts.found = true
			facts.linkType = link.Type
			fabricByID[deviceID] = facts
		}
	}

	devices := make([]resourceapi.Device, 0, len(object.Spec.Devices))
	for _, source := range object.Spec.Devices {
		attributes := map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
			"vendor":             stringAttribute(string(source.Vendor)),
			"product":            stringAttribute(source.Product),
			"resourceName":       stringAttribute(source.ResourceName),
			"memoryMiB":          intAttribute(source.MemoryMiB),
			"health":             stringAttribute(string(source.Health)),
			"observedGeneration": intAttribute(object.Spec.ObservedGeneration),
		}
		if source.NUMANode != nil {
			attributes["numaNode"] = intAttribute(int64(*source.NUMANode))
		}
		if source.PCIeRoot != "" {
			attributes["pcieRoot"] = stringAttribute(source.PCIeRoot)
		}
		if source.FabricGroup != "" {
			attributes["fabricGroup"] = stringAttribute(source.FabricGroup)
		}
		if facts := fabricByID[source.ID]; facts.found {
			attributes["fabricType"] = stringAttribute(string(facts.linkType))
			attributes["fabricBandwidthGBps"] = intAttribute(int64(math.Floor(facts.minBandwidth)))
			attributes["fabricHops"] = intAttribute(int64(facts.maxHops))
		}
		devices = append(devices, resourceapi.Device{Name: source.ID, Attributes: attributes})
	}

	nodeName := object.Spec.NodeName
	return resourceslice.DriverResources{Pools: map[string]resourceslice.Pool{
		nodeName: {Slices: []resourceslice.Slice{{Devices: devices}}},
	}}, nil
}

func EmptyResources() resourceslice.DriverResources {
	return resourceslice.DriverResources{Pools: map[string]resourceslice.Pool{}}
}

func stringAttribute(value string) resourceapi.DeviceAttribute {
	return resourceapi.DeviceAttribute{StringValue: &value}
}

func intAttribute(value int64) resourceapi.DeviceAttribute {
	return resourceapi.DeviceAttribute{IntValue: &value}
}
