# ADR-0001: Version baseline and W1 scope

- Status: Accepted
- Date: 2026-07-19

## Context

The custom scheduler binary imports the Kubernetes scheduler command while the plugin implements interfaces from `k8s.io/kube-scheduler/framework`. Kubernetes staging replacements are not transitive, so an out-of-tree scheduler must keep all Kubernetes modules on one minor and patch.

The project needs a credible first milestone before implementing topology algorithms. A neutral plugin is useful only if it proves registration, lifecycle wiring, deployment, and custom `schedulerName` binding end to end.

## Decision

- Lock Kubernetes modules and the kind node to v1.35.5.
- Build with Go 1.25.5, using a pinned container when host Go is absent.
- Implement neutral PreFilter, Filter, PreScore, Score, Reserve, Unreserve, and PreBind hooks in W1.
- Emit a constant W1 Pod signature so Kubernetes 1.35 can retain scheduling-cycle batching; W2 must sign every decision input.
- Reject plugin arguments until a typed args API and scheme registration exist.
- Use a dedicated scheduler profile named `accelerator-scheduler`.
- Keep leader election disabled for the single-replica W1 deployment.
- Treat the two CRDs as schemas only; controllers and generated Go clients belong to W2.
- Do not import scheduler-plugins before the Phase 2 Gang milestone.

## Consequences

W1 can prove that the binary is a real Scheduling Framework extension, but it must not be described as topology-aware yet. The smoke test proves only custom scheduler registration and binding. W2 must add typed topology ingestion before any Filter or Score claim is valid.
