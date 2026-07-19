# Prototype SLO and alert policy

This document defines measurable targets for the accelerator scheduling control plane. It is an initial production contract, not evidence that the targets have been achieved on physical accelerators. The checked-in rules are plain Prometheus rule files and do not require the Prometheus Operator.

## Discovery

Target: every managed accelerator node completes at least one successful discovery within two minutes, and at least 99.9% of five-minute discovery operations succeed over a rolling 30-day window.

The discovery Agent exports a last-success timestamp and bounded `provider,result` operation labels. `AcceleratorDiscoveryStale` identifies a live but stale Agent; `AcceleratorDiscoveryMetricsMissing` identifies total scrape or deployment loss. Start diagnosis with DaemonSet availability, RBAC denials, provider command availability, and the `scheduling.bupt.dev/discovered-at` annotation. A fixture success proves the Agent contract only. A physical NVIDIA deployment additionally requires a runtime image containing `nvidia-smi`, access to the vendor driver, and hardware-specific acceptance tests.

The current rules use Agent success as the topology freshness signal. A production kube-state-metrics custom-resource mapping should separately expose `AcceleratorTopology.status.lastHeartbeatTime` and the Ready condition so discovery success, API publication, and controller validation can be alerted independently.

## Control Plane

Target: at least 99.9% of topology and policy reconciliations succeed over 30 days; a sustained five-minute error ratio above 5% pages the owning team after ten minutes.

Inspect API-server availability, request throttling, CRD conversion/validation errors, and controller logs. Alerts deliberately use ratios instead of raw errors so isolated retried failures do not page. Production deployment should add leader election, more than one controller replica, API outage tests, and per-controller latency objectives based on observed traffic.

## Scheduling

Target: TopologyFit produces no internal framework errors, scheduler metrics remain continuously available, and no accelerator-profile Pod remains in the scheduler's unschedulable queue for more than 15 minutes without an acknowledged capacity or policy incident.

TopologyFit `reject` is a valid policy/capacity outcome and is not counted as an internal error. Diagnose persistent pending Pods by checking PodGroup completeness, DRA claim allocation, topology freshness, requested vendor/product/count, and node capacity. Coscheduling Permit outcomes are proven by behavior and logs in the current harness. Kubernetes plugin execution-duration histograms are sampled and may expose no Permit series for a low-volume interval, so they are deliberately not used as an outcome alert.

Pending-Pod and missing-metrics alerts require scraping the kube-scheduler metrics endpoint. In the locked v1.35.5 build, `scheduler_pending_pods` has only a `queue` label, so this shared-scheduler process must remain dedicated to the accelerator profile for that alert to be profile-specific. These alerts describe control-plane symptoms, not a latency guarantee for training startup. Production objectives should segment priority class and tenant, and must account for intentional queueing, quotas, preemption, and maintenance.

## DRA Data Plane

Target: every expected node-local driver continuously publishes a non-empty inventory, and at least 99.99% of publish, prepare, and unprepare operations succeed over 30 days.

Zero published devices is critical after five minutes. Diagnose topology Ready/freshness, ResourceSlice ownership, kubelet plugin registration, CDI directory access, and persisted prepare state. Long-lived prepared claims are normal for long-running workloads, so `prepared_claims > 0` is intentionally a dashboard signal rather than a generic alert. Detecting leaked claims requires correlating prepared claim UIDs with live Pods/ResourceClaims, which this prototype does in E2E cleanup checks but does not yet export as a production metric.

## Validation and rollout

Run `make monitoring-smoke` to validate rule syntax with the digest-pinned Prometheus `v3.5.0` image. Before enabling paging:

1. Scrape scheduler, controller, every discovery Agent, and every DRA driver with stable `cluster`, `instance`, and `node` labels.
2. Confirm the scheduler's `queue`, `extension_point`, and `plugin` label values against the deployed metrics endpoint; do not assume `scheduler_pending_pods` has a profile label or sampled duration histograms always have a Permit series.
3. Replay discovery loss, API errors, empty inventory, pending Pods, and Permit timeout in a staging cluster.
4. Tune `for` durations from at least two weeks of baselines and connect each alert to an owned external runbook.

The alert file is executable configuration, but the kind test only proves syntax and synthetic signal availability. It does not establish a real production error budget.
