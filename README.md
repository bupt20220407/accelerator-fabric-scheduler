# Accelerator Fabric Scheduler

`Accelerator Fabric Scheduler` is an out-of-tree Kubernetes scheduler for topology-aware GPU/NPU placement. It models NUMA locality, PCIe roots, and accelerator fabrics such as NVLink, HCCS, and XGMI.

## Current milestone: W7 topology-aware gang scheduling

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
- each DRA driver watches the typed AcceleratorTopology API and publishes ResourceSlices only from a current Ready topology;
- topology links are reduced into conservative per-device fabric bandwidth and hop attributes;
- optional `AcceleratorPlacementPolicy.spec.dra.deviceCount` creates a same-name, owner-managed ResourceClaimTemplate;
- policy vendor, product, memory, health, locality, bandwidth, and hop constraints become native DRA CEL/matchAttribute rules;
- low-cardinality Prometheus metrics cover TopologyFit operations, controller reconciliations, and DRA publish/prepare/unprepare paths;
- readiness probes and directly testable metrics endpoints cover the scheduler, topology controller, and every node-local DRA driver;
- repeatable microbenchmarks, kind allocation-latency samples, and multi-claim fragmentation rounds produce machine-readable evidence;
- the scheduler-plugins `Coscheduling` beta plugin adds PodGroup queue ordering, PreFilter admission, Permit waiting, and group rollback;
- a version-matched `scheduling.x-k8s.io/v1alpha1` PodGroup CRD is installed with read-only scheduler RBAC;
- gang tests combine Coscheduling with TopologyFit to allocate two disjoint four-device cliques and prove timeout cleanup;
- competing equal-priority groups verify that older PodGroup creation time takes precedence even when the newer group's Pods are created first;
- three kind workers publish eight synthetic NVIDIA, Ascend, or AMD resources through the kubelet Device Plugin API;
- end-to-end tests prove both real kubelet Allocate calls and a topology-specific four-card-pass/five-card-reject decision.

The DRA path has an authoritative synthetic claim-to-kubelet identity contract, while the legacy extended-resource/Device Plugin path remains advisory. W7 adds gang admission around scheduling cycles, but it does not turn the legacy advisory device IDs into an authoritative kubelet handoff. Pods must still reference generated DRA templates explicitly. Automatic DRA translation intentionally excludes `fabric-connected` and unknown-topology policies. The project does not discover physical accelerators or prove a real training-throughput improvement.

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

## Run the W7 end-to-end test

```bash
make e2e
```

The command creates a one-control-plane/three-worker kind cluster, builds and loads the image, installs the CRDs, and deploys the scheduler, topology controller, and synthetic Device Plugins. It verifies:

- server-side CRD validation, topology Ready conditions, and fresh heartbeats;
- eight published extended resources on each NVIDIA, Ascend, and AMD worker;
- successful two-device kubelet Allocate calls for all three vendor resource names;
- three node-scoped DRA ResourceSlice pools publish eight attributed devices each;
- a topology health fault propagates into the NVIDIA ResourceSlice;
- a PlacementPolicy generates a four-device ResourceClaimTemplate;
- the generated claim excludes the unhealthy clique and allocates `gpu4` through `gpu7`;
- kubelet NodePrepareResources creates CDI metadata and the container reads the exact claim UID and device IDs;
- deleting the active driver Pod proves persisted prepared-claim recovery before NodeUnprepareResources releases it;
- a four-device NVLink clique workload completes;
- a five-device request remains unbound because `TopologyFit` rejects it, even though the NVIDIA node has scalar capacity for eight devices;
- scheduler logs prove Reserve, PreBind verification, and PostBind release executed for the successful topology-aware Pod;
- two repeated fragmentation rounds pack four concurrent two-device claims into eight unique, clique-local devices and leave an overflow claim unallocated;
- two four-device PodGroup members pass Permit together and occupy disjoint NVIDIA cliques;
- an incomplete 4+5-device gang times out, triggers TopologyFit Unreserve, and leaves capacity available to a recovery Pod;
- two fairness rounds admit the older equal-priority PodGroup before a newer group whose Pods were created first;
- scheduler, controller, and DRA health/Prometheus endpoints expose expected operation and state signals;
- upstream scheduler framework metrics record nonzero Coscheduling evaluations, while gang logs prove Permit waiting;
- an unannotated W1 compatibility Pod is still bound by the custom scheduler;
- cleanup restores both AcceleratorTopology and ResourceSlice health to Healthy;
- the image reports `v1.35.5-accelerator.0.7.0`, preventing a stale `:dev` image from passing.

Clean up with `make kind-down`. The default kind image is pinned by digest in `versions.lock.md`.

## Run performance, fragmentation, and gang experiments

Run stable code-path microbenchmarks with:

```bash
make benchmark
```

Measure sequential Pod creation through DRA allocation, NodePrepare, CDI injection, and workload start in the current kind environment with:

```bash
BENCH_ITERATIONS=10 REPORT_PATH=evidence/allocation-latency.csv make benchmark-allocation
```

