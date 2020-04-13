#!/usr/bin/env bash
set -e

OPERATOR_IMAGE=redhat-marketplace-operator
AGENT_IMAGE=redhat-marketplace-agent
PULL_POLICY=IfNotPresent RELATED_IMAGE_MARKETPLACE_OPERATOR=${OPERATOR_IMAGE} RELATED_IMAGE_MARKETPLACE_AGENT=${AGENT_IMAGE} ./scripts/gen_files.sh

mkdir -p build/_output
[ -d "build/_output/assets" ] && rm -rf build/_output/assets
[ -f "build/_output/bin/redhat-marketplace-operator" ] && rm -f build/_output/bin/redhat-marketplace-operator
cp -r ./assets build/_output

GOOS=linux GOARCH=amd64 go build -o build/_output/bin/redhat-marketplace-operator ./cmd/manager

docker build . -f ./build/Dockerfile -t $IMAGE

if $PUSH_IMAGE; then
    docker push $IMAGE
fi
