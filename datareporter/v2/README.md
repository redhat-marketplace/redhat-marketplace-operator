# IBM Data Reporter Operator

# Introduction

The IBM Data Reporter Operator accepts events and transforms them into reports submitted to the Data Service of the IBM Metrics Operator.

# Details

The IBM Data Reporter Operator deploys a service that exposes an endpoint to which callers can send raw json event data. The event data is transformed into a report and is sent to the IBM Metrics Operator's Data Service. The IBM Metrics Operator Data Service periodically uploads the reports to Red Hat Marketplace. It can also filter, transform and forward events to alternate endpoints.

## Prerequisites

- OpenShift Container Platform, major version 4 with any available supported minor version
- Install IBM Metrics Operator
  - IBM Data Reporter Operator prerequisties the IBM Metrics Operator's Data Service
  - Register the Cluster by creating a `redhat-marketplace-pull-secret`, as per the instructions
  - Once `rhm-data-service` has started IBM Data Reporter Operator will accept requests

## SecurityContextConstraints Requirements

- The operator runs under Red Hat restricted SCC

## Resources Required

- The operator requires at least 85 Mi memory.

## Limitations

- Only runs on amd64, s390x and ppc64le architectures

## Installing

- A user with the cluster administrator role.
- Install this operator in the same namespace as the IBM Metrics Operator
  - default namespace: `redhat-marketplace`

## Upgrade Policy

The operator releases adhere to semantic versioning and provides a seamless upgrade path for minor and patch releases within the current stable channel.

# Configuration

Optional:
- Configure the Custom Resource Definition `DataReporterConfig` named `datareporterconfig` as per the following sample
  - Configuration that can not be reconciled successfully will be reported on the DataReporterConfig Status
  - jsonPath expressions are handled by: https://github.com/ohler55/ojg
    - jsonPath comparison: https://cburgmer.github.io/json-path-comparison/
  - Transformer type currently supported is `kazaam`
    - https://github.com/willie68/kazaam

Sample DataReporterConfig:
```YAML
apiVersion: marketplace.redhat.com/v1alpha1
kind: DataReporterConfig
metadata:
  name: datareporterconfig
spec:
  # true: report generated for 1 event and provide return code from data-service, confirming delivery
  # false: events will be accumulated before building report, and immediately return 200, delivery unfconfirmed
  confirmDelivery: false
  dataFilters:   # dataFilters matches the first dataFilter selector, applies transforms and forwards to data-service and destinations
  - altDestinations:   # List of desintations to transform and foward events to
    - authorization:   # optional endpoint if an authorization token must first be requested 
        authDestHeader: Authorization   # http header key to be appended to destination request
        authDestHeaderPrefix: 'Bearer '   # optional prefix for constructing destination request http header value
        bodyData:   # body data to send to authorization request, often an apiKey
          secretKeyRef:
            name: auth-body-data-secret
            key: bodydata
        header:   # secret map to use for authorization request headers
          secret:
            name: auth-header-map-secret
        tokenExpr: $.token   # optionally extract the returned body data using a jsonPath expression
        url: https://127.0.0.1:8443/api/2.0/accounts/my_account_id/apikeys/token   # the url to make an authorization request to
      header:   # secret map to use for destination request headers
        secret:
          name: dest-header-map-secret
      transformer:   # The transformer to apply to event data for this destination
        configMapKeyRef:
          key: kazaam.json
          name: kazaam-configmap
        type: kazaam
      url: https://127.0.0.1:8443/metering/api/1.0/usage   # the url to send the event to
      urlSuffixExpr: $.properties.productId   # optional jsonPath expression on the event to extract and set a suffix to the url path
    manifestType: dataReporter   # override manifestType of report sent to data-service
    selector:   # Matches against event data and a user. Empty selector matches all.
      matchExpressions:   # jsonPath expression must return a result for all expressions to match (AND)
      - $[?($.event == "Account Contractual Usage")]
      - $.properties.productId
      - $[?($.properties.source != null)]
      - $[?($.properties.unit == "AppPoints")]
      - $[?($.properties.quantity >= 0)]
      - $[?($.timestamp != null)]
      matchUsers:   # must match one of these users (OR). Omitting users matches any user
      - system:serviceaccount:redhat-marketplace:ibm-data-reporter-operator-api
    transformer:   # The transformer to apply to the event for data-service
      configMapKeyRef:
        key: kazaam.json
        name: kazaam-configmap
      type: kazaam
  tlsConfig:   # TLS configuration for requests outbound to destinations
    caCerts:
    - name: tls-ca-certificates
      key: ca.crt
    certificates:
    - clientCert:
        secretKeyRef:
          name: tls-ca-certificates
          key: tls.crt
      clientKey:
        secretKeyRef:
          name: tls-ca-certificates
          key: tls.key
    cipherSuites: 
    - TLS_AES_128_GCM_SHA256
    - TLS_AES_256_GCM_SHA384
    - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
    - TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
    insecureSkipVerify: false
    minVersion: VersionTLS12
  userConfig:   # Optional metadata to apply to report sent to data-service for a specific user
  - metadata:
      ameta1: ametadata1
      bmeta1: bmetadata1
      cmeta1: cmetadata1
      dmeta1: dmetadata1
    userName: system:serviceaccount:redhat-marketplace:ibm-data-reporter-operator-api
```

