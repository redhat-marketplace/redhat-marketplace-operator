#!/usr/bin/env bash

set -e

cd ./assets/prometheus

kubectl config set-context --current --namespace=ibm-software-central
kubectl create secret generic additional-scrape-configs --from-file=prometheus-additional.yaml --dry-run -oyaml > additional-scrape-configs.yaml
kubectl apply -f additional-scrape-configs.yaml
#kubectl apply prometheus-rules.yaml
kubectl apply -f prometheus.yaml -n ibm-software-central
