#!/usr/bin/env bash
set -uo pipefail

echo "Setting up environment"
make setup-minikube
kubectl create ns openshift-redhat-marketplace
kubectl config set-context --current --namespace=openshift-redhat-marketplace
kubectl apply -f deploy/role.yaml -f deploy/service_account.yaml -f deploy/role_binding.yaml

sleep 10

echo "Running scorecard"
operator-sdk scorecard -b ./deploy/olm-catalog/redhat-marketplace-operator --config ./.osdk-scorecard.yaml -o json >scorecard-output.json

echo "Scorecard complete"


cat scorecard-output.json

cat scorecard-output.json | sed 's/.*\.{/{/' | jq -r '.results[] | select(.state == "fail") | "::error" +  "::" + .name + " failed"'
