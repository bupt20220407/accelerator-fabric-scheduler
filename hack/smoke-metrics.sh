#!/usr/bin/env sh
set -eu

temporary="$(mktemp -d)"
pids=""

cleanup() {
  for pid in $pids; do
    kill "$pid" >/dev/null 2>&1 || true
    wait "$pid" >/dev/null 2>&1 || true
  done
  rm -rf "$temporary"
}
trap cleanup EXIT

driver_pod="$(kubectl -n accelerator-system get pods -l app.kubernetes.io/name=synthetic-dra-driver --field-selector spec.nodeName=accelerator-fabric-worker -o jsonpath='{.items[0].metadata.name}')"
discovery_pod="$(kubectl -n accelerator-system get pods -l app.kubernetes.io/name=accelerator-topology-discovery --field-selector spec.nodeName=accelerator-fabric-worker -o jsonpath='{.items[0].metadata.name}')"
scheduler_token="$(kubectl -n accelerator-system create token accelerator-scheduler --duration=10m)"
kubectl -n accelerator-system port-forward deployment/accelerator-scheduler 19059:10259 >"$temporary/scheduler-port-forward.log" 2>&1 &
pids="$pids $!"
kubectl -n accelerator-system port-forward deployment/accelerator-topology-controller 19081:8080 >"$temporary/controller-port-forward.log" 2>&1 &
pids="$pids $!"
kubectl -n accelerator-system port-forward "pod/$driver_pod" 19082:8080 >"$temporary/dra-port-forward.log" 2>&1 &
pids="$pids $!"
kubectl -n accelerator-system port-forward "pod/$discovery_pod" 19083:8080 >"$temporary/discovery-port-forward.log" 2>&1 &
pids="$pids $!"

attempt=0
while [ "$attempt" -lt 30 ]; do
  scheduler_ready=false
  controller_ready=false
  dra_ready=false
  discovery_ready=false
  curl -kfsS -H "Authorization: Bearer $scheduler_token" https://127.0.0.1:19059/healthz >/dev/null 2>&1 && scheduler_ready=true
  curl -fsS http://127.0.0.1:19081/healthz >/dev/null 2>&1 && controller_ready=true
  curl -fsS http://127.0.0.1:19082/healthz >/dev/null 2>&1 && dra_ready=true
  curl -fsS http://127.0.0.1:19083/healthz >/dev/null 2>&1 && discovery_ready=true
  if [ "$scheduler_ready" = true ] && [ "$controller_ready" = true ] && [ "$dra_ready" = true ] && [ "$discovery_ready" = true ]; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
if [ "$scheduler_ready" != true ] || [ "$controller_ready" != true ] || [ "$dra_ready" != true ] || [ "$discovery_ready" != true ]; then
  cat "$temporary"/*-port-forward.log >&2
  echo "one or more component health endpoints were not reachable" >&2
  exit 1
fi

curl -kfsS -H "Authorization: Bearer $scheduler_token" https://127.0.0.1:19059/metrics > "$temporary/scheduler.metrics"
curl -fsS http://127.0.0.1:19081/metrics > "$temporary/controller.metrics"
curl -fsS http://127.0.0.1:19082/metrics > "$temporary/dra.metrics"
curl -fsS http://127.0.0.1:19083/metrics > "$temporary/discovery.metrics"

if [ -n "${METRICS_EVIDENCE_DIR:-}" ]; then
  mkdir -p "$METRICS_EVIDENCE_DIR"
  cp "$temporary/scheduler.metrics" "$METRICS_EVIDENCE_DIR/scheduler.metrics"
  cp "$temporary/controller.metrics" "$METRICS_EVIDENCE_DIR/controller.metrics"
  cp "$temporary/dra.metrics" "$METRICS_EVIDENCE_DIR/dra.metrics"
  cp "$temporary/discovery.metrics" "$METRICS_EVIDENCE_DIR/discovery.metrics"
fi

if ! grep 'accelerator_fabric_scheduler_topologyfit_operations_total{' "$temporary/scheduler.metrics" | grep 'operation="reserve"' | grep -q 'result="success"'; then
  echo "scheduler metrics did not contain a successful TopologyFit reservation" >&2
  exit 1
fi
if ! grep '^scheduler_plugin_evaluation_total{' "$temporary/scheduler.metrics" | grep 'plugin="Coscheduling"' | grep -qv ' 0$'; then
  echo "scheduler metrics did not contain a Coscheduling evaluation" >&2
  exit 1
fi
if ! grep -q '^scheduler_pending_pods{queue="unschedulable"} ' "$temporary/scheduler.metrics"; then
  echo "scheduler metrics did not expose the queue label expected by pending-Pod alerts" >&2
  exit 1
fi
if ! grep -q '^scheduler_plugin_execution_duration_seconds_count{extension_point="Permit",plugin="Coscheduling",status="Success"} ' "$temporary/scheduler.metrics"; then
  echo "scheduler metrics did not expose the labels expected by the Coscheduling Permit alert" >&2
  exit 1
fi
if ! grep 'accelerator_fabric_controller_reconciles_total{' "$temporary/controller.metrics" | grep 'controller="policy"' | grep -q 'result="success"'; then
  echo "controller metrics did not contain a successful policy reconciliation" >&2
  exit 1
fi
if ! grep -q '^accelerator_fabric_dra_published_devices 8$' "$temporary/dra.metrics"; then
  echo "DRA metrics did not report eight published devices" >&2
  exit 1
fi
if ! grep -q '^accelerator_fabric_dra_prepared_claims 0$' "$temporary/dra.metrics"; then
  echo "DRA metrics did not return to zero prepared claims" >&2
  exit 1
fi
if ! grep 'accelerator_fabric_discovery_operations_total{' "$temporary/discovery.metrics" | grep 'provider="fixture"' | grep 'result="success"' | grep -qv ' 0$'; then
  echo "discovery metrics did not contain a successful fixture refresh" >&2
  exit 1
fi
if ! grep -q '^accelerator_fabric_discovery_last_success_timestamp_seconds{provider="fixture"} ' "$temporary/discovery.metrics"; then
  echo "discovery metrics did not expose a last-success timestamp" >&2
  exit 1
fi

echo "scheduler, Coscheduling, controller, discovery, and node-local DRA health/metrics endpoints exposed expected W8 signals"
cleanup
trap - EXIT
