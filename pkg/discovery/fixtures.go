package discovery

import (
	schedulingv1alpha1 "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	"github.com/bupt20220407/accelerator-fabric-scheduler/pkg/fixtures"
)

func fixtureTopologies() []*schedulingv1alpha1.AcceleratorTopology {
	return fixtures.ThreeNodeTopologies()
}
