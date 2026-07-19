# W10 controller retry and alert baseline - 2026-07-20

## Scope

- Project version: `v1.35.5-accelerator.0.10.0`
- Kubernetes modules: `v1.35.5` / `v0.35.5`
- Prometheus validator: digest-pinned `v3.5.0`
- Physical accelerators: none

## Result

```text
make verify              PASS
make e2e                 PASS
promtool check rules     PASS (10 rules)
promtool test rules      PASS (10 exact alert expectations)
```

The final kind inspection found all scheduler/controller DaemonSets and Deployments Ready, all three topology objects Ready, and no residual PodGroup, ResourceClaim, or ResourceClaimTemplate objects.

## Controller fault injection

The topology test prepends a fake-client reactor to the `acceleratortopologies/status` update path. Attempt one returns `503 ServiceUnavailable`; the controller records an error and rate-limits the key. The next attempt uses the normal fake tracker and writes `Ready=True`. Acceptance requires an atomic attempt counter of at least two and the final condition.

The policy test applies the same pattern to `resourceclaimtemplates` creation. Attempt one returns 503; a later workqueue attempt creates the template. Acceptance requires at least two create calls and an exact device count of four.

These tests run the real shared informer, queue, worker, and sync implementation. They do not claim behavior during a long API outage or process restart.

## Alert behavior

`prometheus-rules.test.yaml` contains two deterministic scenarios:

- sustained stale discovery, component error ratios, TopologyFit errors, unschedulable Pods, and zero DRA inventory produce all seven non-absence alerts after their `for` periods;
- an empty input set produces all three missing-metrics alerts after ten minutes.

The test checks exact propagated labels, severity, summary, and runbook annotations for all ten alerts. `promtool check rules` and `promtool test rules` both pass from the same digest-pinned image.

## Build hygiene

The production image already copies only `cmd/` and `pkg/`. W10 additionally excludes `**/*_test.go` from the Docker context, so controller-test-only changes do not invalidate production binary layers. Host/containerized `make verify` still runs every test before image acceptance.

## Remaining gaps

- Inject a real API-server outage or network policy interruption and measure retry/backoff/recovery latency.
- Test throttling, prolonged error ratios, and workqueue depth under load.
- Run a Prometheus server against live scraped targets, evaluate the same rules, and verify Alertmanager delivery.
- Establish environment-specific error-budget thresholds from production baselines rather than fixture traffic.
