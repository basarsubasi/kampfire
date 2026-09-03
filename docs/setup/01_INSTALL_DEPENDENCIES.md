# Step 1: Install Dependencies (Agent Sandbox CRDs & Controller)

Before Kampfire can provision containers, your cluster must have the official Kubernetes SIG **`agent-sandbox`** Custom Resource Definitions (CRDs) and controller installed.

---

## Prerequisites
* A running Kubernetes cluster (v1.28+)
* Cluster-admin access via `kubectl`

---

## 1. Deploy Agent Sandbox Controller & CRDs

Deploy the latest official release (`v1.0.0`) from Kubernetes SIGs:

```bash
export VERSION="v1.0.0"

kubectl apply -f https://github.com/kubernetes-sigs/agent-sandbox/releases/download/${VERSION}/sandbox-with-extensions.yaml
```

---

## 2. Verify CRD Installation

Ensure the four core Custom Resource Definitions are registered:

```bash
kubectl get crds | grep -E "agents\.x-k8s\.io"
```

You should see:
* `sandboxes.agents.x-k8s.io` — The primary sandbox definition.
* `sandboxclaims.extensions.agents.x-k8s.io` — Warmpool claims and fast-boot allocations.
* `sandboxtemplates.extensions.agents.x-k8s.io` — Reusable sandbox templates.
* `sandboxwarmpools.extensions.agents.x-k8s.io` — Pre-warmed sandbox pools.

---

## 3. Verify Controller Pod

Check that the controller manager is running healthy in the `agent-sandbox-system` namespace:

```bash
kubectl get pods -n agent-sandbox-system
```

Wait until the deployment condition is `Available`:
```bash
kubectl wait --for=condition=Available deployment/agent-sandbox-controller -n agent-sandbox-system --timeout=60s
```

Once the controller is ready, proceed to [Step 2: Create Tight Kubeconfig for Users](02_CREATE_TIGHT_KUBECONFIG.md).
