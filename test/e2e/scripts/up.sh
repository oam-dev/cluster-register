#!/usr/bin/env bash
# Prints `export ...` lines for E2E_HUB_KUBECONFIG, E2E_SPOKE_KUBECONFIG,
# E2E_HUB_API_SERVER, E2E_CLUSTER_NAME to stdout; progress goes to stderr.
#   test/e2e/scripts/up.sh > /tmp/ocm-e2e.env
#   set -a && source /tmp/ocm-e2e.env && set +a
set -euo pipefail

KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.35.5}"
HUB_CLUSTER="${HUB_CLUSTER:-ocm-e2e-hub}"
SPOKE_CLUSTER="${SPOKE_CLUSTER:-ocm-e2e-spoke}"
OCM_CLUSTER_MANAGER_MANIFEST="${OCM_CLUSTER_MANAGER_MANIFEST:-https://raw.githubusercontent.com/oam-dev/kubevela/master/vela-templates/addons/auto-gen/ocm-cluster-manager.yaml}"
WORKDIR="$(mktemp -d -t ocm-e2e-XXXXXX)"

log() { echo "==> $*" >&2; }

log "creating hub cluster '${HUB_CLUSTER}' (${KIND_NODE_IMAGE})"
kind create cluster --name "${HUB_CLUSTER}" --image "${KIND_NODE_IMAGE}" --wait 5m

log "creating spoke cluster '${SPOKE_CLUSTER}' (${KIND_NODE_IMAGE})"
kind create cluster --name "${SPOKE_CLUSTER}" --image "${KIND_NODE_IMAGE}" --wait 5m

HUB_KUBECONFIG="${WORKDIR}/hub.kubeconfig"
SPOKE_KUBECONFIG="${WORKDIR}/spoke.kubeconfig"
kind get kubeconfig --name "${HUB_CLUSTER}" >"${HUB_KUBECONFIG}"
kind get kubeconfig --name "${SPOKE_CLUSTER}" >"${SPOKE_KUBECONFIG}"

log "installing the OCM hub control plane (ocm-cluster-manager) on '${HUB_CLUSTER}'"
kubectl --kubeconfig "${HUB_KUBECONFIG}" apply -f "${OCM_CLUSTER_MANAGER_MANIFEST}"

log "waiting for the open-cluster-management-hub namespace to appear"
for _ in $(seq 1 30); do
  if kubectl --kubeconfig "${HUB_KUBECONFIG}" get ns open-cluster-management-hub >/dev/null 2>&1; then
    break
  fi
  sleep 10
done

log "waiting for the OCM hub controllers to become available"
kubectl --kubeconfig "${HUB_KUBECONFIG}" wait deployment \
  --all -n open-cluster-management \
  --for=condition=Available --timeout=5m
kubectl --kubeconfig "${HUB_KUBECONFIG}" wait deployment \
  --all -n open-cluster-management-hub \
  --for=condition=Available --timeout=5m

# kind clusters share a docker network ("kind"); this is how the spoke's
# registration agent reaches the hub apiserver.
HUB_CONTAINER_IP="$(docker inspect -f '{{ (index .NetworkSettings.Networks "kind").IPAddress }}' "${HUB_CLUSTER}-control-plane")"
if [[ -z "${HUB_CONTAINER_IP}" ]]; then
  echo "failed to determine the hub control-plane container's docker IP" >&2
  exit 1
fi

log "hub and spoke clusters are ready"

cat <<EOF
export E2E_HUB_KUBECONFIG="${HUB_KUBECONFIG}"
export E2E_SPOKE_KUBECONFIG="${SPOKE_KUBECONFIG}"
export E2E_HUB_API_SERVER="https://${HUB_CONTAINER_IP}:6443"
export E2E_CLUSTER_NAME="e2e-spoke"
EOF
