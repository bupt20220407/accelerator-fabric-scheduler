# ADR 0012: retry and alert behavior verification

## Status

Accepted for W10.

## Context

The controllers already returned reconciliation errors to rate-limited workqueues, but prior tests only exercised successful API calls. Prometheus rules passed parser validation, but syntax alone cannot prove that label aggregation, thresholds, and `for` durations produce the intended alerts. Both gaps weaken operational claims even when the happy-path E2E is green.

## Decision

- Inject one `503 ServiceUnavailable` response into the topology status update and require a second API attempt plus an eventual Ready condition.
- Inject one `503 ServiceUnavailable` response into ResourceClaimTemplate creation and require a second API attempt plus an eventual four-device template.
- Use client-go fake reactors at the API boundary while running the real informer, workqueue, worker, and reconciliation code.
- Add digest-pinned `promtool test rules` cases for all ten alerts. Seven sustained-condition alerts must fire after their configured `for` durations, and three missing-series alerts must fire when no discovery, scheduler, or DRA source exists.
- Keep `promtool check rules` as a separate syntax check so parsing failures and behavioral expectation failures remain distinguishable.

## Consequences

W10 directly proves single-transient-failure recovery for the two project controllers and evaluates every alert against deterministic time series. It does not simulate a live API-server partition, request throttling over time, workqueue saturation, Prometheus scrape discovery, Alertmanager delivery, or paging integration. Those remain integration and production-environment tests.
