#!/usr/bin/env sh
set -eu

iterations="${BENCH_ITERATIONS:-10}"
pod="w6-allocation-benchmark"
policy="w6-allocation-benchmark"
temporary="$(mktemp -d)"
csv="$temporary/allocation-latency.csv"
durations="$temporary/durations"

cleanup() {
  kubectl delete pod "$pod" --ignore-not-found --wait=true >/dev/null 2>&1 || true
  kubectl delete acceleratorplacementpolicy "$policy" --ignore-not-found --wait=true >/dev/null 2>&1 || true
  rm -rf "$temporary"
}
trap cleanup EXIT

kubectl delete pod "$pod" --ignore-not-found --wait=true >/dev/null
kubectl apply -f config/experiments/dra-allocation-benchmark.yaml >/dev/null
attempt=0
while ! kubectl get resourceclaimtemplate "$policy" >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    echo "benchmark ResourceClaimTemplate did not converge" >&2
    exit 1
  fi
  sleep 2
done

printf 'iteration,latency_ms,node,devices\n' > "$csv"
: > "$durations"
iteration=1
while [ "$iteration" -le "$iterations" ]; do
  start_ns="$(date +%s%N)"
  kubectl apply -f config/experiments/dra-allocation-benchmark.yaml >/dev/null
  kubectl wait --for=jsonpath='{.status.phase}'=Running "pod/$pod" --timeout=180s >/dev/null
  end_ns="$(date +%s%N)"
  latency_ms=$(( (end_ns - start_ns) / 1000000 ))
  claim="$(kubectl get pod "$pod" -o jsonpath='{.status.resourceClaimStatuses[0].resourceClaimName}')"
  node="$(kubectl get pod "$pod" -o jsonpath='{.spec.nodeName}')"
  devices="$(kubectl get resourceclaim "$claim" -o jsonpath='{range .status.allocation.devices.results[*]}{.device}{"+"}{end}')"
  printf '%s,%s,%s,%s\n' "$iteration" "$latency_ms" "$node" "${devices%+}" >> "$csv"
  printf '%s\n' "$latency_ms" >> "$durations"
  kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "pod/$pod" --timeout=30s >/dev/null
  kubectl delete pod "$pod" --wait=true >/dev/null
  attempt=0
  while kubectl get resourceclaim "$claim" >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
      echo "benchmark claim $claim was not garbage collected" >&2
      exit 1
    fi
    sleep 1
  done
  iteration=$((iteration + 1))
done

sorted="$temporary/sorted"
sort -n "$durations" > "$sorted"
p50_index=$(( (iterations + 1) / 2 ))
p95_index=$(( (iterations * 95 + 99) / 100 ))
minimum="$(sed -n '1p' "$sorted")"
p50="$(sed -n "${p50_index}p" "$sorted")"
p95="$(sed -n "${p95_index}p" "$sorted")"
maximum="$(sed -n "${iterations}p" "$sorted")"
cat "$csv"
echo "summary: samples=$iterations min=${minimum}ms p50=${p50}ms p95=${p95}ms max=${maximum}ms" >&2

if [ -n "${REPORT_PATH:-}" ]; then
  mkdir -p "$(dirname "$REPORT_PATH")"
  cp "$csv" "$REPORT_PATH"
  echo "wrote raw benchmark samples to $REPORT_PATH" >&2
fi

cleanup
trap - EXIT
