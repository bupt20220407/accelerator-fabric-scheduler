# ADR-0005: Authoritative synthetic DRA handoff

- Status: Accepted
- Date: 2026-07-19

## Context

The W4a ledger can prevent overlap during a scheduling transaction, but the legacy Device Plugin Allocate request has no Pod identity and cannot consume the scheduler's advisory IDs. Kubernetes 1.35.5 exposes stable `resource.k8s.io/v1` APIs and kubelet DRA v1, which provide an authoritative allocation result and claim identity.

## Decision

- Run one synthetic DRA driver on each fixture worker with the Kubernetes `kubeletplugin` helper and v1 service only.
- Publish one node-scoped ResourceSlice pool per worker. Each of the eight devices carries vendor, product, extended-resource name, memory, health, NUMA, PCIe root, fabric type, and fabric-group attributes derived from the shared fixtures.
- Use a DeviceClass that selects `accelerator.scheduling.bupt.dev`. The acceptance ResourceClaim requests four NVIDIA devices and uses `matchAttribute` on `fabricGroup`, so the built-in DRA allocator must choose one clique.
- Treat `ResourceClaim.status.allocation.devices.results` as the authoritative identity. NodePrepare validates that every allocated result belongs to the local node pool and rejects a device already prepared for another claim.
- Persist prepared claim ownership under the kubelet plugin host directory with atomic file replacement. Reload it on driver restart and recreate the corresponding CDI specification.
- Generate one CDI device per claim. It injects the claim UID and allocated pool/device IDs into consuming containers. This is synthetic metadata and exposes no host hardware.
- Make Prepare and Unprepare idempotent. Unprepare removes CDI first, then persists the ownership release; a persistence failure restores both ownership and CDI so kubelet can retry safely.
- Keep the legacy extended-resource/Device Plugin path for comparison. Its W4a IDs remain advisory; only the DRA path has an authoritative claim-to-kubelet handoff.

## Consequences

W4b proves native allocation, kubelet preparation, CDI consumption, driver restart recovery, and cleanup through stable Kubernetes DRA APIs. The driver still publishes fixture devices rather than discovering physical hardware, and the placement-policy CRD is not yet translated into ResourceClaim selectors automatically. Real vendor CDI edits, health reconciliation, multi-node claims, and production hardening remain future work.
