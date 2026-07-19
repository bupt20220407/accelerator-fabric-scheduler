package observability

import (
	"testing"
	"time"

	"k8s.io/component-base/metrics/legacyregistry"
)

func TestComponentMetricsAreGatherable(t *testing.T) {
	RegisterSchedulerMetrics()
	RegisterControllerMetrics()
	RegisterDRAMetrics()
	RegisterDiscoveryMetrics()
	ObserveSchedulerOperation("filter", ResultSuccess, "fabric-clique", time.Millisecond)
	SetSchedulerActiveReservations(1)
	ObserveControllerReconcile("policy", ResultSuccess, time.Millisecond)
	ObserveDRAOperation("prepare", ResultSuccess, time.Millisecond)
	SetDRAPreparedClaims(1)
	SetDRAPublishedDevices(8)
	ObserveDiscovery("fixture", true, time.Millisecond)

	families, err := legacyregistry.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{
		"accelerator_fabric_scheduler_topologyfit_operations_total":    false,
		"accelerator_fabric_scheduler_topologyfit_active_reservations": false,
		"accelerator_fabric_controller_reconciles_total":               false,
		"accelerator_fabric_dra_operations_total":                      false,
		"accelerator_fabric_dra_prepared_claims":                       false,
		"accelerator_fabric_dra_published_devices":                     false,
		"accelerator_fabric_discovery_operations_total":                false,
		"accelerator_fabric_discovery_last_success_timestamp_seconds":  false,
	}
	for _, family := range families {
		if _, found := wanted[family.GetName()]; found {
			wanted[family.GetName()] = true
		}
	}
	for name, found := range wanted {
		if !found {
			t.Errorf("metric family %q was not gathered", name)
		}
	}
}
