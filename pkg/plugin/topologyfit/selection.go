package topologyfit

import (
	"fmt"
	"sort"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/topology"
)

func selectDeviceIDs(
	snapshot *topology.Snapshot,
	devices []schedulingv1alpha1.AcceleratorDevice,
	policy schedulingv1alpha1.AcceleratorPlacementPolicySpec,
	requested int,
) []string {
	if requested < 1 || len(devices) < requested {
		return nil
	}
	switch policy.TopologyMode {
	case schedulingv1alpha1.TopologyModeBestEffort:
		return firstDeviceIDs(devices, requested)
	case schedulingv1alpha1.TopologyModeSingleNUMA:
		return selectDeviceGroup(devices, requested, func(device schedulingv1alpha1.AcceleratorDevice) (string, bool) {
			if device.NUMANode == nil {
				return "~unknown", policy.AllowUnknownTopology
			}
			return fmt.Sprintf("numa:%010d", *device.NUMANode), true
		})
	case schedulingv1alpha1.TopologyModeSamePCIeRoot:
		return selectDeviceGroup(devices, requested, func(device schedulingv1alpha1.AcceleratorDevice) (string, bool) {
			if device.PCIeRoot == "" {
				return "~unknown", policy.AllowUnknownTopology
			}
			return device.PCIeRoot, true
		})
	case schedulingv1alpha1.TopologyModeFabricClique:
		return selectFabricClique(snapshot, devices, policy, requested)
	case schedulingv1alpha1.TopologyModeFabricConnected:
		return selectFabricConnected(snapshot, devices, policy, requested)
	default:
		return nil
	}
}

func firstDeviceIDs(devices []schedulingv1alpha1.AcceleratorDevice, requested int) []string {
	selected := make([]string, 0, requested)
	for _, device := range devices[:requested] {
		selected = append(selected, device.ID)
	}
	return selected
}

func selectDeviceGroup(
	devices []schedulingv1alpha1.AcceleratorDevice,
	requested int,
	groupKey func(schedulingv1alpha1.AcceleratorDevice) (string, bool),
) []string {
	groups := make(map[string][]schedulingv1alpha1.AcceleratorDevice)
	for _, device := range devices {
		key, usable := groupKey(device)
		if usable {
			groups[key] = append(groups[key], device)
		}
	}
	keys := make([]string, 0, len(groups))
	for key, group := range groups {
		if len(group) >= requested {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(left, right int) bool {
		leftSize, rightSize := len(groups[keys[left]]), len(groups[keys[right]])
		if leftSize != rightSize {
			return leftSize < rightSize
		}
		return keys[left] < keys[right]
	})
	if len(keys) == 0 {
		return nil
	}
	return firstDeviceIDs(groups[keys[0]], requested)
}

func selectFabricClique(
	snapshot *topology.Snapshot,
	devices []schedulingv1alpha1.AcceleratorDevice,
	policy schedulingv1alpha1.AcceleratorPlacementPolicySpec,
	requested int,
) []string {
	ids := firstDeviceIDs(devices, len(devices))
	var search func(chosen, candidates []string) []string
	search = func(chosen, candidates []string) []string {
		if len(chosen) >= requested {
			return append([]string(nil), chosen...)
		}
		if len(chosen)+len(candidates) < requested {
			return nil
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
			if connected {
				next := append(append([]string(nil), chosen...), candidate)
				if selected := search(next, candidates[index+1:]); len(selected) > 0 {
					return selected
				}
			}
		}
		return nil
	}
	return search(nil, ids)
}

func selectFabricConnected(
	snapshot *topology.Snapshot,
	devices []schedulingv1alpha1.AcceleratorDevice,
	policy schedulingv1alpha1.AcceleratorPlacementPolicySpec,
	requested int,
) []string {
	ids := firstDeviceIDs(devices, len(devices))
	for _, start := range ids {
		visited := map[string]struct{}{start: {}}
		queue := []string{start}
		selected := make([]string, 0, requested)
		for len(queue) > 0 && len(selected) < requested {
			current := queue[0]
			queue = queue[1:]
			selected = append(selected, current)
			neighbors := make([]string, 0)
			for _, candidate := range ids {
				if _, found := visited[candidate]; found {
					continue
				}
				link, found := snapshot.LinkBetween(current, candidate)
				if found && qualifiesFabricLink(link, policy) {
					neighbors = append(neighbors, candidate)
				}
			}
			sort.Strings(neighbors)
			for _, neighbor := range neighbors {
				visited[neighbor] = struct{}{}
				queue = append(queue, neighbor)
			}
		}
		if len(selected) == requested {
			return selected
		}
	}
	return nil
}
