package dra

import (
	"testing"

	"github.com/bupt20220407/accelerator-fabric-scheduler/pkg/fixtures"
)

func TestResourcesForNode(t *testing.T) {
	resources, err := ResourcesForTopology(fixtures.ThreeNodeTopologies()[0])
	if err != nil {
		t.Fatal(err)
	}
	pool, found := resources.Pools["accelerator-fabric-worker"]
	if !found || len(pool.Slices) != 1 || len(pool.Slices[0].Devices) != 8 {
		t.Fatalf("published resources = %+v", resources)
	}
	first := pool.Slices[0].Devices[0]
	if first.Name != "gpu0" || first.Attributes["vendor"].StringValue == nil || *first.Attributes["vendor"].StringValue != "nvidia" {
		t.Fatalf("first DRA device = %+v", first)
	}
	if first.Attributes["fabricGroup"].StringValue == nil || *first.Attributes["fabricGroup"].StringValue != "nvlink-clique-0" {
		t.Fatalf("fabric group attribute = %+v", first.Attributes["fabricGroup"])
	}
	if first.Attributes["fabricBandwidthGBps"].IntValue == nil || *first.Attributes["fabricBandwidthGBps"].IntValue != 450 {
		t.Fatalf("fabric bandwidth attribute = %+v", first.Attributes["fabricBandwidthGBps"])
	}
	if _, err := ResourcesForTopology(nil); err == nil {
		t.Fatal("ResourcesForTopology() accepted nil")
	}
}
