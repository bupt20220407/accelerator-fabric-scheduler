package topologyfit

import (
	"context"
	"fmt"
	"sort"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	framework "k8s.io/kube-scheduler/framework"

	schedulingv1alpha1 "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	clientset "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/clientset/versioned"
	informers "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/informers/externalversions"
	listers "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/listers/scheduling/v1alpha1"
	"github.com/bupt20220407/accelerator-fabric-scheduler/pkg/observability"
	topologycache "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/topology/cache"
)

const (
	Name             = "TopologyFit"
	PolicyAnnotation = "scheduling.bupt.dev/placement-policy"
	defaultCacheTTL  = 2 * time.Minute
	stateKey         = framework.StateKey("scheduling.bupt.dev/TopologyFit")
)

type Plugin struct {
	store        *topologycache.Store
	policyLister listers.AcceleratorPlacementPolicyLister
	ledger       *reservationLedger
	now          func() time.Time
	cacheTTL     time.Duration
}

var (
	_ framework.PreFilterPlugin = (*Plugin)(nil)
	_ framework.FilterPlugin    = (*Plugin)(nil)
	_ framework.PreScorePlugin  = (*Plugin)(nil)
	_ framework.ScorePlugin     = (*Plugin)(nil)
	_ framework.ReservePlugin   = (*Plugin)(nil)
	_ framework.PreBindPlugin   = (*Plugin)(nil)
	_ framework.PostBindPlugin  = (*Plugin)(nil)
	_ framework.SignPlugin      = (*Plugin)(nil)
)

func New(ctx context.Context, config runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	if config != nil {
		return nil, newUnexpectedConfigError(config)
	}
	if handle == nil || handle.KubeConfig() == nil {
		return nil, fmt.Errorf("%s requires a scheduler handle with kubeconfig", Name)
	}

	client, err := clientset.NewForConfig(handle.KubeConfig())
	if err != nil {
		return nil, fmt.Errorf("create scheduling client: %w", err)
	}
	factory := informers.NewSharedInformerFactory(client, 30*time.Second)
	topologyInformer := factory.Scheduling().V1alpha1().AcceleratorTopologies()
	policyInformer := factory.Scheduling().V1alpha1().AcceleratorPlacementPolicies()
	topologySharedInformer := topologyInformer.Informer()
	policySharedInformer := policyInformer.Informer()
	store := topologycache.NewStore()
	if _, err := topologycache.ObserveAcceleratorTopologies(topologyInformer, store, func(err error) {
		klog.ErrorS(err, "TopologyFit topology cache update failed")
	}); err != nil {
		return nil, err
	}

	factory.Start(ctx.Done())
	if !toolscache.WaitForCacheSync(ctx.Done(), topologySharedInformer.HasSynced, policySharedInformer.HasSynced) {
		return nil, fmt.Errorf("wait for %s informer caches", Name)
	}
	return newPlugin(store, policyInformer.Lister()), nil
}

func newPlugin(store *topologycache.Store, policyLister listers.AcceleratorPlacementPolicyLister) *Plugin {
	return &Plugin{
		store:        store,
		policyLister: policyLister,
		ledger:       newReservationLedger(),
		now:          time.Now,
		cacheTTL:     defaultCacheTTL,
	}
}

func (p *Plugin) Name() string { return Name }

func (p *Plugin) SignPod(_ context.Context, pod *v1.Pod) ([]framework.SignFragment, *framework.Status) {
	policyName := ""
	if pod != nil {
		policyName = pod.Annotations[PolicyAnnotation]
	}
	fragments := []framework.SignFragment{{
		Key:   "scheduling.bupt.dev/TopologyFit.policy",
		Value: policyName,
	}}
	requests := extendedResourceRequests(pod)
	resourceNames := make([]string, 0, len(requests))
	for resourceName := range requests {
		resourceNames = append(resourceNames, string(resourceName))
	}
	sort.Strings(resourceNames)
	for _, resourceName := range resourceNames {
		fragments = append(fragments, framework.SignFragment{
			Key:   "scheduling.bupt.dev/TopologyFit.request." + resourceName,
			Value: fmt.Sprintf("%d", requests[v1.ResourceName(resourceName)]),
		})
	}
	return fragments, nil
}

