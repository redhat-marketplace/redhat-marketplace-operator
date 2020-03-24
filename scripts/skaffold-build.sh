#!/usr/bin/env bash
set -e

make code-templates
mkdir -p build/_output
[ -d "build/_output/assets" ] && rm -rf build/_output/assets
[ -f "build/_output/bin/redhat-marketplace-operator" ] && rm -f build/_output/bin/redhat-marketplace-operator
cp -r ./assets build/_output

GOOS=linux GOARCH=amd64 go build -o build/_output/bin/redhat-marketplace-operator ./cmd/manager/main.go

docker build . -f ./build/Dockerfile -t $IMAGE

if $PUSH_IMAGE; then
    docker push $IMAGE
fi
