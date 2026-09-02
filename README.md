# campfire

> **A developer-first, Docker-style CLI for Kubernetes Agent Sandboxes.**

Campfire brings the familiar ergonomics of Docker directly to Kubernetes Agent Sandboxes. It features realistic interactive PTY terminals, zero-configuration token injection, dynamic namespace scoping, and one-command browser VS Code launch.

---

## Quick Start

### 1. Prerequisites
You must have access to a working Kubernetes cluster (v1.28+) with `kubectl` configured.

### 2. Deploy Kubernetes Agent Sandbox
Deploy the official Kubernetes SIG `agent-sandbox` controller and Custom Resource Definitions (CRDs):

```bash
# Replace "vX.Y.Z" with a specific version tag (e.g., "v1.0.0") from
# https://github.com/kubernetes-sigs/agent-sandbox/releases
export VERSION="v1.0.0"

kubectl apply -f https://github.com/kubernetes-sigs/agent-sandbox/releases/download/${VERSION}/sandbox-with-extensions.yaml
```

Verify that the controller pod is running:
```bash
kubectl get pods -n agent-sandbox-system
```

---

### 3. Setup Campfire RBAC & Mint Tokens

Campfire delegates 100% of its authentication and authorization to **native Kubernetes RBAC**. Users only need permissions in their assigned namespace.

#### Step 3.1: Create the Standard ClusterRole
Apply the `campfire-user` ClusterRole to your cluster once:

```yaml
# campfire-rbac.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: campfire-user
rules:
  # 1. Agent Sandbox CRDs
  - apiGroups: ["agents.x-k8s.io"]
    resources: ["sandboxes"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["extensions.agents.x-k8s.io"]
    resources: ["sandboxclaims"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  # 2. Pod metadata & status
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
  # 3. Interactive TTY, command exec, file copy, & IDE port-forwarding
  - apiGroups: [""]
    resources: ["pods/exec", "pods/portforward"]
    verbs: ["create", "get"]
```

Apply with:
```bash
kubectl apply -f campfire-rbac.yaml
```

#### Step 3.2: Create a Tenant Namespace & RoleBinding
For each tenant or developer (e.g., `alice` in namespace `team-alice`):

```bash
# 1. Create tenant namespace
kubectl create namespace team-alice

# 2. Create tenant ServiceAccount
kubectl create serviceaccount alice -n team-alice

# 3. Bind the ServiceAccount to campfire-user within their namespace only
kubectl create rolebinding alice-campfire \
  --clusterrole=campfire-user \
  --serviceaccount=team-alice:alice \
  -n team-alice
```

#### Step 3.3: Mint a Tenant API Token
Generate a scoped bearer token for the user:

```bash
# Mint a token valid for 1 year (8760h)
kubectl create token alice -n team-alice --duration=8760h
```

---

### 4. Build & Configure Campfire

#### Step 4.1: Build the CLI
```bash
git clone https://github.com/your-org/campfire.git
cd campfire
go build -o bin/campfire .
sudo mv bin/campfire /usr/local/bin/campfire
```

#### Step 4.2: Authenticate
You can authenticate via environment variable or the `config` command:

**Option A: Shell Environment Variable (Recommended)**
```bash
export CAMPFIRE_API_TOKEN="<minted-token>"
```
> Campfire automatically reuses the API server URL, CA certificate, and TLS settings from your active `kubeconfig`, injecting `CAMPFIRE_API_TOKEN` for authentication.

**Option B: CLI Config File**
```bash
campfire config set --token "<minted-token>"
```

#### Step 4.3: Namespace Resolution
Campfire resolves your active namespace dynamically in the following order:
1. `-n, --namespace <ns>` command-line flag.
2. The active context's namespace in `~/.kube/config` (or `$KUBECONFIG`).
3. Fallback to `default`.

---

## 🚀 Command Walkthrough

### 1. Interactive Run (Primary Hero Flow)
Launch a new isolated sandbox and immediately drop into a realistic interactive shell with PTY raw mode and dynamic window resizing:

```bash
# Launch an Alpine sandbox and attach immediately
campfire run --image alpine -it

# Launch an ephemeral sandbox that deletes itself on exit
campfire run --image alpine --rm -it
```

