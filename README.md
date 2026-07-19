# Accelerator Fabric Scheduler

`Accelerator Fabric Scheduler` is an out-of-tree Kubernetes scheduler for topology-aware GPU/NPU placement. It models NUMA locality, PCIe roots, and accelerator fabrics such as NVLink, HCCS, and XGMI.

## Current milestone: W4b authoritative DRA handoff

The repository now provides a reproducible policy-to-scheduling path:

- a custom kube-scheduler binary registers `TopologyFit` and handles Pods with `schedulerName: accelerator-scheduler`;
- generated DeepCopy, clientset, fake client, listers, and shared informers cover both `scheduling.bupt.dev/v1alpha1` CRDs;
- an immutable graph snapshot and copy-on-write cache track Ready, generation, heartbeat, and TTL state;
- a controller validates topology objects and updates their status subresource;
- `PreFilter` resolves a namespaced placement policy into CycleState;
- `Filter` implements single-NUMA, same-PCIe-root, fabric-clique, fabric-connected, and best-effort modes;
- `Score` deterministically combines locality, fabric density, fragmentation, and headroom features;
- each topology mode returns a deterministic advisory device combination instead of only a boolean result;
- a concurrency-safe ledger atomically reserves device combinations across overlapping scheduling and binding cycles;
- `PreBind` verifies ownership, while `Unreserve` and `PostBind` provide failure and success cleanup paths;
- a native `resource.k8s.io/v1` DeviceClass and per-node ResourceSlices expose the same three-vendor fixture devices to Kubernetes DRA;
- a kubelet DRA v1 driver consumes authoritative ResourceClaim allocation results and rejects cross-claim device reuse;
- prepared claim ownership is persisted across driver restarts, then released idempotently by NodeUnprepareResources;
- CDI injects the exact allocated pool/device IDs into the consuming synthetic container;
- three kind workers publish eight synthetic NVIDIA, Ascend, or AMD resources through the kubelet Device Plugin API;
- end-to-end tests prove both real kubelet Allocate calls and a topology-specific four-card-pass/five-card-reject decision.

The DRA path now has an authoritative synthetic claim-to-kubelet identity contract. The legacy extended-resource/Device Plugin path remains advisory and is retained for comparison. Neither path discovers or exposes physical accelerators, and the project still does **not** implement gang scheduling or prove a real training-throughput improvement. The placement-policy CRD also does not yet generate DRA claims automatically; the DRA smoke uses explicit native selectors and a fabric-group constraint.

## Prerequisites

The recommended path is WSL2 or Linux with Docker, kubectl, kind v0.32.0, and make. Host Go is optional; `hack/go.sh` uses the pinned Go container when Go is not installed.

## Verify code

```bash
make verify
```

This runs formatting checks, `go vet`, unit tests, the race detector, and builds all project binaries. Regenerate API clients and deterministic fixture YAML with:

```bash
make generate
```

## Use a placement policy

Create an `AcceleratorPlacementPolicy` in the workload namespace, then annotate a Pod that requests exactly one extended accelerator resource:

```yaml
metadata:
  annotations:
    scheduling.bupt.dev/placement-policy: high-bandwidth-training
spec:
  schedulerName: accelerator-scheduler
  containers:
    - name: trainer
      resources:
        requests:
          nvidia.com/gpu: 4
```

See `config/samples/high-bandwidth-training-policy.yaml` for a complete policy. Pods without the annotation retain neutral `TopologyFit` behavior.

## Run the W4b end-to-end test

```bash
make e2e
```

The command creates a one-control-plane/three-worker kind cluster, builds and loads the image, installs the CRDs, and deploys the scheduler, topology controller, and synthetic Device Plugins. It verifies:

- server-side CRD validation, topology Ready conditions, and fresh heartbeats;
- eight published extended resources on each NVIDIA, Ascend, and AMD worker;
- successful two-device kubelet Allocate calls for all three vendor resource names;
- three node-scoped DRA ResourceSlice pools publish eight attributed devices each;
- a native ResourceClaim allocates four NVIDIA devices from one fabric group;
- kubelet NodePrepareResources creates CDI metadata and the container reads the exact claim UID and device IDs;
- deleting the active driver Pod proves persisted prepared-claim recovery before NodeUnprepareResources releases it;
- a four-device NVLink clique workload completes;
- a five-device request remains unbound because `TopologyFit` rejects it, even though the NVIDIA node has scalar capacity for eight devices;
- scheduler logs prove Reserve, PreBind verification, and PostBind release executed for the successful topology-aware Pod;
- an unannotated W1 compatibility Pod is still bound by the custom scheduler;
- the image reports `v1.35.5-accelerator.0.4.0`, preventing a stale `:dev` image from passing.

Clean up with `make kind-down`. The default kind image is pinned by digest in `versions.lock.md`.

## Repository map

```text
cmd/scheduler/                  custom kube-scheduler entry point
cmd/topology-controller/       topology status controller
cmd/synthetic-device-plugin/   kubelet Device Plugin entry point
cmd/synthetic-dra-driver/      kubelet DRA v1 driver and ResourceSlice publisher
cmd/synthetic-dra-workload/    CDI allocation acceptance workload
pkg/plugin/topologyfit/         policy, selection, scoring, and reservation ledger
pkg/apis/                       typed v1alpha1 API
pkg/generated/                  generated clients, listers, and informers
pkg/topology/                   graph snapshots and concurrent cache
pkg/deviceplugin/               synthetic Device Plugin service
pkg/dra/                        DRA resources, persistent Prepare state, and CDI handoff
pkg/fixtures/                   deterministic three-vendor topology models
config/crd/                     v1alpha1 API schemas
config/fixtures/                generated three-worker topologies
config/smoke/                   scheduler, Allocate, and topology acceptance Pods
deploy/base/                    scheduler, controller, Device Plugins, and RBAC
docs/adr/                       architecture decisions and scope boundaries
hack/                           containerized Go and kind automation
```

## W4b acceptance criteria

- `make verify` and `make e2e` pass from a clean checkout.
- generated API code and fixture YAML are reproducible.
- all five topology modes and both availability failure policies have unit coverage.
- concurrent reservations are disjoint, atomic, race-free, idempotent, and rollback-safe.
- DRA publishes 24 attributed devices across three node-scoped pools.
- a four-device ResourceClaim allocation remains inside one NVIDIA fabric group.
- ResourceClaim allocation IDs, NodePrepare response IDs, and container CDI IDs agree.
- a driver restart recovers prepared ownership and the replacement handles Unprepare.
- scheduler informers start and synchronize under read-only CRD RBAC.
- the topology-specific positive and negative scheduling cases pass.
- a real scheduler cycle emits Reserve, PreBind, and PostBind lifecycle evidence.
- the default scheduler is not modified or replaced.
- all Kubernetes modules remain on the locked v1.35.5/v0.35.5 baseline.
- documentation does not imply exact device reservation or physical accelerator testing.

## Next milestone: W5

W5 should reconcile `AcceleratorTopology` into ResourceSlices and translate `AcceleratorPlacementPolicy` into native ResourceClaim selectors and constraints, removing the duplicated fixture/policy configuration. It should then add health-change fault injection, allocation latency benchmarks, and multi-Pod fragmentation experiments. Gang scheduling remains a separate later milestone.

## Repository visibility and license

The project is intended to remain private during early implementation. No open-source license is granted by this repository at W4b; choose a license deliberately before making it public.
