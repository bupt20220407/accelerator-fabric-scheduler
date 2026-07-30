package policy

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	fakeclient "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/clientset/versioned/fake"
	informers "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/informers/externalversions"
)

func TestControllerReconcilesAndRemovesTemplate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	policy := testPolicy()
	schedulingClient := fakeclient.NewSimpleClientset(policy)
	kubeClient := kubefake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(schedulingClient, 0)
	controller := New(kubeClient, factory.Scheduling().V1alpha1().AcceleratorPlacementPolicies())
	factory.Start(ctx.Done())
	go func() { _ = controller.Run(ctx, 1) }()

	eventually(t, func() bool {
		template, err := kubeClient.ResourceV1().ResourceClaimTemplates(policy.Namespace).Get(ctx, policy.Name, metav1.GetOptions{})
		return err == nil && template.Spec.Spec.Devices.Requests[0].Exactly.Count == 4
	})

	updated, err := schedulingClient.SchedulingV1alpha1().AcceleratorPlacementPolicies(policy.Namespace).Get(ctx, policy.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	updated.Spec.DRA.DeviceCount = 2
	if _, err := schedulingClient.SchedulingV1alpha1().AcceleratorPlacementPolicies(policy.Namespace).Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		template, err := kubeClient.ResourceV1().ResourceClaimTemplates(policy.Namespace).Get(ctx, policy.Name, metav1.GetOptions{})
		return err == nil && template.Spec.Spec.Devices.Requests[0].Exactly.Count == 2
	})

	updated, err = schedulingClient.SchedulingV1alpha1().AcceleratorPlacementPolicies(policy.Namespace).Get(ctx, policy.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	updated.Spec.DRA = nil
	if _, err := schedulingClient.SchedulingV1alpha1().AcceleratorPlacementPolicies(policy.Namespace).Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		_, err := kubeClient.ResourceV1().ResourceClaimTemplates(policy.Namespace).Get(ctx, policy.Name, metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	})
}

func TestControllerRetriesTransientTemplateCreate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	policy := testPolicy()
	schedulingClient := fakeclient.NewSimpleClientset(policy)
	kubeClient := kubefake.NewSimpleClientset()
	var attempts atomic.Int32
	kubeClient.Fake.PrependReactor("create", "resourceclaimtemplates", func(k8stesting.Action) (bool, runtime.Object, error) {
		if attempts.Add(1) == 1 {
			return true, nil, apierrors.NewServiceUnavailable("injected template create failure")
		}
		return false, nil, nil
	})
	factory := informers.NewSharedInformerFactory(schedulingClient, 0)
	controller := New(kubeClient, factory.Scheduling().V1alpha1().AcceleratorPlacementPolicies())
	factory.Start(ctx.Done())
	go func() { _ = controller.Run(ctx, 1) }()

	eventually(t, func() bool {
		template, err := kubeClient.ResourceV1().ResourceClaimTemplates(policy.Namespace).Get(ctx, policy.Name, metav1.GetOptions{})
		return attempts.Load() >= 2 && err == nil && template.Spec.Spec.Devices.Requests[0].Exactly.Count == 4
	})
}

func eventually(t *testing.T, condition func() bool) {
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
