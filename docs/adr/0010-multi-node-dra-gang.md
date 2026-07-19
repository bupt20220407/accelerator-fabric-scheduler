# ADR 0010: multi-node DRA gang semantics

## Status

Accepted for W8.

## Context

A Kubernetes Pod is scheduled to one node, and its structured device ResourceClaim is allocated from devices reachable on that node. Calling one ResourceClaim "cross-node" would misrepresent the API. The project still needs to demonstrate that gang admission can coordinate heterogeneous, node-local DRA allocations across multiple Pods and nodes.

## Decision

- Create one three-member PodGroup with NVIDIA, Huawei Ascend, and AMD member Pods.
- Give each member a distinct generated ResourceClaimTemplate with an exact count of one and a vendor selector.
- Let native DRA allocation place each claim in its matching node-scoped pool while Coscheduling coordinates the three scheduling cycles at Permit.
- Accept the experiment only if all three Pods become Running, each Pod node equals its claim pool, and CDI exposes the exact claim UID and pool/device ID.
- After completion, require NodeUnprepareResources evidence and garbage collection of all generated claims and owner-managed templates.

## Consequences

The accepted description is "a multi-node DRA gang with one node-local claim per member." The test proves API allocation, kubelet preparation, CDI identity, gang admission, and cleanup across three synthetic vendor inventories. It does not prove a ResourceClaim spanning nodes, cross-node fabric reservation, collective communication, training throughput, or coordinated failure recovery for a real distributed job.
