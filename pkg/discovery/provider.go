package discovery

import (
	"context"
	"fmt"

	schedulingv1alpha1 "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
)

// Provider discovers the accelerator topology visible from one node.
type Provider interface {
	Name() string
	Discover(context.Context, string) (schedulingv1alpha1.AcceleratorTopologySpec, error)
}

type FixtureProvider struct{}

func (FixtureProvider) Name() string { return "fixture" }

func (FixtureProvider) Discover(_ context.Context, nodeName string) (schedulingv1alpha1.AcceleratorTopologySpec, error) {
	for _, object := range fixtureTopologies() {
		if object.Spec.NodeName == nodeName {
			return *object.Spec.DeepCopy(), nil
		}
	}
	return schedulingv1alpha1.AcceleratorTopologySpec{}, fmt.Errorf("no fixture topology for node %q", nodeName)
}
