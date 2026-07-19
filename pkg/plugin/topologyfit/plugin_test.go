package topologyfit

import (
	"context"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	toolscache "k8s.io/client-go/tools/cache"
	framework "k8s.io/kube-scheduler/framework"
	internalframework "k8s.io/kubernetes/pkg/scheduler/framework"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/fixtures"
	listers "github.com/bupt/accelerator-fabric-scheduler/pkg/generated/listers/scheduling/v1alpha1"
	topologycache "github.com/bupt/accelerator-fabric-scheduler/pkg/topology/cache"
)

const testNodeName = "accelerator-fabric-worker"

func TestNewRejectsInvalidInputs(t *testing.T) {
	if _, err := New(context.Background(), &runtime.Unknown{}, nil); err == nil {
		t.Fatal("New() accepted plugin configuration")
	}
	if _, err := New(context.Background(), nil, nil); err == nil {
		t.Fatal("New() accepted a nil scheduler handle")
	}
}

func TestUnannotatedPodIsNeutral(t *testing.T) {
	plugin := newPlugin(topologycache.NewStore(), nil)
	state := internalframework.NewCycleState()
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "ordinary", Namespace: "default"}}

	if _, status := plugin.PreFilter(context.Background(), state, pod, nil); !status.IsSuccess() {
		t.Fatalf("PreFilter() status = %v, want success", status)
	}
	if status := plugin.Filter(context.Background(), state, pod, testNodeInfo(nil)); !status.IsSuccess() {
		t.Fatalf("Filter() status = %v, want success", status)
	}
	score, status := plugin.Score(context.Background(), state, pod, testNodeInfo(nil))
	if !status.IsSuccess() || score != framework.MinNodeScore {
		t.Fatalf("Score() = (%d, %v), want neutral", score, status)
	}
}

func TestFilterTopologyModes(t *testing.T) {
	tests := []struct {
		name      string
		mode      schedulingv1alpha1.TopologyMode
		requested int64
		wantFit   bool
	}{
		{name: "single NUMA fits four", mode: schedulingv1alpha1.TopologyModeSingleNUMA, requested: 4, wantFit: true},
		{name: "single NUMA rejects five", mode: schedulingv1alpha1.TopologyModeSingleNUMA, requested: 5, wantFit: false},
		{name: "same PCIe root fits four", mode: schedulingv1alpha1.TopologyModeSamePCIeRoot, requested: 4, wantFit: true},
		{name: "same PCIe root rejects five", mode: schedulingv1alpha1.TopologyModeSamePCIeRoot, requested: 5, wantFit: false},
		{name: "fabric clique fits four", mode: schedulingv1alpha1.TopologyModeFabricClique, requested: 4, wantFit: true},
		{name: "fabric clique rejects five", mode: schedulingv1alpha1.TopologyModeFabricClique, requested: 5, wantFit: false},
		{name: "fabric connected fits four", mode: schedulingv1alpha1.TopologyModeFabricConnected, requested: 4, wantFit: true},
		{name: "fabric connected excludes PCIe bridge", mode: schedulingv1alpha1.TopologyModeFabricConnected, requested: 5, wantFit: false},
		{name: "best effort fits all devices", mode: schedulingv1alpha1.TopologyModeBestEffort, requested: 8, wantFit: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := testPolicy(test.mode, schedulingv1alpha1.FailurePolicyFailClosed)
			plugin := testPlugin(t, policy, true)
			state := internalframework.NewCycleState()
			pod := testPod(test.requested)
			if _, status := plugin.PreFilter(context.Background(), state, pod, nil); !status.IsSuccess() {
				t.Fatalf("PreFilter() status = %v", status)
			}
			status := plugin.Filter(context.Background(), state, pod, testNodeInfo(nil))
			if status.IsSuccess() != test.wantFit {
				t.Fatalf("Filter() status = %v, want fit %v", status, test.wantFit)
			}
		})
	}
}

func TestTopologyAvailabilityFailurePolicy(t *testing.T) {
	tests := []struct {
		name    string
		failure schedulingv1alpha1.FailurePolicy
		wantFit bool
	}{
		{name: "fail closed", failure: schedulingv1alpha1.FailurePolicyFailClosed, wantFit: false},
		{name: "best effort", failure: schedulingv1alpha1.FailurePolicyBestEffort, wantFit: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plugin := testPlugin(t, testPolicy(schedulingv1alpha1.TopologyModeFabricClique, test.failure), false)
			state := internalframework.NewCycleState()
			pod := testPod(2)
			if _, status := plugin.PreFilter(context.Background(), state, pod, nil); !status.IsSuccess() {
				t.Fatalf("PreFilter() status = %v", status)
			}
			status := plugin.Filter(context.Background(), state, pod, testNodeInfo(nil))
			if status.IsSuccess() != test.wantFit {
				t.Fatalf("Filter() status = %v, want fit %v", status, test.wantFit)
			}
		})
	}
}

func TestScoreUsesDeterministicFeatures(t *testing.T) {
	policy := testPolicy(schedulingv1alpha1.TopologyModeFabricClique, schedulingv1alpha1.FailurePolicyFailClosed)
	policy.Spec.Weights = schedulingv1alpha1.ScoreWeights{Locality: 100}
	plugin := testPlugin(t, policy, true)
	state := internalframework.NewCycleState()
	pod := testPod(2)
	nodeInfo := testNodeInfo(nil)

	if _, status := plugin.PreFilter(context.Background(), state, pod, nil); !status.IsSuccess() {
		t.Fatalf("PreFilter() status = %v", status)
	}
	if status := plugin.PreScore(context.Background(), state, pod, []framework.NodeInfo{nodeInfo}); !status.IsSuccess() {
		t.Fatalf("PreScore() status = %v", status)
	}
	score, status := plugin.Score(context.Background(), state, pod, nodeInfo)
	if !status.IsSuccess() || score != 50 {
		t.Fatalf("Score() = (%d, %v), want (50, success)", score, status)
	}
}

