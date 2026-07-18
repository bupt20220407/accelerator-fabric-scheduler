#!/usr/bin/env sh
set -eu

CLUSTER_NAME="${KIND_CLUSTER_NAME:-accelerator-fabric}"
NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node@sha256:ce977ae6d65918d0b58a5f8b5e940429c2ce42fa3a5619ec2bbc60b949c0ac95}"

if kind get clusters 2>/dev/null | grep -Fx "$CLUSTER_NAME" >/dev/null 2>&1; then
  echo "kind cluster $CLUSTER_NAME already exists"
  exit 0
fi

kind create cluster \
  --name "$CLUSTER_NAME" \
  --image "$NODE_IMAGE" \
  --config config/kind/cluster.yaml \
  --wait 180s
