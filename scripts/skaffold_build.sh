#!/usr/bin/env bash
set -e

echo ${DOCKER_EXEC}

if [ "${DOCKER_EXEC}" == "" ]; then
DOCKER_EXEC=$(command -v docker)
fi

${DOCKER_EXEC} build . -f ./build/Dockerfile -t "$IMAGE" --label version="${VERSION}"

if $PUSH_IMAGE; then
    docker push "$IMAGE"
fi
