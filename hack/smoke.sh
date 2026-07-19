#!/usr/bin/env sh
set -eu

EXPECTED_VERSION="${SCHEDULER_VERSION:-v1.35.5-accelerator.0.3.0}"

kubectl delete pod topologyfit-smoke --ignore-not-found --wait=true
kubectl apply -f config/smoke/pod.yaml

attempt=0
while [ "$attempt" -lt 60 ]; do
  node_name="$(kubectl get pod topologyfit-smoke -o jsonpath='{.spec.nodeName}' 2>/dev/null || true)"
  if [ -n "$node_name" ]; then
    scheduler_name="$(kubectl get pod topologyfit-smoke -o jsonpath='{.spec.schedulerName}')"
    if [ "$scheduler_name" != "accelerator-scheduler" ]; then
      echo "unexpected schedulerName: $scheduler_name" >&2
      exit 1
    fi
    break
  fi
  attempt=$((attempt + 1))
  sleep 2
done

if [ -z "${node_name:-}" ]; then
  kubectl describe pod topologyfit-smoke >&2 || true
  kubectl -n accelerator-system logs deployment/accelerator-scheduler --tail=200 >&2 || true
  echo "smoke pod was not assigned within 120 seconds" >&2
  exit 1
fi

attempt=0
while [ "$attempt" -lt 30 ]; do
  phase="$(kubectl get pod topologyfit-smoke -o jsonpath='{.status.phase}' 2>/dev/null || true)"
  case "$phase" in
    Succeeded)
      output="$(kubectl logs topologyfit-smoke)"
      case "$output" in
        *"$EXPECTED_VERSION"*)
          echo "topologyfit-smoke scheduled by accelerator-scheduler onto $node_name; image reported $EXPECTED_VERSION"
          exit 0
          ;;
        *)
          echo "smoke image version output did not contain $EXPECTED_VERSION: $output" >&2
          exit 1
          ;;
      esac
      ;;
    Failed)
      kubectl describe pod topologyfit-smoke >&2 || true
      echo "smoke pod failed" >&2
      exit 1
      ;;
  esac
  attempt=$((attempt + 1))
  sleep 2
done

kubectl describe pod topologyfit-smoke >&2 || true
echo "smoke pod did not complete within 60 seconds" >&2
exit 1
