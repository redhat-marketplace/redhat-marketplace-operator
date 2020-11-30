#!/usr/bin/env bash
set -uo pipefail

kubectl delete -f ./test/mockcontrollers/mockauth-deployment.yaml
docker buildx build \
	--tag mockauthcontroller:latest \
	-f ./test/mockcontrollers/Dockerfile \
	.
kind load docker-image mockauthcontroller:latest --name test
kubectl apply -f ./test/mockcontrollers/mockauth-deployment.yaml
