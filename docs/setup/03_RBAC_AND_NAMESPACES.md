# Step 3: RBAC and Namespaces

Campfire delegates 100% of user authentication and authorization to **Kubernetes native RBAC**. Users only possess permissions inside their assigned tenant namespace.

---

## 1. Create the `campfire-user` ClusterRole

Apply the standard `campfire-user` ClusterRole once to your cluster. This defines the exact set of privileges required to run, list, exec, copy, and port-forward sandboxes:

```yaml
# campfire-user-clusterrole.yaml
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
  # 3. Interactive PTY execution, command exec, and port-forwarding
  - apiGroups: [""]
    resources: ["pods/exec", "pods/portforward"]
    verbs: ["create", "get"]
```

Apply with:
```bash
kubectl apply -f campfire-user-clusterrole.yaml
```

---

## 2. Provision Tenant Namespace & ServiceAccount

For each tenant or developer (e.g. `alice` in team `team-alice`):

```bash
# 1. Create tenant namespace
kubectl create namespace team-alice

# 2. Create tenant ServiceAccount
kubectl create serviceaccount alice -n team-alice
```

---

## 3. Bind ServiceAccount via Namespace-Scoped RoleBinding

Bind the `alice` ServiceAccount to the `campfire-user` ClusterRole **inside her namespace only**:

```bash
kubectl create rolebinding alice-campfire \
  --clusterrole=campfire-user \
  --serviceaccount=team-alice:alice \
  -n team-alice
```

> [!IMPORTANT]
> Use a **`RoleBinding`**, not a `ClusterRoleBinding`.
> A `RoleBinding` restricts the ClusterRole's permissions strictly to the target namespace (`team-alice`). Alice will have **zero access** to resources in other namespaces or cluster-level objects.

---

## 4. Multi-Tenant Cross-Namespace Isolation

Because permissions are bound via a namespace `RoleBinding`:
* **Alice in `team-alice`** can create, list, exec, port-forward, and remove sandboxes in `team-alice`.
* If Alice attempts to access another tenant's namespace (e.g. `campfire -n team-bob ps` or `campfire -n team-bob port-forward ...`), the Kubernetes API server unconditionally denies the request with:
  ```
  403 Forbidden: User "system:serviceaccount:team-alice:alice" cannot list resource "sandboxes" in the namespace "team-bob"
  ```

Next, proceed to [Step 4: Mint API Tokens](04_MINT_API_TOKENS.md).
