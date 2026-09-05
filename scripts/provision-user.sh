#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Kampfire Tenant Provisioning Script
#
# 1. Ensures 'kampfire-user' ClusterRole exists in the cluster
# 2. Ensures the target Namespace and ServiceAccount exist
# 3. Creates the namespace-scoped RoleBinding
# 4. Mints a cryptographically signed API token via TokenRequest API
# 5. Generates a minimal, tight kubeconfig for the tenant user
# ==============================================================================

# Default values
ADMIN_KUBECONFIG=""
USER_NAME=""
NAMESPACE=""
OUTPUT_FILE=""
DURATION="${DURATION:-8760h}" # 1 year default
EMBED_TOKEN=true

usage() {
    cat <<EOF
Usage:
  $(basename "$0") <kubeconfig-path> <user-name> <namespace-name> [output-file]
  $(basename "$0") -k <kubeconfig> -u <user> -n <namespace> [-o <output-file>] [-d <duration>]

Arguments:
  kubeconfig-path   Path to cluster-admin kubeconfig
  user-name         Name of the tenant user / ServiceAccount (e.g., alice)
  namespace-name    Target tenant namespace (e.g., team-alice)
  output-file       Optional output path for minimal kubeconfig (default: kampfire-<user>.yaml)

Options:
  -k, --kubeconfig  Path to admin kubeconfig
  -u, --user        Username / ServiceAccount name
  -n, --namespace   Tenant namespace
  -o, --output      Path to output kubeconfig file
  -d, --duration    Token validity duration (e.g., 720h, 8760h, default: 8760h)
      --bare        Do not embed token into kubeconfig (for Option A: shell token distribution)
  -h, --help        Show this help message

Examples:
  $(basename "$0") ~/.kube/config alice team-alice
  $(basename "$0") -k /etc/kubernetes/admin.conf -u bob -n team-bob -o ~/kampfire-bob.yaml -d 720h
EOF
    exit 1
}

# Parse flags or positional arguments
POSITIONAL=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        -k|--kubeconfig)
            ADMIN_KUBECONFIG="$2"
            shift 2
            ;;
        -u|--user)
            USER_NAME="$2"
            shift 2
            ;;
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        -d|--duration)
            DURATION="$2"
            shift 2
            ;;
        --bare)
            EMBED_TOKEN=false
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            POSITIONAL+=("$1")
            shift
            ;;
    esac
done

