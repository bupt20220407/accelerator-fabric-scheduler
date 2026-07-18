# Accelerator Fabric Scheduler

`Accelerator Fabric Scheduler` is an out-of-tree Kubernetes scheduler for topology-aware GPU/NPU placement. The long-term design models NUMA locality, PCIe roots, and accelerator fabrics such as NVLink, HCCS, and XGMI.

## Current milestone: W1 skeleton

The repository currently proves the scheduler integration path, not topology-aware behavior:

- a custom kube-scheduler binary registers `TopologyFit`;
- a dedicated profile handles Pods with `schedulerName: accelerator-scheduler`;
- the W1 plugin implements neutral lifecycle hooks for PreFilter, Filter, PreScore, Score, Reserve, Unreserve, and PreBind;
- a kind cluster and smoke test verify that a Pod is bound by the custom scheduler;
- draft `AcceleratorTopology` and `AcceleratorPlacementPolicy` CRDs establish the W2 contract.

W1 does **not** select GPU/NPU device IDs, filter by topology, rank fabric locality, publish synthetic accelerator resources, or provide production availability. Those capabilities must be backed by later code and tests before they are claimed.

## Prerequisites

The recommended path is WSL2 or Linux with:

- Docker;
- kubectl;
- kind v0.32.0;
- make.

Host Go is optional. `hack/go.sh` uses the pinned Go container when `go` is not installed.

## Verify code

```bash
make verify
```

This runs formatting checks, `go vet`, unit tests, and a scheduler build.

## Run the W1 end-to-end smoke test

```bash
make e2e
```

The command creates a one-control-plane/three-worker kind cluster, builds and loads the scheduler image, installs the CRDs, verifies valid and invalid CRD samples against the API server, deploys the custom scheduler, and submits `topologyfit-smoke`. Success prints the worker selected for the Pod.

The smoke container also runs `accelerator-scheduler --version` and requires `v1.35.5-accelerator.0.1.0`, preventing a stale `:dev` image from passing acceptance.

Clean up with:

```bash
make kind-down
```

The default kind image is pinned by digest in `versions.lock.md`. Override `KIND_NODE_IMAGE` only in a dedicated version-upgrade test.

## Repository map

```text
cmd/scheduler/                  custom kube-scheduler entry point
pkg/plugin/topologyfit/         Scheduling Framework plugin skeleton
config/crd/                     W2 API contract drafts
config/kind/                    reproducible local cluster
config/scheduler/               standalone scheduler profile
config/smoke/                   W1 acceptance Pod
config/samples/                 API-server-validated CRD examples
deploy/base/                    RBAC, config, and scheduler Deployment
docs/adr/                       architecture decisions
hack/                           containerized Go and kind automation
```

## W1 acceptance criteria

- `make verify` passes from a clean checkout.
- `make e2e` schedules `topologyfit-smoke` with `accelerator-scheduler`.
- API server validation accepts the sample topology/policy and rejects weights that do not sum to 100.
- The default scheduler is not modified or replaced.
- All Kubernetes modules remain on the locked v1.35.5/v0.35.5 baseline.
- README language stays within the W1 boundary above.

## Next milestone: W2

W2 introduces generated Go types and clients for the two CRDs, a fixture-backed topology cache, three synthetic node topologies, freshness/health validation, and unit tests for graph invariants. Filter/Score logic starts only after that cache is deterministic and typed.

## Repository visibility and license

The project is intended to remain private during early implementation. No open-source license is granted by this repository at W1; choose a license deliberately before making it public.
