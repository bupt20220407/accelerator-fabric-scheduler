package topologyfit

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	framework "k8s.io/kube-scheduler/framework"
)

func TestNew(t *testing.T) {
	plugin, err := New(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if plugin.Name() != Name {
		t.Fatalf("plugin name = %q, want %q", plugin.Name(), Name)
	}
}

func TestNewRejectsConfigurationBeforeTypedArgsExist(t *testing.T) {
	_, err := New(context.Background(), &runtime.Unknown{}, nil)
	if err == nil {
		t.Fatal("New() accepted untyped configuration")
	}
}

func TestW1LifecycleIsNeutral(t *testing.T) {
	p := &Plugin{}
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "smoke", Namespace: "default"}}
	ctx := context.Background()
	var state framework.CycleState
	var nodeInfo framework.NodeInfo

	fragments, status := p.SignPod(ctx, pod)
	if !status.IsSuccess() || len(fragments) != 1 || fragments[0].Value != "neutral-v1" {
		t.Fatalf("SignPod() = (%v, %v), want one neutral fragment", fragments, status)
	}
	result, status := p.PreFilter(ctx, state, pod, nil)
	if result != nil || !status.IsSuccess() {
		t.Fatalf("PreFilter() = (%v, %v), want neutral success", result, status)
	}
	if status := p.Filter(ctx, state, pod, nodeInfo); !status.IsSuccess() {
		t.Fatalf("Filter() status = %v, want success", status)
	}
	if status := p.PreScore(ctx, state, pod, nil); !status.IsSuccess() {
		t.Fatalf("PreScore() status = %v, want success", status)
	}
	score, status := p.Score(ctx, state, pod, nodeInfo)
	if !status.IsSuccess() || score != framework.MinNodeScore {
		t.Fatalf("Score() = (%d, %v), want (%d, success)", score, status, framework.MinNodeScore)
	}
	if status := p.Reserve(ctx, state, pod, "worker"); !status.IsSuccess() {
		t.Fatalf("Reserve() status = %v, want success", status)
	}
	p.Unreserve(ctx, state, pod, "worker")
	if status := p.PreBindPreFlight(ctx, state, pod, "worker"); !status.IsSuccess() {
		t.Fatalf("PreBindPreFlight() status = %v, want success", status)
	}
	if status := p.PreBind(ctx, state, pod, "worker"); !status.IsSuccess() {
		t.Fatalf("PreBind() status = %v, want success", status)
	}
}
