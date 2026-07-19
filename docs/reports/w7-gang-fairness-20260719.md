# W7 gang scheduling baseline - 2026-07-19

## Environment

- Project version: `v1.35.5-accelerator.0.7.0`
- Scheduler: Kubernetes v1.35.5 plus scheduler-plugins `v0.35.4-devel` Coscheduling
- Cluster: one kind control plane, three workers, and eight synthetic NVIDIA devices in two four-device cliques
- Coscheduling: 10-second default Permit wait, 5-second backoff, 100 percent PostFilter reject threshold

## Gang admission and rollback

A two-member PodGroup requested four NVIDIA devices per member. The first member reserved `gpu0-gpu3` and waited in Permit; the second reserved `gpu4-gpu7`; both then bound to `accelerator-fabric-worker` and ran concurrently.

An incomplete group used one four-device member and one infeasible five-device member. The four-device member reached Reserve and Permit, timed out after five seconds, and emitted TopologyFit `Unreserve`. After deleting the incomplete group, an independent four-device recovery Pod completed, proving that the advisory ledger retained no leaked clique.

## Equal-priority ordering

The scheduler was scaled to zero while each initial queue was constructed. PodGroup A was created one second before PodGroup B, while B's Pods were deliberately submitted before A's Pods. Both groups required all eight NVIDIA devices.

| Round | Older group A start | Newer group B start | Observed gap |
|---:|---|---|---:|
| 1 | `2026-07-19T14:19:40Z` | `2026-07-19T14:19:48Z` | 8 s |
| 2 | `2026-07-19T14:20:03Z` | `2026-07-19T14:20:13Z` | 10 s |

In both rounds, B remained unbound while A ran and started only after A released all eight devices. This confirms the configured QueueSort behavior for equal-priority PodGroups in the fixed experiment.

## Interpretation boundary

The result is not a general fairness benchmark. It does not test starvation, mixed priorities, preemption, quotas, many queues, scheduler failover, or real accelerator workloads. Pausing the scheduler makes the initial queue deterministic and is part of the test method, not a production operating procedure.
