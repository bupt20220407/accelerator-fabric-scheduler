# ADR 0009: node-local topology discovery ownership

## Status

Accepted for W8.

## Context

Earlier milestones rendered deterministic `AcceleratorTopology` YAML and applied it directly from `hack/deploy.sh`. That proved scheduler and DRA consumption, but skipped the node-side publication boundary that a hardware integration requires. A discovery process must refresh topology without racing the status controller or silently overwriting an operator/vendor-owned object.

## Decision

- Run one discovery Agent on every accelerator-labelled node and identify the node through the Downward API.
- Hide inventory sources behind a small `Provider` interface. kind uses the deterministic fixture provider; a conservative `nvidia-smi` adapter parses sanitized query and topology-matrix output.
- Let the Agent own `AcceleratorTopology.spec`, discovery labels, and `scheduling.bupt.dev/discovered-at`. The existing controller alone owns `status`, Ready validation, and heartbeat.
- Refresh `discovered-at` on every successful interval so the informer invokes status reconciliation. Increment `spec.observedGeneration` only when devices, links, source, or node content changes.
- Mark Agent-owned objects with `app.kubernetes.io/managed-by=accelerator-topology-discovery` and `scheduling.bupt.dev/provider=<provider>`.
- Adopt a legacy unmanaged object only when its topology content exactly matches current discovery after ignoring `observedGeneration`. Reject different unmanaged content, a different manager, or a different provider.
- Keep discovery RBAC cluster-scoped but narrow: get, create, update, and patch only `acceleratortopologies`; no status or delete permission.

## Consequences

Fresh kind deployments no longer apply fixture topology YAML directly, and upgrades can safely adopt the three prior generated objects. An Agent API write occurs every refresh interval to drive the controller heartbeat; production installations may replace this contract with explicit leases or controller requeueing if scale demands it.

The NVIDIA adapter does not infer bandwidth, PCIe roots, or health that `nvidia-smi topo -m` cannot establish reliably. It reports unknown health, zero bandwidth, conservative PCIe hop classes, and only labels a fully connected NVLink component as a clique. The scratch project image does not contain `nvidia-smi`; a real deployment needs a vendor-runtime image/mount and hardware acceptance evidence. W8 therefore proves the provider boundary and parser, not physical GPU discovery.
