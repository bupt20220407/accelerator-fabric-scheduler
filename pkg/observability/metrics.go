package observability

import (
	"sync"
	"time"

	componentmetrics "k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

const (
	ResultSuccess = "success"
	ResultError   = "error"
	ResultReject  = "reject"
)

var (
	schedulerOperations = componentmetrics.NewCounterVec(
		&componentmetrics.CounterOpts{
			Namespace:      "accelerator_fabric",
			Subsystem:      "scheduler",
			Name:           "topologyfit_operations_total",
			Help:           "TopologyFit operations partitioned by operation, result, and topology mode.",
			StabilityLevel: componentmetrics.ALPHA,
		},
		[]string{"operation", "result", "topology_mode"},
	)
	schedulerOperationDuration = componentmetrics.NewHistogramVec(
		&componentmetrics.HistogramOpts{
			Namespace:      "accelerator_fabric",
			Subsystem:      "scheduler",
			Name:           "topologyfit_operation_duration_seconds",
			Help:           "TopologyFit operation latency in seconds.",
			Buckets:        latencyBuckets,
			StabilityLevel: componentmetrics.ALPHA,
		},
		[]string{"operation", "result", "topology_mode"},
	)
	schedulerActiveReservations = componentmetrics.NewGauge(
		&componentmetrics.GaugeOpts{
			Namespace:      "accelerator_fabric",
			Subsystem:      "scheduler",
			Name:           "topologyfit_active_reservations",
			Help:           "Current number of advisory TopologyFit reservations.",
			StabilityLevel: componentmetrics.ALPHA,
		},
	)

	controllerReconciles = componentmetrics.NewCounterVec(
		&componentmetrics.CounterOpts{
			Namespace:      "accelerator_fabric",
			Subsystem:      "controller",
			Name:           "reconciles_total",
			Help:           "Controller reconciliations partitioned by controller and result.",
			StabilityLevel: componentmetrics.ALPHA,
		},
		[]string{"controller", "result"},
	)
	controllerReconcileDuration = componentmetrics.NewHistogramVec(
		&componentmetrics.HistogramOpts{
			Namespace:      "accelerator_fabric",
			Subsystem:      "controller",
			Name:           "reconcile_duration_seconds",
			Help:           "Controller reconciliation latency in seconds.",
			Buckets:        latencyBuckets,
			StabilityLevel: componentmetrics.ALPHA,
		},
		[]string{"controller", "result"},
	)

	draOperations = componentmetrics.NewCounterVec(
		&componentmetrics.CounterOpts{
			Namespace:      "accelerator_fabric",
			Subsystem:      "dra",
			Name:           "operations_total",
			Help:           "DRA driver operations partitioned by operation and result.",
			StabilityLevel: componentmetrics.ALPHA,
		},
		[]string{"operation", "result"},
	)
	draOperationDuration = componentmetrics.NewHistogramVec(
		&componentmetrics.HistogramOpts{
			Namespace:      "accelerator_fabric",
			Subsystem:      "dra",
			Name:           "operation_duration_seconds",
			Help:           "DRA driver operation latency in seconds.",
			Buckets:        latencyBuckets,
			StabilityLevel: componentmetrics.ALPHA,
		},
		[]string{"operation", "result"},
	)
	draPreparedClaims = componentmetrics.NewGauge(
		&componentmetrics.GaugeOpts{
			Namespace:      "accelerator_fabric",
			Subsystem:      "dra",
			Name:           "prepared_claims",
			Help:           "Current number of claims prepared by this node-local DRA driver.",
			StabilityLevel: componentmetrics.ALPHA,
		},
	)
	draPublishedDevices = componentmetrics.NewGauge(
		&componentmetrics.GaugeOpts{
			Namespace:      "accelerator_fabric",
			Subsystem:      "dra",
			Name:           "published_devices",
			Help:           "Current number of devices published by this node-local DRA driver.",
			StabilityLevel: componentmetrics.ALPHA,
		},
	)

	registerSchedulerOnce  sync.Once
	registerControllerOnce sync.Once
	registerDRAOnce        sync.Once
)

var latencyBuckets = []float64{0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

func RegisterSchedulerMetrics() {
	registerSchedulerOnce.Do(func() {
		legacyregistry.MustRegister(schedulerOperations, schedulerOperationDuration, schedulerActiveReservations)
	})
}

func ObserveSchedulerOperation(operation, result, topologyMode string, duration time.Duration) {
	schedulerOperations.WithLabelValues(operation, result, normalizeLabel(topologyMode)).Inc()
	schedulerOperationDuration.WithLabelValues(operation, result, normalizeLabel(topologyMode)).Observe(duration.Seconds())
}

func SetSchedulerActiveReservations(count int) {
	schedulerActiveReservations.Set(float64(count))
}

func RegisterControllerMetrics() {
	registerControllerOnce.Do(func() {
		legacyregistry.MustRegister(controllerReconciles, controllerReconcileDuration)
	})
}

func ObserveControllerReconcile(controller, result string, duration time.Duration) {
	controllerReconciles.WithLabelValues(controller, result).Inc()
	controllerReconcileDuration.WithLabelValues(controller, result).Observe(duration.Seconds())
}

func RegisterDRAMetrics() {
	registerDRAOnce.Do(func() {
		legacyregistry.MustRegister(draOperations, draOperationDuration, draPreparedClaims, draPublishedDevices)
	})
}

func ObserveDRAOperation(operation, result string, duration time.Duration) {
	draOperations.WithLabelValues(operation, result).Inc()
	draOperationDuration.WithLabelValues(operation, result).Observe(duration.Seconds())
}

func SetDRAPreparedClaims(count int) {
	draPreparedClaims.Set(float64(count))
}

func SetDRAPublishedDevices(count int) {
	draPublishedDevices.Set(float64(count))
}

func normalizeLabel(value string) string {
	if value == "" {
		return "none"
	}
	return value
}
