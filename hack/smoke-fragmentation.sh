#!/usr/bin/env sh
set -eu

rounds="${FRAGMENTATION_ROUNDS:-3}"
pods="w6-fragmentation-0 w6-fragmentation-1 w6-fragmentation-2 w6-fragmentation-3"
overflow="w6-fragmentation-overflow"
policy="w6-fragmentation"
temporary="$(mktemp -d)"

cleanup() {
  kubectl delete pod $pods "$overflow" --ignore-not-found --wait=true >/dev/null 2>&1 || true
  kubectl delete acceleratorplacementpolicy "$policy" --ignore-not-found --wait=true >/dev/null 2>&1 || true
  rm -rf "$temporary"
}
trap cleanup EXIT

kubectl apply -f config/experiments/dra-fragmentation-policy.yaml >/dev/null
attempt=0
while ! kubectl get resourceclaimtemplate "$policy" >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    echo "fragmentation ResourceClaimTemplate did not converge" >&2
    exit 1
  fi
  sleep 2
done

round=1
while [ "$round" -le "$rounds" ]; do
  kubectl delete pod $pods "$overflow" --ignore-not-found --wait=true >/dev/null
  kubectl apply -f config/experiments/dra-fragmentation-pods.yaml >/dev/null
  claims=""
  device_file="$temporary/devices-$round"
  : > "$device_file"
  for pod in $pods; do
    kubectl wait --for=jsonpath='{.status.phase}'=Running "pod/$pod" --timeout=180s >/dev/null
    claim="$(kubectl get pod "$pod" -o jsonpath='{.status.resourceClaimStatuses[0].resourceClaimName}')"
    claims="$claims $claim"
    devices="$(kubectl get resourceclaim "$claim" -o jsonpath='{range .status.allocation.devices.results[*]}{.device}{" "}{end}')"
    set -- $devices
    if [ "$#" -ne 2 ]; then
      echo "$pod received $# devices, want 2: $devices" >&2
      exit 1
    fi
    case "$1:$2" in
      gpu[0-3]:gpu[0-3]|gpu[4-7]:gpu[4-7]) ;;
      *)
        echo "$pod crossed fabric groups: $devices" >&2
        exit 1
        ;;
    esac
    printf '%s\n' "$1" "$2" >> "$device_file"
  done
  unique="$(sort -u "$device_file" | wc -l | tr -d ' ')"
  if [ "$unique" -ne 8 ]; then
    echo "round $round allocated $unique unique devices across four claims, want 8" >&2
    exit 1
  fi

  kubectl apply -f config/experiments/dra-fragmentation-overflow.yaml >/dev/null
  sleep 5
  overflow_node="$(kubectl get pod "$overflow" -o jsonpath='{.spec.nodeName}')"
  overflow_claim="$(kubectl get pod "$overflow" -o jsonpath='{.status.resourceClaimStatuses[0].resourceClaimName}')"
  overflow_allocation="$(kubectl get resourceclaim "$overflow_claim" -o jsonpath='{.status.allocation}' 2>/dev/null || true)"
  if [ -n "$overflow_node" ] || [ -n "$overflow_allocation" ]; then
    echo "overflow claim unexpectedly allocated while all eight devices were occupied" >&2
    exit 1
  fi
  kubectl delete pod "$overflow" --wait=true >/dev/null
  kubectl delete pod $pods --wait=true >/dev/null
  for claim in $claims; do
    attempt=0
    while kubectl get resourceclaim "$claim" >/dev/null 2>&1; do
      attempt=$((attempt + 1))
      if [ "$attempt" -ge 60 ]; then
        echo "claim $claim was not garbage collected" >&2
        exit 1
      fi
      sleep 1
    done
  done
  echo "fragmentation round $round/$rounds packed four two-device claims into two cliques; overflow remained Pending"
  round=$((round + 1))
done

cleanup
trap - EXIT
