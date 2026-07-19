package topology

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
)

type Snapshot struct {
	nodeName           string
	observedGeneration int64
	devices            []schedulingv1alpha1.AcceleratorDevice
	links              []schedulingv1alpha1.AcceleratorLink
	deviceIndex        map[string]int
	numaIndex          map[int32][]int
	unknownNUMA        []int
	pcieRootIndex      map[string][]int
	fabricGroupIndex   map[string][]int
	adjacency          map[string]map[string]schedulingv1alpha1.AcceleratorLink
}

func Build(spec schedulingv1alpha1.AcceleratorTopologySpec) (*Snapshot, error) {
	if spec.NodeName == "" {
		return nil, fmt.Errorf("nodeName is required")
	}
	if spec.ObservedGeneration < 1 {
		return nil, fmt.Errorf("observedGeneration must be positive")
	}
	if len(spec.Devices) == 0 {
		return nil, fmt.Errorf("at least one accelerator device is required")
	}

	devices := cloneDevices(spec.Devices)
	sort.Slice(devices, func(i, j int) bool { return devices[i].ID < devices[j].ID })

	snapshot := &Snapshot{
		nodeName:           spec.NodeName,
		observedGeneration: spec.ObservedGeneration,
		devices:            devices,
		deviceIndex:        make(map[string]int, len(devices)),
		numaIndex:          make(map[int32][]int),
		pcieRootIndex:      make(map[string][]int),
		fabricGroupIndex:   make(map[string][]int),
		adjacency:          make(map[string]map[string]schedulingv1alpha1.AcceleratorLink, len(devices)),
	}

	for i, device := range devices {
		if err := validateDevice(device); err != nil {
			return nil, fmt.Errorf("device %q: %w", device.ID, err)
		}
		if _, exists := snapshot.deviceIndex[device.ID]; exists {
			return nil, fmt.Errorf("duplicate device ID %q", device.ID)
		}
		snapshot.deviceIndex[device.ID] = i
		snapshot.adjacency[device.ID] = make(map[string]schedulingv1alpha1.AcceleratorLink)
		if device.NUMANode == nil {
			snapshot.unknownNUMA = append(snapshot.unknownNUMA, i)
		} else {
			snapshot.numaIndex[*device.NUMANode] = append(snapshot.numaIndex[*device.NUMANode], i)
		}
		if device.PCIeRoot != "" {
			snapshot.pcieRootIndex[device.PCIeRoot] = append(snapshot.pcieRootIndex[device.PCIeRoot], i)
		}
		if device.FabricGroup != "" {
			snapshot.fabricGroupIndex[device.FabricGroup] = append(snapshot.fabricGroupIndex[device.FabricGroup], i)
		}
	}

	links := append([]schedulingv1alpha1.AcceleratorLink(nil), spec.Links...)
	sort.Slice(links, func(i, j int) bool { return linkKey(links[i]) < linkKey(links[j]) })
	seenLinks := make(map[string]struct{}, len(links))
	for i := range links {
		link := links[i]
		if err := validateLink(link, snapshot.deviceIndex); err != nil {
			return nil, fmt.Errorf("link %q-%q: %w", link.Source, link.Target, err)
		}
		key := linkKey(link)
		if _, exists := seenLinks[key]; exists {
			return nil, fmt.Errorf("duplicate undirected link %q", key)
		}
		seenLinks[key] = struct{}{}

		if link.Target < link.Source {
			link.Source, link.Target = link.Target, link.Source
			links[i] = link
		}
		snapshot.adjacency[link.Source][link.Target] = link
		reverse := link
		reverse.Source, reverse.Target = link.Target, link.Source
		snapshot.adjacency[link.Target][link.Source] = reverse
	}
	snapshot.links = links

	return snapshot, nil
}

func (s *Snapshot) NodeName() string { return s.nodeName }

func (s *Snapshot) ObservedGeneration() int64 { return s.observedGeneration }

func (s *Snapshot) Devices() []schedulingv1alpha1.AcceleratorDevice {
	return cloneDevices(s.devices)
}

func (s *Snapshot) Links() []schedulingv1alpha1.AcceleratorLink {
	return append([]schedulingv1alpha1.AcceleratorLink(nil), s.links...)
}

func (s *Snapshot) Device(id string) (schedulingv1alpha1.AcceleratorDevice, bool) {
	index, found := s.deviceIndex[id]
	if !found {
		return schedulingv1alpha1.AcceleratorDevice{}, false
	}
	return cloneDevice(s.devices[index]), true
}

func (s *Snapshot) DevicesByNUMA(numa int32) []schedulingv1alpha1.AcceleratorDevice {
	return s.devicesAt(s.numaIndex[numa])
}

