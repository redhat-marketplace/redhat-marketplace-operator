# ibm-data-reporter-operator
The IBM Data Reporter Operator accepts events and transforms them into Reports submitted to the Data Service of the IBM Metrics Operator.

## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Getting Started

### Prerequisites
- Install IBM Metrics Operator
  - Register the Cluster by creating a `redhat-marketplace-pull-secret`, as per the instructions
  - `rhm-data-service` has started

### Install & Configuration

- Install the IBM Data Reporter Operator
- Set your context to the IBM Data Reporter Operator namespace
  - `oc project redhat-marketplace`
- Create a secret containing an X-API-KEY used for posting events
  - `oc create secret generic mysecret1 --from-literal=X-API-KEY=123abc`
- Create the DataReporterConfig named `datareporterconfig` as per the sample `config/samples/marketplace_v1alpha1_datareporterconfig.yaml`
  - Reference the secret name containing the X-API-KEY
  - Optionally provide any metadata that will be attached to events submitted by this X-API-KEY
- Apply the ClusterRole, ServiceAccount and Secret that will be used to authorize access to the IBM Data Reporter service
  - `oc apply -f hack/role/role.yaml`
- Create the CluterRoleBinding
    ```
    NAMESPACE=$(oc config view --minify -o jsonpath='{..namespace}') && \
    oc create clusterrolebinding ibm-data-reporter-operator-api --clusterrole=ibm-data-reporter-operator-api --serviceaccount=${NAMESPACE}:ibm-data-reporter-operator-api
    oc label clusterrolebinding/ibm-data-reporter-operator-api redhat.marketplace.com/name=ibm-data-reporter-operator
    ```

### Usage

#### Get the Status

```
DRHOST=$(oc get route ibm-data-reporter --template='{{ .spec.host }}') && \
DRTOKEN=$(oc get secret/ibm-data-reporter-operator-api -o jsonpath='{.data.token}' | base64 --decode) && \
curl -k -H "Authorization: Bearer ${DRTOKEN}" https://${DRHOST}/v1/status 
```

#### Post an Event



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

