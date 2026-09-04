#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="kampfire-e2e"
KUBECONFIG_FILE="/tmp/kampfire-e2e-kubeconfig"
VERSION="${AGENT_SANDBOX_VERSION:-v1.0.0}"
KIND_BIN="${KIND_BIN:-./bin/kind}"

# Ensure docker is available and running
if ! command -v docker &>/dev/null; then
    echo "❌ Error: docker is not installed. Please install Docker to run KinD E2E tests." >&2
    exit 1
fi

if ! docker info &>/dev/null; then
    echo "❌ Error: docker daemon is not running or current user lacks access." >&2
    echo "Please start the Docker daemon and verify permissions before running." >&2
    exit 1
fi

# Ensure kind binary is available
if ! command -v "$KIND_BIN" &>/dev/null && [ ! -x "$KIND_BIN" ]; then
    echo "kind binary not found at $KIND_BIN, downloading..."
    mkdir -p ./bin
    curl -Lo ./bin/kind https://kind.sigs.k8s.io/dl/v0.24.0/kind-linux-amd64
    chmod +x ./bin/kind
    KIND_BIN="./bin/kind"
fi

cleanup() {
    echo ""
    echo "🧹 Tearing down KinD cluster: $CLUSTER_NAME..."
    "$KIND_BIN" delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
    rm -f "$KUBECONFIG_FILE"
    echo "✓ Environment cleaned up."
}
trap cleanup EXIT

echo "🔥 ==================================================="
echo "🔥 Kampfire E2E Test Runner (KinD)"
echo "🔥 ==================================================="

echo "==> Spinning up fresh KinD cluster: $CLUSTER_NAME..."
"$KIND_BIN" delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
"$KIND_BIN" create cluster --name "$CLUSTER_NAME" --kubeconfig "$KUBECONFIG_FILE"

export KAMPFIRE_KUBECONFIG="$KUBECONFIG_FILE"

echo "==> Deploying Kubernetes Agent Sandbox ($VERSION)..."
kubectl apply -f "https://github.com/kubernetes-sigs/agent-sandbox/releases/download/${VERSION}/sandbox-with-extensions.yaml"

echo "==> Waiting for agent-sandbox-controller to be ready..."
kubectl wait --for=condition=Available deployment/agent-sandbox-controller -n agent-sandbox-system --timeout=120s

echo "==> Building campfire binary..."
go build -o bin/kampfire .

echo "==> Executing E2E Test Suite in Parallel..."
go test -v -parallel 8 -timeout 10m ./test/e2e/...

echo ""
echo "🎉 All E2E Tests Passed!"
