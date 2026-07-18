package topologyfit

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	framework "k8s.io/kube-scheduler/framework"
)

// Name is used in the scheduler registry and KubeSchedulerConfiguration.
const Name = "TopologyFit"

// Plugin is the W1 scheduling lifecycle skeleton. It deliberately makes no
// topology decision until the typed topology cache is introduced in W2.
type Plugin struct{}

var (
	_ framework.PreFilterPlugin = (*Plugin)(nil)
	_ framework.FilterPlugin    = (*Plugin)(nil)
	_ framework.PreScorePlugin  = (*Plugin)(nil)
	_ framework.ScorePlugin     = (*Plugin)(nil)
	_ framework.ReservePlugin   = (*Plugin)(nil)
	_ framework.PreBindPlugin   = (*Plugin)(nil)
	_ framework.SignPlugin      = (*Plugin)(nil)
)

// New creates a stateless W1 plugin. Configuration is added with the typed API
// in W2; rejecting unknown config before then prevents silent misconfiguration.
func New(_ context.Context, config runtime.Object, _ framework.Handle) (framework.Plugin, error) {
	if config != nil {
		return nil, newUnexpectedConfigError(config)
	}
	return &Plugin{}, nil
}

func (p *Plugin) Name() string { return Name }

// SignPod returns a constant fragment because the W1 plugin is deliberately
// neutral for every Pod. W2 must include every policy input that can change a
// topology decision in this signature.
func (p *Plugin) SignPod(
	_ context.Context,
	_ *v1.Pod,
) ([]framework.SignFragment, *framework.Status) {
	return []framework.SignFragment{{
		Key:   "scheduling.bupt.dev/TopologyFit.W1()",
		Value: "neutral-v1",
	}}, nil
}

func (p *Plugin) PreFilter(
	_ context.Context,
	_ framework.CycleState,
	_ *v1.Pod,
	_ []framework.NodeInfo,
) (*framework.PreFilterResult, *framework.Status) {
	return nil, nil
}

func (p *Plugin) PreFilterExtensions() framework.PreFilterExtensions { return nil }

func (p *Plugin) Filter(
	_ context.Context,
	_ framework.CycleState,
	_ *v1.Pod,
	_ framework.NodeInfo,
) *framework.Status {
	return nil
}

func (p *Plugin) PreScore(
	_ context.Context,
	_ framework.CycleState,
	_ *v1.Pod,
	_ []framework.NodeInfo,
) *framework.Status {
	return nil
}

func (p *Plugin) Score(
	_ context.Context,
	_ framework.CycleState,
	_ *v1.Pod,
	_ framework.NodeInfo,
) (int64, *framework.Status) {
	return framework.MinNodeScore, nil
}

func (p *Plugin) ScoreExtensions() framework.ScoreExtensions { return nil }

func (p *Plugin) Reserve(
	_ context.Context,
	_ framework.CycleState,
	_ *v1.Pod,
	_ string,
) *framework.Status {
	return nil
}

func (p *Plugin) Unreserve(
	_ context.Context,
	_ framework.CycleState,
	_ *v1.Pod,
	_ string,
) {
}

func (p *Plugin) PreBindPreFlight(
	_ context.Context,
	_ framework.CycleState,
	_ *v1.Pod,
	_ string,
) *framework.Status {
	return nil
}

func (p *Plugin) PreBind(
	_ context.Context,
	_ framework.CycleState,
	_ *v1.Pod,
	_ string,
) *framework.Status {
	return nil
}
