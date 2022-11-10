#!/bin/sh
set -euo pipefail

server=$1
namespace=$2
saname=$3

# the name of the secret containing the service account token
secretname=$(kubectl get sa/$saname -n $namespace -o json | jq -r '.secrets[].name' | grep redhat-marketplace-operator-token)

ca=$(kubectl get secret/$secretname -n $namespace -o jsonpath='{.data.ca\.crt}')

token=$(kubectl get secret/$secretname -n $namespace -o jsonpath='{.data.token}' | base64 --decode)

echo "
apiVersion: v1
kind: Config
clusters:
- name: default-cluster
  cluster:
    certificate-authority-data: ${ca}
    server: ${server}
contexts:
- name: rhm-operator-context
  context:
    cluster: default-cluster
    namespace: ${namespace}
    user: rhm-operator-sa
current-context: rhm-operator-context
users:
- name: rhm-operator-sa
  user:
    token: ${token}
" > sa.kubeconfig