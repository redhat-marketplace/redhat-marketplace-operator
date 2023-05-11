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
- Create or Configure the DataReporterConfig named `datareporterconfig` as per the sample `config/samples/marketplace_v1alpha1_datareporterconfig.yaml`
  - Reference the service account in the format example `system:serviceaccount:openshift-redhat-marketplace:ibm-data-reporter-operator-api`
  - Optionally provide any metadata that will be attached to events submitted by this user
- Apply the ClusterRole, ServiceAccount and Secret that will be used to authorize access to the IBM Data Reporter service
  - `oc apply -f hack/role/role.yaml`
- Create the CluterRoleBinding
    ```
    NAMESPACE=$(oc config view --minify -o jsonpath='{..namespace}') && \
    oc create clusterrolebinding ibm-data-reporter-operator-api --clusterrole=ibm-data-reporter-operator-api --serviceaccount=${NAMESPACE}:ibm-data-reporter-operator-api
    oc label clusterrolebinding/ibm-data-reporter-operator-api redhat.marketplace.com/name=ibm-data-reporter-operator
    ```

### Usage

#### Create a Token

Create a (time-bound) token to be used to authenticate to the Data Reporter service

```
export DRTOKEN=$(kubectl create token ibm-data-reporter-operator-api --namespace redhat-marketplace --duration 2160h)
```

For more information about service account tokens, refer to the [kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#manually-create-an-api-token-for-a-serviceaccount).

#### Get the Status

```
DRHOST=$(oc get route ibm-data-reporter --template='{{ .spec.host }}') && \
curl -k -H "Authorization: Bearer ${DRTOKEN}" https://${DRHOST}/v1/status 
```

#### Post an Event

```
DRHOST=$(oc get route ibm-data-reporter --template='{{ .spec.host }}') && \
curl -k -H "Authorization: Bearer ${DRTOKEN}" -X POST -d '{"event":"myevent"}' https://${DRHOST}/v1/event
```

#### Troubleshooting

If you recieve a response such as the following, then a clusterrolebinding for the serviceaccount was probbaly not created in the [Install & Configuration](README.md#install--configuration) step.
```
Forbidden (user=system:serviceaccount:redhat-marketplace:ibm-data-reporter-operator-api, verb=get, resource=, subresource=)
```



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

