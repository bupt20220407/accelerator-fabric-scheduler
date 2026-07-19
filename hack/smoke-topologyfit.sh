#!/usr/bin/env sh
set -eu

pass_pod="topologyfit-clique-pass"
reject_pod="topologyfit-clique-reject"

kubectl delete pod "$pass_pod" "$reject_pod" --ignore-not-found --wait=true
kubectl apply -f config/smoke/topology-aware-scheduling.yaml
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "pod/$pass_pod" --timeout=180s

allocated="$(kubectl logs "$pass_pod")"
case "$allocated" in
  *"resource=nvidia.com/gpu"*"ids=nvidia-com-gpu-"*) ;;
  *)
    echo "$pass_pod did not complete the synthetic Allocate path: $allocated" >&2
    exit 1
    ;;
esac

scheduler_logs="$(kubectl -n accelerator-system logs deployment/accelerator-scheduler --since=5m)"
for lifecycle_message in \
  'TopologyFit reserved advisory device combination' \
  'TopologyFit verified advisory device combination' \
  'TopologyFit released advisory device combination'; do
  if ! printf '%s\n' "$scheduler_logs" | grep "$lifecycle_message" | grep -q "pod=\"default/$pass_pod\""; then
    echo "$pass_pod did not emit scheduler lifecycle evidence: $lifecycle_message" >&2
    exit 1
  fi
done

attempt=0
while [ "$attempt" -lt 60 ]; do
  node_name="$(kubectl get pod "$reject_pod" -o jsonpath='{.spec.nodeName}')"
  if [ -n "$node_name" ]; then
    echo "$reject_pod was unexpectedly bound to $node_name" >&2
    exit 1
  fi
  messages="$(kubectl get events --field-selector "involvedObject.kind=Pod,involvedObject.name=$reject_pod" -o jsonpath='{range .items[*]}{.message}{"\n"}{end}')"
  if printf '%s' "$messages" | grep -q 'TopologyFit' && printf '%s' "$messages" | grep -q 'cannot satisfy topology mode fabric-clique'; then
    echo "$pass_pod completed a four-device clique allocation"
    echo "$reject_pod remained Pending because TopologyFit rejected a five-device clique"
    kubectl delete pod "$reject_pod" --wait=true
    exit 0
  fi
  attempt=$((attempt + 1))
  sleep 2
done

echo "$reject_pod did not report the expected TopologyFit rejection" >&2
kubectl describe pod "$reject_pod" >&2
exit 1
