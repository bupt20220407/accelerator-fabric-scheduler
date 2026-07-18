SHELL := /bin/sh
SCHEDULER_VERSION ?= v1.35.5-accelerator.0.1.0
VERSION_LDFLAG := -X k8s.io/component-base/version.gitVersion=$(SCHEDULER_VERSION)

.PHONY: fmt fmt-check vet test test-race build image kind-up deploy crd-smoke smoke e2e kind-down verify

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
	./hack/go.sh build -trimpath -ldflags '$(VERSION_LDFLAG)' -o bin/accelerator-scheduler ./cmd/scheduler

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

e2e: kind-up deploy crd-smoke smoke

kind-down:
	./hack/kind-down.sh

verify: fmt-check vet test build