func (s *Snapshot) DevicesWithUnknownNUMA() []schedulingv1alpha1.AcceleratorDevice {
	return s.devicesAt(s.unknownNUMA)
}

func (s *Snapshot) DevicesByPCIeRoot(root string) []schedulingv1alpha1.AcceleratorDevice {
	return s.devicesAt(s.pcieRootIndex[root])
}

func (s *Snapshot) DevicesByFabricGroup(group string) []schedulingv1alpha1.AcceleratorDevice {
	return s.devicesAt(s.fabricGroupIndex[group])
}

func (s *Snapshot) LinkBetween(source, target string) (schedulingv1alpha1.AcceleratorLink, bool) {
	neighbors, found := s.adjacency[source]
	if !found {
		return schedulingv1alpha1.AcceleratorLink{}, false
	}
	link, found := neighbors[target]
	return link, found
}

func (s *Snapshot) devicesAt(indexes []int) []schedulingv1alpha1.AcceleratorDevice {
	devices := make([]schedulingv1alpha1.AcceleratorDevice, 0, len(indexes))
	for _, index := range indexes {
		devices = append(devices, cloneDevice(s.devices[index]))
	}
	return devices
}

func validateDevice(device schedulingv1alpha1.AcceleratorDevice) error {
	if device.ID == "" {
		return fmt.Errorf("id is required")
	}
	if errs := validation.IsQualifiedName(device.ResourceName); len(errs) > 0 || !strings.Contains(device.ResourceName, "/") {
		return fmt.Errorf("resourceName %q is not a qualified extended resource name", device.ResourceName)
	}
	if !validVendor(device.Vendor) {
		return fmt.Errorf("unsupported vendor %q", device.Vendor)
	}
	if device.Product == "" {
		return fmt.Errorf("product is required")
	}
	if device.NUMANode != nil && *device.NUMANode < 0 {
		return fmt.Errorf("numaNode cannot be negative")
	}
	if device.MemoryMiB < 0 {
		return fmt.Errorf("memoryMiB cannot be negative")
	}
	if !validHealth(device.Health) {
		return fmt.Errorf("unsupported health %q", device.Health)
	}
	return nil
}

func validateLink(link schedulingv1alpha1.AcceleratorLink, devices map[string]int) error {
	if link.Source == link.Target {
		return fmt.Errorf("self links are not allowed")
	}
	if _, found := devices[link.Source]; !found {
		return fmt.Errorf("source device does not exist")
	}
	if _, found := devices[link.Target]; !found {
		return fmt.Errorf("target device does not exist")
	}
	if !validLinkType(link.Type) {
		return fmt.Errorf("unsupported type %q", link.Type)
	}
	if link.BandwidthGBps < 0 {
		return fmt.Errorf("bandwidthGBps cannot be negative")
	}
	if link.Hops < 0 {
		return fmt.Errorf("hops cannot be negative")
	}
	if !validHealth(link.Health) {
		return fmt.Errorf("unsupported health %q", link.Health)
	}
	return nil
}

func validVendor(vendor schedulingv1alpha1.AcceleratorVendor) bool {
	switch vendor {
	case schedulingv1alpha1.VendorNVIDIA, schedulingv1alpha1.VendorHuawei, schedulingv1alpha1.VendorAMD:
		return true
	default:
		return false
	}
}

func validHealth(health schedulingv1alpha1.Health) bool {
	switch health {
	case schedulingv1alpha1.HealthHealthy, schedulingv1alpha1.HealthUnhealthy, schedulingv1alpha1.HealthUnknown:
		return true
	default:
		return false
	}
}

func validLinkType(linkType schedulingv1alpha1.LinkType) bool {
	switch linkType {
	case schedulingv1alpha1.LinkNVLink, schedulingv1alpha1.LinkHCCS, schedulingv1alpha1.LinkXGMI, schedulingv1alpha1.LinkPCIe:
		return true
	default:
		return false
	}
}

func linkKey(link schedulingv1alpha1.AcceleratorLink) string {
	left, right := link.Source, link.Target
	if right < left {
		left, right = right, left
	}
	return left + "\x00" + right + "\x00" + string(link.Type)
}

func cloneDevices(devices []schedulingv1alpha1.AcceleratorDevice) []schedulingv1alpha1.AcceleratorDevice {
	cloned := make([]schedulingv1alpha1.AcceleratorDevice, len(devices))
	for i, device := range devices {
		cloned[i] = cloneDevice(device)
	}
	return cloned
}

func cloneDevice(device schedulingv1alpha1.AcceleratorDevice) schedulingv1alpha1.AcceleratorDevice {
	if device.NUMANode != nil {
		numa := *device.NUMANode
		device.NUMANode = &numa
	}
	return device
}
