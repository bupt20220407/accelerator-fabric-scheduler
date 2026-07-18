#!/usr/bin/env sh
set -eu

CLUSTER_NAME="${KIND_CLUSTER_NAME:-accelerator-fabric}"
IMAGE="${SCHEDULER_IMAGE:-accelerator-fabric-scheduler:dev}"
SCHEDULER_VERSION="${SCHEDULER_VERSION:-v1.35.5-accelerator.0.1.0}"

docker build --build-arg SCHEDULER_VERSION="$SCHEDULER_VERSION" -t "$IMAGE" .
kind load docker-image --name "$CLUSTER_NAME" "$IMAGE"
kubectl apply -f config/crd
kubectl apply -f deploy/base/scheduler.yaml
kubectl -n accelerator-system rollout restart deployment/accelerator-scheduler
kubectl -n accelerator-system rollout status deployment/accelerator-scheduler --timeout=180s
