package discovery

import (
	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/fixtures"
)

func fixtureTopologies() []*schedulingv1alpha1.AcceleratorTopology {
	return fixtures.ThreeNodeTopologies()
}
