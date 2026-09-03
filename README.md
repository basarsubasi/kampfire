# campfire
A developer-first, Docker-style CLI for Kubernetes Agent Sandboxes.

---

## Quick Start

### 1. Install the CLI

```bash
git clone https://github.com/your-org/campfire.git
cd campfire
go build -o bin/campfire .
sudo mv bin/campfire /usr/local/bin/campfire
```

### 2. Connect to your Cluster

Campfire discovers your cluster endpoint and namespace from your kubeconfig, and authenticates API calls using either the kubeconfig's credentials or a scoped bearer token.

You can configure them permanently or per shell session:

```bash
# Option A: Save default settings (stored in ~/.config/campfire/config.json)
campfire config set --kubeconfig ~/campfire-user.yaml
campfire config set --token "<your-token>"

# Option B: Use environment variables (takes precedence over saved config)
export KUBECONFIG=~/campfire-user.yaml
export CAMPFIRE_API_TOKEN="<your-token>"
```

Inspect your active configuration and sources at any time:
```bash
campfire config
```

> **Cluster Administrators**: If you need to set up the cluster, RBAC, tokens, quotas, or MicroVM runtimes, see the **[Cluster Setup Guide](docs/setup/README.md)**.

---

## Command Walkthrough

### 1. Launch a Sandbox (`run`)
Create and start a new containerized sandbox with standard Docker flags:

```bash
# Run an interactive shell in an Alpine container
campfire run --image alpine -it /bin/sh

# Run in background (detached mode)
campfire run --name my-sandbox --image python:3.12 -d

# Override container command
campfire run --name web-server --image nginx:alpine -d
```

### 2. Execute Commands & Interactive Shells (`exec`)
Attach to running sandboxes with full PTY terminal emulation (VT100/ANSI, raw mode, window resizing, `Ctrl+C`):

```bash
# Interactive bash session
campfire exec -it my-sandbox /bin/bash

# Run one-off command
campfire exec my-sandbox uname -a
```

### 3. List Sandboxes (`ps`)
Display formatted status of running sandboxes in the active namespace:

```bash
# Human-readable table
campfire ps

# Output only 12-character short IDs (useful for scripting/piping)
campfire ps -q

# List sandboxes across all accessible namespaces
campfire ps -A
```

### 4. Bidirectional File Copy (`cp`)
Transfer files and folders to and from sandboxes using Docker `cp` syntax:

```bash
# Copy local file into sandbox
campfire cp ./app.py my-sandbox:/workspace/app.py

# Copy file from sandbox to host
campfire cp my-sandbox:/etc/os-release ./os-release

# Copy directories recursively
campfire cp ./src my-sandbox:/app/src
```

### 5. Port Forwarding (`port-forward`)
Forward local ports to a sandbox container over a secure SPDY tunnel:

```bash
# Forward local port 8080 to container port 80
campfire port-forward my-sandbox 8080:80

# Forward local port 3000 to container port 3000 (shorthand)
campfire port-forward my-sandbox 3000

# Forward multiple ports simultaneously (using name or short ID)
campfire port-forward 0cceb66ac7b7 8080:80 5432:5432
```
*Aliases: `campfire port-forward`, `campfire forward`, `campfire pf`.*

### 6. One-Click Desktop VS Code (`ide vscode`)
Connect your desktop VS Code directly to a remote sandbox:

```bash
# Open directly in desktop VS Code (default)
campfire ide vscode my-sandbox

# Open in web browser instead
campfire ide vscode my-sandbox --browser
```
> Campfire automatically checks for `code-server` in the container, installs it if missing, creates a secure SPDY tunnel, and launches your desktop VS Code.

### 7. Cleanup (`rm`)
Remove one or more sandboxes by name or ID:

```bash
# Remove specific sandboxes
campfire rm my-sandbox 0cceb66ac7b7

# Batch remove all sandboxes in current namespace
campfire rm $(campfire ps -q)
```

---

## 🧪 Testing & CI/CD

Campfire includes an automated parallel end-to-end (E2E) test suite covering RBAC isolation, unauthorized operation rejection (401/403), file transfer, resource quotas, and port forwarding.

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