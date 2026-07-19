# ADR-0002: Typed topology data plane and synthetic devices

- Status: Accepted
- Date: 2026-07-19

## Context

Topology-aware scheduling requires a deterministic and concurrency-safe source of device and link facts before Filter or Score can be trusted. The project also needs a reproducible resource-publication path that does not depend on access to physical GPU/NPU servers.

## Decision

- Keep `scheduling.bupt.dev/v1alpha1` as the single typed API and generate DeepCopy, clientset, fake client, lister, and informer code with Kubernetes code-generator v0.35.5.
- Normalize fixture fabric links as undirected edges and reject duplicate devices, missing endpoints, self-links, duplicate edges, invalid health, and invalid extended resource names.
- Store immutable graph snapshots behind an atomic copy-on-write map. Readers do not acquire the writer lock.
- Treat a topology as usable only when the Ready condition observes the current Kubernetes generation and its heartbeat is within the caller's TTL.
- Run a dedicated controller that validates topology objects and updates only their status subresource.
- Generate the three vendor fixtures from Go source so unit tests and deployed YAML share one model.
- Implement the kubelet Device Plugin v1beta1 protocol with stable IDs and NUMA hints. Allocate injects synthetic metadata only; it exposes no physical device.

## Consequences

W2 proves typed topology ingestion, status convergence, resource publication, and synthetic allocation. It does not make `TopologyFit` topology-aware yet. W3 must connect the informer-backed store to scheduling CycleState and implement Filter/Score. Physical device-ID guarantees remain out of scope until DRA or a vendor allocation integration closes that contract.