func (p *Plugin) PreFilter(
	_ context.Context,
	state framework.CycleState,
	pod *v1.Pod,
	_ []framework.NodeInfo,
) (*framework.PreFilterResult, *framework.Status) {
	if pod == nil {
		return nil, framework.NewStatus(framework.Error, "pod is nil")
	}
	policyName := pod.Annotations[PolicyAnnotation]
	if policyName == "" {
		state.Write(stateKey, &cycleData{})
		return nil, nil
	}
	if p.policyLister == nil {
		return nil, framework.NewStatus(framework.Error, "placement policy lister is unavailable")
	}
	policy, err := p.policyLister.AcceleratorPlacementPolicies(pod.Namespace).Get(policyName)
	if err != nil {
		return nil, framework.NewStatus(
			framework.UnschedulableAndUnresolvable,
			fmt.Sprintf("placement policy %s/%s is unavailable: %v", pod.Namespace, policyName, err),
		)
	}
	if err := validatePolicy(policy.Spec); err != nil {
		return nil, framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
	}
	requests := extendedResourceRequests(pod)
	if len(requests) != 1 {
		return nil, framework.NewStatus(
			framework.UnschedulableAndUnresolvable,
			fmt.Sprintf("placement policy requires exactly one positive extended resource request, got %d", len(requests)),
		)
	}
	for resourceName, requested := range requests {
		state.Write(stateKey, &cycleData{
			active:       true,
			podKey:       podKey(pod),
			policy:       *policy.Spec.DeepCopy(),
			resourceName: resourceName,
			requested:    requested,
		})
	}
	return nil, nil
}

func (p *Plugin) PreFilterExtensions() framework.PreFilterExtensions { return nil }

func (p *Plugin) Filter(
	_ context.Context,
	state framework.CycleState,
	_ *v1.Pod,
	nodeInfo framework.NodeInfo,
) (resultStatus *framework.Status) {
	started := time.Now()
	topologyMode := ""
	defer func() {
		observability.ObserveSchedulerOperation("filter", metricResult(resultStatus), topologyMode, time.Since(started))
	}()
	data, status := readCycleData(state)
	if !status.IsSuccess() || !data.active {
		return status
	}
	topologyMode = string(data.policy.TopologyMode)
	result := p.evaluate(data, nodeInfo)
	if result.feasible {
		return nil
	}
	return framework.NewStatus(framework.Unschedulable, Name+": "+result.reason)
}

func (p *Plugin) PreScore(
	_ context.Context,
	state framework.CycleState,
	_ *v1.Pod,
	nodes []framework.NodeInfo,
) *framework.Status {
	data, status := readCycleData(state)
	if !status.IsSuccess() || !data.active {
		return status
	}
	scores := make(map[string]int64, len(nodes))
	for _, nodeInfo := range nodes {
		if nodeInfo == nil || nodeInfo.Node() == nil {
			continue
		}
		result := p.evaluate(data, nodeInfo)
		if result.feasible {
			scores[nodeInfo.Node().Name] = result.score
		}
	}
	data.scores = scores
	state.Write(stateKey, data)
	return nil
}

func (p *Plugin) Score(
	_ context.Context,
	state framework.CycleState,
	_ *v1.Pod,
	nodeInfo framework.NodeInfo,
) (int64, *framework.Status) {
	data, status := readCycleData(state)
	if !status.IsSuccess() || !data.active {
		return framework.MinNodeScore, status
	}
	if nodeInfo == nil || nodeInfo.Node() == nil {
		return framework.MinNodeScore, framework.NewStatus(framework.Error, "node info has no node")
	}
	if score, found := data.scores[nodeInfo.Node().Name]; found {
		return score, nil
	}
	result := p.evaluate(data, nodeInfo)
	if !result.feasible {
		return framework.MinNodeScore, nil
	}
	return result.score, nil
}

func (p *Plugin) ScoreExtensions() framework.ScoreExtensions { return nil }

