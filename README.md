# kampfire
A developer-first, Docker-style CLI for Kubernetes Agent Sandboxes.

## Prerequisites

Before using `kampfire`, you will need:
- **Kubernetes Cluster** with [Kubernetes Agent Sandbox](https://github.com/kubernetes-sigs/agent-sandbox) installed.
- **Cluster Access**: A kubeconfig pointing to your cluster and namespace (with a valid token or credentials).

> **Tip:** To set everything up automatically with Ansible (Kata Containers, Firecracker, Cilium CNI, and Agent Sandbox), refer to **[kata-fc-cilium](https://github.com/basarsubasi/kata-fc-cilium)**.

> **Info:** To quickly provision a new tenant user (namespace, ServiceAccount, RBAC, token, and tight kubeconfig), run [`scripts/provision-user.sh`](scripts/provision-user.sh):
> ```bash
> ./scripts/provision-user.sh <admin-kubeconfig> <username> <namespace>
> ```

## Quick Start

### 1. Install the CLI

#### macOS (arm64)
```bash
curl -Lo kampfire https://github.com/basarsubasi/kampfire/releases/download/1.2.1/kampfire-1.2.1-darwin-arm64
chmod +x kampfire
sudo mv kampfire /usr/local/bin/kampfire
```

#### Linux (amd64)
```bash
curl -Lo kampfire https://github.com/basarsubasi/kampfire/releases/download/1.2.1/kampfire-1.2.1-linux-amd64
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


Inspect your active configuration and sources at any time:
```bash
kampfire config
```

---

## Commands

| Command | Description | Example |
| :--- | :--- | :--- |
| `run` | Launch a sandbox | `kampfire run --image alpine -it /bin/sh` |
| `exec` | Run a command / attach shell | `kampfire exec -it my-sandbox /bin/bash` |
| `logs` | Fetch or stream container logs | `kampfire logs -f --tail 50 my-sandbox` |
| `ps` | List sandboxes and status | `kampfire ps` |
| `cp` | Copy files to/from a sandbox | `kampfire cp ./app.py my-sandbox:/workspace/` |
| `port-forward` | Forward local ports (WebSocket) | `kampfire port-forward my-sandbox 8080:80` |
| `ide` | Open VS Code / Antigravity IDE | `kampfire ide code my-sandbox` |
| `stop` / `start` | Suspend / resume a sandbox | `kampfire stop my-sandbox` |
| `restart` | Stop + start in one step | `kampfire restart my-sandbox` |
| `rm` | Delete sandboxes | `kampfire rm -a` |
| `top` | Live CPU & memory usage | `kampfire top -w` |
| `config` | View / set configuration | `kampfire config set --kubeconfig ~/k8s.yaml` |


### Examples

```bash
# Interactive Alpine shell
kampfire run --image alpine -it /bin/sh

# Detached with resource limits, published port, and persistent storage
kampfire run --name my-agent --image python:3.12 \
  --cpu 500m --memory 1Gi -p 8080:80 --persist /workspace -d

# Inject SSH keys and clone a private repo on startup
kampfire run --image alpine/git --with-private-ssh-keys \
  --clone-repo "git@github.com:org/repo.git" -it

# Pull from a private registry
kampfire run --image ghcr.io/org/agent:v1 --with-pull-secret ghcr-creds -d

# Exec, copy, port-forward, and IDE
kampfire exec my-agent uname -a
kampfire cp ./src my-agent:/workspace/src
kampfire port-forward my-agent 8080:80 5432:5432
kampfire ide code my-agent

# Lifecycle
kampfire stop -a                     # suspend all sandboxes
kampfire start my-agent              # resume
kampfire rm my-agent                 # delete
```