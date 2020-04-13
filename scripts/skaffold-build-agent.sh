#!/usr/bin/env bash

set -e

cd ../marketplace-agent

docker build . -f ./Dockerfile -t $IMAGE

if $PUSH_IMAGE; then
    docker push $IMAGE
fi
