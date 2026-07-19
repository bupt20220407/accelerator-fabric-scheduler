#!/usr/bin/env sh
set -eu

kubectl wait --for=condition=Ready acceleratortopology --all --timeout=120s

count="$(kubectl get acceleratortopology -o name | wc -l | tr -d ' ')"
if [ "$count" != "3" ]; then
  kubectl get acceleratortopology -o wide >&2
  echo "expected 3 ready topology fixtures, got $count" >&2
  exit 1
fi

for node in accelerator-fabric-worker accelerator-fabric-worker2 accelerator-fabric-worker3; do
  heartbeat="$(kubectl get acceleratortopology "$node" -o jsonpath='{.status.lastHeartbeatTime}')"
  if [ -z "$heartbeat" ]; then
    echo "$node topology has no heartbeat" >&2
    exit 1
  fi
  echo "$node topology is Ready with heartbeat $heartbeat"
done
