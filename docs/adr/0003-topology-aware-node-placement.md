# ADR-0003: Topology-aware node placement

- Status: Accepted
- Date: 2026-07-19

## Context

W2 provides typed, freshness-aware topology snapshots but `TopologyFit` remains neutral. W3 must make reproducible scheduling decisions without claiming that the legacy Device Plugin API lets the scheduler reserve exact physical device IDs.

## Decision

- A Pod opts in with the `scheduling.bupt.dev/placement-policy` annotation. The value names an `AcceleratorPlacementPolicy` in the Pod namespace.
- An opted-in W3 Pod must request exactly one positive extended resource. Requests use Kubernetes `PodRequests` semantics, including init containers and Pod overhead.
- `PreFilter` resolves and validates the policy and stores an immutable decision input in CycleState. Pods without the annotation remain neutral.
- The scheduler starts generated topology and policy informers before waiting for both caches to sync. Its service account has read-only access to the two CRDs.
- `Filter` applies vendor, product, memory, resource-name, and health constraints before one of five topology modes: single NUMA, same PCIe root, fabric clique, fabric connected, or best effort.
- Fabric modes use healthy non-PCIe links that satisfy minimum bandwidth and maximum hops. `allowUnknownTopology` admits explicitly unknown device/link health or locality fields; it does not turn a known disconnected graph into a match.
- Missing, stale, or not-Ready topology fails a `FailClosed` policy and is neutral under `BestEffort`.
- `Score` combines four absolute 0-100 features with policy weights: locality concentration, qualifying fabric-edge density, post-placement fragmentation, and scalar-resource headroom. Policy weights must sum to 100.
- Reserve, Unreserve, and PreBind remain neutral. Kubernetes NodeInfo accounts for scalar-resource headroom, but W3 does not infer which device IDs existing Pods occupy.

## Consequences

W3 proves policy-driven topology Filter and deterministic Score behavior at node granularity. A four-device clique can pass while a five-device request is rejected on an eight-device node, demonstrating that the decision is not merely Kubernetes scalar capacity filtering. Exact concurrent device-combination reservation, allocation handoff, and physical device identity require a later DRA or vendor integration milestone.
