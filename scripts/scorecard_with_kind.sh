#!/usr/bin/env bash
set -uo pipefail

VERSION=${VERSION:-$(make current-version)}
FILTER=${FILTER:-""}
CLUSTER_NAME=${CLUSTER_NAME:-'operator-test'}
OUR_CLUSTER=$(kind get clusters | grep "$CLUSTER_NAME")
CRS=$(find './deploy/crds/' | grep -E '_cr.yaml$')

if [ "$OUR_CLUSTER" != "" ]; then
	kind delete cluster --name "$CLUSTER_NAME"
fi

if [ "$FILTER" != "" ]; then
    CRS=$(echo "$CRS" | grep -E "$FILTER")
fi

echo "Testing $CRS"

for cr in $CRS; do
	kind create cluster --name "$CLUSTER_NAME"
  yq w .osdk-scorecard.yaml.tpl 'scorecard.plugins.*.*.cr-manifest.+' "$cr" > .osdk-scorecard.yaml
  yq w -i .osdk-scorecard.yaml 'scorecard.plugins.*.*.csv-path' deploy/olm-catalog/redhat-marketplace-operator/${VERSION}/redhat-marketplace-operator.v${VERSION}.clusterserviceversion.yaml

  ./scripts/scorecard.sh

	kind delete cluster --name "$CLUSTER_NAME"
done
