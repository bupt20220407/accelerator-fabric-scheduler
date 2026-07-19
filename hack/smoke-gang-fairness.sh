#!/usr/bin/env sh
set -eu

rounds="${GANG_FAIRNESS_ROUNDS:-2}"
a_pods="w7-fair-a-0 w7-fair-a-1"
b_pods="w7-fair-b-0 w7-fair-b-1"

scale_scheduler() {
  replicas="$1"
  kubectl -n accelerator-system scale deployment/accelerator-scheduler --replicas="$replicas" >/dev/null
  kubectl -n accelerator-system rollout status deployment/accelerator-scheduler --timeout=180s >/dev/null
}

cleanup_round() {
  kubectl delete pod $a_pods $b_pods --ignore-not-found --wait=true >/dev/null 2>&1 || true
  kubectl delete podgroup w7-fair-a w7-fair-b --ignore-not-found --wait=true >/dev/null 2>&1 || true
}

cleanup() {
  cleanup_round
  scale_scheduler 1 >/dev/null 2>&1 || true
  kubectl delete acceleratorplacementpolicy w7-fairness-clique --ignore-not-found --wait=true >/dev/null 2>&1 || true
}
trap cleanup EXIT

kubectl apply -f config/experiments/gang-fairness-policy.yaml >/dev/null
round=1
while [ "$round" -le "$rounds" ]; do
  scale_scheduler 0
  cleanup_round
  kubectl apply -f config/experiments/gang-fairness-a.yaml >/dev/null
  sleep 1
  kubectl apply -f config/experiments/gang-fairness-b.yaml >/dev/null
  kubectl apply -f config/experiments/gang-fairness-pods.yaml >/dev/null
  scale_scheduler 1

  for pod in $a_pods; do
    kubectl wait --for=jsonpath='{.status.phase}'=Running "pod/$pod" --timeout=180s >/dev/null
  done
  for pod in $b_pods; do
    node="$(kubectl get pod "$pod" -o jsonpath='{.spec.nodeName}')"
    if [ -n "$node" ]; then
      echo "newer PodGroup member $pod was bound before the older group released capacity" >&2
      exit 1
    fi
  done
  a_started="$(kubectl get pod w7-fair-a-0 -o jsonpath='{.status.startTime}')"
  for pod in $a_pods; do
    kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "pod/$pod" --timeout=60s >/dev/null
  done
  for pod in $b_pods; do
    kubectl wait --for=jsonpath='{.status.phase}'=Running "pod/$pod" --timeout=180s >/dev/null
  done
  b_started="$(kubectl get pod w7-fair-b-0 -o jsonpath='{.status.startTime}')"
  for pod in $b_pods; do
    kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "pod/$pod" --timeout=60s >/dev/null
  done
  echo "fairness round $round/$rounds: older group A started at $a_started; newer group B started at $b_started after A released all eight devices"
  cleanup_round
  round=$((round + 1))
done

cleanup
trap - EXIT
