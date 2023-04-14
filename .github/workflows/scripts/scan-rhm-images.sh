#!/bin/bash
IMAGE_REGISTRY_PATH=${IMAGE_REGISTRY}
TEMP=(${IMAGE_REGISTRY_PATH//// })
TARGET_REGISTRY="${TEMP[0]}"
TARGET_REPO="${TEMP[1]}"
TARGET_TAG="{$TAG}"

declare -a IMAGE_NAMES=("redhat-marketplace-operator"
	"redhat-marketplace-reporter"
	"redhat-marketplace-authcheck"
    "redhat-marketplace-data-service"
	"redhat-marketplace-metric-state"
	"ibm-metrics-operator"
)
echo ${IMAGE_REPO}
SKOPEO_VERSION=$(skopeo --version)
echo ${SKOPEO_VERSION}
JQ_VERSION=$(jq --version)
echo ${JQ_VERSION}
echo ${IMAGE_TAG}

# Read the array values with space
for IMAGE_NAME in "${IMAGE_NAMES[@]}"; do
	REPORT_NAME=`basename $IMAGE_NAME`.json
    echo "running $REPORT_NAME"

	# Must get child digest
	IMAGE_DIGEST=`skopeo inspect --raw docker://$TARGET_REGISTRY/$TARGET_REPO/$IMAGE_NAME:$IMAGE_TAG | jq -r '.manifests | .[] | select(.platform.architecture | contains("amd64")) | .digest'`
	
	echo "$IMAGE_NAME: $IMAGE_DIGEST"
	
	curl --retry 3 -s -o $REPORT_NAME https://$TARGET_REGISTRY/api/v1/repository/$TARGET_REPO/$IMAGE_NAME/manifest/$IMAGE_DIGEST/security?vulnerabilities=true
	
	# All Vulnerabilities
	echo "=== $REPORT_NAME ALL VULNERABILITIES ==="
	jq '.data.Layer.Features | .[] | .Vulnerabilities | select(length > 0)' $REPORT_NAME

	# Vulnerabilities that can be fixed
	echo "=== $REPORT_NAME MUST-FIX VULNERABILITIES ==="
	jq '.data.Layer.Features | .[] | .Vulnerabilities | .[] | select(.FixedBy | length > 0)' $REPORT_NAME
	
done
