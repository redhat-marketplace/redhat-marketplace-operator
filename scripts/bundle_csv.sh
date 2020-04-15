#!/usr/bin/env bash
ROOT=$1
VERSION=$2
REGISTRY_IMAGE=registry.connect.redhat.com/ibm/redhat-marketplace-operator:${VERSION}

CREATED_TIME=`date +"%FT%H:%M:%SZ"`

rm $ROOT/bundle/*
cp $ROOT/deploy/olm-catalog/redhat-marketplace-operator/redhat-marketplace-operator.package.yaml ./bundle
cp $ROOT/deploy/olm-catalog/redhat-marketplace-operator/${VERSION}/* ./bundle
go run github.com/mikefarah/yq/v3 w -i $ROOT/bundle/redhat-marketplace-operator.v${VERSION}.clusterserviceversion.yaml 'spec.install.spec.deployments[0].spec.template.spec.containers[0].image' ${REGISTRY_IMAGE}
go run github.com/mikefarah/yq/v3 w -i $ROOT/bundle/redhat-marketplace-operator.v${VERSION}.clusterserviceversion.yaml 'metadata.annotations.createdAt' ${CREATED_TIME}
cd $ROOT/bundle && zip -r redhat-marketplace-operator-bundle-${VERSION}.zip .
