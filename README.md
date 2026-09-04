# kampfire
A developer-first, Docker-style CLI for Kubernetes Agent Sandboxes.

## Quick Start

### 1. Install the CLI

```bash
git clone https://github.com/your-org/kampfire.git
cd kampfire
go build -o bin/kampfire .
sudo mv bin/kampfire /usr/local/bin/kampfire
```

### 2. Connect to your Cluster

Kampfire discovers your cluster endpoint and namespace from your kubeconfig, and authenticates API calls using either the kubeconfig's credentials or a scoped bearer token.

You can configure them permanently or per shell session:

```bash
# Option A: Save default settings (stored in ~/.config/kampfire/config.json)
kampfire config set --kubeconfig ~/kampfire-user.yaml
kampfire config set --token "<your-token>"

# Option B: Use environment variables (takes precedence over saved config)
export KUBECONFIG=~/kampfire-user.yaml
export KAMPFIRE_API_TOKEN="<your-token>"
```

Inspect your active configuration and sources at any time:
```bash
kampfire config
```

> **Cluster Administrators**: If you need to set up the cluster, RBAC, tokens, quotas, or MicroVM runtimes, see the **[Cluster Setup Guide](docs/setup/README.md)**.

---

## Command Walkthrough

### 1. Launch a Sandbox (`run`)
Create and start a new containerized sandbox with standard Docker flags:

```bash
# Run an interactive shell in an Alpine container
kampfire run --image alpine -it /bin/sh

# Run in background (detached mode)
kampfire run --name my-sandbox --image debian:bookworm -d

# Inject host SSH keys to access private repositories
kampfire run --image alpine/git --with-private-ssh-keys -it

# Clone a git repository into sandbox home upon creation
kampfire run --image alpine/git --clone-repo "https://github.com/org/repo.git" -it

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
Display formatted status of running sandboxes in the active namespace:

```bash
# Human-readable table
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
Connect your desktop VS Code directly to a remote sandbox:

```bash
# Open directly in desktop VS Code (default)
kampfire ide vscode my-sandbox

# Open in web browser instead
kampfire ide vscode my-sandbox --browser
```
> Kampfire automatically checks for `code-server` in the container, installs it if missing, creates a secure SPDY tunnel, and launches your desktop VS Code.

### 8. Cleanup (`rm`)
Remove one or more sandboxes by name or ID:

```bash
# Remove specific sandboxes
kampfire rm my-sandbox 0cceb66ac7b7

# Batch remove all sandboxes in current namespace
kampfire rm $(kampfire ps -q)
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