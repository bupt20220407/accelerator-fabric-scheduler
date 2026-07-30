#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CODEGEN_VERSION="${CODEGEN_VERSION:-v0.35.5}"
CODEGEN_PKG="$(go env GOMODCACHE)/k8s.io/code-generator@${CODEGEN_VERSION}"

go mod download "k8s.io/code-generator@${CODEGEN_VERSION}"
source "${CODEGEN_PKG}/kube_codegen.sh"

THIS_PKG="github.com/bupt20220407/accelerator-fabric-scheduler"

kube::codegen::gen_helpers \
  --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
  "${SCRIPT_ROOT}/pkg/apis"
kube::codegen::gen_client \
  --with-watch \
  --output-dir "${SCRIPT_ROOT}/pkg/generated" \
  --output-pkg "${THIS_PKG}/pkg/generated" \
  --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
  "${SCRIPT_ROOT}/pkg/apis"
