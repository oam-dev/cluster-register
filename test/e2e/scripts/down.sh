#!/usr/bin/env bash
set -uo pipefail

HUB_CLUSTER="${HUB_CLUSTER:-ocm-e2e-hub}"
SPOKE_CLUSTER="${SPOKE_CLUSTER:-ocm-e2e-spoke}"

kind delete cluster --name "${HUB_CLUSTER}"
kind delete cluster --name "${SPOKE_CLUSTER}"
