#!/bin/bash

_BASEDIR=$(dirname "$0")

oc delete managedcluster $(yq eval '.managedCluster.name' $_BASEDIR/import-cluster/values.yaml)