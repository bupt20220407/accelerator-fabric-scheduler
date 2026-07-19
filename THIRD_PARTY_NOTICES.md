# Third-party notices

This repository does not grant a license for its own code. The following dependency retains its upstream license:

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
