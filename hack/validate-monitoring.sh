#!/usr/bin/env sh
set -eu

image="prom/prometheus@sha256:63805ebb8d2b3920190daf1cb14a60871b16fd38bed42b857a3182bc621f4996"
rules="deploy/monitoring/prometheus-rules.yaml"
tests="deploy/monitoring/prometheus-rules.test.yaml"

docker run --rm --entrypoint=/bin/promtool \
  -v "$PWD:/workspace:ro" \
  "$image" check rules "/workspace/$rules"

docker run --rm --entrypoint=/bin/promtool \
  --workdir=/workspace/deploy/monitoring \
  -v "$PWD:/workspace:ro" \
  "$image" test rules "$(basename "$tests")"

for alert in \
  AcceleratorDiscoveryMetricsMissing \
  AcceleratorDiscoveryStale \
  AcceleratorDiscoveryErrors \
  AcceleratorControllerReconcileErrors \
  AcceleratorTopologyFitErrors \
  AcceleratorSchedulerPodsUnschedulable \
  AcceleratorSchedulerMetricsMissing \
  AcceleratorDRAPublishedDevicesMissing \
  AcceleratorDRAPublishedDevicesZero \
  AcceleratorDRAOperationErrors; do
  grep -q "alert: $alert" "$rules"
done

echo "promtool accepted and evaluated all W10 alert expressions"
