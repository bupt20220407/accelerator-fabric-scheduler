# Accelerator Fabric Scheduler

[![verify](https://github.com/bupt20220407/accelerator-fabric-scheduler/actions/workflows/verify.yaml/badge.svg)](https://github.com/bupt20220407/accelerator-fabric-scheduler/actions/workflows/verify.yaml)

`Accelerator Fabric Scheduler` is an out-of-tree Kubernetes scheduler for topology-aware GPU/NPU placement. It models NUMA locality, PCIe roots, and accelerator fabrics such as NVLink, HCCS, and XGMI.

> **Status:** `v0.1.x` engineering research prototype. The checked-in evidence
> uses deterministic synthetic accelerator inventories and local kind clusters;
> it does not establish physical-accelerator performance or production readiness.

## Current milestone: W10 controller retries and executable alerts

The repository now provides a reproducible policy-to-scheduling path:

- a custom kube-scheduler binary registers `TopologyFit` and handles Pods with `schedulerName: accelerator-scheduler`;
- generated DeepCopy, clientset, fake client, listers, and shared informers cover both `scheduling.bupt.dev/v1alpha1` CRDs;
- an immutable graph snapshot and copy-on-write cache track Ready, generation, heartbeat, and TTL state;
- a controller validates topology objects and updates their status subresource;
- a node-local discovery DaemonSet publishes topology through a pluggable provider interface instead of deployment-time YAML apply;
- explicit ownership labels, content-equal legacy adoption, periodic discovery timestamps, and change-only observed generations separate Agent spec ownership from controller status ownership;
- a fixture provider preserves deterministic kind behavior, while a conservative, command-runner-injected `nvidia-smi` provider has sanitized parser tests;
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
- a three-member heterogeneous PodGroup consumes three independent node-local DRA claims on NVIDIA, Ascend, and AMD workers and verifies CDI identity plus cleanup;
- a failure harness withdraws one vendor pool, restarts the scheduler while two DRA members wait at Permit, proves no partial commit, and then validates discovery plus workload recovery;
- fake-client reactors inject transient 503 responses into both controllers and prove rate-limited workqueue retry plus eventual convergence;
- digest-pinned promtool validation covers ten discovery, controller, scheduler, and DRA alert rules with an explicit prototype SLO contract;
- deterministic promtool time-series tests verify thresholds, `for` durations, labels, severities, and annotations for every alert;
- three kind workers publish eight synthetic NVIDIA, Ascend, or AMD resources through the kubelet Device Plugin API;
- end-to-end tests prove both real kubelet Allocate calls and a topology-specific four-card-pass/five-card-reject decision.

The DRA path has an authoritative synthetic claim-to-kubelet identity contract, while the legacy extended-resource/Device Plugin path remains advisory. Gang admission does not turn legacy advisory device IDs into an authoritative kubelet handoff. Pods must still reference generated DRA templates explicitly. Automatic DRA translation intentionally excludes `fabric-connected` and unknown-topology policies. W10 proves one-shot API retry and deterministic alert behavior, but it does not run discovery on physical GPUs, provide scheduler HA, deliver real pages, or prove training-throughput improvement.

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

## Run the demo console

The browser console turns the checked-in three-vendor fixture into an
interactive scheduling demonstration. It shows component health, device
fabrics, Filter/Score evidence, reservation choices, ResourceClaim/CDI
identity, PodGroup admission, and a policy simulator covering all five
topology modes.

The console is intentionally a deterministic replay. **Baseline** presents the
W10 fixture, while **Fault drill** withdraws the AMD topology and ResourceSlice
pool. Running the simulator changes only in-memory browser state; it never
reads from or mutates a Kubernetes cluster.

Start the development server with:

```bash
cd frontend
npm ci
npm run dev
```

Open `http://127.0.0.1:5174/console/`. Verify a production frontend with
`npm test`, `npm run typecheck`, and `npm run build`.

### Two-minute demonstration

1. Start with **Baseline** and use the overview to establish scheduler,
   controller, discovery, and DRA health across the three-vendor fixture.
2. Open the fabric view and compare NUMA, PCIe-root, and NVLink/HCCS/XGMI
   relationships instead of relying on scalar accelerator capacity alone.
3. Run the policy simulator for a four-device `fabric-clique` request, then
   compare its deterministic Filter, Score, and selected-device evidence with
   an infeasible five-device request.
4. Inspect ResourceClaim, NodePrepare, and CDI identity to distinguish the
   authoritative DRA handoff from the advisory legacy extended-resource path.
5. Select **Fault drill** to withdraw the AMD topology and ResourceSlice pool,
   then show FailClosed behavior, PodGroup waiting, and rollback without a
   partial device allocation or Pod binding.

The simulator and fault drill mutate only browser memory, so this path is
deterministic and safe for interviews without a Kubernetes cluster or physical
accelerators.

The normal project image still starts the scheduler. The same image also
contains the compiled console and a separate static server for demonstrations:

```bash
docker build -t accelerator-fabric-scheduler:demo .
docker run --rm -p 8080:8080 \
  --entrypoint /demo-console \
  accelerator-fabric-scheduler:demo \
  --address 0.0.0.0:8080 \
  --directory /frontend/dist
```

Open `http://127.0.0.1:8080/console/`. This alternate entry point does not run
the scheduler or require cluster credentials.

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

## Run the W10 end-to-end test

```bash
make e2e
```

The command creates a one-control-plane/three-worker kind cluster, builds and loads the image, installs the CRDs, and deploys the scheduler, topology controller, and synthetic Device Plugins. It verifies:

- server-side CRD validation, topology Ready conditions, and fresh heartbeats;
- node-local fixture Agents create or safely adopt all three topology objects, continuously refresh discovery timestamps, and expose success metrics;
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
- a three-member heterogeneous gang receives one NVIDIA, Ascend, and AMD node-local claim, consumes exact CDI identities, unprepares all claims, and leaves no generated objects;
- withdrawing the AMD topology removes its DRA devices, leaves two feasible members waiting at Permit, and commits neither claim allocation nor Pod binding;
- restarting the scheduler clears both provisional nominations, after which discovery restores the AMD topology and a complete three-vendor gang succeeds;
- scheduler, controller, discovery, and DRA health/Prometheus endpoints expose expected operation and state signals;
- upstream scheduler framework metrics record nonzero Coscheduling evaluations, while gang logs prove Permit waiting;
- an unannotated W1 compatibility Pod is still bound by the custom scheduler;
- cleanup restores both AcceleratorTopology and ResourceSlice health to Healthy;
- promtool accepts all ten SLO/alert expressions from a digest-pinned Prometheus image;
- promtool evaluates all ten alerts against exact sustained-failure and missing-series expectations;
- the image reports `v1.35.5-accelerator.0.10.0`, preventing a stale `:dev` image from passing.

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
make dra-gang-failover-smoke
```

The checked-in W7 result is in `docs/reports/w7-gang-fairness-20260719.md`. The fairness harness pauses the custom scheduler while constructing a deterministic initial queue; it tests queue order, not production throughput, starvation freedom, or multi-tenant fairness.

The checked-in W8 result is in `docs/reports/w8-discovery-dra-gang-20260719.md`. It records the W7-to-W8 ownership migration, heterogeneous gang allocation/cleanup, live scheduler metric labels, and the remaining physical-hardware and production-operations gaps.

The checked-in W9 result is in `docs/reports/w9-dra-gang-failover-20260720.md`. It records the missing-pool injection, scheduler restart commitment boundary, recovery behavior, and a failed cleanup assumption that was corrected in the final harness.

The checked-in W10 result is in `docs/reports/w10-retry-alerts-20260720.md`. It records controller API fault injection, exact alert-firing expectations, and the boundary between deterministic rule tests and a live Prometheus/Alertmanager deployment.

## Repository map

```text
cmd/scheduler/                  custom kube-scheduler entry point
cmd/topology-controller/       topology status controller
cmd/topology-discovery-agent/ node-local provider runner and topology publisher
cmd/synthetic-device-plugin/   kubelet Device Plugin entry point
cmd/synthetic-dra-driver/      kubelet DRA v1 driver and ResourceSlice publisher
cmd/synthetic-dra-workload/    CDI allocation acceptance workload
cmd/synthetic-gang-workload/   holdable gang and fairness acceptance workload
cmd/demo-console/              optional static server for the replay console
pkg/plugin/topologyfit/         policy, selection, scoring, and reservation ledger
pkg/controller/policy/          PlacementPolicy to ResourceClaimTemplate reconciler
pkg/apis/                       typed v1alpha1 API
pkg/generated/                  generated clients, listers, and informers
pkg/topology/                   graph snapshots and concurrent cache
pkg/deviceplugin/               synthetic Device Plugin service
pkg/dra/                        DRA resources, persistent Prepare state, and CDI handoff
pkg/observability/              low-cardinality component metrics and HTTP serving
pkg/discovery/                  Agent ownership contract and fixture/NVIDIA providers
pkg/fixtures/                   deterministic three-vendor topology models
config/crd/                     v1alpha1 API schemas
config/fixtures/                generated three-worker topologies
config/smoke/                   scheduler, Allocate, and topology acceptance Pods
config/experiments/             repeatable allocation, fragmentation, and gang workloads
deploy/base/                    scheduler, controller, Device Plugins, and RBAC
deploy/monitoring/              executable Prometheus alert rules
docs/adr/                       architecture decisions and scope boundaries
docs/reports/                   environment-specific reproducible baselines
docs/testing/                   fault-injection coverage matrix
hack/                           containerized Go and kind automation
frontend/                       responsive fixture replay and policy simulator
```

## W10 acceptance criteria

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
- deployment no longer directly applies fixture topology YAML; three Agents own and refresh the objects.
- unchanged refreshes preserve `observedGeneration`, changed discoveries increment it, and conflicting unmanaged/provider-owned objects are rejected.
- sanitized `nvidia-smi` samples prove conservative parsing without asserting health, bandwidth, PCIe-root, or hardware validation.
- a multi-node DRA gang uses one node-local claim per member across all three synthetic vendor pools.
- all three gang claims are prepared, exposed through exact CDI identities, unprepared, and garbage collected with their templates.
- ten Prometheus rules pass digest-pinned promtool validation, while the SLO document distinguishes executable configuration from production evidence.
- deleting one topology while discovery is paused withdraws all devices from that node-local DRA pool.
- exactly two feasible heterogeneous members can wait at Permit without committing claim allocation or Pod binding.
- a scheduler restart plus PodGroup timeout clears all provisional nominations and preserves all-or-nothing gang behavior.
- explicitly removing the injected DaemonSet field restores all discovery Agents, the missing topology, and eight DRA devices.
- the recovered cluster completes the normal three-vendor gang and leaves no PodGroup, claim, or template residue.
- one injected topology status 503 produces at least two update attempts and eventually writes `Ready=True`.
- one injected ResourceClaimTemplate create 503 produces at least two create attempts and eventually creates the correct template.
- all ten alert rules have exact promtool firing tests in addition to parser validation.
- production Docker builds exclude test-only Go files while `make verify` continues to execute them.

## Next milestone: physical-hardware validation

The next milestone should run one provider against physical hardware, reconcile vendor health signals with hysteresis, and add a distributed-job controller or workload integration that owns PodGroup and per-member claim lifecycle. API outage, node reboot, repeated scheduler crashes, and alert-firing tests remain higher-value than adding more synthetic policy modes.

## Repository visibility and license

The project is released under the [Apache License 2.0](LICENSE). Third-party
dependency terms and adapted material are recorded in
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
