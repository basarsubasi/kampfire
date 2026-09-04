# kampfire
A developer-first, Docker-style CLI for Kubernetes Agent Sandboxes.

## Prerequisites

Before using `kampfire`, you will need:
- **Kubernetes Cluster** with [Kubernetes Agent Sandbox](https://github.com/kubernetes-sigs/agent-sandbox) installed.
- **Cluster Access**: A kubeconfig pointing to your cluster and namespace (with a valid token or credentials).

> **Tip:**: To install the CRDs/controller, RBAC, tokens, and quotas, follow the **[Cluster Setup Guide](docs/setup/README.md)**.

## Quick Start

### 1. Install the CLI

#### macOS (arm64)
```bash
curl -Lo kampfire https://github.com/basarsubasi/kampfire/releases/download/1.0.0/kampfire-1.0.0-darwin-arm64
chmod +x kampfire
sudo mv kampfire /usr/local/bin/kampfire
```

#### Linux (amd64)
```bash
curl -Lo kampfire https://github.com/basarsubasi/kampfire/releases/download/1.0.0/kampfire-1.0.0-linux-amd64
chmod +x kampfire
sudo mv kampfire /usr/local/bin/kampfire
```

### 2. Connect to your Cluster

Kampfire discovers your cluster endpoint and namespace from your kubeconfig, and authenticates API calls using either the kubeconfig's credentials or a scoped bearer token.

You can configure them permanently or per shell session:

```bash
# Option A: Save default settings (stored in ~/.config/kampfire/config.json)
kampfire config set --kubeconfig ~/kampfire-user.yaml
kampfire config set --token "<your-token>"

# Option B: Use environment variables (takes precedence over saved config)
export KAMPFIRE_KUBECONFIG=~/kampfire-user.yaml
export KAMPFIRE_API_TOKEN="<your-token>"
```

> **Note**: Kampfire uses `KAMPFIRE_KUBECONFIG` (rather than standard `KUBECONFIG`) so your Kampfire cluster and credentials remain cleanly isolated from your shell's default Kubernetes environment.

Inspect your active configuration and sources at any time:
```bash
kampfire config
```

---

## Command Walkthrough

### 1. Launch a Sandbox (`run`)
Create and start a new containerized sandbox with standard Docker flags:

```bash
# Run an interactive shell in an Alpine container
kampfire run --image alpine -it /bin/sh

# Run in background (detached mode)
kampfire run --name my-sandbox --image debian:bookworm -d

# Set CPU and Memory resource limits
kampfire run --image python:3.12 --cpu 500m --memory 1Gi -it

# Publish container port(s) to localhost
kampfire run --image nginx:alpine -p 8080:80 -d

# Inject host SSH keys to access private repositories
kampfire run --image alpine/git --with-private-ssh-keys -it

# Clone a git repository into sandbox home upon creation
kampfire run --image alpine/git --clone-repo "https://github.com/org/repo.git" -it

# Attach persistent volume to /workspace (retains files across stop/start)
kampfire run --image python:3.12 --persist /workspace -it

# Attach persistent storage with custom size (e.g. 10Gi)
kampfire run --image python:3.12 --persist /data --persist-size 10Gi -d

# Pull image from private registry using a Kubernetes imagePullSecret
kampfire run --image ghcr.io/org/private-agent:v1 --with-pull-secret ghcr-creds -it
```

### 2. Execute Commands & Interactive Shells (`exec`)
Attach to running sandboxes with full PTY terminal emulation (VT100/ANSI, raw mode, window resizing, `Ctrl+C`):

```bash
# Interactive bash session
kampfire exec -it my-sandbox /bin/bash

# Run one-off command
kampfire exec my-sandbox uname -a
```

### 3. Fetch & Stream Logs (`logs`)
Inspect container standard output and standard error, tail recent lines, view initial lines, or stream logs in real time:

```bash
# Fetch all logs for a sandbox
kampfire logs my-sandbox

# Follow logs in real time (stream until Ctrl+C)
kampfire logs -f my-sandbox

# Output only the last 50 lines
kampfire logs --tail 50 my-sandbox

# Output only the first 20 lines
kampfire logs --head 20 my-sandbox

# Follow logs starting from the last 10 lines
kampfire logs -f --tail 10 my-sandbox
```

### 4. List Sandboxes (`ps`)
Display formatted status, IP, published container ports, and active port-forward sessions (`127.0.0.1:8080->80/TCP`) of running sandboxes in the active namespace:

```bash
# Human-readable table (shows live port-forwards and container ports)
kampfire ps

# Output only 12-character short IDs (useful for scripting/piping)
kampfire ps -q

# List sandboxes across all accessible namespaces
kampfire ps -A
```

### 5. Bidirectional File Copy (`cp`)
Transfer files and folders to and from sandboxes using Docker `cp` syntax:

```bash
# Copy local file into sandbox
kampfire cp ./app.py my-sandbox:/workspace/app.py

# Copy file from sandbox to host
kampfire cp my-sandbox:/etc/os-release ./os-release

# Copy directories recursively
kampfire cp ./src my-sandbox:/app/src
```

### 6. Port Forwarding (`port-forward`)
Forward local ports to a sandbox container over a secure SPDY tunnel:

```bash
# Forward local port 8080 to container port 80
kampfire port-forward my-sandbox 8080:80

# Forward local port 3000 to container port 3000 (shorthand)
kampfire port-forward my-sandbox 3000

# Forward multiple ports simultaneously (using name or short ID)
kampfire port-forward 0cceb66ac7b7 8080:80 5432:5432
```
*Aliases: `kampfire port-forward`, `kampfire forward`, `kampfire pf`.*

### 7. IDE Integration (`ide`)
Connect your desktop VS Code or Antigravity IDE directly to a remote sandbox:

```bash
# Open directly in desktop VS Code (via kubectl exec)
kampfire ide code my-sandbox

# Open directly in desktop Antigravity IDE (via kubectl exec)
kampfire ide agy my-sandbox

# Open in web browser instead (via code-server)
kampfire ide vscode my-sandbox --browser
kampfire ide agy my-sandbox --browser
```
> Kampfire connects your desktop IDE directly into the container's home directory over `kubectl exec`. With `--browser`, it runs `code-server` in the container and establishes a port-forwarded web session.

### 8. Stop, Start & Restart (`stop`, `start`, `restart`)
Suspend and resume sandboxes to drop CPU/memory consumption to zero while keeping persistent files intact:

```bash
# Stop a sandbox (frees compute quota, retains attached persistent volumes)
kampfire stop my-sandbox

# Stop multiple sandboxes or all in current namespace
kampfire stop sb-1 sb-2
kampfire stop -a

# Start / resume a stopped sandbox (re-attaches persistent volume)
kampfire start my-sandbox

# Start in background without waiting for readiness
kampfire start -d my-sandbox

# Restart a running or stopped sandbox
kampfire restart my-sandbox
```

### 9. Cleanup (`rm`)
Remove one or more sandboxes by name or ID:

```bash
# Remove specific sandboxes
kampfire rm my-sandbox 0cceb66ac7b7

# Remove all sandboxes in current namespace
kampfire rm -a
# or
kampfire rm --all
```

### 10. Monitor Resource Usage (`top`)
View real-time CPU and Memory utilization of your sandboxes:

```bash
# Snapshot resource usage for all sandboxes
kampfire top

# Watch live usage continuously
kampfire top -w

# View usage for a specific sandbox
kampfire top my-sandbox
```

### 11. Shell Autocompletion (`completion`)
Enable dynamic tab completion for commands, flags, and running sandbox names/IDs:

```bash
# Bash
source <(kampfire completion bash)

# Zsh
source <(kampfire completion zsh)

# Fish
kampfire completion fish | source
```

---

## 🧪 Testing & CI/CD

Kampfire includes an automated parallel end-to-end (E2E) test suite covering RBAC isolation, unauthorized operation rejection (401/403), file transfer, resource quotas, and port forwarding.

### Run in KinD (Automated from scratch)
Spins up a clean KinD cluster, installs Agent Sandbox `v1.0.0`, runs all test cases concurrently in parallel, and automatically tears down on completion:

```bash
make e2e
```

### Run Against Existing Cluster
Execute the E2E tests against your currently active `kubectl` context:

```bash
make test-e2e
```