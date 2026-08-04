# Third-party notices

Project-authored code is licensed under the Apache License 2.0. The following
dependencies and adapted material retain their upstream licenses:

## Kubernetes scheduler-plugins

- Module: `sigs.k8s.io/scheduler-plugins`
- Version: `v0.35.4-devel`
- Source commit: `2c75c8b5cb943435e94ffd325d9f1542d01f175f`
- License: Apache License 2.0
- Source: https://github.com/kubernetes-sigs/scheduler-plugins

The `scheduling.x-k8s.io/v1alpha1` PodGroup schema in `config/crd/scheduling.x-k8s.io_podgroups.yaml` is adapted from that version's generated CRD. Copyright and license remain with the Kubernetes Authors under Apache License 2.0.

## Prometheus

- Tool image: `prom/prometheus:v3.5.0`
- Use: `promtool check rules` validation only; the image is not redistributed
- License: Apache License 2.0
- Source: https://github.com/prometheus/prometheus

## Demo console dependencies

The browser console installs, but does not modify, the following direct
dependencies. Their transitive dependencies retain their own upstream terms.

| Component | Version | License | Source |
|---|---|---|---|
| React | 19.2.8 | MIT | https://github.com/facebook/react |
| React DOM | 19.2.8 | MIT | https://github.com/facebook/react |
| React Flow (XYFlow) | 12.11.2 | MIT | https://github.com/xyflow/xyflow |
| Recharts | 3.10.1 | MIT | https://github.com/recharts/recharts |
| Lucide React | 1.28.0 | ISC | https://github.com/lucide-icons/lucide |
| Vite | 8.2.0 | MIT | https://github.com/vitejs/vite |
| TypeScript | 7.0.2 | Apache-2.0 | https://github.com/microsoft/TypeScript |
| Vitest | 4.1.10 | MIT | https://github.com/vitest-dev/vitest |

Node.js is used only in the console build stage and is not copied into the
final runtime image. Its upstream license remains with the Node.js project.