func (p *Plugin) Reserve(_ context.Context, state framework.CycleState, pod *v1.Pod, nodeName string) (resultStatus *framework.Status) {
	started := time.Now()
	topologyMode := ""
	defer func() {
		observability.ObserveSchedulerOperation("reserve", metricResult(resultStatus), topologyMode, time.Since(started))
		observability.SetSchedulerActiveReservations(p.ledger.Len())
	}()
	data, status := readCycleData(state)
	if !status.IsSuccess() || !data.active {
		return status
	}
	topologyMode = string(data.policy.TopologyMode)
	cached := p.store.Get(nodeName, p.now(), p.cacheTTL)
	if cached.Availability != topologycache.AvailabilityReady {
		if data.policy.FailurePolicy == schedulingv1alpha1.FailurePolicyBestEffort {
			return nil
		}
		return framework.NewStatus(framework.Unschedulable, fmt.Sprintf("%s: topology for node %q is %s", Name, nodeName, cached.Availability))
	}

	reserved, err := p.ledger.Reserve(data.podKey, nodeName, func(blocked map[string]struct{}) []string {
		devices := excludeReservedDevices(eligibleDevices(cached.Snapshot, data), blocked)
		return selectDeviceIDs(cached.Snapshot, devices, data.policy, int(data.requested))
	})
	if err != nil {
		return framework.NewStatus(framework.Unschedulable, fmt.Sprintf("%s: reserve device combination on %q: %v", Name, nodeName, err))
	}
	klog.InfoS("TopologyFit reserved advisory device combination", "pod", klog.KObj(pod), "podUID", podKey(pod), "node", nodeName, "deviceIDs", reserved.deviceIDs)
	return nil
}

func (p *Plugin) Unreserve(_ context.Context, state framework.CycleState, pod *v1.Pod, nodeName string) {
	started := time.Now()
	topologyMode, result := "", observability.ResultSuccess
	defer func() {
		observability.ObserveSchedulerOperation("unreserve", result, topologyMode, time.Since(started))
		observability.SetSchedulerActiveReservations(p.ledger.Len())
	}()
	data, status := readCycleData(state)
	if !status.IsSuccess() || !data.active {
		result = metricResult(status)
		return
	}
	topologyMode = string(data.policy.TopologyMode)
	p.ledger.Release(data.podKey)
	klog.InfoS("TopologyFit released advisory device combination", "pod", klog.KObj(pod), "podUID", podKey(pod), "node", nodeName, "outcome", "unreserve")
}

func (p *Plugin) PreBindPreFlight(context.Context, framework.CycleState, *v1.Pod, string) *framework.Status {
	return nil
}

func (p *Plugin) PreBind(_ context.Context, state framework.CycleState, pod *v1.Pod, nodeName string) (resultStatus *framework.Status) {
	started := time.Now()
	topologyMode := ""
	defer func() {
		observability.ObserveSchedulerOperation("prebind", metricResult(resultStatus), topologyMode, time.Since(started))
	}()
	data, status := readCycleData(state)
	if !status.IsSuccess() || !data.active {
		return status
	}
	topologyMode = string(data.policy.TopologyMode)
	reserved, found := p.ledger.Get(data.podKey)
	if !found {
		cached := p.store.Get(nodeName, p.now(), p.cacheTTL)
		if data.policy.FailurePolicy == schedulingv1alpha1.FailurePolicyBestEffort && cached.Availability != topologycache.AvailabilityReady {
			return nil
		}
		return framework.NewStatus(framework.Error, fmt.Sprintf("%s: advisory reservation for pod %q is missing", Name, podKey(pod)))
	}
	if reserved.nodeName != nodeName {
		return framework.NewStatus(framework.Error, fmt.Sprintf("%s: reservation node %q does not match bind node %q", Name, reserved.nodeName, nodeName))
	}
	klog.InfoS("TopologyFit verified advisory device combination", "pod", klog.KObj(pod), "podUID", podKey(pod), "node", nodeName, "deviceIDs", reserved.deviceIDs)
	return nil
}

