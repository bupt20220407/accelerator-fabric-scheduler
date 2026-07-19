# W6 kind baseline - 2026-07-19

## Environment

- Project version: `v1.35.5-accelerator.0.6.0`
- Host CPU exposed to WSL: Intel Core i7-10750H, 12 logical CPUs
- Go toolchain: `go1.25.5 linux/amd64`
- Cluster: kind v0.32.0, Kubernetes v1.35.5, one control plane and three workers
- Accelerator inventory: synthetic NVIDIA H100-SXM topology, two four-device cliques

## Go microbenchmarks

Command: `BENCH_COUNT=5 make benchmark`

| Benchmark | Five-run range | Memory | Allocations | Measured path |
|---|---:|---:|---:|---|
| `TopologyFitFabricCliqueSelection` | 870.9-1028 ns/op | 496 B/op | 9 allocs/op | Deterministic four-of-eight clique selection |
| `DRAClaimLifecycle` | 337.3-421.7 us/op | 7.997-8.028 KiB/op | 75 allocs/op | Core prepare+unprepare including CDI and atomic state persistence |

The DRA microbenchmark intentionally excludes the public method's audit logging and batch wrapper. Those paths remain covered by unit and kind tests.

## Kind allocation latency

Command: `BENCH_ITERATIONS=10 make benchmark-allocation`

Latency starts before Pod creation and stops when the Pod is Running after ResourceClaim allocation, kubelet NodePrepare, CDI injection, and container start.

| Iteration | Latency (ms) | Node | Devices |
|---:|---:|---|---|
| 1 | 915 | `accelerator-fabric-worker` | `gpu0,gpu1,gpu2,gpu3` |
| 2 | 1545 | `accelerator-fabric-worker` | `gpu0,gpu1,gpu2,gpu3` |
| 3 | 1537 | `accelerator-fabric-worker` | `gpu0,gpu1,gpu2,gpu3` |
| 4 | 1550 | `accelerator-fabric-worker` | `gpu0,gpu1,gpu2,gpu3` |
| 5 | 1521 | `accelerator-fabric-worker` | `gpu0,gpu1,gpu2,gpu3` |
| 6 | 1581 | `accelerator-fabric-worker` | `gpu0,gpu1,gpu2,gpu3` |
| 7 | 1558 | `accelerator-fabric-worker` | `gpu0,gpu1,gpu2,gpu3` |
| 8 | 1526 | `accelerator-fabric-worker` | `gpu0,gpu1,gpu2,gpu3` |
| 9 | 1558 | `accelerator-fabric-worker` | `gpu0,gpu1,gpu2,gpu3` |
| 10 | 1546 | `accelerator-fabric-worker` | `gpu0,gpu1,gpu2,gpu3` |

Summary: min 915 ms, p50 1545 ms, p95 1581 ms, max 1581 ms. The first sample differs materially from the remaining samples, so raw values are retained and no universal threshold is inferred.

## Fragmentation experiment

Command: `FRAGMENTATION_ROUNDS=2 make fragmentation-smoke`

Both rounds allocated four concurrent two-device claims across eight unique devices. Every claim remained within one four-device clique. A fifth two-device claim remained unallocated while the pool was full, and all generated claims were garbage collected after Pod deletion.

## Interpretation boundary

This report is a reproducibility baseline for one local kind environment. It does not measure physical GPU/NPU discovery, vendor driver calls, kernel execution, collective communication, multi-node training throughput, control-plane scale, or production SLOs.
