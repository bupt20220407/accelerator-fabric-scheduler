# ADR-0006: Project API to DRA reconciliation

- Status: Accepted
- Date: 2026-07-19

## Context

W4b proves an authoritative DRA handoff but duplicates fixture devices in the driver and native claim selectors in smoke YAML. That split allows AcceleratorTopology, ResourceSlice, AcceleratorPlacementPolicy, and ResourceClaimTemplate to drift.

## Decision

- Make AcceleratorTopology the DRA publication source. Each node driver watches the typed topology informer and publishes resources only while the object has a current Ready condition and fresh heartbeat.
- Convert device identity, vendor, product, resource name, memory, health, NUMA, PCIe root, fabric group, and observed generation into DRA attributes.
- Derive conservative per-device fabric type, minimum healthy intra-group bandwidth, and maximum hops from topology links.
- Add optional `spec.dra.deviceCount` to AcceleratorPlacementPolicy. Its presence asks the policy controller to reconcile a same-name ResourceClaimTemplate owned by the policy.
- Translate vendor, product, memory, health, bandwidth, and hop requirements into CEL selectors. Translate single-NUMA, same-PCIe-root, and fabric-clique modes into matchAttribute constraints. Best-effort has no topology equality constraint.
- Reject automatic DRA generation for fabric-connected and unknown-topology policies because stable DRA v1 cannot encode the project's graph and uncertainty semantics without loss.
- Delete only controller-owned templates when DRA parameters are removed. Policy deletion relies on namespaced owner garbage collection.

## Consequences

Topology health and generation changes now flow into ResourceSlices, and one policy object defines both legacy TopologyFit behavior and native DRA claim templates. W5 eliminates duplicated fixture and selector configuration for supported modes. It does not synthesize Pod resourceClaims automatically, translate fabric-connected graphs, or discover real vendor hardware.