func (p *Plugin) PostBind(_ context.Context, state framework.CycleState, pod *v1.Pod, nodeName string) {
	started := time.Now()
	topologyMode, result := "", observability.ResultSuccess
	defer func() {
		observability.ObserveSchedulerOperation("postbind", result, topologyMode, time.Since(started))
		observability.SetSchedulerActiveReservations(p.ledger.Len())
	}()
	data, status := readCycleData(state)
	if !status.IsSuccess() || !data.active {
		result = metricResult(status)
		return
	}
	topologyMode = string(data.policy.TopologyMode)
	p.ledger.Release(data.podKey)
	klog.InfoS("TopologyFit released advisory device combination", "pod", klog.KObj(pod), "podUID", podKey(pod), "node", nodeName, "outcome", "bound")
}

func metricResult(status *framework.Status) string {
	if status == nil || status.IsSuccess() {
		return observability.ResultSuccess
	}
	if status.Code() == framework.Error {
		return observability.ResultError
	}
	return observability.ResultReject
}

func readCycleData(state framework.CycleState) (*cycleData, *framework.Status) {
	if state == nil {
		return nil, framework.NewStatus(framework.Error, "cycle state is nil")
	}
	stored, err := state.Read(stateKey)
	if err != nil {
		return nil, framework.NewStatus(framework.Error, fmt.Sprintf("read cycle state: %v", err))
	}
	data, ok := stored.(*cycleData)
	if !ok {
		return nil, framework.NewStatus(framework.Error, fmt.Sprintf("unexpected cycle state %T", stored))
	}
	return data, nil
}

func validatePolicy(spec schedulingv1alpha1.AcceleratorPlacementPolicySpec) error {
	if spec.DRA != nil {
		if spec.DRA.DeviceCount < 1 {
			return fmt.Errorf("DRA device count must be positive")
		}
		if spec.TopologyMode == schedulingv1alpha1.TopologyModeFabricConnected || spec.AllowUnknownTopology {
			return fmt.Errorf("DRA template generation requires known non-connected topology mode")
		}
	}
	switch spec.Vendor {
	case "", schedulingv1alpha1.VendorAny, schedulingv1alpha1.VendorNVIDIA, schedulingv1alpha1.VendorHuawei, schedulingv1alpha1.VendorAMD:
	default:
		return fmt.Errorf("unsupported accelerator vendor %q", spec.Vendor)
	}
	if spec.MinimumMemoryMiB < 0 || spec.MinimumLinkBandwidthGBps < 0 || spec.MaxFabricHops < 0 {
		return fmt.Errorf("placement policy memory, bandwidth, and hop constraints must be non-negative")
	}
	for _, product := range spec.Products {
		if product == "" {
			return fmt.Errorf("placement policy products cannot contain an empty value")
		}
	}
	switch spec.TopologyMode {
	case schedulingv1alpha1.TopologyModeSingleNUMA,
		schedulingv1alpha1.TopologyModeSamePCIeRoot,
		schedulingv1alpha1.TopologyModeFabricClique,
		schedulingv1alpha1.TopologyModeFabricConnected,
		schedulingv1alpha1.TopologyModeBestEffort:
	default:
		return fmt.Errorf("unsupported topology mode %q", spec.TopologyMode)
	}
	if spec.FailurePolicy != schedulingv1alpha1.FailurePolicyFailClosed &&
		spec.FailurePolicy != schedulingv1alpha1.FailurePolicyBestEffort {
		return fmt.Errorf("unsupported failure policy %q", spec.FailurePolicy)
	}
	weights := spec.Weights
	if weights.Locality < 0 || weights.Fabric < 0 || weights.Fragmentation < 0 || weights.Headroom < 0 ||
		weights.Locality+weights.Fabric+weights.Fragmentation+weights.Headroom != 100 {
		return fmt.Errorf("placement policy weights must be non-negative and sum to 100")
	}
	return nil
}

func podKey(pod *v1.Pod) string {
	if pod == nil {
		return ""
	}
	if pod.UID != "" {
		return string(pod.UID)
	}
	return pod.Namespace + "/" + pod.Name
}
