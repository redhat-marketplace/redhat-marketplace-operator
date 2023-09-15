# IBM Data Reporter Operator

# Introduction

The IBM Data Reporter Operator accepts events and transforms them into reports submitted to the Data Service of the IBM Metrics Operator.

# Details

The IBM Data Reporter Operator deploys a service that exposes an endpoint to which callers can send raw json event data. The event data is transformed into a report and is sent to the IBM Data Service. The IBM Data Service periodically uploads the reports to Red Hat Marketplace.

## Prerequisites

- OpenShift Container Platform, major version 4 with any available supported minor version
- Install IBM Metrics Operator and Red Hat Marketplace Deployment Operator
  - IBM Data Reporter Operator prerequisties the IBM Metrics Operator data-service and registration with Red Hat Marketplace
  - Register the Cluster by creating a `redhat-marketplace-pull-secret`, as per the instructions
  - `rhm-data-service` has started

## SecurityContextConstraints Requirements

- The operator runs under Red Hat restricted SCC

## Resources Required

- The operator requires at least 85 Mi memory.

## Limitations

- Only runs on amd64, s390x and ppc64le architectures

## Installing

- A user with the cluster administrator role.
- Install this operator in the same namespace as the IBM Metrics Operator and Red Hat Marketplace Deployment Operator
  - default namespace: `redhat-marketplace`

## Upgrade Policy

The operator releases adhere to semantic versioning and provides a seamless upgrade path for minor and patch releases within the current stable channel.

# Configuration

Optional:
- Configure the DataReporterConfig named `datareporterconfig` as per the following example
- Reference the service account name that will be used to access the service
  - Reports generated for events sent by this user will be decorated with the additional metadata

```YAML
apiVersion: marketplace.redhat.com/v1alpha1
kind: DataReporterConfig
metadata:
  name: datareporterconfig
spec:
  userConfig:
  - metadata:
      ameta1: ametadata1
      bmeta1: bmetadata1
      cmeta1: cmetadata1
      dmeta1: dmetadata1
    userName: system:serviceaccount:openshift-redhat-marketplace:ibm-data-reporter-operator-api
```

### User Configuration

- The ClusterRole for api access is `clusterrole/ibm-data-reporter-operator-api`
- The default ServiceAccount provided as an api user is `system:serviceaccount:openshift-redhat-marketplace:ibm-data-reporter-operator-api`
  - The default ClusterRoleBinding for this user is `clusterrolebinding/ibm-data-reporter-operator-api`

Optional:

- To create an additional ServiceAccount and ClusterRoleBinding for api access

```SHELL
NAMESPACE=$(oc config view --minify -o jsonpath='{..namespace}') && \
oc create serviceaccount my-api-service-account && \
oc create clusterrolebinding ibm-data-reporter-operator-api --clusterrole=ibm-data-reporter-operator-api --serviceaccount=${NAMESPACE}:my-api-service-account
```

- Update datareporterconfig to attach metadata to reports associated with this user

## Usage

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
