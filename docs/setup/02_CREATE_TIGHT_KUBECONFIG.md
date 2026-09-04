# Step 2: Create Tight Kubeconfig for Users

A **tight kubeconfig** provides developers with cluster connectivity (the API server URL and public CA certificate) and sets their active namespace—**without embedding any cluster-admin credentials, private keys, or elevated rights**.

---

## 🎯 The Philosophy of a Tight Kubeconfig

In a production or shared cluster:
1. **Zero High-Privilege Secrets**: Never hand users certificates or tokens from `kube-admin` or cluster-level roles.
2. **Server-Side Security**: All permissions are enforced by the Kubernetes API server using the user's scoped token (see [Step 4](04_MINT_API_TOKENS.md)).
3. **Public Connection Info Only**: The cluster's HTTPS endpoint and public TLS CA certificate are non-sensitive and only ensure secure TLS verification.

---

## 🛠️ Automated Generator Script

Use the following bash script to generate a scoped, tight kubeconfig for any user:

```bash
#!/usr/bin/env bash
set -euo pipefail

TENANT_NS="${1:-team-alice}"
TENANT_USER="${2:-alice}"
OUTPUT_FILE="kampfire-${TENANT_USER}.yaml"

# 1. Extract cluster API server endpoint and CA cert from your admin kubeconfig
CLUSTER_SERVER=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
CLUSTER_CA=$(kubectl config view --minify --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')

# 2. Write the tight kubeconfig file
cat <<EOF > "${OUTPUT_FILE}"
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: ${CLUSTER_CA}
    server: ${CLUSTER_SERVER}
  name: kampfire-cluster
contexts:
- context:
    cluster: kampfire-cluster
    namespace: ${TENANT_NS}
    user: ${TENANT_USER}
  name: default
current-context: default
users:
- name: ${TENANT_USER}
  user: {} # No credentials embedded by default
EOF

echo "✓ Created tight kubeconfig: ${OUTPUT_FILE}"
```

---

## 📦 Two Distribution Options

### Option A: Read-Only Base Kubeconfig + Shell Token (Recommended for CI/CD)
1. Distribute `kampfire-alice.yaml` with an empty `user: {}` section.
2. User sets their token as an environment variable in their shell:
   ```bash
   export KAMPFIRE_KUBECONFIG=~/kampfire-alice.yaml
   export KAMPFIRE_API_TOKEN="<minted-token>"
   kampfire ps
   ```
   Kampfire automatically injects `KAMPFIRE_API_TOKEN` as a Bearer token on top of the cluster endpoint.

---

### Option B: Self-Contained Kubeconfig (Recommended for Developers)
If you prefer single-file distribution, bake their minted token directly into their personal kubeconfig:

```yaml
# kampfire-alice.yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: <BASE64_CA>
    server: https://k8s.example.com:6443
  name: kampfire-cluster
contexts:
- context:
    cluster: kampfire-cluster
    namespace: team-alice
    user: alice
  name: default
current-context: default
users:
- name: alice
  user:
    token: <MINTED_SERVICE_ACCOUNT_TOKEN> # Token embedded directly
```

The user only needs one command:
```bash
export KAMPFIRE_KUBECONFIG=~/kampfire-alice.yaml
kampfire ps
```

Next, proceed to [Step 3: RBAC and Namespaces](03_RBAC_AND_NAMESPACES.md).
