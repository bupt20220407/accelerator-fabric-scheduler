package deviceplugin

import (
	"context"
	"testing"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func TestNewBuildsStableDevicesAndNUMA(t *testing.T) {
	plugin, err := New(Config{ResourceName: "nvidia.com/gpu", Count: 8, DevicesPerNUMA: 4, SocketDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	devices := plugin.deviceList()
	if len(devices) != 8 {
		t.Fatalf("device count = %d, want 8", len(devices))
	}
	if devices[0].ID != "nvidia-com-gpu-0" || devices[0].Topology.Nodes[0].ID != 0 || devices[4].Topology.Nodes[0].ID != 1 {
		t.Fatalf("unexpected device topology: first=%+v fifth=%+v", devices[0], devices[4])
	}
}

func TestAllocate(t *testing.T) {
	plugin, err := New(Config{ResourceName: "nvidia.com/gpu", Count: 2, DevicesPerNUMA: 1, SocketDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	response, err := plugin.Allocate(context.Background(), &pluginapi.AllocateRequest{ContainerRequests: []*pluginapi.ContainerAllocateRequest{{
		DevicesIds: []string{"nvidia-com-gpu-1", "nvidia-com-gpu-0"},
	}}})
	if err != nil {
		t.Fatalf("Allocate() error = %v", err)
	}
	if got := response.ContainerResponses[0].Envs["SYNTHETIC_ACCELERATOR_IDS"]; got != "nvidia-com-gpu-0,nvidia-com-gpu-1" {
		t.Fatalf("allocated IDs = %q", got)
	}

	if err := plugin.SetHealth("nvidia-com-gpu-0", pluginapi.Unhealthy); err != nil {
		t.Fatalf("SetHealth() error = %v", err)
	}
	if _, err := plugin.Allocate(context.Background(), &pluginapi.AllocateRequest{ContainerRequests: []*pluginapi.ContainerAllocateRequest{{DevicesIds: []string{"nvidia-com-gpu-0"}}}}); err == nil {
		t.Fatal("Allocate() accepted an unhealthy device")
	}
}

func TestPreferredDevices(t *testing.T) {
	selected, err := preferredDevices(&pluginapi.ContainerPreferredAllocationRequest{
		AvailableDeviceIDs:   []string{"gpu3", "gpu1", "gpu2", "gpu0"},
		MustIncludeDeviceIDs: []string{"gpu2"},
		AllocationSize:       3,
	})
	if err != nil {
		t.Fatalf("preferredDevices() error = %v", err)
	}
	want := []string{"gpu2", "gpu0", "gpu1"}
	for index := range want {
		if selected[index] != want[index] {
			t.Fatalf("selected = %v, want %v", selected, want)
		}
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	for name, config := range map[string]Config{
		"resource": {ResourceName: "gpu", Count: 1, DevicesPerNUMA: 1},
		"count":    {ResourceName: "nvidia.com/gpu", Count: 0, DevicesPerNUMA: 1},
		"numa":     {ResourceName: "nvidia.com/gpu", Count: 1, DevicesPerNUMA: 0},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(config); err == nil {
				t.Fatal("New() accepted invalid config")
			}
		})
	}
}
