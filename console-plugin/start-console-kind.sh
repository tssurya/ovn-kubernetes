#!/usr/bin/env bash
#
# Launches the OpenShift console (bridge) connected to a KIND cluster,
# with the VPC console plugin loaded from the local dev server.
#
# Prerequisites:
#   - kubectl configured for the KIND cluster
#   - docker or podman available
#   - plugin dev server running on port 9001  (yarn start)
#
# Usage:
#   ./start-console-kind.sh
#

set -euo pipefail

CONSOLE_IMAGE="${CONSOLE_IMAGE:-quay.io/openshift/origin-console:latest}"
CONSOLE_PORT="${CONSOLE_PORT:-9000}"
PLUGIN_NAME="${PLUGIN_NAME:-ovn-vpc-console-plugin}"
SA_NAME="console-sa"
SA_NAMESPACE="kube-system"

echo "=== Detecting KIND cluster API endpoint ==="
API_SERVER=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
echo "API Server: ${API_SERVER}"

echo "=== Ensuring console ServiceAccount exists ==="
kubectl get sa "${SA_NAME}" -n "${SA_NAMESPACE}" &>/dev/null || \
  kubectl create sa "${SA_NAME}" -n "${SA_NAMESPACE}"

kubectl get clusterrolebinding console-sa-admin &>/dev/null || \
  kubectl create clusterrolebinding console-sa-admin \
    --clusterrole=cluster-admin \
    --serviceaccount="${SA_NAMESPACE}:${SA_NAME}"

echo "=== Creating token for console ServiceAccount ==="
BEARER_TOKEN=$(kubectl create token "${SA_NAME}" -n "${SA_NAMESPACE}" --duration=24h)

echo "=== Starting OpenShift Console ==="
echo "  Console URL:  http://localhost:${CONSOLE_PORT}"
echo "  API Server:   ${API_SERVER}"
echo "  Plugin:       ${PLUGIN_NAME} -> http://localhost:9001"
echo ""

BRIDGE_USER_AUTH="disabled"
BRIDGE_K8S_MODE="off-cluster"
BRIDGE_K8S_AUTH="bearer-token"
BRIDGE_K8S_AUTH_BEARER_TOKEN="${BEARER_TOKEN}"
BRIDGE_K8S_MODE_OFF_CLUSTER_ENDPOINT="${API_SERVER}"
BRIDGE_K8S_MODE_OFF_CLUSTER_SKIP_VERIFY_TLS=true
BRIDGE_USER_SETTINGS_LOCATION="localstorage"
BRIDGE_PLUGINS="${PLUGIN_NAME}=http://localhost:9001"
BRIDGE_I18N_NAMESPACES="plugin__${PLUGIN_NAME}"

export BRIDGE_USER_AUTH
export BRIDGE_K8S_MODE
export BRIDGE_K8S_AUTH
export BRIDGE_K8S_AUTH_BEARER_TOKEN
export BRIDGE_K8S_MODE_OFF_CLUSTER_ENDPOINT
export BRIDGE_K8S_MODE_OFF_CLUSTER_SKIP_VERIFY_TLS
export BRIDGE_USER_SETTINGS_LOCATION
export BRIDGE_PLUGINS
export BRIDGE_I18N_NAMESPACES

if command -v podman &>/dev/null; then
  echo "Using podman (Linux host networking)"
  podman run --pull always \
    --platform linux/amd64 \
    --rm \
    --network=host \
    --env-file <(set | grep '^BRIDGE_') \
    "${CONSOLE_IMAGE}"
else
  echo "Using docker (Linux host networking)"
  docker run --pull always \
    --platform linux/amd64 \
    --rm \
    --network=host \
    --env-file <(set | grep '^BRIDGE_') \
    "${CONSOLE_IMAGE}"
fi
