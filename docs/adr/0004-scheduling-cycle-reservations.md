# ADR-0004: Scheduling-cycle advisory reservations

- Status: Accepted
- Date: 2026-07-19

## Context

W3 proves node-level topology feasibility but returns only a boolean decision. Concurrent scheduling and binding cycles can therefore evaluate the same topology combination. The legacy Device Plugin API still provides no Pod identity in Allocate, so the scheduler cannot honestly claim an end-to-end physical device-ID contract.

## Decision

- Replace boolean topology checks with deterministic selectors that return concrete advisory device IDs for all five topology modes.
- Keep an in-memory, mutex-protected ledger indexed by Pod UID and node/device ID. Selection and ownership publication occur under one lock.
- Exclude reservations owned by other scheduling cycles in Filter and Score. Reserve repeats selection atomically because state may change after Filter.
- Treat Reserve as idempotent for the same Pod and node. Reject cross-node reuse, duplicate IDs, already-owned IDs, and empty selections without partially changing the ledger.
- Verify the reservation in PreBind. Release it on Unreserve after a failed binding cycle and on PostBind after a successful bind.
- Emit structured scheduler logs for Reserve, PreBind verification, Unreserve, and PostBind release. Device IDs are explicitly described as advisory.
- Do not persist or reconstruct the ledger. Do not pass selected IDs to the synthetic Device Plugin as if they were authoritative.

## Consequences

W4a prevents overlapping scheduler binding cycles from reserving the same topology combination and gives deterministic, testable decision evidence. The reservation ends after Bind and therefore does not model long-running Pod occupancy. Scheduler restart recovery, authoritative device identity, and kubelet allocation handoff require W4b with DRA or a rigorously specified vendor integration.
