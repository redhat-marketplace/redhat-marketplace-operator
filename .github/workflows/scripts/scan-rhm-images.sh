IMAGE_REGISTRY="quay.io"
IMAGE_REPO="rh-marketplace"
IMAGE_TAG="2.10.0-1426"

declare -a IMAGE_NAMES=("redhat-marketplace-operator"
	"redhat-marketplace-reporter"
	"redhat-marketplace-authcheck"
    "redhat-marketplace-data-service"
	"redhat-marketplace-metric-state"
	"ibm-metrics-operator"
)

# Read the array values with space
for IMAGE_NAME in "${IMAGE_NAMES[@]}"; do
	REPORT_NAME=`basename $IMAGE_NAME`.json

	# Must get child digest
	IMAGE_DIGEST=`skopeo inspect -n --raw docker://$IMAGE_REGISTRY/$IMAGE_REPO/$IMAGE_NAME:$IMAGE_TAG | jq -r '.manifests | .[] | select(.platform.architecture | contains("amd64")) | .digest'`
	
	echo "$IMAGE_NAME: $IMAGE_DIGEST"
	
	curl --retry 3 -s -o $REPORT_NAME https://$IMAGE_REGISTRY/api/v1/repository/$IMAGE_REPO/$IMAGE_NAME/manifest/$IMAGE_DIGEST/security?vulnerabilities=true
	
	# All Vulnerabilities
	echo "=== $REPORT_NAME ALL VULNERABILITIES ==="
	jq '.data.Layer.Features | .[] | .Vulnerabilities | select(length > 0)' $REPORT_NAME

	# Vulnerabilities that can be fixed
	echo "=== $REPORT_NAME MUST-FIX VULNERABILITIES ==="
	jq '.data.Layer.Features | .[] | .Vulnerabilities | .[] | select(.FixedBy | length > 0)' $REPORT_NAME
	
done
