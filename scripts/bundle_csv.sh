#!/usr/bin/env bash
set -e

ROOT=$1
VERSION=$2
REGISTRY_IMAGE=$3
CREATED_TIME=`date +"%FT%H:%M:%SZ"`
DATETIME=$(date +"%FT%H%M%SZ")

rm -rf $ROOT/bundle || echo "nothing to clean"
mkdir -p $ROOT/bundle
#cp $ROOT/deploy/olm-catalog/redhat-marketplace-operator/redhat-marketplace-operator.package.yaml $ROOT/bundle
#cp $ROOT/deploy/olm-catalog/redhat-marketplace-operator/${VERSION}/* $ROOT/bundle
#find -E $ROOT/deploy/olm-catalog/redhat-marketplace-operator -name "*.clusterserviceversion.yaml" -exec cp {} $ROOT/bundle \;
#find -E $ROOT/bundle -name "*.clusterserviceversion.yaml" -exec go run github.com/mikefarah/yq/v3 w -i {} 'metadata.annotations.createdAt' ${CREATED_TIME} \;

cp -r "$ROOT/deploy/olm-catalog/redhat-marketplace-operator" "$ROOT/bundle"

PACKAGE_PATH="$ROOT/bundle/redhat-marketplace-operator"
CSV_PATH="$ROOT/bundle/redhat-marketplace-operator/${VERSION}"

go run github.com/mikefarah/yq/v3 w \
    -i $CSV_PATH/redhat-marketplace-operator.v${VERSION}.clusterserviceversion.yaml \
    'metadata.annotations.containerImage' ${REGISTRY_IMAGE}

go run github.com/mikefarah/yq/v3 w \
    -i $CSV_PATH/redhat-marketplace-operator.v${VERSION}.clusterserviceversion.yaml \
    'metadata.annotations.createdAt' ${CREATED_TIME}

go run github.com/mikefarah/yq/v3 w \
    -i $CSV_PATH/redhat-marketplace-operator.v${VERSION}.clusterserviceversion.yaml \
    'spec.install.spec.deployments[0].spec.template.spec.containers[0].image' ${REGISTRY_IMAGE}

operator-courier verify --ui_validate_io $PACKAGE_PATH

FILENAME="redhat-marketplace-operator-bundle-${VERSION}-${DATETIME}.zip"

cd $ROOT/deploy/olm-catalog/redhat-marketplace-operator || echo "failed to cd"
zip -r ${ROOT}/bundle/${FILENAME} .

echo "::set-output name=filename::${FILENAME}"

FILENAME="redhat-marketplace-operator-bundle-${VERSION}-${DATETIME}"

echo "::set-output name=bundlename::${FILENAME}"
