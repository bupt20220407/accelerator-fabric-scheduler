#!/usr/bin/env sh
set -eu

pass_pods="w7-gang-pass-0 w7-gang-pass-1"
rollback_pods="w7-gang-rollback-0 w7-gang-rollback-1"
recovery_pod="w7-gang-recovery"
policy="w7-gang-clique"

scale_scheduler() {
  replicas="$1"
  kubectl -n accelerator-system scale deployment/accelerator-scheduler --replicas="$replicas" >/dev/null
  kubectl -n accelerator-system rollout status deployment/accelerator-scheduler --timeout=180s >/dev/null
}

cleanup() {
  kubectl delete pod $pass_pods $rollback_pods "$recovery_pod" --ignore-not-found --wait=true >/dev/null 2>&1 || true
  kubectl delete podgroup w7-gang-pass w7-gang-rollback --ignore-not-found --wait=true >/dev/null 2>&1 || true
  kubectl delete acceleratorplacementpolicy "$policy" --ignore-not-found --wait=true >/dev/null 2>&1 || true
  scale_scheduler 1 >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup

scale_scheduler 0
kubectl apply -f config/smoke/gang-scheduling.yaml >/dev/null
scale_scheduler 1
for pod in $pass_pods; do
  kubectl wait --for=jsonpath='{.status.phase}'=Running "pod/$pod" --timeout=180s >/dev/null
  node="$(kubectl get pod "$pod" -o jsonpath='{.spec.nodeName}')"
  if [ "$node" != "accelerator-fabric-worker" ]; then
    echo "$pod ran on $node, want NVIDIA fixture worker" >&2
    exit 1
  fi
  output="$(kubectl logs "$pod")"
  case "$output" in
    *"member=${pod#w7-gang-}"*) ;;
    *)
      echo "$pod did not start the gang workload: $output" >&2
      exit 1
      ;;
  esac
done

scheduler_logs="$(kubectl -n accelerator-system logs deployment/accelerator-scheduler --since=3m)"
for pod in $pass_pods; do
  if ! printf '%s\n' "$scheduler_logs" | grep 'TopologyFit reserved advisory device combination' | grep -q "pod=\"default/$pod\""; then
    echo "$pod did not reserve a topology clique before gang Permit" >&2
    exit 1
  fi
done
if ! printf '%s\n' "$scheduler_logs" | grep -q 'Pod is waiting to be scheduled to node'; then
  echo "Coscheduling did not emit Permit wait evidence" >&2
  exit 1
fi
for pod in $pass_pods; do
  kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "pod/$pod" --timeout=60s >/dev/null
done
echo "two four-device members passed gang Permit and ran together on the two NVIDIA cliques"

kubectl delete pod $pass_pods --wait=true >/dev/null
kubectl delete podgroup w7-gang-pass --wait=true >/dev/null
scale_scheduler 0
kubectl apply -f config/smoke/gang-rollback.yaml >/dev/null
scale_scheduler 1

attempt=0
while [ "$attempt" -lt 45 ]; do
  scheduler_logs="$(kubectl -n accelerator-system logs deployment/accelerator-scheduler --since=3m)"
  reserved=false
  released=false
  printf '%s\n' "$scheduler_logs" | grep 'TopologyFit reserved advisory device combination' | grep -q 'pod="default/w7-gang-rollback-0"' && reserved=true
  printf '%s\n' "$scheduler_logs" | grep 'TopologyFit released advisory device combination' | grep 'pod="default/w7-gang-rollback-0"' | grep -q 'outcome="unreserve"' && released=true
  if [ "$reserved" = true ] && [ "$released" = true ]; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
if [ "$reserved" != true ] || [ "$released" != true ]; then
  kubectl -n accelerator-system logs deployment/accelerator-scheduler --since=5m >&2
  echo "gang timeout did not produce TopologyFit reservation rollback evidence" >&2
  exit 1
fi
for pod in $rollback_pods; do
  node="$(kubectl get pod "$pod" -o jsonpath='{.spec.nodeName}')"
  if [ -n "$node" ]; then
    echo "$pod was bound to $node despite an incomplete gang" >&2
    exit 1
  fi
done

kubectl delete pod $rollback_pods --wait=true >/dev/null
kubectl delete podgroup w7-gang-rollback --wait=true >/dev/null
kubectl apply -f config/smoke/gang-recovery.yaml >/dev/null
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "pod/$recovery_pod" --timeout=180s >/dev/null
echo "incomplete gang timed out and rolled back its advisory clique; an independent recovery Pod then completed"

cleanup
trap - EXIT
