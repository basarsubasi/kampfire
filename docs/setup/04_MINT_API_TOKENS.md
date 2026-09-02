# Step 4: Mint API Tokens

Campfire authenticates to Kubernetes using cryptographically signed **Bearer Tokens** minted from Kubernetes ServiceAccounts via the official `TokenRequest` API.

---

## 1. Minting a Scoped Token

Administrators generate time-bound tokens using `kubectl create token`:

```bash
# Mint a token valid for 30 days (720 hours)
kubectl create token alice -n team-alice --duration=720h

# Mint an ephemeral token for CI/CD or temporary testing (1 hour)
kubectl create token alice -n team-alice --duration=1h

# Mint a long-lived developer token (1 year)
kubectl create token alice -n team-alice --duration=8760h
```

The output is an OIDC-compliant JWT bearer token:
```
eyJhbGciOiJSUzI1NiIsImtpZCI6Ij...
```

---

## 2. Setting the Token for Campfire

Users can authenticate using one of the following methods:

### Method 1: Environment Variable (Recommended for Shell Sessions & CI)
```bash
export CAMPFIRE_API_TOKEN="<minted-token>"
```
Campfire automatically detects this variable and attaches `Authorization: Bearer <token>` to all Kubernetes API calls.

### Method 2: CLI Configuration
```bash
campfire config set --token "<minted-token>"
```
Stores the token securely in `~/.config/campfire/config.json`.

### Method 3: Embedded in Kubeconfig
Bake the token into the `users` section of the tenant's kubeconfig (see [Step 2](02_CREATE_TIGHT_KUBECONFIG.md)).

---

## 3. Verifying Authentication

To test that the token works and is correctly scoped:

```bash
# Inspect your active configuration
campfire config

# Should display:
# API Token:       eyJhbG... (from $CAMPFIRE_API_TOKEN)
# Namespace:       team-alice (from context)
```

Run a quick status check:
```bash
campfire ps
```

---

## 4. Token Rotation and Revocation

* **Automatic Expiry**: The token automatically invalidates when its `--duration` expires.
* **Instant Revocation**: If a token is compromised or a developer leaves, deleting the `ServiceAccount` or rotating the cluster's ServiceAccount signing key immediately invalidates all issued tokens:
  ```bash
  # Delete and recreate to instantly invalidate all existing tokens
  kubectl delete serviceaccount alice -n team-alice
  kubectl create serviceaccount alice -n team-alice
  ```

Next, proceed to [Step 5: Limit Resources](05_LIMIT_RESOURCES.md).
