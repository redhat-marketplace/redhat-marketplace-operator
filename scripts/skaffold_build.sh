#!/usr/bin/env bash
set -e

echo "Checking if golang can compile"
go vet ./...

if [ "${DOCKER_EXEC}" == "" ]; then
DOCKER_EXEC=$(command -v docker)
fi

${DOCKER_EXEC} build . -f ${BUILD_FILE} -t "$IMAGE" --label version="${VERSION}"

if $PUSH_IMAGE; then
    docker push "$IMAGE"
fi