Sample Header Secrets:
```YAML
apiVersion: v1
stringData:
  accept: | 
    */*
kind: Secret
metadata:
  name: dest-header-map-secret
type: Opaque
---
apiVersion: v1
stringData:
  accept: application/json
  Content-Type: application/json
kind: Secret
metadata:
  name: auth-header-map-secret
type: Opaque
```

Sample Authentication Body Secret:
```YAML
apiVersion: v1
stringData:
  bodydata: |
    {"apikey": "myapikey"}
kind: Secret
metadata:
  name: auth-body-data-secret
type: Opaque
```

Sample TLS Secret:
```YAML
apiVersion: v1
data:
  ca.crt: LS0t...
  tls.key: LS0t...
  tls.crt: LS0t...
kind: Secret
metadata:
  name: tls-ca-certificates
```

Sample Kazaam ConfigMap:
```YAML
apiVersion: v1
kind: ConfigMap
metadata:
  name: kazaam-configmap
data:
  kazaam.json: |
    [
        {
            "operation": "timestamp",
            "spec": {
                "timestamp": {
                "inputFormat": "2006-01-02T15:04:05.999999+00:00",
                "outputFormat": "$unixmilli"
                }
            }
        },
        {
            "operation": "shift",
            "spec": {
                "instances[0].instanceId": "properties.source",
                "instances[0].startTime": "timestamp",
                "instances[0].endTime": "timestamp",
                "instances[0].metricUsage[0].metricId": "properties.unit",
                "instances[0].metricUsage[0].quantity": "properties.quantity"
            }
        },
        {
            "operation": "default",
            "spec": {
                "meteringModel": "point-in-time",
                "meteringPlan": "contract"
            }
        }
    ]
```

### API Service User Configuration

- The ClusterRole for api access is `clusterrole/ibm-data-reporter-operator-api`
- The default ServiceAccount provided as an api user is `system:serviceaccount:redhat-marketplace:ibm-data-reporter-operator-api`
  - The default ClusterRoleBinding for this user is `clusterrolebinding/ibm-data-reporter-operator-api`

Optional:

- To create an additional ServiceAccount and ClusterRoleBinding for api access

```SHELL
NAMESPACE=$(oc config view --minify -o jsonpath='{..namespace}') && \
oc create serviceaccount my-api-service-account && \
oc create clusterrolebinding ibm-data-reporter-operator-api --clusterrole=ibm-data-reporter-operator-api --serviceaccount=${NAMESPACE}:my-api-service-account
```

- Update datareporterconfig to attach metadata to reports associated with this user

## API Service Usage

- Get Token & Host

```SHELL
oc project redhat-marketplace
export DRTOKEN=$(oc create token ibm-data-reporter-operator-api --namespace redhat-marketplace --duration 1h)
export DRHOST=$(oc get route ibm-data-reporter --template='{{ .spec.host }}')
```

- Get the Status

```SHELL
curl -k -H "Authorization: Bearer ${DRTOKEN}" https://${DRHOST}/v1/status 
```

- Post an Event

```SHELL
curl -k -H "Authorization: Bearer ${DRTOKEN}" -X POST -d '{"event":"myevent"}' https://${DRHOST}/v1/event
```

## Storage

- The operator temporarily stores events in pod memory, and writes to the IBM Metrics Operator data-service, which requires a PersistentVolume

## License

Copyright IBM Corporation 2023. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
