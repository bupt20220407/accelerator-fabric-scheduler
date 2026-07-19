#!/usr/bin/env sh
set -eu

if command -v go >/dev/null 2>&1; then
  exec bash ./hack/update-codegen.sh
fi

GO_IMAGE="${GO_IMAGE:-golang:1.25.5-bookworm@sha256:d9132cce84391efab786495288756d60e1da215b1f94e87860aeefc3d4c45b6d}"

exec docker run --rm \
  -e GOCACHE=/cache/build \
  -e GOMODCACHE=/cache/mod \
  -v accelerator-fabric-scheduler-go-cache:/cache \
  -v "$(pwd):/workspace" \
  -w /workspace \
  "$GO_IMAGE" bash ./hack/update-codegen.sh
