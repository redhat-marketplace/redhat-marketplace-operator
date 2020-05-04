#!/usr/bin/env bash
set -e

echo ${DOCKER_EXEC}

if [ "${DOCKER_EXEC}" == "" ]; then
DOCKER_EXEC=$(command -v docker)
fi

mkdir -p build/_output
[ -d "build/_output/assets" ] && rm -rf build/_output/assets
[ -f "build/_output/bin/redhat-marketplace-operator" ] && rm -f build/_output/bin/redhat-marketplace-operator
cp -r ./assets build/_output

GOOS=linux GOARCH=amd64 go build -o build/_output/bin/redhat-marketplace-operator ./cmd/manager

${DOCKER_EXEC} build . -f ./build/Dockerfile -t "$IMAGE" --label version="${VERSION}"

if $PUSH_IMAGE; then
    docker push "$IMAGE"
fi
