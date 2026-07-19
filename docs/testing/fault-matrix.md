# W6 fault matrix

| Fault | Injection | Expected behavior | Automated evidence | Remaining production work |
|---|---|---|---|---|
| Stale topology heartbeat | Evaluate a Ready topology after its TTL | `FailClosed` rejects it; `BestEffort` may continue; DRA publishes no devices | topology cache and DRA publisher unit tests | Pause a real topology source and alert on sustained staleness |
| Invalid topology graph | Submit duplicate/missing devices or invalid links | Controller sets `Ready=False`; scheduler and DRA do not consume the graph | graph/controller/CRD tests | Fuzz larger vendor inventories and malformed discovery payloads |
| Unhealthy device | Patch `gpu0` to `Unhealthy` | ResourceSlice health changes and allocation moves to the healthy clique | `hack/smoke-dra.sh` | Correlate vendor XID/firmware events and recovery hysteresis |
| Unhealthy fabric link | Mark a required clique link unhealthy | Clique feasibility rejects combinations using that link | TopologyFit selection/filter unit tests | Inject real NVLink/HCCS/XGMI degradation and validate telemetry mapping |
| DRA driver restart | Delete the driver Pod that owns a prepared claim | Replacement reloads persisted state and completes Unprepare | `hack/smoke-dra.sh` | Node reboot, corrupted state, disk-full, and CDI runtime restart tests |
| Claim/device conflict | Prepare a second claim for an owned device | Driver rejects the second owner without corrupting persisted state | `pkg/dra/driver_test.go` | API-level forced allocation conflict and tenant-facing event contract |
| Resource API error | Return create/update/status errors from the Kubernetes API | Controller returns an error and its rate-limited queue retries | retry path is implemented; direct fault injection is not yet automated | Fake-client reactor coverage, API-server outage, throttling, and retry/alert SLO tests |
| Full two-clique occupancy | Run four concurrent two-device claims, then a fifth | Four claims use eight unique clique-local devices; overflow remains unallocated | `hack/smoke-fragmentation.sh` | Larger heterogeneous pools, cancellation races, and long-running churn |
| Metrics endpoint loss | Stop or isolate a component Pod | Scrape fails while Kubernetes readiness reflects process availability | `hack/smoke-metrics.sh` validates healthy endpoints | Prometheus alert rules, NetworkPolicy, TLS/auth for non-scheduler endpoints |

The matrix records prototype evidence, not a production reliability claim. Items in the final column remain explicitly out of scope for W6.