The command prints raw CSV plus min/p50/p95/max. It deliberately has no pass/fail latency threshold because host load and kind control-plane timing vary. Run repeated concurrent packing independently with `FRAGMENTATION_ROUNDS=10 make fragmentation-smoke`. These results characterize the fixed synthetic fixture; they are not GPU/NPU kernel, collective, or training-throughput benchmarks.

The checked-in W6 reference run is in `docs/reports/w6-kind-baseline-20260719.md` and retains all raw kind samples.

Run gang correctness and equal-priority admission ordering independently with:

```bash
make gang-smoke
GANG_FAIRNESS_ROUNDS=10 make gang-fairness-smoke
```

The checked-in W7 result is in `docs/reports/w7-gang-fairness-20260719.md`. The fairness harness pauses the custom scheduler while constructing a deterministic initial queue; it tests queue order, not production throughput, starvation freedom, or multi-tenant fairness.

## Repository map

```text
cmd/scheduler/                  custom kube-scheduler entry point
cmd/topology-controller/       topology status controller
cmd/synthetic-device-plugin/   kubelet Device Plugin entry point
cmd/synthetic-dra-driver/      kubelet DRA v1 driver and ResourceSlice publisher
cmd/synthetic-dra-workload/    CDI allocation acceptance workload
cmd/synthetic-gang-workload/   holdable gang and fairness acceptance workload
pkg/plugin/topologyfit/         policy, selection, scoring, and reservation ledger
pkg/controller/policy/          PlacementPolicy to ResourceClaimTemplate reconciler
pkg/apis/                       typed v1alpha1 API
pkg/generated/                  generated clients, listers, and informers
pkg/topology/                   graph snapshots and concurrent cache
pkg/deviceplugin/               synthetic Device Plugin service
pkg/dra/                        DRA resources, persistent Prepare state, and CDI handoff
pkg/observability/              low-cardinality component metrics and HTTP serving
pkg/fixtures/                   deterministic three-vendor topology models
config/crd/                     v1alpha1 API schemas
config/fixtures/                generated three-worker topologies
config/smoke/                   scheduler, Allocate, and topology acceptance Pods
config/experiments/             repeatable allocation, fragmentation, and gang workloads
deploy/base/                    scheduler, controller, Device Plugins, and RBAC
docs/adr/                       architecture decisions and scope boundaries
docs/reports/                   environment-specific reproducible baselines
docs/testing/                   fault-injection coverage matrix
hack/                           containerized Go and kind automation
```

## W7 acceptance criteria

- `make verify` and `make e2e` pass from a clean checkout.
- generated API code and fixture YAML are reproducible.
- all five topology modes and both availability failure policies have unit coverage.
- concurrent reservations are disjoint, atomic, race-free, idempotent, and rollback-safe.
- DRA publishes 24 attributed devices across three node-scoped pools.
- a four-device ResourceClaim allocation remains inside one NVIDIA fabric group.
- ResourceClaim allocation IDs, NodePrepare response IDs, and container CDI IDs agree.
- a driver restart recovers prepared ownership and the replacement handles Unprepare.
- ResourceSlices are driven by Ready AcceleratorTopology objects rather than direct fixture reads.
- a policy-created template carries the expected CEL selectors and matchAttribute constraint.
- topology health changes affect native allocation and are restored after fault injection.
- scheduler, controller, and DRA metrics expose bounded labels, operation counts/latencies, and current state gauges.
- all three component health/metrics endpoints are reached and semantically checked in kind.
- repeated four-claim experiments allocate eight unique devices inside valid cliques and reject an overflow claim.
- microbenchmarks report allocation-path time and allocations; the kind harness exports raw per-sample CSV.
- the fault matrix maps each failure to expected behavior, automated evidence, and remaining production work.
- Coscheduling and TopologyFit are active in the same scheduler profile with Coscheduling as the sole QueueSort plugin.
- the scheduler reads PodGroups through narrow RBAC and rejects a zero-member PodGroup through CRD validation.
- two four-device gang members reach Running together on two disjoint NVIDIA cliques.
- a Permit timeout invokes TopologyFit Unreserve and an independent recovery Pod proves reservation cleanup.
- repeated equal-priority contention admits the older PodGroup before the newer group despite reverse Pod creation order.
- scheduler framework metrics expose nonzero Coscheduling plugin evaluations.
- scheduler informers start and synchronize under read-only CRD RBAC.
- the topology-specific positive and negative scheduling cases pass.
- a real scheduler cycle emits Reserve, PreBind, and PostBind lifecycle evidence.
- the default scheduler is not modified or replaced.
- all Kubernetes modules remain on the locked v1.35.5/v0.35.5 baseline.
- documentation does not imply exact device reservation or physical accelerator testing.

## Next milestone: W8

W8 should replace fixture publication with a pluggable discovery-provider interface and one vendor-shaped adapter, add multi-node DRA claim experiments, and define production SLO/alert rules. Physical hardware claims remain prohibited until a real vendor runtime and device inventory are available.

## Repository visibility and license

The project is intended to remain private during early implementation. No open-source license is granted by this repository at W7; choose a license deliberately before making it public. Third-party dependency terms are recorded in `THIRD_PARTY_NOTICES.md`.
