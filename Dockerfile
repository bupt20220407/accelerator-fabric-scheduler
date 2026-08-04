FROM node:24-bookworm-slim AS console-builder

WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm test \
    && npm run build

FROM golang:1.25.5-bookworm@sha256:d9132cce84391efab786495288756d60e1da215b1f94e87860aeefc3d4c45b6d AS builder

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY pkg ./pkg
ARG SCHEDULER_VERSION=v1.35.5-accelerator.0.10.0
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath \
    -ldflags="-s -w -X k8s.io/component-base/version.gitVersion=${SCHEDULER_VERSION}" \
    -o /out/accelerator-scheduler ./cmd/scheduler
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" \
    -o /out/synthetic-device-plugin ./cmd/synthetic-device-plugin
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" \
    -o /out/topology-controller ./cmd/topology-controller
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" \
    -o /out/topology-discovery-agent ./cmd/topology-discovery-agent
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" \
    -o /out/synthetic-workload ./cmd/synthetic-workload
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" \
    -o /out/synthetic-dra-driver ./cmd/synthetic-dra-driver
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" \
    -o /out/synthetic-dra-workload ./cmd/synthetic-dra-workload
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" \
    -o /out/synthetic-gang-workload ./cmd/synthetic-gang-workload
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" \
    -o /out/demo-console ./cmd/demo-console

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/accelerator-scheduler /accelerator-scheduler
COPY --from=builder /out/synthetic-device-plugin /synthetic-device-plugin
COPY --from=builder /out/topology-controller /topology-controller
COPY --from=builder /out/topology-discovery-agent /topology-discovery-agent
COPY --from=builder /out/synthetic-workload /synthetic-workload
COPY --from=builder /out/synthetic-dra-driver /synthetic-dra-driver
COPY --from=builder /out/synthetic-dra-workload /synthetic-dra-workload
COPY --from=builder /out/synthetic-gang-workload /synthetic-gang-workload
COPY --from=builder /out/demo-console /demo-console
COPY --from=console-builder /frontend/dist /frontend/dist
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/accelerator-scheduler"]
