# Step 5: Limit Resources (ResourceQuotas)

To prevent resource exhaustion and noisy-neighbor issues in multi-tenant environments, administrators can enforce hard limits on the number of concurrent sandboxes and compute resources each user can consume.

---

## 1. Apply a ResourceQuota to the Tenant Namespace

Because Agent Sandbox is a Kubernetes SIG Custom Resource, Kubernetes supports counting and capping it using `count/sandboxes.agents.x-k8s.io`:

```yaml
# sandbox-quota.yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: sandbox-quota
  namespace: team-alice
spec:
  hard:
    # 1. Cap max concurrent sandboxes
    count/sandboxes.agents.x-k8s.io: "3"
    # 2. Cap max underlying pods
    count/pods: "3"
    # 3. Optional: cap aggregate CPU & memory
    limits.cpu: "8"
    limits.memory: "16Gi"
    requests.cpu: "4"
    requests.memory: "8Gi"
```

Apply the quota to the tenant namespace:
```bash
kubectl apply -f sandbox-quota.yaml
```

---

## 2. Inspecting Active Quotas

Administrators can monitor a tenant's resource utilization with `kubectl`:

```bash
kubectl get resourcequota -n team-alice
```

Output:
```
NAME            REQUEST                                                LIMIT   AGE
sandbox-quota   count/pods: 1/3, count/sandboxes.agents.x-k8s.io: 1/3   3/3     10m
```

---

## 3. What Happens When a User Exceeds the Limit?

When a user attempts to create a 4th sandbox beyond their limit of 3:

```bash
$ kampfire run my-extra-box --image alpine
• Provisioning sandbox my-extra-box (alpine)...
✗ sandbox limit reached in namespace team-alice (ResourceQuota exceeded)
  Use 'kampfire ps' and 'kampfire rm' to free up capacity
```

Kampfire catches the server-side rejection from Kubernetes and provides clear, actionable instructions to the user.

---

## 4. Tiered Quota Examples

You can create different tiers for teams by applying custom quotas per namespace:

* **Free / Developer Tier**: `count/sandboxes.agents.x-k8s.io: "1"`
* **Team Tier**: `count/sandboxes.agents.x-k8s.io: "5"`
* **CI/CD Automation Tier**: `count/sandboxes.agents.x-k8s.io: "20"`

Next, proceed to [Step 6: Enforce Kata-FC Runtime](06_ENFORCE_KATA_FC.md).
