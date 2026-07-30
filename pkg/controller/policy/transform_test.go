package policy

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	schedulingv1alpha1 "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	"github.com/bupt20220407/accelerator-fabric-scheduler/pkg/dra"
)

func TestResourceClaimTemplateForPolicy(t *testing.T) {
	policy := testPolicy()
	template, err := ResourceClaimTemplateForPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	request := template.Spec.Spec.Devices.Requests[0].Exactly
	if request.DeviceClassName != dra.DeviceClassName || request.Count != 4 || len(request.Selectors) != 6 {
		t.Fatalf("DRA request = %+v", request)
	}
	expressions := make([]string, 0, len(request.Selectors))
	for _, selector := range request.Selectors {
		expressions = append(expressions, selector.CEL.Expression)
	}
	joined := strings.Join(expressions, "\n")
	for _, expected := range []string{
		`.health == "Healthy"`,
		`.vendor == "nvidia"`,
		`.product in ["H100-SXM"]`,
		`.memoryMiB >= 80000`,
		`.fabricBandwidthGBps >= 300`,
		`.fabricHops <= 1`,
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("selectors %q do not contain %q", joined, expected)
		}
	}
	constraint := template.Spec.Spec.Devices.Constraints[0]
	if constraint.MatchAttribute == nil || string(*constraint.MatchAttribute) != dra.DriverName+"/fabricGroup" {
		t.Fatalf("DRA constraint = %+v", constraint)
	}
	if !metav1.IsControlledBy(template, policy) {
		t.Fatal("template is not controlled by policy")
	}
}

func TestResourceClaimTemplateRejectsUnsupportedPolicy(t *testing.T) {
	policy := testPolicy()
	policy.Spec.TopologyMode = schedulingv1alpha1.TopologyModeFabricConnected
	if _, err := ResourceClaimTemplateForPolicy(policy); err == nil {
		t.Fatal("fabric-connected policy was accepted")
	}
	policy.Spec.TopologyMode = schedulingv1alpha1.TopologyModeBestEffort
	policy.Spec.AllowUnknownTopology = true
	if _, err := ResourceClaimTemplateForPolicy(policy); err == nil {
		t.Fatal("unknown topology policy was accepted")
	}
}

func testPolicy() *schedulingv1alpha1.AcceleratorPlacementPolicy {
	return &schedulingv1alpha1.AcceleratorPlacementPolicy{
		TypeMeta: metav1.TypeMeta{APIVersion: schedulingv1alpha1.SchemeGroupVersion.String(), Kind: "AcceleratorPlacementPolicy"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "training", Namespace: "default", UID: types.UID("policy-uid"),
		},
		Spec: schedulingv1alpha1.AcceleratorPlacementPolicySpec{
			Vendor:                   schedulingv1alpha1.VendorNVIDIA,
			Products:                 []string{"H100-SXM"},
			MinimumMemoryMiB:         80000,
			TopologyMode:             schedulingv1alpha1.TopologyModeFabricClique,
			MinimumLinkBandwidthGBps: 300,
			MaxFabricHops:            1,
			DRA:                      &schedulingv1alpha1.DRAParameters{DeviceCount: 4},
		},
	}
}
