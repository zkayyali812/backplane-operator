#!/bin/bash
set -e
# 1. Validate values.yaml (Needs kubeconfig or token and server)
# 2. Copy values.yaml over to hub-cluster and spoke-cluster folders
# 3. Apply hub-cluster manifests
# 4. Sign into spoke cluster and apply spoke-cluster manifests

_BASEDIR=$(dirname "$0")

_SPOKE_TOKEN=$(yq eval '.managedCluster.token' $_BASEDIR/import-cluster/values.yaml)
_SPOKE_SERVER=$(yq eval '.managedCluster.server' $_BASEDIR/import-cluster/values.yaml)
_SPOKE_KUBECONFIG=$(yq eval '.managedCluster.kubeConfig' $_BASEDIR/import-cluster/values.yaml)


if [[ -z "$_SPOKE_TOKEN" || -z "$_SPOKE_SERVER" ]]; then
    echo "INFO: Missing token or server in values.yaml. Checking for kubeconfig"
    if [[ -z "$_SPOKE_KUBECONFIG" ]]; then
        echo "Error: No auth methods provided. Failing. A token and server or kubeconfig must be provided"
        exit 1
    fi
fi

resources=$(helm template hub $_BASEDIR/import-cluster)

for filename in $_BASEDIR/import-cluster/templates/*.yaml; do
    filename=$(basename $filename)
    output=$(helm template hub $_BASEDIR/import-cluster -s templates/$filename)
    echo "$output" | kubectl apply -f -
done