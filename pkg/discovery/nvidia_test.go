package discovery

import (
	"context"
	"fmt"
	"testing"

	schedulingv1alpha1 "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
)

type sampleRunner struct{ calls int }

func (r *sampleRunner) Run(context.Context, string, ...string) ([]byte, error) {
	r.calls++
	switch r.calls {
	case 1:
		return []byte("0, 00000000:17:00.0, NVIDIA H100, 81559\n1, 00000000:65:00.0, NVIDIA H100, 81559\n"), nil
	case 2:
		return []byte("        GPU0 GPU1 CPU Affinity NUMA Affinity GPU NUMA ID\nGPU0   X    NV4  0-31         0             N/A\nGPU1   NV4  X    0-31         0             N/A\n"), nil
	default:
		return nil, fmt.Errorf("unexpected call")
	}
}

func TestNVIDIAProviderParsesSanitizedSamplesConservatively(t *testing.T) {
	provider := NewNVIDIAProvider(&sampleRunner{})
	spec, err := provider.Discover(context.Background(), "worker")
	if err != nil {
		t.Fatal(err)
	}
	if spec.NodeName != "worker" || spec.Source != schedulingv1alpha1.TopologySourceVendor || len(spec.Devices) != 2 || len(spec.Links) != 1 {
		t.Fatalf("unexpected topology: %#v", spec)
	}
	for _, device := range spec.Devices {
		if device.Health != schedulingv1alpha1.HealthUnknown || device.PCIeRoot != "" || device.FabricGroup != "nvlink-clique-0" {
			t.Fatalf("provider inferred unsupported device facts: %#v", device)
		}
	}
	link := spec.Links[0]
	if link.Type != schedulingv1alpha1.LinkNVLink || link.Hops != 1 || link.BandwidthGBps != 0 || link.Health != schedulingv1alpha1.HealthUnknown {
		t.Fatalf("provider inferred unsupported link facts: %#v", link)
	}
}

func TestParseNVIDIATopologyRejectsUnknownRelationship(t *testing.T) {
	_, _, err := parseNVIDIATopology("GPU0 GPU1\nGPU0 X WTF\nGPU1 WTF X\n", 2)
	if err == nil {
		t.Fatal("expected unsupported relationship error")
	}
}

func TestParseNVIDIATopologyRejectsAsymmetricMatrix(t *testing.T) {
	_, _, err := parseNVIDIATopology("GPU0 GPU1\nGPU0 X NV4\nGPU1 PHB X\n", 2)
	if err == nil {
		t.Fatal("expected asymmetric matrix error")
	}
}

func TestIncompleteNVLinkComponentIsNotAdvertisedAsClique(t *testing.T) {
	matrix := [][]bool{{false, true, false}, {true, false, true}, {false, true, false}}
	groups := completeNVLinkGroups(matrix)
	for _, group := range groups {
		if group != "" {
			t.Fatalf("incomplete component received clique %q", group)
		}
	}
}
