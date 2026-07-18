FROM golang:1.25.5-bookworm@sha256:d9132cce84391efab786495288756d60e1da215b1f94e87860aeefc3d4c45b6d AS builder

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SCHEDULER_VERSION=v1.35.5-accelerator.0.1.0
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X k8s.io/component-base/version.gitVersion=${SCHEDULER_VERSION}" \
    -o /out/accelerator-scheduler ./cmd/scheduler

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/accelerator-scheduler /accelerator-scheduler
USER 65532:65532
ENTRYPOINT ["/accelerator-scheduler"]
