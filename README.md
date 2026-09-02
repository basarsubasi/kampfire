# 🔥 campfire

> **A developer-first, Docker-style CLI for Kubernetes Agent Sandboxes (`agents.x-k8s.io`).**

Campfire brings the familiar ergonomics of Docker directly to Kubernetes Agent Sandboxes. It features realistic interactive PTY terminals, zero-configuration token injection, dynamic namespace scoping, and one-command browser VS Code launch.

---

## ⚡ Quick Start

### 1. Prerequisites
You must have access to a working Kubernetes cluster (v1.28+) with `kubectl` configured.

### 2. Deploy Kubernetes Agent Sandbox
Deploy the official Kubernetes SIG `agent-sandbox` controller and Custom Resource Definitions (CRDs):

```bash
# Replace "vX.Y.Z" with a specific version tag (e.g., "v0.1.0") from
# https://github.com/kubernetes-sigs/agent-sandbox/releases
export VERSION="v0.1.0"

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

### 7. Cleanup
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
* **No Shared Secrets**: Users only possess their scoped ServiceAccount or OIDC token.

---

## 🛠️ Global Flags

* `-n, --namespace string`: Override target Kubernetes namespace.
* `--config string`: Path to custom Campfire configuration file (defaults to `~/.config/campfire/config.json`).
* `-v, --verbose`: Enable verbose debug output.
