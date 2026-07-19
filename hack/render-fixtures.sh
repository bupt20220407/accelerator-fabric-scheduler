#!/usr/bin/env sh
set -eu

mkdir -p config/fixtures
temporary="config/fixtures/topologies.generated.yaml.tmp"
./hack/go.sh run ./cmd/fixture-generator > "$temporary"
mv "$temporary" config/fixtures/topologies.generated.yaml
