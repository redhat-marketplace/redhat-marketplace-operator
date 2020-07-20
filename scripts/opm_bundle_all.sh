#!/usr/bin/env bash
set -euo pipefail

OLM_REPO=$1
OLM_PACKAGE_NAME=$2
OVERRIDE="${OVERRIDE:-false}"

VERSIONS=$(ls deploy/olm-catalog/redhat-marketplace-operator | egrep '\d+\.\d+\.\d+')

for VERSION in $VERSIONS; do
	TAG="$OLM_REPO:v$VERSION"
	EXISTS=false
	skopeo inspect docker://${TAG} >/dev/null

	if [[ $? == 0 ]]; then
		EXISTS=true
	fi

	if [ "$EXISTS" == "false" || "$OVERRIDE" == "true" ]; then
		echo "Building bundle for $VERSION b/c != $LAST_VERSION"
		operator-sdk bundle create "$TAG" \
			--directory "./deploy/olm-catalog/redhat-marketplace-operator/$VERSION" \
			-c stable,beta \
			--package "${OLM_PACKAGE_NAME}" \
			--default-channel stable \
			--overwrite
		echo "Pushing bundle for $VERSION"
		docker push "$TAG"
	fi
done
