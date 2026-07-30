package topology

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	schedulingv1alpha1 "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	fakeclient "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/clientset/versioned/fake"
	informers "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/informers/externalversions"
)

func TestControllerWritesReadyStatus(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	object := validObject()
	client := fakeclient.NewSimpleClientset(object)
	factory := informers.NewSharedInformerFactory(client, 0)
	controller := New(client, factory.Scheduling().V1alpha1().AcceleratorTopologies())
	controller.now = func() time.Time { return now }
	factory.Start(ctx.Done())
	go func() { _ = controller.Run(ctx, 1) }()

	eventually(t, func() bool {
		updated, err := client.SchedulingV1alpha1().AcceleratorTopologies().Get(ctx, object.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		condition := apiMeta.FindStatusCondition(updated.Status.Conditions, "Ready")
		return condition != nil && condition.Status == metav1.ConditionTrue && updated.Status.LastHeartbeatTime != nil
	})
}

func TestControllerMarksInvalidGraphNotReady(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	object := validObject()
	object.Spec.Links = []schedulingv1alpha1.AcceleratorLink{{
		Source: "gpu0", Target: "missing", Type: schedulingv1alpha1.LinkNVLink, Health: schedulingv1alpha1.HealthHealthy,
	}}
	client := fakeclient.NewSimpleClientset(object)
	factory := informers.NewSharedInformerFactory(client, 0)
	controller := New(client, factory.Scheduling().V1alpha1().AcceleratorTopologies())
	factory.Start(ctx.Done())
	go func() { _ = controller.Run(ctx, 1) }()

	eventually(t, func() bool {
		updated, err := client.SchedulingV1alpha1().AcceleratorTopologies().Get(ctx, object.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		condition := apiMeta.FindStatusCondition(updated.Status.Conditions, "Ready")
		return condition != nil && condition.Status == metav1.ConditionFalse && condition.Reason == "InvalidTopology"
	})
}

func TestControllerRetriesTransientStatusUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	object := validObject()
	client := fakeclient.NewSimpleClientset(object)
	var attempts atomic.Int32
	client.Fake.PrependReactor("update", "acceleratortopologies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		if attempts.Add(1) == 1 {
			return true, nil, apierrors.NewServiceUnavailable("injected status update failure")
		}
		return false, nil, nil
	})
	factory := informers.NewSharedInformerFactory(client, 0)
	controller := New(client, factory.Scheduling().V1alpha1().AcceleratorTopologies())
	factory.Start(ctx.Done())
	go func() { _ = controller.Run(ctx, 1) }()

	eventually(t, func() bool {
		updated, err := client.SchedulingV1alpha1().AcceleratorTopologies().Get(ctx, object.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		condition := apiMeta.FindStatusCondition(updated.Status.Conditions, "Ready")
		return attempts.Load() >= 2 && condition != nil && condition.Status == metav1.ConditionTrue
	})
}

func validObject() *schedulingv1alpha1.AcceleratorTopology {
	return &schedulingv1alpha1.AcceleratorTopology{
		TypeMeta:   metav1.TypeMeta{APIVersion: schedulingv1alpha1.SchemeGroupVersion.String(), Kind: "AcceleratorTopology"},
		ObjectMeta: metav1.ObjectMeta{Name: "worker", Generation: 1},
		Spec: schedulingv1alpha1.AcceleratorTopologySpec{
			NodeName:           "worker",
			ObservedGeneration: 1,
			Source:             schedulingv1alpha1.TopologySourceFixture,
			Devices: []schedulingv1alpha1.AcceleratorDevice{{
				ID: "gpu0", Vendor: schedulingv1alpha1.VendorNVIDIA, Product: "H100", ResourceName: "nvidia.com/gpu", Health: schedulingv1alpha1.HealthHealthy,
			}},
		},
	}
}

func eventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}
