# W9 DRA gang failover baseline - 2026-07-20

## Environment

- Project version: `v1.35.5-accelerator.0.9.0`
- Cluster: kind `accelerator-fabric`, one control plane and three fixture workers
- Kubernetes server: `v1.35.5`
- Fault scope: one missing AMD topology/DRA pool and one custom scheduler restart during Coscheduling Permit
- Physical accelerators: none

## Injected sequence

1. Added an unsatisfiable node selector to the discovery DaemonSet and waited for all discovery Pods to stop.
2. Deleted `AcceleratorTopology/accelerator-fabric-worker3` and observed zero AMD devices in its node-scoped ResourceSlice pool.
3. Submitted one three-member PodGroup with independent NVIDIA, Huawei, and AMD ResourceClaimTemplates.
4. Observed NVIDIA and Huawei members nominated to their matching nodes and waiting at Coscheduling Permit; the AMD member had no feasible DRA allocation.
5. Verified that no Pod was bound and no ResourceClaim allocation had been persisted.
6. Restarted `Deployment/accelerator-scheduler` and waited for the PodGroup timeout.
7. Observed all nominations cleared, all Pods still unbound, and all claims still unallocated.
8. Removed the injected node selector, waited for three discovery Agents, and observed the AMD topology and all eight devices return.
9. Re-ran the normal heterogeneous gang and verified CDI identity, NodeUnprepareResources, claim GC, and template GC.

## Result

```text
make verify  PASS
make e2e     PASS
paused discovery and withdrew the AMD node-local DRA pool
two feasible DRA members reached Permit with internal nominations, while no claim allocation or Pod binding was committed
scheduler restart preserved gang atomicity and cleared all provisional nominations without persisting DRA allocations
discovery recreated the AMD topology and restored all eight DRA devices
three PodGroup members consumed one node-local NVIDIA, Ascend, and AMD DRA claim
all three claims were unprepared and all generated claims/templates were garbage collected
```

The first implementation attempt incorrectly expected `ResourceClaim.status.allocation` to be populated before Permit. Live evidence showed only `nominatedNodeName` at that stage. The acceptance contract was corrected to reflect the actual v1.35.5 framework commitment order.

The first cleanup attempt also showed that client-side apply does not remove a node selector introduced by a different patch field manager. Recovery now explicitly merges `nodeSelector: null`, waits for the DaemonSet rollout, and then validates topology and ResourceSlice restoration. This failed attempt is retained here because it materially improved the fault harness.

The first full E2E attempt then failed an exact `scheduler_plugin_execution_duration_seconds_count{extension_point="Permit",status="Success"}` assertion even though logs proved multiple successful Permit cycles. Inspection showed that the upstream duration histogram is sampled and had no series in that scheduler process. The monitoring contract now alerts on scheduler metric loss and pending queues; non-sampled Coscheduling evaluations plus logs and workload behavior remain the Permit evidence.

The next full run exposed a pre-existing race in the DRA unhealthy-device test: the discovery Agent could restore fixture health before the claim was allocated, allowing `gpu0-3` instead of the expected healthy `gpu4-7` clique. The health-injection test now pauses discovery, restores topology and ResourceSlice health explicitly, resumes the DaemonSet in its EXIT trap, and is immediately followed by the W9 failover test. That ordered regression passed before the final complete E2E run.

The Docker builder now copies only `cmd/` and `pkg/` after dependency download. Documentation and smoke-script changes therefore no longer invalidate the expensive scheduler compilation layer; the final rebuild reused all Go build layers while still loading a newly identified image into kind.

## Claim boundary

This baseline proves one synthetic missing-pool event, one scheduler restart, atomic non-binding, nomination cleanup, discovery recovery, and a subsequent successful run. It does not prove persisted allocation rollback because no allocation was committed before Permit. It also does not prove physical accelerator recovery, production scheduler HA, or distributed training recovery.
