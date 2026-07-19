#!/usr/bin/env sh
set -eu

pods="w8-dra-nvidia w8-dra-huawei w8-dra-amd"
policies="w8-dra-nvidia w8-dra-huawei w8-dra-amd"

cleanup() {
  kubectl delete pod $pods --ignore-not-found --wait=true >/dev/null 2>&1 || true
  kubectl delete podgroup w8-multi-node-dra --ignore-not-found --wait=true >/dev/null 2>&1 || true
  kubectl delete acceleratorplacementpolicy $policies --ignore-not-found --wait=true >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup

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
for pod in $pods; do
  kubectl wait --for=jsonpath='{.status.phase}'=Running "pod/$pod" --timeout=180s >/dev/null
done

claims=""
claim_uids=""
for pod in $pods; do
  claim="$(kubectl get pod "$pod" -o jsonpath='{.status.resourceClaimStatuses[0].resourceClaimName}')"
  claim_uid="$(kubectl get resourceclaim "$claim" -o jsonpath='{.metadata.uid}')"
  pool="$(kubectl get resourceclaim "$claim" -o jsonpath='{.status.allocation.devices.results[0].pool}')"
  device="$(kubectl get resourceclaim "$claim" -o jsonpath='{.status.allocation.devices.results[0].device}')"
  case "$pod:$pool:$device" in
    w8-dra-nvidia:accelerator-fabric-worker:gpu*) ;;
    w8-dra-huawei:accelerator-fabric-worker2:ascend*) ;;
    w8-dra-amd:accelerator-fabric-worker3:gpu*) ;;
    *)
      echo "$pod received unexpected allocation $pool/$device" >&2
      exit 1
      ;;
  esac
  node="$(kubectl get pod "$pod" -o jsonpath='{.spec.nodeName}')"
  if [ "$node" != "$pool" ]; then
    echo "$pod ran on $node but its node-local DRA pool is $pool" >&2
    exit 1
  fi
  output="$(kubectl logs "$pod")"
  case "$output" in
    *"claim=$claim_uid"*"$pool/$device"*) ;;
    *)
      echo "$pod did not consume its authoritative claim allocation: $output" >&2
      exit 1
      ;;
  esac
  claims="$claims $claim"
  claim_uids="$claim_uids $claim_uid"
done
echo "three PodGroup members consumed one node-local NVIDIA, Ascend, and AMD DRA claim"

for pod in $pods; do
  kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "pod/$pod" --timeout=90s >/dev/null
done
kubectl delete pod $pods --wait=true >/dev/null

for claim in $claims; do
  attempt=0
  while kubectl get resourceclaim "$claim" >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
      echo "generated claim $claim was not garbage collected" >&2
      exit 1
    fi
    sleep 2
  done
done
for claim_uid in $claim_uids; do
  attempt=0
  while [ "$attempt" -lt 60 ]; do
    logs="$(kubectl -n accelerator-system logs -l app.kubernetes.io/name=synthetic-dra-driver --prefix --since=5m 2>/dev/null || true)"
    if printf '%s\n' "$logs" | grep 'unprepared authoritative DRA allocation' | grep -q "$claim_uid"; then
      break
    fi
    attempt=$((attempt + 1))
    sleep 2
  done
  if [ "$attempt" -ge 60 ]; then
    echo "claim $claim_uid did not emit NodeUnprepareResources evidence" >&2
    exit 1
  fi
done

kubectl delete podgroup w8-multi-node-dra --wait=true >/dev/null
kubectl delete acceleratorplacementpolicy $policies --wait=true >/dev/null
for policy in $policies; do
  attempt=0
  while kubectl get resourceclaimtemplate "$policy" >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
      echo "generated template $policy was not garbage collected" >&2
      exit 1
    fi
    sleep 2
  done
done

echo "all three claims were unprepared and all generated claims/templates were garbage collected"
trap - EXIT
