package topologyfit

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	resourcehelper "k8s.io/component-helpers/resource"
	framework "k8s.io/kube-scheduler/framework"
	corev1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/topology"
	topologycache "github.com/bupt/accelerator-fabric-scheduler/pkg/topology/cache"
)

type evaluation struct {
	feasible bool
	reason   string
	score    int64
}

func extendedResourceRequests(pod *v1.Pod) map[v1.ResourceName]int64 {
	requests := make(map[v1.ResourceName]int64)
	if pod == nil {
		return requests
	}
	for resourceName, quantity := range resourcehelper.PodRequests(pod, resourcehelper.PodResourcesOptions{}) {
		if corev1helper.IsExtendedResourceName(resourceName) && quantity.Sign() > 0 {
			requests[resourceName] = quantity.Value()
		}
	}
	return requests
}

func (p *Plugin) evaluate(data *cycleData, nodeInfo framework.NodeInfo) evaluation {
	if nodeInfo == nil || nodeInfo.Node() == nil {
		return evaluation{reason: "node info has no node"}
	}
	if p.store == nil {
		return evaluation{reason: "topology store is unavailable"}
	}
	nodeName := nodeInfo.Node().Name
	cached := p.store.Get(nodeName, p.now(), p.cacheTTL)
	if cached.Availability != topologycache.AvailabilityReady {
		if data.policy.FailurePolicy == schedulingv1alpha1.FailurePolicyBestEffort {
			return evaluation{feasible: true, reason: fmt.Sprintf("topology %s; allowed by BestEffort", cached.Availability)}
		}
		return evaluation{reason: fmt.Sprintf("topology for node %q is %s", nodeName, cached.Availability)}
	}

	devices := eligibleDevices(cached.Snapshot, data)
	if int64(len(devices)) < data.requested {
		return evaluation{reason: fmt.Sprintf("node %q has %d eligible topology devices, need %d", nodeName, len(devices), data.requested)}
	}
	remaining, allocatable := scalarHeadroom(nodeInfo, data.resourceName)
	if remaining < data.requested {
		return evaluation{reason: fmt.Sprintf("node %q has %d remaining %s, need %d", nodeName, remaining, data.resourceName, data.requested)}
	}

	if !topologyModeFits(cached.Snapshot, devices, data.policy, int(data.requested)) {
		return evaluation{reason: fmt.Sprintf("node %q cannot satisfy topology mode %s for %d devices", nodeName, data.policy.TopologyMode, data.requested)}
	}
	features := scoreFeatures(cached.Snapshot, devices, data.policy, data.requested, remaining, allocatable)
	return evaluation{feasible: true, score: weightedScore(features, data.policy.Weights)}
}

func eligibleDevices(snapshot *topology.Snapshot, data *cycleData) []schedulingv1alpha1.AcceleratorDevice {
	products := make(map[string]struct{}, len(data.policy.Products))
	for _, product := range data.policy.Products {
		products[product] = struct{}{}
	}
	eligible := make([]schedulingv1alpha1.AcceleratorDevice, 0)
	for _, device := range snapshot.Devices() {
		if device.ResourceName != string(data.resourceName) {
			continue
		}
		if data.policy.Vendor != "" && data.policy.Vendor != schedulingv1alpha1.VendorAny && device.Vendor != data.policy.Vendor {
			continue
		}
		if len(products) > 0 {
			if _, found := products[device.Product]; !found {
				continue
			}
		}
		if device.MemoryMiB < data.policy.MinimumMemoryMiB {
			continue
		}
		if device.Health != schedulingv1alpha1.HealthHealthy &&
			!(data.policy.AllowUnknownTopology && device.Health == schedulingv1alpha1.HealthUnknown) {
			continue
		}
		eligible = append(eligible, device)
	}
	return eligible
}

func scalarHeadroom(nodeInfo framework.NodeInfo, resourceName v1.ResourceName) (remaining, allocatable int64) {
	allocatable = nodeInfo.GetAllocatable().GetScalarResources()[resourceName]
	requested := nodeInfo.GetRequested().GetScalarResources()[resourceName]
	remaining = allocatable - requested
	if remaining < 0 {
		remaining = 0
	}
	return remaining, allocatable
}

func topologyModeFits(
	snapshot *topology.Snapshot,
	devices []schedulingv1alpha1.AcceleratorDevice,
	policy schedulingv1alpha1.AcceleratorPlacementPolicySpec,
	requested int,
) bool {
	if requested < 1 || len(devices) < requested {
		return false
	}
	switch policy.TopologyMode {
	case schedulingv1alpha1.TopologyModeBestEffort:
		return true
	case schedulingv1alpha1.TopologyModeSingleNUMA:
		if maxNUMAGroup(devices, policy.AllowUnknownTopology) >= requested {
			return true
		}
	case schedulingv1alpha1.TopologyModeSamePCIeRoot:
		if maxStringGroup(devices, func(device schedulingv1alpha1.AcceleratorDevice) string { return device.PCIeRoot }, policy.AllowUnknownTopology) >= requested {
			return true
		}
	case schedulingv1alpha1.TopologyModeFabricClique:
		if hasFabricClique(snapshot, devices, policy, requested) {
			return true
		}
	case schedulingv1alpha1.TopologyModeFabricConnected:
		if maxFabricComponent(snapshot, devices, policy) >= requested {
			return true
		}
	}
	return false
}

