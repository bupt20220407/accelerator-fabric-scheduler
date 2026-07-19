package cache

import (
	"errors"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
)

func TestStoreFreshnessAndGeneration(t *testing.T) {
	now := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	store := NewStore()
	object := readyTopology(now, 2, 10)
	if err := store.Upsert(object); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if result := store.Get("worker", now, time.Minute); result.Availability != AvailabilityReady {
		t.Fatalf("availability = %s, want Ready", result.Availability)
	}
	if result := store.Get("worker", now.Add(2*time.Minute), time.Minute); result.Availability != AvailabilityStale {
		t.Fatalf("availability = %s, want Stale", result.Availability)
	}

	older := readyTopology(now, 1, 9)
	if err := store.Upsert(older); !errors.Is(err, ErrOlderGeneration) {
		t.Fatalf("older Upsert() error = %v, want ErrOlderGeneration", err)
	}
	if result := store.Get("worker", now, time.Minute); result.ObjectGeneration != 2 || result.ObservedGeneration != 10 {
		t.Fatalf("older update replaced cache: %+v", result)
	}
}

func TestStoreRequiresReadyConditionForCurrentGeneration(t *testing.T) {
	now := time.Now()
	object := readyTopology(now, 3, 4)
	object.Status.Conditions[0].ObservedGeneration = 2
	store := NewStore()
	if err := store.Upsert(object); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if result := store.Get("worker", now, time.Minute); result.Availability != AvailabilityNotReady {
		t.Fatalf("availability = %s, want NotReady", result.Availability)
	}
}

func TestStoreConcurrentReadersAndWriters(t *testing.T) {
	store := NewStore()
	now := time.Now()
	if err := store.Upsert(readyTopology(now, 1, 1)); err != nil {
		t.Fatalf("initial Upsert() error = %v", err)
	}

	var wg sync.WaitGroup
	for reader := 0; reader < 16; reader++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				result := store.Get("worker", now, time.Minute)
				if result.Snapshot == nil {
					t.Errorf("reader observed nil snapshot")
					return
				}
			}
		}()
	}
	for generation := int64(2); generation <= 20; generation++ {
		if err := store.Upsert(readyTopology(now, generation, generation)); err != nil {
			t.Fatalf("Upsert(%d) error = %v", generation, err)
		}
	}
	wg.Wait()
}

func TestStoreDelete(t *testing.T) {
	store := NewStore()
	if err := store.Upsert(readyTopology(time.Now(), 1, 1)); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	store.Delete("worker")
	if result := store.Get("worker", time.Now(), time.Minute); result.Availability != AvailabilityMissing || store.Len() != 0 {
		t.Fatalf("delete result = %+v, len = %d", result, store.Len())
	}
}

func readyTopology(now time.Time, objectGeneration, observedGeneration int64) *schedulingv1alpha1.AcceleratorTopology {
	return &schedulingv1alpha1.AcceleratorTopology{
		ObjectMeta: metav1.ObjectMeta{Name: "worker", Generation: objectGeneration, ResourceVersion: "1"},
		Spec: schedulingv1alpha1.AcceleratorTopologySpec{
			NodeName:           "worker",
			ObservedGeneration: observedGeneration,
			Source:             schedulingv1alpha1.TopologySourceFixture,
			Devices: []schedulingv1alpha1.AcceleratorDevice{
				{ID: "gpu0", Vendor: schedulingv1alpha1.VendorNVIDIA, Product: "H100", ResourceName: "nvidia.com/gpu", Health: schedulingv1alpha1.HealthHealthy},
			},
		},
		Status: schedulingv1alpha1.AcceleratorTopologyStatus{
			Conditions:        []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue, ObservedGeneration: objectGeneration, Reason: "Valid", LastTransitionTime: metav1.NewTime(now)}},
			LastHeartbeatTime: func() *metav1.Time { value := metav1.NewTime(now); return &value }(),
		},
	}
}
