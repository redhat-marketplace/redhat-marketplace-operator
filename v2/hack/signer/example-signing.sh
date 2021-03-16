#!/bin/bash
set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd $DIR

# Generate Cerificates
./generate-certs.sh

# Build the reporter tool
cd ../../../reporter/v2
CGO_ENABLED=0 go build -o redhat-marketplace-reporter ./cmd/reporter/main.go

# Sign a MeterDefinition
cd ../../v2/hack/signer

cat <<EOF | ./../../../reporter/v2/redhat-marketplace-reporter sign --publickey server-cert.pem --privatekey server-key.pem > out.yaml
apiVersion: marketplace.redhat.com/v1beta1
kind: MeterDefinition
metadata:
  name: nginx-metrics-pod-count-v1beta1
  namespace: openshift-redhat-marketplace
spec:
  group: partner.metering.com
  kind: IBMLicensing
  meters:
  - aggregation: max
    metric: pod_max
    name: DaemonSetPodsCount
    period: 1h0m0s
    query: product_license_usage{}
    workloadType: Service
  resourceFilters:
  - label:
      labelSelector:
        matchExpressions:
        - key: app
          operator: In
          values:
          - nginx
    namespace:
      useOperatorGroup: true
    workloadType: Service
EOF

# Verify the signed MeterDefinition agains the CA
cat out.yaml| ./../../../reporter/v2/redhat-marketplace-reporter verify --ca ca.pem

# Cleanup
rm -Rf ca-key.pem ca.pem server-key.pem server-req.pem server-cert.pem out.yaml ../../../reporter/v2/redhat-marketplace-reporter