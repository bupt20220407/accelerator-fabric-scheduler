#!/usr/bin/env sh
set -eu

kubectl apply --dry-run=server -f config/samples >/dev/null
kubectl apply --dry-run=server -f config/fixtures >/dev/null

if kubectl apply --dry-run=server -f test/fixtures/invalid-policy-weights.yaml >/dev/null 2>&1; then
  echo "invalid policy weights unexpectedly passed API server validation" >&2
  exit 1
fi

if kubectl apply --dry-run=server -f test/fixtures/invalid-policy-dra-connected.yaml >/dev/null 2>&1; then
  echo "unsupported DRA fabric-connected policy unexpectedly passed API server validation" >&2
  exit 1
fi

if kubectl apply --dry-run=server -f test/fixtures/invalid-podgroup-min-member.yaml >/dev/null 2>&1; then
  echo "zero-member PodGroup unexpectedly passed API server validation" >&2
  exit 1
fi

echo "CRD validation accepted valid samples and rejected invalid weights/DRA mode/PodGroup size"
