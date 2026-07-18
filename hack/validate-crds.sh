#!/usr/bin/env sh
set -eu

kubectl apply --dry-run=server -f config/samples >/dev/null

if kubectl apply --dry-run=server -f test/fixtures/invalid-policy-weights.yaml >/dev/null 2>&1; then
  echo "invalid policy weights unexpectedly passed API server validation" >&2
  exit 1
fi

echo "CRD validation accepted valid samples and rejected invalid weights"
