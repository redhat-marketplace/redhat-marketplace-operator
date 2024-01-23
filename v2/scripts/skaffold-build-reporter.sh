#!/usr/bin/env bash

set -e

mkdir -p build/_output
[ -d "build/_output/assets" ] && rm -rf build/_output/assets
[ -f "build/_output/bin/redhat-marketplace-reporter" ] && rm -f build/_output/bin/redhat-marketplace-reporter
cp -r ./assets build/_output

GOOS=linux GOARCH=amd64 go build -o build/_output/bin/redhat-marketplace-reporter ./cmd/reporter

podman build . -f ./build/reporter.Dockerfile -t $IMAGE --label version="${VERSION}"

if $PUSH_IMAGE; then
    podman push $IMAGE
fi
