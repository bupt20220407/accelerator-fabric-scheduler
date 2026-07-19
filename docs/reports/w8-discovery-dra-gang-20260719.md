# W8 discovery and heterogeneous DRA gang baseline - 2026-07-19

## Environment

- Project version: `v1.35.5-accelerator.0.8.0`
- Cluster: kind `accelerator-fabric`, one control plane and three fixture workers
- Kubernetes server: `v1.35.5`
- kubectl client: `v1.36.2`
- Providers exercised in cluster: three node-local `fixture` Agents
- Vendor adapter unit fixture: sanitized `nvidia-smi` query and `topo -m` samples; no physical GPU
- Rule validator: `prom/prometheus:v3.5.0@sha256:63805ebb8d2b3920190daf1cb14a60871b16fd38bed42b857a3182bc621f4996`

## Commands and result

```text
make verify  PASS
make e2e     PASS
```

`make verify` completed formatting, vet, all unit tests, race tests, and all eight binary builds. The full E2E run rebuilt and loaded the W8 image, upgraded the existing W7 cluster, and retained all prior scheduling, Device Plugin, DRA, fragmentation, gang rollback, fairness, compatibility, and metrics checks.

## Discovery migration

The deployment script no longer applies `config/fixtures/topologies.generated.yaml`. All three discovery Pods became Ready and adopted the content-identical legacy objects without changing their node identity. The smoke test observed:

```text
accelerator-fabric-worker  fixture  Ready=True
accelerator-fabric-worker2 fixture  Ready=True
accelerator-fabric-worker3 fixture  Ready=True
three node-local discovery Agents own and refresh the fixture topologies
```

Every object carries `app.kubernetes.io/managed-by=accelerator-topology-discovery`, `scheduling.bupt.dev/provider=fixture`, and a non-empty `scheduling.bupt.dev/discovered-at`. Unit tests separately prove create, unchanged refresh, generation advance on content change, exact-content legacy adoption, and conflicting unmanaged-content refusal.

The NVIDIA adapter parsed two sanitized H100 rows and one NVLink relationship while preserving `Health=Unknown`, `bandwidthGBps=0`, and an empty PCIe root. No physical device command ran in kind.

## Multi-node DRA gang

One PodGroup with `minMember: 3` used three generated templates with exact count one. The E2E result was:

| Member | Vendor selector | Pod node / claim pool | CDI device pattern |
|---|---|---|---|
| `w8-dra-nvidia` | `nvidia` | `accelerator-fabric-worker` | `accelerator-fabric-worker/gpu*` |
| `w8-dra-huawei` | `huawei` | `accelerator-fabric-worker2` | `accelerator-fabric-worker2/ascend*` |
| `w8-dra-amd` | `amd` | `accelerator-fabric-worker3` | `accelerator-fabric-worker3/gpu*` |

Each container output included its exact ResourceClaim UID and allocated pool/device ID. After all members succeeded, the test found a matching `unprepared authoritative DRA allocation` record for every UID, then observed garbage collection of all three generated claims and all three owner-managed templates. Final inspection returned no PodGroups, ResourceClaims, or ResourceClaimTemplates in the default namespace.

This is a multi-node DRA gang with one node-local claim per member. It is not a cross-node ResourceClaim and does not reserve inter-node bandwidth.

## SLO and rule evidence

The discovery endpoint exposed a nonzero success counter and last-success timestamp. Live scheduler metrics confirmed these locked labels:

```text
scheduler_pending_pods{queue="unschedulable"}
scheduler_plugin_evaluation_total{extension_point="PreFilter",plugin="Coscheduling",profile="accelerator-scheduler"}
```

The rules were corrected against those live series and promtool accepted all ten expressions. A later restart-heavy W9 run showed that plugin execution-duration histograms are sampled and may omit low-volume Permit series; the final rules therefore use scheduler metric availability plus pending queues rather than sampled Permit outcomes. The kind result demonstrates rule syntax and source-signal shape only; it does not prove alert delivery, a 30-day availability target, or production error-budget compliance.

## Final cluster state

- scheduler and topology-controller Deployments: `1/1 Ready`
- discovery DaemonSet: `3/3 Ready`
- synthetic DRA driver DaemonSet: `3/3 Ready`
- NVIDIA, Ascend, and AMD Device Plugin DaemonSets: each `1/1 Ready`
- three AcceleratorTopologies: `Ready=True`
- no test PodGroups, ResourceClaims, or ResourceClaimTemplates remain

## Remaining production gaps

- Run a vendor provider in a purpose-built runtime image on physical hardware and validate hot-add, device loss, health recovery, and stable IDs.
- Export topology Ready/heartbeat through kube-state-metrics instead of relying only on Agent last-success as a freshness proxy.
- Add alert-firing tests against a Prometheus server and external notification path.
- Inject a missing vendor pool into the three-member gang and verify allocation/Permit cleanup.
- Test scheduler restart during Permit, controller API outage, node reboot, and corrupted DRA persisted state.
- Integrate a distributed-job controller that owns PodGroup, member Pods, and per-member claim lifecycle.