### 2. Detached Background Execution
Run containers in the background just like `docker run -d`:

```bash
campfire run my-box --image python:3.12 -d
```

### 3. List Running Sandboxes
Output aligns cleanly and displays 12-character short IDs by default:

```bash
campfire ps
```
```
SANDBOX ID     NAME           IMAGE    STATUS      AGE   IP
0cceb66ac7b7   test-sandbox   alpine    RUNNING    37m   10.244.2.3
```

List only IDs (ideal for scripting and batch cleanup):
```bash
campfire ps -q
```

### 4. Interactive & Non-Interactive Exec
Attach to existing containers using either the **name** or the **short ID**:

```bash
# Interactive shell
campfire exec -it 0cceb66ac7b7 /bin/sh
campfire exec -it test-sandbox /bin/sh

# Non-interactive command execution
campfire exec 0cceb66ac7b7 uname -a
```

### 5. Bidirectional File Copy (`cp`)
Transfer files and folders to and from sandboxes using Docker `cp` syntax:

```bash
# Copy local file into sandbox
campfire cp ./app.py 0cceb66ac7b7:/workspace/app.py

# Copy file from sandbox to host
campfire cp 0cceb66ac7b7:/etc/os-release ./os-release

# Copy directories recursively
campfire cp ./src test-sandbox:/app/src
```

### 6. One-Click Desktop VS Code
Launch your desktop VS Code connected directly to the sandbox:

```bash
# Open directly in desktop VS Code (default)
campfire ide vscode 0cceb66ac7b7

# Open in web browser instead
campfire ide vscode 0cceb66ac7b7 --browser
```
> Campfire detects if `code-server` is installed in the container, automatically downloads and installs it if missing, establishes a secure Kubernetes SPDY port-forward tunnel, and opens your desktop VS Code application.

### 7. Port Forwarding
Forward one or more local ports to a sandbox container over a secure SPDY tunnel:

```bash
# Forward local port 8080 to container port 80
campfire port-forward my-sandbox 8080:80

# Forward local port 3000 to container port 3000 (shorthand)
campfire port-forward my-sandbox 3000

# Forward multiple ports simultaneously (using name or short ID)
campfire port-forward 0cceb66ac7b7 8080:80 5432:5432
```

### 8. Cleanup
Remove one or more sandboxes by ID or name:

```bash
# Remove specific sandboxes
campfire rm 0cceb66ac7b7
campfire rm my-box test-sandbox

# Batch remove all sandboxes in current namespace
campfire rm $(campfire ps -q)
```

---

## 🔒 Security & Multi-Tenancy

* **Zero Custom Daemons**: Campfire is a pure client tool that talks directly to the standard Kubernetes API server.
* **Hard RBAC Enforcement**: Any request attempting to access or modify sandboxes outside the tenant's bound namespace is immediately rejected with a standard `403 Forbidden` by the Kubernetes API server.
* **No Shared Secrets**: Users only possess their scoped ServiceAccount token.

---

### 🔑 Generating Tenant-Scoped Kubeconfigs

Administrators can generate a single, self-contained kubeconfig file for each tenant. This file contains only public cluster connection details and the tenant's scoped token—with zero cluster-admin permissions:

```bash
#!/usr/bin/env bash
# generate-tenant-config.sh <namespace> <username> [duration]
TENANT_NS="${1:-team-alice}"
TENANT_USER="${2:-alice}"
DURATION="${3:-720h}" # Defaults to 30 days

# 1. Grab public cluster endpoint and CA cert from active cluster
CLUSTER_SERVER=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
CLUSTER_CA=$(kubectl config view --minify --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')

# 2. Mint tenant token
TOKEN=$(kubectl create token "${TENANT_USER}" -n "${TENANT_NS}" --duration="${DURATION}")

# 3. Generate ready-to-use tenant kubeconfig
OUTPUT_FILE="campfire-${TENANT_USER}.yaml"
cat <<EOF > "${OUTPUT_FILE}"
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: ${CLUSTER_CA}
    server: ${CLUSTER_SERVER}
  name: campfire-cluster
contexts:
- context:
    cluster: campfire-cluster
    namespace: ${TENANT_NS}
    user: ${TENANT_USER}
  name: default
current-context: default
users:
- name: ${TENANT_USER}
  user:
    token: ${TOKEN}
EOF

echo "✓ Generated tenant config: ${OUTPUT_FILE}"
echo "Distribute this file to ${TENANT_USER}. They can use it immediately with:"
echo "  export KUBECONFIG=~/${OUTPUT_FILE}"
echo "  campfire ps"
```

