## Name

The IBM Data Reporter Operator

## Introduction

The IBM Data Reporter Operator accepts events and transforms them into Reports submitted to the Data Service of the IBM Metrics Operator.

## Details

The IBM Data Reporter Operator deploys a service that exposes an endpoint to which callers can send raw json event data to the IBM Data Service.

### Usage

#### Get the Status

```SHELL
DRHOST=$(oc get route ibm-data-reporter --template='{{ .spec.host }}') && \
DRTOKEN=$(oc get secret/ibm-data-reporter-operator-api -o jsonpath='{.data.token}' | base64 --decode) && \
curl -k -H "Authorization: Bearer ${DRTOKEN}" https://${DRHOST}/v1/status 
```

#### Post an Event

```SHELL
DRHOST=$(oc get route ibm-data-reporter --template='{{ .spec.host }}') && \
DRTOKEN=$(oc get secret/ibm-data-reporter-operator-api -o jsonpath='{.data.token}' | base64 --decode) && \
curl -k -H "Authorization: Bearer ${DRTOKEN}" -H "X-API-KEY: 123abc" -X POST -d '{"event":"myevent"}' https://${DRHOST}/v1/event
```

## Prerequisites

- Install IBM Metrics Operator
  - Register the Cluster by creating a `redhat-marketplace-pull-secret`, as per the instructions
  - `rhm-data-service` has started

## SecurityContextConstraints Requirements

## Resources Required

- The operator requires at least 50 Mi memory.

## Installing

- Set your context to the IBM Data Reporter Operator namespace
  - `oc project redhat-marketplace`
- Create a secret containing an X-API-KEY used for posting events
  - `oc create secret generic mysecret1 --from-literal=X-API-KEY=123abc`

## Configuration

- Create the DataReporterConfig named `datareporterconfig` as per this example:

```YAML
apiVersion: marketplace.redhat.com/v1alpha1
kind: DataReporterConfig
metadata:
  labels:
    app.kubernetes.io/name: datareporterconfig
    app.kubernetes.io/instance: datareporterconfig-sample
    app.kubernetes.io/part-of: ibm-data-reporter-operator
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/created-by: ibm-data-reporter-operator
  name: datareporterconfig
spec:
  apiKeys:
  - metadata:
      metaA: metadataA
      metaB: metadataB
    secretRef:
      name: mysecret1
  - metadata:
      metaC: metadataC
      metaD: metadataD
    secretRef:
      name: mysecret2
```

- Reference the secret name containing the X-API-KEY
  - Optionally provide any metadata that will be attached to events submitted by this X-API-KEY
- Apply the ClusterRole, ServiceAccount and Secret that will be used to authorize access to the IBM Data Reporter service

```YAML
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ibm-data-reporter-operator-api
  labels:
    redhat.marketplace.com/name: ibm-data-reporter-operator
rules:
- nonResourceURLs:
  - "/v1/status"
  verbs:
  - get
- nonResourceURLs:
  - "/v1/event"
  verbs:
  - create
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ibm-data-reporter-operator-api
  labels:
    redhat.marketplace.com/name: ibm-data-reporter-operator
---
apiVersion: v1
kind: Secret
metadata:
  name: ibm-data-reporter-operator-api
  labels:
    name: ibm-data-reporter-operator
  annotations:
    kubernetes.io/service-account.name: ibm-data-reporter-operator-api
type: kubernetes.io/service-account-token
```

- Create the CluterRoleBinding

    ```SHELL
    NAMESPACE=$(oc config view --minify -o jsonpath='{..namespace}') && \
    oc create clusterrolebinding ibm-data-reporter-operator-api --clusterrole=ibm-data-reporter-operator-api --serviceaccount=${NAMESPACE}:ibm-data-reporter-operator-api
    oc label clusterrolebinding/ibm-data-reporter-operator-api redhat.marketplace.com/name=ibm-data-reporter-operator
    ```

## Storage

- The operator temporarily stores events in pod memory.

## Limitations

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
