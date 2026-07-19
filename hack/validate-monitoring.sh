#!/usr/bin/env sh
set -eu

image="prom/prometheus@sha256:63805ebb8d2b3920190daf1cb14a60871b16fd38bed42b857a3182bc621f4996"
rules="deploy/monitoring/prometheus-rules.yaml"

docker run --rm --entrypoint=/bin/promtool \
  -v "$PWD:/workspace:ro" \
  "$image" check rules "/workspace/$rules"

for alert in \
  AcceleratorDiscoveryMetricsMissing \
  AcceleratorDiscoveryStale \
  AcceleratorDiscoveryErrors \
  AcceleratorControllerReconcileErrors \
  AcceleratorTopologyFitErrors \
  AcceleratorSchedulerPodsUnschedulable \
  AcceleratorCoschedulingPermitFailures \
  AcceleratorDRAPublishedDevicesMissing \
  AcceleratorDRAPublishedDevicesZero \
  AcceleratorDRAOperationErrors; do
  grep -q "alert: $alert" "$rules"
done

echo "promtool accepted all W8 recording and alert expressions"
