package discovery

import (
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

type NVIDIAProvider struct{ runner CommandRunner }

func NewNVIDIAProvider(runner CommandRunner) *NVIDIAProvider {
	if runner == nil {
		runner = execRunner{}
	}
	return &NVIDIAProvider{runner: runner}
}

func (*NVIDIAProvider) Name() string { return "nvidia-smi" }

func (p *NVIDIAProvider) Discover(ctx context.Context, nodeName string) (schedulingv1alpha1.AcceleratorTopologySpec, error) {
	query, err := p.runner.Run(ctx, "nvidia-smi", "--query-gpu=index,pci.bus_id,name,memory.total", "--format=csv,noheader,nounits")
	if err != nil {
		return schedulingv1alpha1.AcceleratorTopologySpec{}, fmt.Errorf("query nvidia devices: %w", err)
	}
	topo, err := p.runner.Run(ctx, "nvidia-smi", "topo", "-m")
	if err != nil {
		return schedulingv1alpha1.AcceleratorTopologySpec{}, fmt.Errorf("query nvidia topology: %w", err)
	}
	devices, err := parseNVIDIAQuery(string(query))
	if err != nil {
		return schedulingv1alpha1.AcceleratorTopologySpec{}, err
	}
	links, cliques, err := parseNVIDIATopology(string(topo), len(devices))
	if err != nil {
		return schedulingv1alpha1.AcceleratorTopologySpec{}, err
	}
	for index, group := range cliques {
		devices[index].FabricGroup = group
	}
	return schedulingv1alpha1.AcceleratorTopologySpec{
		NodeName: nodeName, Source: schedulingv1alpha1.TopologySourceVendor,
		Devices: devices, Links: links,
	}, nil
}

func parseNVIDIAQuery(raw string) ([]schedulingv1alpha1.AcceleratorDevice, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimSpace(raw)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse nvidia-smi CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("nvidia-smi returned no devices")
	}
	devices := make([]schedulingv1alpha1.AcceleratorDevice, 0, len(records))
	for _, record := range records {
		if len(record) != 4 {
			return nil, fmt.Errorf("nvidia-smi CSV row has %d fields, want 4", len(record))
		}
		index, err := strconv.Atoi(strings.TrimSpace(record[0]))
		if err != nil || index != len(devices) {
			return nil, fmt.Errorf("nvidia device indexes must be contiguous from zero")
		}
		memory, err := strconv.ParseInt(strings.TrimSpace(record[3]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse memory for GPU%d: %w", index, err)
		}
		devices = append(devices, schedulingv1alpha1.AcceleratorDevice{
			ID: fmt.Sprintf("gpu%d", index), Vendor: schedulingv1alpha1.VendorNVIDIA,
			Product: strings.TrimSpace(record[2]), ResourceName: "nvidia.com/gpu",
			MemoryMiB: memory, Health: schedulingv1alpha1.HealthUnknown,
		})
	}
	return devices, nil
}

func parseNVIDIATopology(raw string, count int) ([]schedulingv1alpha1.AcceleratorLink, []string, error) {
	rows := make(map[int][]string, count)
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < count+1 || !strings.HasPrefix(fields[0], "GPU") {
			continue
		}
		index, err := strconv.Atoi(strings.TrimPrefix(fields[0], "GPU"))
		if err == nil && index >= 0 && index < count {
			rows[index] = fields[1 : count+1]
		}
	}
	if len(rows) != count {
		return nil, nil, fmt.Errorf("nvidia topology matrix has %d GPU rows, want %d", len(rows), count)
	}
	links := make([]schedulingv1alpha1.AcceleratorLink, 0, count*(count-1)/2)
	nvlink := make([][]bool, count)
	for i := range nvlink {
		nvlink[i] = make([]bool, count)
	}
	for left := 0; left < count; left++ {
		for right := left + 1; right < count; right++ {
			code := rows[left][right]
			if reverse := rows[right][left]; reverse != code {
				return nil, nil, fmt.Errorf("asymmetric NVIDIA topology relationship GPU%d-GPU%d: %q != %q", left, right, code, reverse)
			}
			link := schedulingv1alpha1.AcceleratorLink{
				Source: fmt.Sprintf("gpu%d", left), Target: fmt.Sprintf("gpu%d", right),
				BandwidthGBps: 0, Health: schedulingv1alpha1.HealthUnknown,
			}
			switch {
			case strings.HasPrefix(code, "NV"):
				link.Type, link.Hops = schedulingv1alpha1.LinkNVLink, 1
				nvlink[left][right], nvlink[right][left] = true, true
			case code == "PIX":
				link.Type, link.Hops = schedulingv1alpha1.LinkPCIe, 1
			case code == "PXB":
				link.Type, link.Hops = schedulingv1alpha1.LinkPCIe, 2
			case code == "PHB":
				link.Type, link.Hops = schedulingv1alpha1.LinkPCIe, 3
			case code == "NODE":
				link.Type, link.Hops = schedulingv1alpha1.LinkPCIe, 4
			case code == "SYS":
				link.Type, link.Hops = schedulingv1alpha1.LinkPCIe, 5
			default:
				return nil, nil, fmt.Errorf("unsupported NVIDIA topology code %q", code)
			}
			links = append(links, link)
		}
	}
	groups := completeNVLinkGroups(nvlink)
	return links, groups, nil
}

func completeNVLinkGroups(matrix [][]bool) []string {
	groups := make([]string, len(matrix))
	visited := make([]bool, len(matrix))
	groupIndex := 0
	for start := range matrix {
		if visited[start] {
			continue
		}
		component := []int{start}
		visited[start] = true
		for cursor := 0; cursor < len(component); cursor++ {
			for neighbor, linked := range matrix[component[cursor]] {
				if linked && !visited[neighbor] {
					visited[neighbor] = true
					component = append(component, neighbor)
				}
			}
		}
		complete := len(component) > 1
		for i := 0; complete && i < len(component); i++ {
			for j := i + 1; j < len(component); j++ {
				complete = complete && matrix[component[i]][component[j]]
			}
		}
		if complete {
			name := fmt.Sprintf("nvlink-clique-%d", groupIndex)
			groupIndex++
			for _, device := range component {
				groups[device] = name
			}
		}
	}
	return groups
}
