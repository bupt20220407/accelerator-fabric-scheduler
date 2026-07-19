#!/usr/bin/env sh
set -eu

CLUSTER_NAME="${KIND_CLUSTER_NAME:-accelerator-fabric}"
IMAGE="${SCHEDULER_IMAGE:-accelerator-fabric-scheduler:dev}"
SCHEDULER_VERSION="${SCHEDULER_VERSION:-v1.35.5-accelerator.0.2.0}"

docker build --build-arg SCHEDULER_VERSION="$SCHEDULER_VERSION" -t "$IMAGE" .
kind load docker-image --name "$CLUSTER_NAME" "$IMAGE"
kubectl apply -f config/crd
kubectl apply -f deploy/base
kubectl -n accelerator-system rollout restart deployment/accelerator-scheduler
kubectl -n accelerator-system rollout status deployment/accelerator-scheduler --timeout=180s
kubectl -n accelerator-system rollout restart deployment/accelerator-topology-controller
kubectl -n accelerator-system rollout status deployment/accelerator-topology-controller --timeout=180s
for daemonset in synthetic-nvidia-device-plugin synthetic-ascend-device-plugin synthetic-amd-device-plugin; do
  kubectl -n accelerator-system rollout restart "daemonset/${daemonset}"
  kubectl -n accelerator-system rollout status "daemonset/${daemonset}" --timeout=180s
done
kubectl apply -f config/fixtures/topologies.generated.yaml
