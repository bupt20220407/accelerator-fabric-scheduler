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
scheduler_token="$(kubectl -n accelerator-system create token accelerator-scheduler --duration=10m)"
kubectl -n accelerator-system port-forward deployment/accelerator-scheduler 19059:10259 >"$temporary/scheduler-port-forward.log" 2>&1 &
pids="$pids $!"
kubectl -n accelerator-system port-forward deployment/accelerator-topology-controller 19081:8080 >"$temporary/controller-port-forward.log" 2>&1 &
pids="$pids $!"
kubectl -n accelerator-system port-forward "pod/$driver_pod" 19082:8080 >"$temporary/dra-port-forward.log" 2>&1 &
pids="$pids $!"

attempt=0
while [ "$attempt" -lt 30 ]; do
  scheduler_ready=false
  controller_ready=false
  dra_ready=false
  curl -kfsS -H "Authorization: Bearer $scheduler_token" https://127.0.0.1:19059/healthz >/dev/null 2>&1 && scheduler_ready=true
  curl -fsS http://127.0.0.1:19081/healthz >/dev/null 2>&1 && controller_ready=true
  curl -fsS http://127.0.0.1:19082/healthz >/dev/null 2>&1 && dra_ready=true
  if [ "$scheduler_ready" = true ] && [ "$controller_ready" = true ] && [ "$dra_ready" = true ]; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
if [ "$scheduler_ready" != true ] || [ "$controller_ready" != true ] || [ "$dra_ready" != true ]; then
  cat "$temporary"/*-port-forward.log >&2
  echo "one or more component health endpoints were not reachable" >&2
  exit 1
fi

curl -kfsS -H "Authorization: Bearer $scheduler_token" https://127.0.0.1:19059/metrics > "$temporary/scheduler.metrics"
curl -fsS http://127.0.0.1:19081/metrics > "$temporary/controller.metrics"
curl -fsS http://127.0.0.1:19082/metrics > "$temporary/dra.metrics"

if [ -n "${METRICS_EVIDENCE_DIR:-}" ]; then
  mkdir -p "$METRICS_EVIDENCE_DIR"
  cp "$temporary/scheduler.metrics" "$METRICS_EVIDENCE_DIR/scheduler.metrics"
  cp "$temporary/controller.metrics" "$METRICS_EVIDENCE_DIR/controller.metrics"
  cp "$temporary/dra.metrics" "$METRICS_EVIDENCE_DIR/dra.metrics"
fi

if ! grep 'accelerator_fabric_scheduler_topologyfit_operations_total{' "$temporary/scheduler.metrics" | grep 'operation="reserve"' | grep -q 'result="success"'; then
  echo "scheduler metrics did not contain a successful TopologyFit reservation" >&2
  exit 1
fi
if ! grep '^scheduler_plugin_evaluation_total{' "$temporary/scheduler.metrics" | grep 'plugin="Coscheduling"' | grep -qv ' 0$'; then
  echo "scheduler metrics did not contain a Coscheduling evaluation" >&2
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

echo "scheduler, Coscheduling, controller, and node-local DRA health/metrics endpoints exposed expected W7 signals"
cleanup
trap - EXIT
