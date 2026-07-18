#!/usr/bin/env sh
set -eu

kind delete cluster --name "${KIND_CLUSTER_NAME:-accelerator-fabric}"
