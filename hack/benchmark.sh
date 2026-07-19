#!/usr/bin/env sh
set -eu

./hack/go.sh test -run '^$' -bench 'Benchmark(TopologyFitFabricCliqueSelection|DRAClaimLifecycle)$' -benchmem -count="${BENCH_COUNT:-5}" ./pkg/plugin/topologyfit ./pkg/dra
