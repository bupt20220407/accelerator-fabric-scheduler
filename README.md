# Accelerator Fabric Scheduler

`Accelerator Fabric Scheduler` is an out-of-tree Kubernetes scheduler for topology-aware GPU/NPU placement. It models NUMA locality, PCIe roots, and accelerator fabrics such as NVLink, HCCS, and XGMI.

## Current milestone: W3 topology-aware node placement

The repository now provides a reproducible policy-to-scheduling path:

- a custom kube-scheduler binary registers `TopologyFit` and handles Pods with `schedulerName: accelerator-scheduler`;
- generated DeepCopy, clientset, fake client, listers, and shared informers cover both `scheduling.bupt.dev/v1alpha1` CRDs;
- an immutable graph snapshot and copy-on-write cache track Ready, generation, heartbeat, and TTL state;
- a controller validates topology objects and updates their status subresource;
- `PreFilter` resolves a namespaced placement policy into CycleState;
- `Filter` implements single-NUMA, same-PCIe-root, fabric-clique, fabric-connected, and best-effort modes;
- `Score` deterministically combines locality, fabric density, fragmentation, and headroom features;
- three kind workers publish eight synthetic NVIDIA, Ascend, or AMD resources through the kubelet Device Plugin API;
- end-to-end tests prove both real kubelet Allocate calls and a topology-specific four-card-pass/five-card-reject decision.

W3 is node-level placement. It does **not** reserve a concrete device combination, guarantee that kubelet allocates the same IDs evaluated by the scheduler, implement gang scheduling, use DRA, or prove a real training-throughput improvement. Synthetic IDs demonstrate the protocol path only; they are not evidence of physical GPU/NPU allocation.

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

## Run the W3 end-to-end test

```bash
make e2e
```

The command creates a one-control-plane/three-worker kind cluster, builds and loads the image, installs the CRDs, and deploys the scheduler, topology controller, and synthetic Device Plugins. It verifies:

- server-side CRD validation, topology Ready conditions, and fresh heartbeats;
- eight published extended resources on each NVIDIA, Ascend, and AMD worker;
- successful two-device kubelet Allocate calls for all three vendor resource names;
- a four-device NVLink clique workload completes;
- a five-device request remains unbound because `TopologyFit` rejects it, even though the NVIDIA node has scalar capacity for eight devices;
- an unannotated W1 compatibility Pod is still bound by the custom scheduler;
- the image reports `v1.35.5-accelerator.0.2.0`, preventing a stale `:dev` image from passing.

Clean up with `make kind-down`. The default kind image is pinned by digest in `versions.lock.md`.

## Repository map

```text
cmd/scheduler/                  custom kube-scheduler entry point
cmd/topology-controller/       topology status controller
cmd/synthetic-device-plugin/   kubelet Device Plugin entry point
pkg/plugin/topologyfit/         W3 policy, Filter, Score, and CycleState logic
pkg/apis/                       typed v1alpha1 API
pkg/generated/                  generated clients, listers, and informers
pkg/topology/                   graph snapshots and concurrent cache
pkg/deviceplugin/               synthetic Device Plugin service
pkg/fixtures/                   deterministic three-vendor topology models
config/crd/                     v1alpha1 API schemas
config/fixtures/                generated three-worker topologies
config/smoke/                   scheduler, Allocate, and topology acceptance Pods
deploy/base/                    scheduler, controller, Device Plugins, and RBAC
docs/adr/                       architecture decisions and scope boundaries
hack/                           containerized Go and kind automation
```

## W3 acceptance criteria

- `make verify` and `make e2e` pass from a clean checkout.
- generated API code and fixture YAML are reproducible.
- all five topology modes and both availability failure policies have unit coverage.
- scheduler informers start and synchronize under read-only CRD RBAC.
- the topology-specific positive and negative scheduling cases pass.
- the default scheduler is not modified or replaced.
- all Kubernetes modules remain on the locked v1.35.5/v0.35.5 baseline.
- documentation does not imply exact device reservation or physical accelerator testing.

## Next milestone: W4

W4 should add a concurrency-safe device-combination reservation ledger and an explicit allocation handoff. DRA is the preferred production-aligned path; a narrower vendor/device-plugin bridge can be used only if its identity and rollback guarantees are documented and tested. Gang scheduling and performance experiments should remain separate milestones.

## Repository visibility and license

The project is intended to remain private during early implementation. No open-source license is granted by this repository at W3; choose a license deliberately before making it public.
