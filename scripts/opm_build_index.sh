#!/usr/bin/env bash
set -e

OLM_REPO=$1
OLM_BUNDLE_REPO=$2
TAG=$3

OLM_REPO_E=$(echo $OLM_REPO | gsed -e 's/[\/&]/\\&/g')

VERSIONS=$(ls deploy/olm-catalog/redhat-marketplace-operator | egrep '\d+\.\d+\.\d+')
VERSIONS_LIST=$(echo $VERSIONS | xargs | gsed -e "s/ / $OLM_REPO_E:v/g" | gsed -e "s/^/$OLM_REPO_E:v/g" | gsed -e 's/ /,/g')

echo "Using tag ${OLM_BUNDLE_REPO}:${TAG}"
echo "Building index with $VERSIONS_LIST"
echo ""
opm index add -u docker --bundles "$VERSIONS_LIST" --tag "${OLM_BUNDLE_REPO}:${TAG}"
docker push "${OLM_BUNDLE_REPO}:${TAG}"
