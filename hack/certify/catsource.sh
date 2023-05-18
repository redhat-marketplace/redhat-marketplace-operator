#!/bin/bash
set -Eeox pipefail

cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ibm-metrics-operator-test
  namespace: openshift-marketplace
spec:
  displayName: IBM Metrics Operator Test
  image: quay.io/rh-marketplace/ibm-metrics-operator-dev-index:${TAG}
  publisher: ''
  sourceType: grpc
EOF

