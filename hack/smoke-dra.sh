#!/usr/bin/env sh
set -eu

driver="accelerator.scheduling.bupt.dev"
pod="synthetic-dra-allocation"

attempt=0
while [ "$attempt" -lt 60 ]; do
  ready=true
  for node in accelerator-fabric-worker accelerator-fabric-worker2 accelerator-fabric-worker3; do
    devices="$(kubectl get resourceslices --field-selector "spec.driver=$driver,spec.nodeName=$node" -o jsonpath='{range .items[*]}{.spec.devices[*].name}{" "}{end}' 2>/dev/null || true)"
    set -- $devices
    if [ "$#" -ne 8 ]; then
      ready=false
      break
    fi
  done
  if [ "$ready" = true ]; then
    echo "DRA driver published eight devices for each fixture worker"
    break
  fi
  attempt=$((attempt + 1))
  sleep 2
done
if [ "$ready" != true ]; then
  kubectl get resourceslices -o wide >&2 || true
  echo "DRA ResourceSlices did not converge" >&2
  exit 1
fi

kubectl delete pod "$pod" --ignore-not-found --wait=true
kubectl delete resourceclaim synthetic-dra-nvidia --ignore-not-found --wait=true
kubectl delete resourceclaimtemplate synthetic-dra-nvidia --ignore-not-found --wait=true
kubectl apply -f config/smoke/dra-allocation.yaml
kubectl wait --for=jsonpath='{.status.phase}'=Running "pod/$pod" --timeout=180s

claim="synthetic-dra-nvidia"
claim_uid="$(kubectl get resourceclaim "$claim" -o jsonpath='{.metadata.uid}')"
allocated="$(kubectl get resourceclaim "$claim" -o jsonpath='{range .status.allocation.devices.results[*]}{.driver}/{.pool}/{.device}{"\n"}{end}')"
set -- $allocated
if [ "$#" -ne 4 ]; then
  echo "$claim allocated $# devices, want 4: $allocated" >&2
  exit 1
fi
case "$allocated" in
  *"$driver/accelerator-fabric-worker/gpu0"*"$driver/accelerator-fabric-worker/gpu1"*"$driver/accelerator-fabric-worker/gpu2"*"$driver/accelerator-fabric-worker/gpu3"*) ;;
  *"$driver/accelerator-fabric-worker/gpu4"*"$driver/accelerator-fabric-worker/gpu5"*"$driver/accelerator-fabric-worker/gpu6"*"$driver/accelerator-fabric-worker/gpu7"*) ;;
  *)
    echo "$claim did not allocate one NVIDIA fabric clique: $allocated" >&2
    exit 1
    ;;
esac

workload_output="$(kubectl logs "$pod")"
case "$workload_output" in
  *"claim=$claim_uid"*"accelerator-fabric-worker/gpu"*) ;;
  *)
    echo "$pod did not receive authoritative DRA CDI metadata: $workload_output" >&2
    exit 1
    ;;
esac
echo "$pod consumed claim $claim with four authoritative DRA device IDs"

node="$(kubectl get pod "$pod" -o jsonpath='{.spec.nodeName}')"
driver_pod="$(kubectl -n accelerator-system get pods -l app.kubernetes.io/name=synthetic-dra-driver --field-selector "spec.nodeName=$node" -o jsonpath='{.items[0].metadata.name}')"
kubectl -n accelerator-system delete pod "$driver_pod" --wait=true
kubectl -n accelerator-system rollout status daemonset/synthetic-dra-driver --timeout=180s
replacement="$(kubectl -n accelerator-system get pods -l app.kubernetes.io/name=synthetic-dra-driver --field-selector "spec.nodeName=$node" -o jsonpath='{.items[0].metadata.name}')"
replacement_logs="$(kubectl -n accelerator-system logs "$replacement")"
if ! printf '%s\n' "$replacement_logs" | grep -q "recovered 1 prepared DRA claims on node $node"; then
  echo "$replacement did not recover the prepared claim after restart" >&2
  exit 1
fi
echo "$replacement recovered the prepared claim on $node"

kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "pod/$pod" --timeout=90s

kubectl delete pod "$pod" --wait=true
attempt=0
while [ "$attempt" -lt 60 ]; do
  driver_logs="$(kubectl -n accelerator-system logs -l app.kubernetes.io/name=synthetic-dra-driver --prefix --since=5m 2>/dev/null || true)"
  if printf '%s\n' "$driver_logs" | grep 'unprepared authoritative DRA allocation' | grep -q "$claim_uid"; then
    kubectl delete resourceclaim "$claim" --wait=true
    echo "$claim was unprepared, released, and deleted after Pod deletion"
    exit 0
  fi
  attempt=$((attempt + 1))
  sleep 2
done

echo "$claim did not emit NodeUnprepareResources evidence" >&2
exit 1
