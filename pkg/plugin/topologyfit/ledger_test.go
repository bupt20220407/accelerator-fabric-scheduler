package topologyfit

import (
	"errors"
	"sort"
	"sync"
	"testing"
)

func TestReservationLedgerConcurrentAtomicity(t *testing.T) {
	ledger := newReservationLedger()
	start := make(chan struct{})
	results := make(chan reservation, 3)
	errorsCh := make(chan error, 3)
	var workers sync.WaitGroup
	for index := 0; index < 3; index++ {
		workers.Add(1)
		go func(podKey string) {
			defer workers.Done()
			<-start
			reserved, err := ledger.Reserve(podKey, "worker", func(blocked map[string]struct{}) []string {
				available := make([]string, 0, 2)
				for _, deviceID := range []string{"gpu0", "gpu1", "gpu2", "gpu3"} {
					if _, found := blocked[deviceID]; !found {
						available = append(available, deviceID)
					}
					if len(available) == 2 {
						return available
					}
				}
				return nil
			})
			if err != nil {
				errorsCh <- err
				return
			}
			results <- reserved
		}(string(rune('a' + index)))
	}
	close(start)
	workers.Wait()
	close(results)
	close(errorsCh)

	reservations := make([]reservation, 0, 2)
	for reserved := range results {
		reservations = append(reservations, reserved)
	}
	if len(reservations) != 2 || ledger.Len() != 2 {
		t.Fatalf("successful reservations = %d, ledger length = %d", len(reservations), ledger.Len())
	}
	failed := 0
	for err := range errorsCh {
		failed++
		if !errors.Is(err, ErrNoAvailableCombination) {
			t.Fatalf("concurrent Reserve() error = %v", err)
		}
	}
	if failed != 1 {
		t.Fatalf("failed reservations = %d, want 1", failed)
	}

	allDeviceIDs := append(append([]string(nil), reservations[0].deviceIDs...), reservations[1].deviceIDs...)
	sort.Strings(allDeviceIDs)
	want := []string{"gpu0", "gpu1", "gpu2", "gpu3"}
	for index := range want {
		if allDeviceIDs[index] != want[index] {
			t.Fatalf("reserved device IDs = %v, want %v", allDeviceIDs, want)
		}
	}
}

func TestReservationLedgerRollbackAndIdempotency(t *testing.T) {
	ledger := newReservationLedger()
	selected := func(map[string]struct{}) []string { return []string{"gpu0", "gpu1"} }
	first, err := ledger.Reserve("pod", "worker", selected)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ledger.Reserve("pod", "worker", func(map[string]struct{}) []string { return nil })
	if err != nil || second.deviceIDs[0] != first.deviceIDs[0] {
		t.Fatalf("idempotent Reserve() = (%+v, %v)", second, err)
	}
	if _, err := ledger.Reserve("other", "worker", selected); err == nil {
		t.Fatal("Reserve() allowed devices already owned by another pod")
	}
	if ledger.Len() != 1 {
		t.Fatalf("failed reservation changed ledger length to %d", ledger.Len())
	}
	ledger.Release("pod")
	if ledger.Len() != 0 || len(ledger.ReservedDeviceIDs("worker", "")) != 0 {
		t.Fatal("Release() leaked reservation state")
	}
}
