package topologyfit

import (
	"testing"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/fixtures"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/topology"
)

func BenchmarkTopologyFitFabricCliqueSelection(b *testing.B) {
	object := fixtures.ThreeNodeTopologies()[0]
	snapshot, err := topology.Build(object.Spec)
	if err != nil {
		b.Fatal(err)
	}
	devices := snapshot.Devices()
	policy := testPolicy(schedulingv1alpha1.TopologyModeFabricClique, schedulingv1alpha1.FailurePolicyFailClosed).Spec
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		selected := selectDeviceIDs(snapshot, devices, policy, 4)
		if len(selected) != 4 {
			b.Fatalf("selected %d devices", len(selected))
		}
	}
}
