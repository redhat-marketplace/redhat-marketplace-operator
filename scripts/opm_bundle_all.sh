#!/usr/bin/env bash
set -euo pipefail

OLM_REPO=$1
OLM_PACKAGE_NAME=$2
LAST_VERSION=$3

VERSIONS=$(ls deploy/olm-catalog/redhat-marketplace-operator | egrep '\d+\.\d+\.\d+')

for VERSION in $VERSIONS; do
	if [ "$VERSION" != "$LAST_VERSION" ]; then
		echo "Building bundle for $VERSION b/c != $LAST_VERSION"
		operator-sdk bundle create "$OLM_REPO:v$VERSION" \
			--directory "./deploy/olm-catalog/redhat-marketplace-operator/$VERSION" \
			-c stable,beta \
			--package "${OLM_PACKAGE_NAME}" \
			--default-channel stable \
			--overwrite
		echo "Pushing bundle for $VERSION"
		docker push "$OLM_REPO:v$VERSION"
	fi
done
