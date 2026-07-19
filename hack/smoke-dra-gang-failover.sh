#!/usr/bin/env sh
set -eu

pods="w8-dra-nvidia w8-dra-huawei w8-dra-amd"
policies="w8-dra-nvidia w8-dra-huawei w8-dra-amd"
driver="accelerator.scheduling.bupt.dev"
missing_node="accelerator-fabric-worker3"
discovery_paused=false

cleanup_workload() {
  kubectl delete pod $pods --ignore-not-found --wait=true >/dev/null 2>&1 || true
  kubectl delete podgroup w8-multi-node-dra --ignore-not-found --wait=true >/dev/null 2>&1 || true
  kubectl delete acceleratorplacementpolicy $policies --ignore-not-found --wait=true >/dev/null 2>&1 || true
}

restore_discovery() {
  if [ "$discovery_paused" = true ]; then
    kubectl -n accelerator-system patch daemonset accelerator-topology-discovery --type=merge \
      -p='{"spec":{"template":{"spec":{"nodeSelector":null}}}}' >/dev/null
    kubectl -n accelerator-system rollout status daemonset/accelerator-topology-discovery --timeout=180s >/dev/null
    discovery_paused=false
  fi
}

cleanup() {
  cleanup_workload
  restore_discovery
}
trap cleanup EXIT
cleanup_workload

kubectl -n accelerator-system patch daemonset accelerator-topology-discovery --type=merge \
  -p='{"spec":{"template":{"spec":{"nodeSelector":{"scheduling.bupt.dev/discovery-paused":"true"}}}}}' >/dev/null
discovery_paused=true
attempt=0
while kubectl -n accelerator-system get pods -l app.kubernetes.io/name=accelerator-topology-discovery -o name | grep -q .; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    echo "discovery Agent Pods did not stop" >&2
    exit 1
  fi
  sleep 2
done

kubectl delete acceleratortopology "$missing_node" --wait=true >/dev/null
attempt=0
while [ "$attempt" -lt 60 ]; do
  devices="$(kubectl get resourceslices --field-selector "spec.driver=$driver,spec.nodeName=$missing_node" -o jsonpath='{range .items[*]}{.spec.devices[*].name}{" "}{end}' 2>/dev/null || true)"
  set -- $devices
  if [ "$#" -eq 0 ]; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 2
done
if [ "$#" -ne 0 ]; then
  echo "AMD ResourceSlice did not withdraw its devices after topology deletion" >&2
  exit 1
fi
echo "paused discovery and withdrew the AMD node-local DRA pool"

kubectl apply -f config/smoke/multi-node-dra-gang-policies.yaml >/dev/null
for policy in $policies; do
  attempt=0
  while ! kubectl get resourceclaimtemplate "$policy" >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
      echo "policy controller did not create ResourceClaimTemplate $policy" >&2
      exit 1
    fi
    sleep 2
  done
done
kubectl apply -f config/smoke/multi-node-dra-gang.yaml >/dev/null

claims=""
waiting_before_restart=false
attempt=0
while [ "$attempt" -lt 30 ]; do
  claims=""
  nominated=0
  for pod in $pods; do
    claim="$(kubectl get pod "$pod" -o jsonpath='{.status.resourceClaimStatuses[0].resourceClaimName}' 2>/dev/null || true)"
    if [ -n "$claim" ]; then
      claims="$claims $claim"
    fi
    nominated_node="$(kubectl get pod "$pod" -o jsonpath='{.status.nominatedNodeName}' 2>/dev/null || true)"
    if [ -n "$nominated_node" ]; then
      nominated=$((nominated + 1))
    fi
  done
  scheduler_logs="$(kubectl -n accelerator-system logs deployment/accelerator-scheduler --since=2m 2>/dev/null || true)"
  permit_waits="$(printf '%s\n' "$scheduler_logs" | grep 'Pod is waiting to be scheduled to node' | grep -c 'w8-dra-' || true)"
  if [ "$nominated" -eq 2 ] && [ "$permit_waits" -ge 2 ]; then
    waiting_before_restart=true
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
if [ "$waiting_before_restart" != true ]; then
  kubectl get pods $pods -o wide >&2 || true
  echo "the two feasible gang members did not both reach Coscheduling Permit wait" >&2
  exit 1
fi
for pod in $pods; do
  if [ -n "$(kubectl get pod "$pod" -o jsonpath='{.spec.nodeName}')" ]; then
    echo "$pod bound before the incomplete gang was admitted" >&2
    exit 1
  fi
done
for claim in $claims; do
  pool="$(kubectl get resourceclaim "$claim" -o jsonpath='{.status.allocation.devices.results[0].pool}' 2>/dev/null || true)"
  if [ -n "$pool" ]; then
    echo "$claim persisted allocation $pool before Permit completed" >&2
    exit 1
  fi
done
echo "two feasible DRA members reached Permit with internal nominations, while no claim allocation or Pod binding was committed"

kubectl -n accelerator-system rollout restart deployment/accelerator-scheduler >/dev/null
kubectl -n accelerator-system rollout status deployment/accelerator-scheduler --timeout=180s >/dev/null

attempt=0
while [ "$attempt" -lt 60 ]; do
  remaining=0
  for claim in $claims; do
    pool="$(kubectl get resourceclaim "$claim" -o jsonpath='{.status.allocation.devices.results[0].pool}' 2>/dev/null || true)"
    if [ -n "$pool" ]; then
      remaining=$((remaining + 1))
    fi
  done
  nominated=0
  for pod in $pods; do
    nominated_node="$(kubectl get pod "$pod" -o jsonpath='{.status.nominatedNodeName}' 2>/dev/null || true)"
    if [ -n "$nominated_node" ]; then
      nominated=$((nominated + 1))
    fi
  done
  if [ "$remaining" -eq 0 ] && [ "$nominated" -eq 0 ]; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 2
done
if [ "$remaining" -ne 0 ] || [ "$nominated" -ne 0 ]; then
  kubectl get resourceclaims -o yaml >&2 || true
  kubectl get pods $pods -o wide >&2 || true
  echo "$remaining claim allocations and $nominated nominations remained after scheduler restart and Permit timeout" >&2
  exit 1
fi
for pod in $pods; do
  if [ -n "$(kubectl get pod "$pod" -o jsonpath='{.spec.nodeName}')" ]; then
    echo "$pod bound despite the missing AMD pool" >&2
    exit 1
  fi
done
echo "scheduler restart preserved gang atomicity and cleared all provisional nominations without persisting DRA allocations"

cleanup_workload
restore_discovery
attempt=0
while [ "$attempt" -lt 60 ]; do
  ready="$(kubectl get acceleratortopology "$missing_node" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)"
  devices="$(kubectl get resourceslices --field-selector "spec.driver=$driver,spec.nodeName=$missing_node" -o jsonpath='{range .items[*]}{.spec.devices[*].name}{" "}{end}' 2>/dev/null || true)"
  set -- $devices
  if [ "$ready" = "True" ] && [ "$#" -eq 8 ]; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 2
done
if [ "$ready" != "True" ] || [ "$#" -ne 8 ]; then
  echo "AMD topology and ResourceSlice did not recover" >&2
  exit 1
fi
echo "discovery recreated the AMD topology and restored all eight DRA devices"

./hack/smoke-multi-node-dra-gang.sh
trap - EXIT
