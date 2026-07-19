SHELL := /bin/sh
SCHEDULER_VERSION ?= v1.35.5-accelerator.0.8.0
VERSION_LDFLAG := -X k8s.io/component-base/version.gitVersion=$(SCHEDULER_VERSION)

.PHONY: generate fmt fmt-check vet test test-race build image kind-up deploy crd-smoke topology-smoke device-smoke dra-smoke multi-node-dra-gang-smoke topology-aware-smoke fragmentation-smoke gang-smoke gang-fairness-smoke metrics-smoke monitoring-smoke smoke benchmark benchmark-allocation e2e kind-down verify

generate:
	./hack/generate.sh
	./hack/render-fixtures.sh

fmt:
	./hack/go.sh fmt ./cmd/... ./pkg/...

fmt-check:
	@files="$$(./hack/go.sh fmt ./cmd/... ./pkg/...)"; test -z "$$files" || { echo "unformatted files:"; echo "$$files"; exit 1; }

vet:
	./hack/go.sh vet ./...

test:
	./hack/go.sh test ./...

test-race:
	CGO_ENABLED=1 ./hack/go.sh test -race ./...

build:
	mkdir -p bin
	./hack/go.sh build -buildvcs=false -trimpath -ldflags '$(VERSION_LDFLAG)' -o bin/accelerator-scheduler ./cmd/scheduler
	./hack/go.sh build -buildvcs=false -trimpath -o bin/synthetic-device-plugin ./cmd/synthetic-device-plugin
	./hack/go.sh build -buildvcs=false -trimpath -o bin/topology-controller ./cmd/topology-controller
	./hack/go.sh build -buildvcs=false -trimpath -o bin/topology-discovery-agent ./cmd/topology-discovery-agent
	./hack/go.sh build -buildvcs=false -trimpath -o bin/synthetic-workload ./cmd/synthetic-workload
	./hack/go.sh build -buildvcs=false -trimpath -o bin/synthetic-dra-driver ./cmd/synthetic-dra-driver
	./hack/go.sh build -buildvcs=false -trimpath -o bin/synthetic-dra-workload ./cmd/synthetic-dra-workload
	./hack/go.sh build -buildvcs=false -trimpath -o bin/synthetic-gang-workload ./cmd/synthetic-gang-workload

image:
	docker build --build-arg SCHEDULER_VERSION='$(SCHEDULER_VERSION)' -t "$${SCHEDULER_IMAGE:-accelerator-fabric-scheduler:dev}" .

kind-up:
	./hack/kind-up.sh

deploy:
	./hack/deploy.sh

smoke:
	./hack/smoke.sh

crd-smoke:
	./hack/validate-crds.sh

topology-smoke:
	./hack/smoke-topologies.sh

device-smoke:
	./hack/smoke-synthetic-devices.sh

dra-smoke:
	./hack/smoke-dra.sh

multi-node-dra-gang-smoke:
	./hack/smoke-multi-node-dra-gang.sh

topology-aware-smoke:
	./hack/smoke-topologyfit.sh

fragmentation-smoke:
	FRAGMENTATION_ROUNDS="$${FRAGMENTATION_ROUNDS:-2}" ./hack/smoke-fragmentation.sh

gang-smoke:
	./hack/smoke-gang.sh

gang-fairness-smoke:
	GANG_FAIRNESS_ROUNDS="$${GANG_FAIRNESS_ROUNDS:-2}" ./hack/smoke-gang-fairness.sh

metrics-smoke:
	./hack/smoke-metrics.sh

monitoring-smoke:
	./hack/validate-monitoring.sh

benchmark:
	./hack/benchmark.sh

benchmark-allocation:
	./hack/benchmark-allocation.sh

e2e: kind-up deploy crd-smoke topology-smoke device-smoke dra-smoke multi-node-dra-gang-smoke topology-aware-smoke fragmentation-smoke gang-smoke gang-fairness-smoke smoke metrics-smoke monitoring-smoke

kind-down:
	./hack/kind-down.sh

verify: fmt-check vet test test-race build