func TestPreFilterRejectsMissingPolicyAndAmbiguousRequests(t *testing.T) {
	plugin := testPlugin(t, nil, true)
	state := internalframework.NewCycleState()
	if _, status := plugin.PreFilter(context.Background(), state, testPod(2), nil); status.Code() != framework.UnschedulableAndUnresolvable {
		t.Fatalf("missing policy status = %v", status)
	}

	policy := testPolicy(schedulingv1alpha1.TopologyModeBestEffort, schedulingv1alpha1.FailurePolicyFailClosed)
	plugin = testPlugin(t, policy, true)
	pod := testPod(2)
	pod.Spec.Containers[0].Resources.Requests["example.com/other"] = resource.MustParse("1")
	state = internalframework.NewCycleState()
	if _, status := plugin.PreFilter(context.Background(), state, pod, nil); status.Code() != framework.UnschedulableAndUnresolvable {
		t.Fatalf("ambiguous request status = %v", status)
	}
}

func TestSignPodIncludesPolicyAndRequest(t *testing.T) {
	plugin := newPlugin(topologycache.NewStore(), nil)
	fragments, status := plugin.SignPod(context.Background(), testPod(2))
	if !status.IsSuccess() || len(fragments) != 2 {
		t.Fatalf("SignPod() = (%v, %v)", fragments, status)
	}
	if fragments[0].Value != "test-policy" || fragments[1].Value != "2" {
		t.Fatalf("SignPod() fragments = %v", fragments)
	}
}

func TestW3BindingHooksRemainNeutral(t *testing.T) {
	plugin := newPlugin(topologycache.NewStore(), nil)
	state := internalframework.NewCycleState()
	pod := testPod(2)
	if status := plugin.Reserve(context.Background(), state, pod, testNodeName); !status.IsSuccess() {
		t.Fatalf("Reserve() status = %v", status)
	}
	plugin.Unreserve(context.Background(), state, pod, testNodeName)
	if status := plugin.PreBindPreFlight(context.Background(), state, pod, testNodeName); !status.IsSuccess() {
		t.Fatalf("PreBindPreFlight() status = %v", status)
	}
	if status := plugin.PreBind(context.Background(), state, pod, testNodeName); !status.IsSuccess() {
		t.Fatalf("PreBind() status = %v", status)
	}
}

func testPlugin(t *testing.T, policy *schedulingv1alpha1.AcceleratorPlacementPolicy, topologyReady bool) *Plugin {
	t.Helper()
	indexer := toolscache.NewIndexer(toolscache.MetaNamespaceKeyFunc, toolscache.Indexers{toolscache.NamespaceIndex: toolscache.MetaNamespaceIndexFunc})
	if policy != nil {
		if err := indexer.Add(policy); err != nil {
			t.Fatal(err)
		}
	}
	store := topologycache.NewStore()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	if topologyReady {
		object := fixtures.ThreeNodeTopologies()[0].DeepCopy()
		object.Generation = 1
		heartbeat := metav1.NewTime(now)
		object.Status = schedulingv1alpha1.AcceleratorTopologyStatus{
			Conditions: []metav1.Condition{{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 1,
				Reason:             "ValidTopology",
			}},
			LastHeartbeatTime: &heartbeat,
		}
		if err := store.Upsert(object); err != nil {
			t.Fatal(err)
		}
	}
	plugin := newPlugin(store, listers.NewAcceleratorPlacementPolicyLister(indexer))
	plugin.now = func() time.Time { return now }
	return plugin
}

func testPolicy(mode schedulingv1alpha1.TopologyMode, failure schedulingv1alpha1.FailurePolicy) *schedulingv1alpha1.AcceleratorPlacementPolicy {
	return &schedulingv1alpha1.AcceleratorPlacementPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
		Spec: schedulingv1alpha1.AcceleratorPlacementPolicySpec{
			Vendor:                   schedulingv1alpha1.VendorNVIDIA,
			Products:                 []string{"H100-SXM"},
			MinimumMemoryMiB:         80000,
			TopologyMode:             mode,
			MinimumLinkBandwidthGBps: 300,
			MaxFabricHops:            1,
			Weights: schedulingv1alpha1.ScoreWeights{
				Locality: 35, Fabric: 35, Fragmentation: 20, Headroom: 10,
			},
			FailurePolicy: failure,
		},
	}
}

func testPod(requested int64) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "training",
			Namespace:   "default",
			Annotations: map[string]string{PolicyAnnotation: "test-policy"},
		},
		Spec: v1.PodSpec{Containers: []v1.Container{{
			Name: "worker",
			Resources: v1.ResourceRequirements{Requests: v1.ResourceList{
				"nvidia.com/gpu": *resource.NewQuantity(requested, resource.DecimalSI),
			}},
		}}},
	}
}

func testNodeInfo(pods []*v1.Pod) framework.NodeInfo {
	info := internalframework.NewNodeInfo(pods...)
	info.SetNode(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Status: v1.NodeStatus{Allocatable: v1.ResourceList{
			"nvidia.com/gpu": resource.MustParse("8"),
		}},
	})
	return info
}