func maxNUMAGroup(devices []schedulingv1alpha1.AcceleratorDevice, allowUnknown bool) int {
	groups := make(map[int32]int)
	unknown := 0
	maximum := 0
	for _, device := range devices {
		if device.NUMANode == nil {
			if allowUnknown {
				unknown++
			}
			continue
		}
		groups[*device.NUMANode]++
		if groups[*device.NUMANode] > maximum {
			maximum = groups[*device.NUMANode]
		}
	}
	if unknown > maximum {
		maximum = unknown
	}
	return maximum
}

func maxStringGroup(
	devices []schedulingv1alpha1.AcceleratorDevice,
	key func(schedulingv1alpha1.AcceleratorDevice) string,
	allowUnknown bool,
) int {
	groups := make(map[string]int)
	maximum := 0
	for _, device := range devices {
		value := key(device)
		if value == "" && !allowUnknown {
			continue
		}
		groups[value]++
		if groups[value] > maximum {
			maximum = groups[value]
		}
	}
	return maximum
}

func qualifiesFabricLink(link schedulingv1alpha1.AcceleratorLink, policy schedulingv1alpha1.AcceleratorPlacementPolicySpec) bool {
	if link.Type == schedulingv1alpha1.LinkPCIe {
		return false
	}
	if link.Health != schedulingv1alpha1.HealthHealthy &&
		!(policy.AllowUnknownTopology && link.Health == schedulingv1alpha1.HealthUnknown) {
		return false
	}
	return link.BandwidthGBps >= policy.MinimumLinkBandwidthGBps && link.Hops <= policy.MaxFabricHops
}

func hasFabricClique(
	snapshot *topology.Snapshot,
	devices []schedulingv1alpha1.AcceleratorDevice,
	policy schedulingv1alpha1.AcceleratorPlacementPolicySpec,
	requested int,
) bool {
	ids := make([]string, 0, len(devices))
	for _, device := range devices {
		ids = append(ids, device.ID)
	}
	var search func(chosen, candidates []string) bool
	search = func(chosen, candidates []string) bool {
		if len(chosen) >= requested {
			return true
		}
		if len(chosen)+len(candidates) < requested {
			return false
		}
		for index, candidate := range candidates {
			connected := true
			for _, member := range chosen {
				link, found := snapshot.LinkBetween(candidate, member)
				if !found || !qualifiesFabricLink(link, policy) {
					connected = false
					break
				}
			}
			if connected && search(append(chosen, candidate), candidates[index+1:]) {
				return true
			}
		}
		return false
	}
	return search(nil, ids)
}

func maxFabricComponent(
	snapshot *topology.Snapshot,
	devices []schedulingv1alpha1.AcceleratorDevice,
	policy schedulingv1alpha1.AcceleratorPlacementPolicySpec,
) int {
	eligible := make(map[string]struct{}, len(devices))
	for _, device := range devices {
		eligible[device.ID] = struct{}{}
	}
	visited := make(map[string]struct{}, len(devices))
	maximum := 0
	for _, start := range devices {
		if _, found := visited[start.ID]; found {
			continue
		}
		queue := []string{start.ID}
		visited[start.ID] = struct{}{}
		size := 0
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			size++
			for candidate := range eligible {
				if _, found := visited[candidate]; found {
					continue
				}
				link, found := snapshot.LinkBetween(current, candidate)
				if found && qualifiesFabricLink(link, policy) {
					visited[candidate] = struct{}{}
					queue = append(queue, candidate)
				}
			}
		}
		if size > maximum {
			maximum = size
		}
	}
	return maximum
}

type features struct {
	locality      int64
	fabric        int64
	fragmentation int64
	headroom      int64
}

func scoreFeatures(
	snapshot *topology.Snapshot,
	devices []schedulingv1alpha1.AcceleratorDevice,
	policy schedulingv1alpha1.AcceleratorPlacementPolicySpec,
	requested, remaining, allocatable int64,
) features {
	localityGroup := maxNUMAGroup(devices, policy.AllowUnknownTopology)
	pcieGroup := maxStringGroup(devices, func(device schedulingv1alpha1.AcceleratorDevice) string { return device.PCIeRoot }, policy.AllowUnknownTopology)
	if pcieGroup > localityGroup {
		localityGroup = pcieGroup
	}

	possibleEdges := int64(len(devices) * (len(devices) - 1) / 2)
	qualifiedEdges := int64(0)
	for left := 0; left < len(devices); left++ {
		for right := left + 1; right < len(devices); right++ {
			link, found := snapshot.LinkBetween(devices[left].ID, devices[right].ID)
			if found && qualifiesFabricLink(link, policy) {
				qualifiedEdges++
			}
		}
	}
	return features{
		locality:      percentage(int64(localityGroup), int64(len(devices))),
		fabric:        percentage(qualifiedEdges, possibleEdges),
		fragmentation: percentage(requested, int64(len(devices))),
		headroom:      percentage(remaining-requested, allocatable),
	}
}

func percentage(numerator, denominator int64) int64 {
	if denominator <= 0 || numerator <= 0 {
		return 0
	}
	value := numerator * 100 / denominator
	if value > 100 {
		return 100
	}
	return value
}

func weightedScore(value features, weights schedulingv1alpha1.ScoreWeights) int64 {
	score := (value.locality*int64(weights.Locality) +
		value.fabric*int64(weights.Fabric) +
		value.fragmentation*int64(weights.Fragmentation) +
		value.headroom*int64(weights.Headroom)) / 100
	if score < framework.MinNodeScore {
		return framework.MinNodeScore
	}
	if score > framework.MaxNodeScore {
		return framework.MaxNodeScore
	}
	return score
}
