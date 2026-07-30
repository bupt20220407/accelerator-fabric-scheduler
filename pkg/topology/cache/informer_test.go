package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"

	fakeclient "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/clientset/versioned/fake"
	informers "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/informers/externalversions"
)

func TestObserveAcceleratorTopologies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now()
	object := readyTopology(now, 1, 1)
	client := fakeclient.NewSimpleClientset(object)
	factory := informers.NewSharedInformerFactory(client, 0)
	store := NewStore()

	var mu sync.Mutex
	var observedErrors []error
	_, err := ObserveAcceleratorTopologies(factory.Scheduling().V1alpha1().AcceleratorTopologies(), store, func(err error) {
		mu.Lock()
		defer mu.Unlock()
		observedErrors = append(observedErrors, err)
	})
	if err != nil {
		t.Fatalf("ObserveAcceleratorTopologies() error = %v", err)
	}

	factory.Start(ctx.Done())
	for informerType, synced := range factory.WaitForCacheSync(ctx.Done()) {
		if !synced {
			t.Fatalf("topology informer %v did not sync", informerType)
		}
	}
	eventually(t, func() bool { return store.Len() == 1 })

	if err := client.SchedulingV1alpha1().AcceleratorTopologies().Delete(ctx, object.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	eventually(t, func() bool { return store.Len() == 0 })

	mu.Lock()
	defer mu.Unlock()
	if len(observedErrors) != 0 {
		t.Fatalf("observer errors = %v", observedErrors)
	}
}

func TestTopologyFromDeleteTombstone(t *testing.T) {
	object := readyTopology(time.Now(), 1, 1)
	got, err := topologyFromDelete(toolscache.DeletedFinalStateUnknown{Key: object.Name, Obj: object})
	if err != nil || got != object {
		t.Fatalf("topologyFromDelete() = (%v, %v)", got, err)
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
