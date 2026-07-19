# ADR-0007: Observable and repeatable scheduling experiments

## Status

Accepted for W6.

## Context

W5 proves policy/topology reconciliation and an authoritative synthetic DRA handoff, but logs and one-shot smoke cases are insufficient for performance work. The project needs bounded operational signals and experiments that another engineer can rerun without claiming that kind fixtures represent physical accelerator performance.

## Decision

- Register low-cardinality alpha Prometheus metrics through the Kubernetes component-base registry.
- Export TopologyFit operation count/latency and active advisory reservations through the scheduler's authenticated HTTPS endpoint.
- Export controller reconciliation count/latency and node-local DRA publish/prepare/unprepare count/latency, prepared claims, and published devices through Pod-local HTTP endpoints.
- Keep controller and DRA metrics without a Service; production exposure requires authentication and NetworkPolicy.
- Add Go microbenchmarks for clique selection and the persisted DRA claim lifecycle.
- Add a sequential kind allocation-latency harness that emits raw CSV samples without enforcing a machine-specific latency threshold.
- Add a repeated four-claim packing experiment that verifies eight unique devices, clique locality, cleanup, and an unallocated overflow claim.
- Maintain a fault matrix that distinguishes automated evidence from planned production tests.

## Consequences

W6 can detect regressions, quantify fixture-path cost, and demonstrate repeatable concurrency behavior. Metric labels do not include Pod, claim, policy, node, or device identities, avoiding unbounded series. Results remain synthetic: they do not measure vendor driver latency, real collectives, multi-node training throughput, or production control-plane scale.
