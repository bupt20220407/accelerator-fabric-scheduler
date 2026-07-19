package fixtures

import (
	"testing"

	"github.com/bupt/accelerator-fabric-scheduler/pkg/topology"
)

func TestThreeNodeTopologies(t *testing.T) {
	topologies := ThreeNodeTopologies()
	if len(topologies) != 3 {
		t.Fatalf("fixture count = %d, want 3", len(topologies))
	}
	resources := map[string]bool{}
	for _, object := range topologies {
		snapshot, err := topology.Build(object.Spec)
		if err != nil {
			t.Fatalf("Build(%s) error = %v", object.Name, err)
		}
		devices := snapshot.Devices()
		if len(devices) != 8 || len(snapshot.Links()) != 13 {
			t.Fatalf("%s has %d devices and %d links", object.Name, len(devices), len(snapshot.Links()))
		}
		if len(snapshot.DevicesByNUMA(0)) != 4 || len(snapshot.DevicesByNUMA(1)) != 4 {
			t.Fatalf("%s NUMA partition is not 4+4", object.Name)
		}
		resources[devices[0].ResourceName] = true
	}
	for _, resource := range []string{"nvidia.com/gpu", "huawei.com/Ascend910", "amd.com/gpu"} {
		if !resources[resource] {
			t.Errorf("missing resource fixture %s", resource)
		}
	}
}
