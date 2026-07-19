# Accelerator Fabric Scheduler

`Accelerator Fabric Scheduler` is an out-of-tree Kubernetes scheduler for topology-aware GPU/NPU placement. The long-term design models NUMA locality, PCIe roots, and accelerator fabrics such as NVLink, HCCS, and XGMI.

## Current milestone: W2 topology data plane

The repository now proves the scheduler integration and topology data plane:

- a custom kube-scheduler binary registers `TopologyFit`;
- a dedicated profile handles Pods with `schedulerName: accelerator-scheduler`;
- generated DeepCopy, clientset, fake client, listers, and shared informers cover both v1alpha1 CRDs;
- a validated immutable graph snapshot indexes devices by ID, NUMA, PCIe root, fabric group, and adjacency;
- a copy-on-write cache provides lock-free reads with Ready, generation, heartbeat, and TTL semantics;
- a controller validates topology objects and writes Ready conditions and heartbeats through the status subresource;
- three kind workers publish eight synthetic NVIDIA, Ascend, or AMD extended resources through the kubelet Device Plugin API;
- end-to-end allocation Pods prove kubelet calls Allocate and injects stable synthetic device IDs.

`TopologyFit` still has neutral Filter and Score behavior. W2 does **not** use the topology cache to select a node, reserve a device combination, allocate real vendor hardware, or provide production availability. Synthetic device IDs prove the protocol path only; they are not physical GPU/NPU allocation evidence.

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

This runs formatting checks, `go vet`, unit tests, the race detector, and builds all project binaries.

Regenerate API clients and deterministic fixture YAML with:

```bash
make generate
```

## Run the W2 end-to-end smoke test

```bash
make e2e
```

The command creates a one-control-plane/three-worker kind cluster, builds and loads the image, installs the CRDs, deploys the custom scheduler and topology controller, and starts one synthetic Device Plugin per worker. It then verifies:

- valid CRDs are accepted and invalid weights are rejected;
- all three topology fixtures become Ready and receive heartbeats;
- the NVIDIA, Ascend, and AMD workers each publish eight extended resources;
- three Pods request two devices each and receive resource names plus synthetic device IDs from Allocate;
- the W1 custom scheduler binding smoke still passes.

The smoke container also runs `accelerator-scheduler --version` and requires `v1.35.5-accelerator.0.1.0`, preventing a stale `:dev` image from passing acceptance.

Clean up with:

```bash
make kind-down
```

The default kind image is pinned by digest in `versions.lock.md`. Override `KIND_NODE_IMAGE` only in a dedicated version-upgrade test.

## Repository map

```text
cmd/scheduler/                  custom kube-scheduler entry point
cmd/topology-controller/       topology status controller
cmd/synthetic-device-plugin/   kubelet Device Plugin entry point
pkg/plugin/topologyfit/         Scheduling Framework plugin skeleton
pkg/apis/                       typed v1alpha1 API
pkg/generated/                  generated clients, listers, and informers
pkg/topology/                   graph snapshots and concurrent cache
pkg/deviceplugin/               synthetic Device Plugin service
pkg/fixtures/                   deterministic three-vendor topology models
config/crd/                     v1alpha1 API schemas
config/fixtures/                generated three-worker topologies
config/kind/                    reproducible local cluster
config/scheduler/               standalone scheduler profile
config/smoke/                   scheduler and Allocate acceptance Pods
config/samples/                 API-server-validated CRD examples
deploy/base/                    scheduler, controller, Device Plugins, and RBAC
docs/adr/                       architecture decisions
hack/                           containerized Go and kind automation
```

## W2 acceptance criteria

- `make verify` passes from a clean checkout.
- generated API code and fixture YAML are reproducible.
- API server validation accepts the sample topology/policy and rejects weights that do not sum to 100.
- all topology fixtures become Ready with fresh heartbeats.
- each fixture worker publishes exactly eight vendor-specific extended resources.
- Allocate succeeds for two devices on each vendor and injects deterministic synthetic IDs.
- `make e2e` still schedules `topologyfit-smoke` with `accelerator-scheduler`.
- The default scheduler is not modified or replaced.
- All Kubernetes modules remain on the locked v1.35.5/v0.35.5 baseline.
- README language stays within the W2 boundary above.

## Next milestone: W3

W3 integrates the generated informers and topology store into `TopologyFit`, parses Pod policy/resource requests into CycleState, implements the five Filter modes, and adds deterministic Score features. It remains node-level placement until DRA closes the device-ID allocation loop.

## Repository visibility and license

The project is intended to remain private during early implementation. No open-source license is granted by this repository at W2; choose a license deliberately before making it public.
