#!/usr/bin/env sh
set -eu

assert_allocatable() {
  node="$1"
  resource_path="$2"
  expected="$3"
  attempt=0
  while [ "$attempt" -lt 60 ]; do
    value="$(kubectl get node "$node" -o "jsonpath={.status.allocatable.${resource_path}}" 2>/dev/null || true)"
    if [ "$value" = "$expected" ]; then
      echo "$node publishes $expected units of $resource_path"
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 2
  done
  kubectl describe node "$node" >&2 || true
  echo "$node did not publish expected resource $resource_path=$expected" >&2
  return 1
}

assert_allocatable accelerator-fabric-worker 'nvidia\.com/gpu' 8
assert_allocatable accelerator-fabric-worker2 'huawei\.com/Ascend910' 8
assert_allocatable accelerator-fabric-worker3 'amd\.com/gpu' 8

kubectl delete -f config/smoke/synthetic-allocations.yaml --ignore-not-found --wait=true >/dev/null
kubectl apply -f config/smoke/synthetic-allocations.yaml >/dev/null

for expectation in \
  'synthetic-nvidia-allocation:nvidia.com/gpu' \
  'synthetic-ascend-allocation:huawei.com/Ascend910' \
  'synthetic-amd-allocation:amd.com/gpu'; do
  pod="${expectation%%:*}"
  resource="${expectation#*:}"
  if ! kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "pod/${pod}" --timeout=120s >/dev/null; then
    kubectl describe pod "$pod" >&2 || true
    exit 1
  fi
  output="$(kubectl logs "$pod")"
  case "$output" in
    "resource=${resource} ids="*,*)
      echo "$pod completed Allocate with $output"
      ;;
    *)
      echo "$pod returned unexpected allocation output: $output" >&2
      exit 1
      ;;
  esac
done
