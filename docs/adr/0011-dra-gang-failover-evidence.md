# ADR 0011: DRA gang failover evidence

## Status

Accepted for W9.

## Context

W8 proved the successful three-vendor path but did not inject a missing node-local pool or restart the scheduler while Coscheduling held feasible members at Permit. The ResourceClaim API and scheduler framework have distinct commitment points, so the test must report observable state accurately rather than assuming a provisional scheduling decision has already persisted claim allocation.

## Decision

- Pause topology discovery by temporarily adding an unsatisfiable node selector to its DaemonSet, then delete the AMD topology and wait until the node-scoped DRA pool publishes zero devices.
- Submit the existing NVIDIA, Huawei, and AMD three-member PodGroup.
- Require exactly two feasible members to have `status.nominatedNodeName` and corresponding Coscheduling Permit-wait logs while every ResourceClaim allocation and Pod binding remains empty.
- Restart the custom scheduler during that wait, then require all nominations to clear, all allocations to remain empty, and all Pods to remain unbound after the PodGroup timeout.
- Restore discovery by explicitly removing the injected node selector. Do not rely on client-side apply to remove a field owned by the patch field manager.
- Require the Agent to recreate the AMD topology, the DRA publisher to restore eight devices, and the normal heterogeneous gang to pass with authoritative CDI identity, Unprepare, and garbage collection.

## Consequences

The experiment proves gang atomicity across a missing synthetic vendor pool and one scheduler process restart. In this Kubernetes version, DRA allocation is not persisted before Coscheduling Permit succeeds; the correct provisional evidence is scheduler nomination plus Permit logs. W9 therefore does not claim that persisted ResourceClaim allocations were rolled back, only that none were committed and all provisional nominations were cleared.

The result does not prove high-availability leader failover, repeated crash loops, API-server partition recovery, node reboot behavior, inter-node network reservation, or physical-device correctness. Those require separate failure campaigns.
