# ADR-0008: Coscheduling and PodGroup integration

## Status

Accepted for W7.

## Context

W6 schedules individual Pods and measures topology/DRA behavior. Distributed training jobs can deadlock or partially consume accelerator capacity when members are admitted independently. Implementing a private gang protocol would add correctness risk and weak portfolio evidence.

## Decision

- Import scheduler-plugins `v0.35.4-devel`, the published Kubernetes 1.35 candidate, while retaining this repository's v1.35.5/v0.35.5 replace baseline.
- Register the beta `Coscheduling` plugin and its config scheme in the custom scheduler.
- Make Coscheduling the sole QueueSort plugin and enable its PreFilter, PostFilter, Reserve, Permit, and Unreserve extension points through MultiPoint.
- Install the upstream-compatible `scheduling.x-k8s.io/v1alpha1` PodGroup schema and grant the scheduler read-only PodGroup access.
- Use a 10-second default Permit wait, 5-second group backoff, and 100 percent reject threshold. The last setting intentionally disables optimistic PostFilter rejection so W7 can prove timeout rollback.
- Keep TopologyFit and Coscheduling independent. TopologyFit reserves first; any Permit failure invokes framework Unreserve and releases advisory IDs.
- Test equal-priority order by PodGroup creation time with the scheduler paused during deterministic queue construction.

## Consequences

W7 proves gang admission, timeout rollback, and deterministic older-group precedence on an eight-device fixture. Enabling Coscheduling disables the scheduler profile's scheduling-cycle signature batching because Coscheduling does not implement SignPlugin; this is an explicit throughput tradeoff. The result does not prove starvation freedom, preemption correctness, scheduler failover recovery, multi-node collectives, or production tenant isolation.