# Fill from positional arguments if not provided via flags
if [[ -z "$ADMIN_KUBECONFIG" && ${#POSITIONAL[@]} -gt 0 ]]; then
    ADMIN_KUBECONFIG="${POSITIONAL[0]}"
fi
if [[ -z "$USER_NAME" && ${#POSITIONAL[@]} -gt 1 ]]; then
    USER_NAME="${POSITIONAL[1]}"
fi
if [[ -z "$NAMESPACE" && ${#POSITIONAL[@]} -gt 2 ]]; then
    NAMESPACE="${POSITIONAL[2]}"
fi
if [[ -z "$OUTPUT_FILE" && ${#POSITIONAL[@]} -gt 3 ]]; then
    OUTPUT_FILE="${POSITIONAL[3]}"
fi

# Prompt interactively if still missing
if [[ -z "$ADMIN_KUBECONFIG" ]]; then
    read -rp "Enter path to admin kubeconfig: " ADMIN_KUBECONFIG
fi
if [[ -z "$USER_NAME" ]]; then
    read -rp "Enter tenant username (ServiceAccount): " USER_NAME
fi
if [[ -z "$NAMESPACE" ]]; then
    read -rp "Enter tenant namespace: " NAMESPACE
fi

if [[ -z "$OUTPUT_FILE" ]]; then
    OUTPUT_FILE="kampfire-${USER_NAME}.yaml"
fi

# Expand tilde in path if present
ADMIN_KUBECONFIG="${ADMIN_KUBECONFIG/#\~/$HOME}"
OUTPUT_FILE="${OUTPUT_FILE/#\~/$HOME}"

# Convert relative paths / filenames to absolute paths based on pwd
if [[ "$ADMIN_KUBECONFIG" != /* ]]; then
    ADMIN_KUBECONFIG="$(pwd)/${ADMIN_KUBECONFIG#./}"
fi
if [[ "$OUTPUT_FILE" != /* ]]; then
    OUTPUT_FILE="$(pwd)/${OUTPUT_FILE#./}"
fi

# Validate admin kubeconfig exists
if [[ ! -f "$ADMIN_KUBECONFIG" ]]; then
    echo "❌ Error: Admin kubeconfig file not found at: $ADMIN_KUBECONFIG" >&2
    exit 1
fi

# Helper function to run kubectl with the admin kubeconfig
k() {
    kubectl --kubeconfig="$ADMIN_KUBECONFIG" "$@"
}

echo "🔥 ======================================================="
echo "🔥 Kampfire Tenant Provisioner"
echo "🔥 ======================================================="
echo "• Admin Kubeconfig : $ADMIN_KUBECONFIG"
echo "• User             : $USER_NAME"
echo "• Namespace        : $NAMESPACE"
echo "• Token Duration   : $DURATION"
echo "• Output Kubeconfig: $OUTPUT_FILE"
echo ""

# --- 1. Ensure kampfire-user ClusterRole exists ---
echo "==> [1/5] Verifying 'kampfire-user' ClusterRole..."
cat <<'EOF' | k apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kampfire-user
rules:
  # 1. Agent Sandbox CRDs
  - apiGroups: ["agents.x-k8s.io"]
    resources: ["sandboxes"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["extensions.agents.x-k8s.io"]
    resources: ["sandboxclaims"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  # 2. Pod metadata, status & events
  - apiGroups: [""]
    resources: ["pods", "events", "persistentvolumeclaims"]
    verbs: ["get", "list", "watch"]
  # 3. Interactive PTY execution, command exec, logs, and port-forwarding
  - apiGroups: [""]
    resources: ["pods/exec", "pods/portforward", "pods/log"]
    verbs: ["create", "get"]
EOF

# --- 2. Ensure Namespace exists ---
echo "==> [2/5] Ensuring namespace '${NAMESPACE}' exists..."
k create namespace "$NAMESPACE" --dry-run=client -o yaml | k apply -f -

# --- 3. Ensure ServiceAccount and RoleBinding exist ---
echo "==> [3/5] Setting up ServiceAccount and namespace-scoped RoleBinding..."
k -n "$NAMESPACE" create serviceaccount "$USER_NAME" --dry-run=client -o yaml | k apply -f -

k -n "$NAMESPACE" create rolebinding "${USER_NAME}-kampfire" \
    --clusterrole=kampfire-user \
    --serviceaccount="${NAMESPACE}:${USER_NAME}" \
    --dry-run=client -o yaml | k apply -f -

# --- 4. Mint Scoped Bearer Token ---
echo "==> [4/5] Minting API token (valid for ${DURATION})..."
API_TOKEN=$(k -n "$NAMESPACE" create token "$USER_NAME" --duration="$DURATION")

# --- 5. Extract Cluster CA & Server Endpoint and Generate Minimal Kubeconfig ---
echo "==> [5/5] Generating minimal tight kubeconfig: ${OUTPUT_FILE}..."

CLUSTER_SERVER=$(k config view --minify -o jsonpath='{.clusters[0].cluster.server}')
CLUSTER_CA=$(k config view --minify --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')

if [[ -z "$CLUSTER_CA" ]]; then
    CA_PATH=$(k config view --minify -o jsonpath='{.clusters[0].cluster.certificate-authority}')
    if [[ -n "$CA_PATH" && -f "$CA_PATH" ]]; then
        CLUSTER_CA=$(base64 -w 0 "$CA_PATH" 2>/dev/null || base64 "$CA_PATH" | tr -d '\n')
    fi
fi

if [[ -z "$CLUSTER_SERVER" ]]; then
    echo "❌ Error: Could not determine cluster server endpoint from $ADMIN_KUBECONFIG" >&2
    exit 1
fi

mkdir -p "$(dirname "$OUTPUT_FILE")"

if [[ "$EMBED_TOKEN" == true ]]; then
    # Self-Contained Kubeconfig (Option B)
    cat <<EOF > "$OUTPUT_FILE"
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
    namespace: ${NAMESPACE}
    user: ${USER_NAME}
  name: default
current-context: default
users:
- name: ${USER_NAME}
  user:
    token: ${API_TOKEN}
EOF
else
    # Bare Kubeconfig without embedded credentials (Option A)
    cat <<EOF > "$OUTPUT_FILE"
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
    namespace: ${NAMESPACE}
    user: ${USER_NAME}
  name: default
current-context: default
users:
- name: ${USER_NAME}
  user: {}
EOF
fi

chmod 0600 "$OUTPUT_FILE"

echo ""
echo "✅ Provisioning complete!"
echo "-------------------------------------------------------"
echo "Minimal Kubeconfig: ${OUTPUT_FILE}"
echo "Namespace Scope   : ${NAMESPACE}"
echo "ServiceAccount    : ${USER_NAME}"
echo "Token Expiry      : ${DURATION}"
echo "-------------------------------------------------------"
echo ""
echo "🚀 To use this configuration:"
echo ""
echo "  export KAMPFIRE_KUBECONFIG=\"${OUTPUT_FILE}\""
if [[ "$EMBED_TOKEN" == false ]]; then
    echo "  export KAMPFIRE_API_TOKEN=\"${API_TOKEN}\""
fi
echo "  kampfire ps"
echo ""
