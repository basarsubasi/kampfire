# 🚀 Production Cluster Setup Guide

This guide walks cluster administrators through setting up an enterprise-grade, secure, multi-tenant Kubernetes cluster for Campfire and Kubernetes SIG Agent Sandbox.

---

## Setup Steps

Follow the guides in sequence:

1. **[Step 1: Install Dependencies (CRDs & Controller)](01_INSTALL_DEPENDENCIES.md)**
   Deploy official `agent-sandbox` Custom Resource Definitions and controller manager.

2. **[Step 2: Create Tight Kubeconfig for Users](02_CREATE_TIGHT_KUBECONFIG.md)**
   Generate non-privileged, cluster-endpoint-only kubeconfigs for developers.

3. **[Step 3: RBAC and Namespaces](03_RBAC_AND_NAMESPACES.md)**
   Apply the `campfire-user` ClusterRole and create tenant namespaces with scoped RoleBindings.

4. **[Step 4: Mint API Tokens](04_MINT_API_TOKENS.md)**
   Mint cryptographically signed ServiceAccount bearer tokens and configure rotation.

5. **[Step 5: Limit Resources (ResourceQuotas)](05_LIMIT_RESOURCES.md)**
   Enforce hard limits on concurrent sandboxes, pods, CPU, and RAM per tenant.

6. **[Step 6: Enforce Kata-FC Runtime (MicroVM Isolation)](06_ENFORCE_KATA_FC.md)**
   Configure cluster-side admission policies to automatically run all sandboxes in dedicated Firecracker MicroVMs.

---

## Security Architecture Summary

```
+-------------------------------------------------------------------+
|                        DEVELOPER LAPTOP                           |
|                                                                   |
|   campfire run my-box --image python:3.12                         |
|   (Reads campfire-alice.yaml + CAMPFIRE_API_TOKEN)                |
+---------------------------------+---------------------------------+
                                  | HTTPS (Bearer Token)
                                  v
+-------------------------------------------------------------------+
|                     KUBERNETES CONTROL PLANE                      |
|                                                                   |
|   1. Authentication  -> Validates ServiceAccount token (alice)    |
|   2. Authorization   -> RoleBinding in team-alice (campfire-user) |
|   3. Admission/Quota -> ResourceQuota checks (max 3 sandboxes)    |
|   4. Mutation        -> Injects spec.runtimeClassName: kata-fc    |
+---------------------------------+---------------------------------+
                                  |
                                  v
+-------------------------------------------------------------------+
|                       WORKER NODE (KATA-FC)                       |
|                                                                   |
|   +-----------------------------------------------------------+   |
|   |                 Firecracker MicroVM (Guest)               |   |
|   |                                                           |   |
|   |   +---------------------+   +-------------------------+   |   |
|   |   |   Guest Kernel      |   |   Sandbox Container     |   |   |
|   |   +---------------------+   +-------------------------+   |   |
|   +-----------------------------------------------------------+   |
+-------------------------------------------------------------------+
```