---

### 🛡️ Enforcing Hardware MicroVM Isolation with Kata Containers (`kata-fc`)

To protect the host kernel from untrusted user code or AI agents, sandboxes should run inside lightweight hardware-isolated MicroVMs powered by **Kata Containers with Firecracker (`kata-fc`)**.

Campfire intentionally keeps the developer CLI pure and simple (`campfire run` does not require users to pass `--runtime-class`). Instead, **Kubernetes enforces `kata-fc` cluster-side** so that *all* sandboxes automatically run inside isolated Firecracker MicroVMs.

#### Step 1: Register the `kata-fc` RuntimeClass
Apply the `kata-fc` RuntimeClass to your cluster:

```yaml
# runtimeclass-kata-fc.yaml
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: kata-fc
handler: kata-fc
```

```bash
kubectl apply -f runtimeclass-kata-fc.yaml
```

#### Step 2: Enforce `kata-fc` Cluster-Side
You can enforce `kata-fc` across all sandboxes using a standard Kubernetes mutating policy (e.g. via **Kyverno** or a **MutatingAdmissionWebhook**):

```yaml
# mutate-kata-fc.yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: enforce-kata-fc-sandboxes
spec:
  rules:
  - name: set-kata-fc-runtime
    match:
      resources:
        kinds:
        - Pod
        selector:
          matchLabels:
            agents.x-k8s.io/created-by: campfire
    mutate:
      patchStrategicMerge:
        spec:
          +(runtimeClassName): kata-fc
```

#### Why cluster-side enforcement is superior:
1. **Zero Developer Friction**: Developers run standard `campfire run --image python:3.12` without needing to remember infrastructure flags.
2. **Immutable Security Guarantee**: Individual developers cannot disable or bypass the VM boundary. Every sandbox is unconditionally jailed inside a dedicated Firecracker MicroVM hypervisor.
3. **Defense-in-Depth**: Even if a sandbox container suffers a root breakout or kernel exploit, it is trapped inside a dedicated guest kernel and virtual machine.

---

## ⚖️ Limiting Sandboxes per User (ResourceQuotas)

Cluster administrators can limit how many concurrent sandboxes or pods a user can run using standard Kubernetes `ResourceQuota` objects applied to their namespace:

```yaml
# sandbox-quota.yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: sandbox-quota
  namespace: team-alice
spec:
  hard:
    # Limit max number of concurrent sandboxes to 3
    count/sandboxes.agents.x-k8s.io: "3"
    # Optional: limit max total pods
    count/pods: "3"
```

Apply with:
```bash
kubectl apply -f sandbox-quota.yaml
```

When a user reaches their limit, Campfire catches the quota rejection from the Kubernetes API and provides clear, actionable feedback:
```
✗ sandbox limit reached in namespace team-alice (ResourceQuota exceeded)
  Use 'campfire ps' and 'campfire rm' to free up capacity
```

---

## 🧪 Testing & CI/CD

Campfire includes a comprehensive end-to-end (E2E) test suite covering:
* Sandbox provisioning & PTY/command execution
* `ps` table formatting, column alignment, and short IDs
* Bidirectional file copy (`cp`)
* Deletion & namespace cleanup (`rm`)
* Kubernetes `ResourceQuota` limit enforcement

### Running Tests Locally with KinD (Starts from scratch)
Spins up a temporary KinD cluster, installs Agent Sandbox, pre-loads the Alpine image for fast testing, executes the test suite, and automatically destroys the cluster on exit:

```bash
make e2e
```

### Running Tests Against an Existing Cluster
To execute tests against your currently active `kubectl` cluster:

```bash
make test-e2e
```

---

## 🛠️ Global Flags

* `-n, --namespace string`: Override target Kubernetes namespace.
* `--config string`: Path to custom Campfire configuration file (defaults to `~/.config/campfire/config.json`).
* `-v, --verbose`: Enable verbose debug output.

