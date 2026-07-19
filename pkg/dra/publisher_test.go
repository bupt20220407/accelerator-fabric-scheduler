package dra

import (
	"context"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/dynamic-resource-allocation/resourceslice"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/fixtures"
	fakeclient "github.com/bupt/accelerator-fabric-scheduler/pkg/generated/clientset/versioned/fake"
	informers "github.com/bupt/accelerator-fabric-scheduler/pkg/generated/informers/externalversions"
)

func TestObserveTopologiesPublishesReadyAndClearsNotReady(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	object := fixtures.ThreeNodeTopologies()[0].DeepCopy()
	object.Generation = 1
	heartbeat := metav1.NewTime(now)
	object.Status = schedulingv1alpha1.AcceleratorTopologyStatus{
		Conditions:        []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue, ObservedGeneration: 1}},
		LastHeartbeatTime: &heartbeat,
	}
	client := fakeclient.NewSimpleClientset(object)
	factory := informers.NewSharedInformerFactory(client, 0)
	informer := factory.Scheduling().V1alpha1().AcceleratorTopologies()
	var mu sync.Mutex
	last := EmptyResources()
	_, err := ObserveTopologies(ctx, informer, object.Spec.NodeName, func(_ context.Context, resources resourceslice.DriverResources) error {
		mu.Lock()
		defer mu.Unlock()
		last = *resources.DeepCopy()
		return nil
	}, func() time.Time { return now }, nil)
	if err != nil {
		t.Fatal(err)
	}
	factory.Start(ctx.Done())
	eventuallyDRA(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		pool, found := last.Pools[object.Spec.NodeName]
		return found && len(pool.Slices) == 1 && len(pool.Slices[0].Devices) == 8
	})

	updated, err := client.SchedulingV1alpha1().AcceleratorTopologies().Get(ctx, object.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	updated.Status.Conditions[0].Status = metav1.ConditionFalse
	if _, err := client.SchedulingV1alpha1().AcceleratorTopologies().UpdateStatus(ctx, updated, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	eventuallyDRA(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(last.Pools) == 0
	})
}

func eventuallyDRA(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}
