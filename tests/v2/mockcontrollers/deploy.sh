#!/usr/bin/env bash
set -o pipefail

if [ "${DOCKER_EXEC}" == "" ]; then
	DOCKER_EXEC=$(command -v docker)
fi

sh -c "${DOCKER_EXEC} buildx" 1>/dev/null 2>/dev/null
if [ $? -eq 0 ]; then
	DOCKER="${DOCKER_EXEC} buildx"
	ARGS="--load"
else
	DOCKER="${DOCKER_EXEC}"
	ARGS="--load"
fi

TAG=$(date +%s)

kubectl delete -f ./test/mockcontrollers/mockauth-deployment.yaml
export DOCKER_BUILDKIT=1

${DOCKER} build --tag mockauthcontroller:${TAG} $ARGS -f ./test/mockcontrollers/Dockerfile .

kind load docker-image mockauthcontroller:${TAG} --name test

cat ./test/mockcontrollers/mockauth-deployment.yaml | sed "s/\${TAG}/${TAG}/" | kubectl apply -f -
