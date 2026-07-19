package topologyfit

import (
	v1 "k8s.io/api/core/v1"
	framework "k8s.io/kube-scheduler/framework"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
)

type cycleData struct {
	active       bool
	policy       schedulingv1alpha1.AcceleratorPlacementPolicySpec
	resourceName v1.ResourceName
	requested    int64
	scores       map[string]int64
}

func (d *cycleData) Clone() framework.StateData {
	if d == nil {
		return (*cycleData)(nil)
	}
	cloned := *d
	cloned.policy.Products = append([]string(nil), d.policy.Products...)
	if d.scores != nil {
		cloned.scores = make(map[string]int64, len(d.scores))
		for nodeName, score := range d.scores {
			cloned.scores[nodeName] = score
		}
	}
	return &cloned
}
