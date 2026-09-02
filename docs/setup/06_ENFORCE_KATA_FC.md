# Step 6: Enforce Kata-FC Runtime (MicroVM Isolation)

When running untrusted code or autonomous AI agents, standard Linux containers share the host kernel via cgroups and namespaces, which is vulnerable to kernel privilege escalation and breakout exploits.

By using **Kata Containers with Firecracker (`kata-fc`)**, each sandbox is executed inside a dedicated, hardware-isolated **Firecracker MicroVM** with its own guest kernel.

---

## 🎯 Architecture: Cluster-Side Enforcement

Campfire intentionally keeps the developer CLI clean (`campfire run` does not require users to pass `--runtime-class`). 

Instead, **Kubernetes enforces `kata-fc` cluster-side** so that every sandbox created by Campfire or Agent Sandbox unconditionally boots in a MicroVM.

---

## 1. Register the `kata-fc` RuntimeClass

Ensure your cluster nodes have Kata Containers installed, then register the `RuntimeClass`:

```yaml
# runtimeclass-kata-fc.yaml
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: kata-fc
handler: kata-fc
```

Apply:
```bash
kubectl apply -f runtimeclass-kata-fc.yaml
```

Verify the RuntimeClass is active:
```bash
kubectl get runtimeclass
```

---

## 2. Enforce `kata-fc` via Kyverno (Recommended)

Using **Kyverno**, a Kubernetes-native policy engine, you can automatically inject `runtimeClassName: kata-fc` into every sandbox pod created by Campfire:

```yaml
# policy-kata-fc.yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: enforce-kata-fc-sandboxes
  annotations:
    policies.kyverno.io/title: Enforce Kata Firecracker on Sandboxes
    policies.kyverno.io/description: Automatically assigns the kata-fc RuntimeClass to all Campfire sandboxes.
spec:
  rules:
  - name: mutate-runtimeclass
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

Apply the policy:
```bash
kubectl apply -f policy-kata-fc.yaml
```

---

## 3. Alternative: Enforce via containerd Default Runtime

If you want *every* pod in the cluster to use Kata Firecracker by default, configure containerd on the worker nodes:

Edit `/etc/containerd/config.toml`:
```toml
[plugins."io.containerd.grpc.v1.cri".containerd]
  default_runtime_name = "kata-fc"

  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-fc]
    runtime_type = "io.containerd.kata-fc.v2"
```

Restart containerd:
```bash
sudo systemctl restart containerd
```

---

## 4. Verifying MicroVM Isolation

Once a sandbox is started via `campfire run`:

```bash
campfire run my-box --image alpine -d
```

Verify that the underlying pod was assigned the `kata-fc` runtime:
```bash
kubectl get pod my-box -o jsonpath='{.spec.runtimeClassName}'
# Output: kata-fc
```

On the Kubernetes host node, verify the sandbox is running inside a Firecracker hypervisor process:
```bash
ps aux | grep firecracker
```

Even if a process escapes root privileges within the container, it remains completely trapped inside the Firecracker guest hypervisor boundary.
