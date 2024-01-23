#!/usr/bin/env bash

OLM_REPO=$1
OLM_BUNDLE_REPO=$2
TAG=$3
VERSION=$4

echo "Running with $1 $2 $3"
OLM_REPO_E=$(echo $OLM_REPO | sed -e 's/[\/&]/\\&/g')

echo "Calculator Versions"
VERSIONS=$(git tag | xargs)
# VERSIONS=$(ls deploy/olm-catalog/redhat-marketplace-operator | grep -E '^[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+$' | grep -Ev "^$VERSION$")
# VERSIONS=$(echo "$VERSIONS,$OLM_REPO:$TAG")

VERSIONS_LIST=""

for version in $VERSIONS; do
		RESULT=`skopeo inspect docker://$OLM_REPO:$version`
		if [ $? -eq 0 ]; then
        VERSIONS_LIST="$OLM_REPO:$version,$VERSIONS_LIST"
        continue
    fi

    RESULT=`skopeo inspect docker://$OLM_REPO:v$version`
		if [ $? -eq 0 ]; then
        VERSIONS_LIST="$OLM_REPO:v$version,$VERSIONS_LIST"
        continue
    fi
done

VERSIONS_LIST="$OLM_REPO:$TAG,$VERSIONS_LIST"
VERSIONS_LIST="${VERSIONS_LIST%?}"

echo "Making version list"

echo "Using tag ${OLM_BUNDLE_REPO}:${TAG}"
echo "Building index with $VERSIONS_LIST"
echo ""
./testbin/opm index add -u docker --generate --bundles "$VERSIONS_LIST" --tag "${OLM_BUNDLE_REPO}:${TAG}"

if [ $? -ne 0 ]; then
    echo "fail to build opm"
    exit 1
fi

podman build -f custom-index.Dockerfile -t "${OLM_BUNDLE_REPO}:${TAG}" .
podamn push "${OLM_BUNDLE_REPO}:${TAG}"
