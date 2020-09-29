#!/usr/bin/env bash
set -euo pipefail

OLM_REPO=$1
OLM_PACKAGE_NAME=$2
LAST_VERSION=$3
OVERRIDE="${OVERRIDE:-false}"

VERSIONS=$(find deploy/olm-catalog/redhat-marketplace-operator -maxdepth 1 -type d -printf "%f\n" | grep -P '\d+\.\d+\.\d+')

for VERSION in $VERSIONS; do
	TAG="$OLM_REPO:v$VERSION"
	EXISTS=false

	if skopeo inspect "docker://${TAG}" >/dev/null; then
    echo "${TAG} exists"
		EXISTS=true
	fi

	if [ "$EXISTS" == "false" ] || [ "$OVERRIDE" == "true" ]; then
		echo "Building bundle for $VERSION b/c != $LAST_VERSION"
		../testbin/operator-sdk bundle create "$TAG" \
			--directory "./deploy/olm-catalog/redhat-marketplace-operator/$VERSION" \
			-c stable,beta \
			--package "${OLM_PACKAGE_NAME}" \
			--default-channel stable \
			--overwrite
		echo "Pushing bundle for $VERSION"
		docker push "$TAG"
	fi
done
