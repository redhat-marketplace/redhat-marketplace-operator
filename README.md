# IBM&reg; Red Hat Marketplace Operator

| Branch  |                                                                                                            Builds                                                                                                             |
| :-----: | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------: |
| Develop |        [![Test](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml/badge.svg)](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml)        |
| Master  | [![Test](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml/badge.svg?branch=master)](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml) |

## Description

The [IBM Metrics Operator](v2/README.md) and [Red Hat Marketplace Deployment Operator by IBM](deployer/v2/README.md) are the OpenShift client side tools for the Red Hat Marketplace. 

The [IBM Metrics Operator](v2/README.md) is used to meter and report workload usage on an OpenShift cluster, and report it through Red Hat Marketplace.

The [Red Hat Marketplace Deployment Operator by IBM](deployer/v2/README.md) is used for cluster and operator subscription management on an OpenShift cluster with the Red Hat Marketplace.

Please visit [https://marketplace.redhat.com](https://marketplace.redhat.com) for more info.


## Installation

### **Upgrade Notice**

The Red Hat Marketplace Operator metering and deployment functionalities have been separated into two operators.
  - The metering functionality is included in the IBM Metrics Operator
    - Admin level functionality and permissions are removed from the IBM Metrics Operator
    - ClusterServiceVersion/ibm-metrics-operator
  - The deployment functionality remains as part of the Red Hat Marketplace Deployment Operator
    - The Red Hat Marketplace Deployment Operator prerequisites the IBM Metrics Operator
    - Some admin level RBAC permissions are required for deployment functionality
    - ClusterServiceVersion/redhat-marketplace-operator

Full registration and visibility of usage metrics on [https://marketplace.redhat.com](https://marketplace.redhat.com) requires both IBM Metrics Operator and Red Hat Marketplace Deployment Operator.

### Prerequisites
* User with **Cluster Admin** role
* OpenShift Container Platform, major version 4 with any available supported minor version
* It is required to [enable monitoring for user-defined projects](https://docs.openshift.com/container-platform/4.12/monitoring/enabling-monitoring-for-user-defined-projects.html) as the Prometheus provider.
  * A minimum retention time of 168h and minimum storage capacity of 40Gi per volume.

### Resources Required

Minimum system resources required:

| Operator                | Memory (MB) | CPU (cores) | Disk (GB) | Nodes |
| ----------------------- | ----------- | ----------- | --------- | ----- |
| **Metrics**   |        750  |     0.25    | 3x1       |    3  |
| **Deployment** |        250  |     0.25    | -         |    1  |

| Prometheus Provider  | Memory (GB) | CPU (cores) | Disk (GB) | Nodes |
| --------- | ----------- | ----------- | --------- | ----- |
| **[Openshift User Workload Monitoring](https://docs.openshift.com/container-platform/4.12/monitoring/enabling-monitoring-for-user-defined-projects.html)** |          1  |     0.1       | 2x40        |   2    |

Multiple nodes are required to provide pod scheduling for high availability for Red Hat Marketplace Data Service and Prometheus.

### Storage

The IBM Metrics Operator creates 3 x 1GB dynamic persistent volumes to store reports as part of the data service, with _ReadWriteOnce_ access mode.

The IBM Metrics Operator requires User Workload Monitoring to be configured with 40Gi persistent volumes at minimum.

### Installing

For installation and configuration see the [Red Hat Marketplace documentation](https://marketplace.redhat.com/en-us/documentation/getting-started/).


## Additional information

### SecurityContextConstraints requirements

The Operators and their components support running under the OpenShift Container Platform default restricted and restricted-v2 security context constraints.

### Installation Namespace and ClusterRoleBinding requirements

The IBM Metrics Operator components require specific ClusterRoleBindings.
- The metric-state component requires a ClusterRoleBinding for the the `view` ClusterRole. 
- The reporter component requires a ClusterRoleBinding for the the `cluster-monitoring-view` ClusterRole. 

Due to limitations of Operator Lifecycle Manager (OLM), this ClusterRoleBinding can not be provided automatically for arbitrary installation target namespaces.

A ClusterRoleBinding is included for installation to the default namespace of `redhat-marketplace`, and namespaces `openshift-redhat-marketplace`, `ibm-common-services`.

To update the ClusterRoleBindings for installation to an alternate namespace
```
oc patch clusterrolebinding ibm-metrics-operator-metric-state-view-binding --type='json' -p='[{"op": "add", "path": "/subjects/1", "value": {"kind": "ServiceAccount", "name": "ibm-metrics-operator-metric-state","namespace": "NAMESPACE" }}]'

oc patch clusterrolebinding ibm-metrics-operator-reporter-cluster-monitoring-binding --type='json' -p='[{"op": "add", "path": "/subjects/1", "value": {"kind": "ServiceAccount", "name": "ibm-metrics-operator-reporter","namespace": "NAMESPACE" }}]'
```

### Metric State scoping requirements
The metric-state Deployment obtains `get/list/watch` access to metered resources via the `view` ClusterRole. For operators deployed using Operator Lifecycle Manager (OLM), permissions are added to `clusterrole/view` dynamically via a generated and annotated `-view` ClusterRole. If you wish to meter an operator, and its Custom Resource Definitions (CRDs) are not deployed through OLM, one of two options are required
1. Add the following label to a clusterrole that has get/list/watch access to your CRD: `rbac.authorization.k8s.io/aggregate-to-view: "true"`, thereby dynamically adding it to `clusterrole/view`
2. Create a ClusterRole that has get/list/watch access to your CRD, and create a ClusterRoleBinding for the metric-state ServiceAccount

Attempting to meter a resource with a MeterDefinition without the required permissions will log an `AccessDeniedError` in metric-state.

### Cluster permission requirements

|IBM Metrics Operator                                               |API group             |Resources                 |Verbs                                     |Description                                                                                                                                             |
|----------------------------------------------------------------|----------------------|--------------------------|------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------|
|ServiceAcount: ibm-metrics-operator                                |                      |                          |                                          |                                                                                                                                                        |
|ClusterRole:metrics-manager-role                               |                      |                          |                                          |                                                                                                                                                        |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|Controller                                                      |                      |                          |                                          |                                                                                                                                                        |
|clusterregistration                                             |config.openshift.io   |clusterversions           |get;list;watch                            |read clusterid from clusterversion                                                                                                                      |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|clusterserviceversion                                           |operators.coreos.com  |clusterserviceversions    |get;list;watch                            |read clusterserviceversions, create meterdefinition from clusterserviceversion annotation                                                               |
|                                                                |marketplace.redhat.com|meterdefinitions          |get;list;watch;create;update;patch;delete |read clusterserviceversions, create meterdefinition from clusterserviceversion annotation                                                               |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|marketplaceconfig                                               |                      |namespaces                |get;list;watch                            |get deployed namespace, cluster scope maybe unnecessary                                                                                                 |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|meterbase                                                       |                      |configmaps                |get;list;watch                            |read userworkloadmonitoring configmap to validate minimum requirements                                                                                  |
|                                                                |storage.k8s.io        |storageclasses            |get;list;watch                            |check if there is a defaultstorageclass                                                                                                                 |
|                                                                |apps                  |statefulsets              |get;list;watch                            |read userworkloadmonitoring readiness                                                                                                                   |
|                                                                |marketplace.redhat.com|meterdefinitions          |get;list;watch;create;update;patch;delete |create default set of uptime meterdefinitions                                                                                                           |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|meterdefinition                                                 |marketplace.redhat.com|meterdefinitions          |get;list;watch;create;update;patch;delete |update meterdefinition status                                                                                                                           |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|meterdefinition_install                                         |operators.coreos.com  |clusterserviceversions    |get;list;watch                            |MeterdefinitionCatalogServer, determine if there is installmapping                                                                                      |
|                                                                |operators.coreos.com  |subscriptions             |get;list;watch                            |MeterdefinitionCatalogServer, determine if there is installmapping                                                                                      |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|ServiceAccount: metrics-servicemonitor-metrics-reader          |                      |                          |                                          |                                                                                                                                                        |
|ClusterRole: metrics-servicemonitor-metrics-reader             |                      |                          |                                          |                                                                                                                                                        |
|                                                                |authentication.k8s.io |tokenreviews              |create                                    |Servicemonitor kube-rbac-proxy protect metrics endpoint for operator and metric-state                                                                   |
|                                                                |authorization.k8s.io  |subjectaccessreviews      |create                                    |Servicemonitor kube-rbac-proxy protect metrics endpoint for operator and metric-state                                                                   |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|ServiceAccount: ibm-metrics-operator-metric-state                           |                      |                          |                                          |                                                                                                                                                        |
|ClusterRole: ibm-metrics-operator-metric-state                              |                      |                          |                                          |                                                                                                                                                        |
|metric-state                                                    |marketplace.redhat.com|meterdefinitions          |get;list;watch                            |read meterdefinitions to associate metered workloads                                                                                                    |
|                                                                |                      |services                  |get;list;watch                            |get userworkloadmonitoring prometheus service                                                                                                           |
|                                                                |marketplace.redhat.com|meterdefinitions/status   |get;list;watch;update                     |update status on meterdefinitions                                                                                                                       |
|                                                                |operators.coreos.com  |operatorgroups            |get;list                                  |find list of namespaces a product is installed to via OperatorGroup                                                                                     |
|                                                                |monitoring.coreos.com |servicemonitors           |get;list;watch;update                     |update servicemonitor labels                                                                                                                            |
|                                                                |authentication.k8s.io |tokenreviews              |create                                    |authchecker, kube-rbac-proxy, protect metrics endpoint                                                                                                  |
|                                                                |authorization.k8s.io  |subjectaccessreviews      |create;delete;get;list;update;patch;watch |kube-rbac-proxy, protect metrics endpoint                                                                                                               |
|                                                                |                      |clusterrole/view          |                                          |Cluster-wide reader (non-sensitive) to track usage via resource filters. https://kubernetes.io/docs/reference/access-authn-authz/rbac/#user-facing-roles|
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|ServiceAcount: ibm-metrics-operator-data-service                            |                      |                          |                                          |                                                                                                                                                        |
|ClusterRole: ibm-metrics-operator-data-service                              |                      |                          |                                          |                                                                                                                                                        |
|data-service                                                    |authentication.k8s.io |tokenreviews              |create                                    |authchecker                                                                                                                                             |
|                                                                |authorization.k8s.io  |subjectaccessreviews      |create                                    |authchecker                                                                                                                                             |


|Deployment Operator                                               |API group             |Resources                 |Verbs                                     |Description                                                                                                                                             |
|----------------------------------------------------------------|----------------------|--------------------------|------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------|
|ServiceAcount: redhat-marketplace-operator                      |                      |                          |                                          |                                                                                                                                                        |
|ClusterRole: redhat-marketplace-manager-role                    |                      |                          |                                          |                                                                                                                                                        |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|Controller                                                      |                      |                          |                                          |                                                                                                                                                        |
|clusterserviceversion                                           |operators.coreos.com  |clusterserviceversions    |get;list;watch;update;patch               |set watch label for installations from marketplace                                                                                                      |
|                                                                |operators.coreos.com  |subscriptions             |get;list;watch                            |check subscription installation status                                                                                                                  |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|razeedeployment                                                 |                      |namespaces                |get;list;watch                            |check if targetnamespace exists; legacy, deprecated                                                                                                     |
|                                                                |operators.coreos.com  |catalogsources            |create;get;list;watch;delete              |create ibm & opencloud catalogsources in the openshift-marketplace namespace                                                                            |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|rhm_subscription                                                |operators.coreos.com  |subscriptions             |get;list;watch;update;patch               |label subscription                                                                                                                                      |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|subscription                                                    |operators.coreos.com  |subscriptions             |get;list;watch;delete                     |delete subscription if tagged for deletion via Marketplace                                                                                              |
|                                                                |operators.coreos.com  |clusterserviceversions    |get;list;watch;delete                     |delete clusterserviceversion if tagged for deletion via Marketplace                                                                                     |
|                                                                |operators.coreos.com  |operatorgroups            |get;list;watch;delete;create              |Razee creates subscription via Marketplace UI, create operatorgroup if needed                                                                           |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|ServiceAcount: redhat-marketplace-remoteresources3deployment    |                      |                          |                                          |                                                                                                                                                        |
|ClusterRole: redhat-marketplace-remoteresources3deployment      |                      |                          |                                          |                                                                                                                                                        |
|remoteresources3deployment                                      |operators.coreos.com  |subscriptions             |*                                         |Manages subscription from Marketplace                                                                                                                   |
|                                                                |marketplace.redhat.com|remoteresources3s         |get;list;watch                            |                                                                                                                                                        |
|                                                                |authentication.k8s.io |tokenreviews              |create                                    |authchecker                                                                                                                                             |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|ServiceAcount: redhat-marketplace-remoteresources3deployment    |                      |                          |                                          |Labeled resources uploaded to Marketplace COS bucket                                                                                                    |
|ClusterRole: redhat-marketplace-remoteresources3deployment      |                      |                          |                                          |Subscription/Install management & Cluster information                                                                                                   |
|watch-keeper                                                    |                      |configmaps                |get;list;watch                            |                                                                                                                                                        |
|                                                                |                      |namespaces                |get;list;watch                            |                                                                                                                                                        |
|                                                                |                      |nodes                     |get;list;watch                            |                                                                                                                                                        |
|                                                                |                      |pods                      |get;list;watch                            |                                                                                                                                                        |
|                                                                |apps                  |deployments               |get;list;watch                            |                                                                                                                                                        |
|                                                                |apps                  |replicasets               |get;list;watch                            |                                                                                                                                                        |
|                                                                |apiextensions.k8s.io  |customresourcedefinitions |get;list;watch                            |                                                                                                                                                        |
|                                                                |operators.coreos.com  |clusterserviceversions    |get;list;watch                            |                                                                                                                                                        |
|                                                                |operators.coreos.com  |subscriptions             |get;list;watch                            |                                                                                                                                                        |
|                                                                |config.openshift.io   |clusterversions           |get;list;watch                            |                                                                                                                                                        |
|                                                                |config.openshift.io   |infrastructures           |get;list;watch                            |                                                                                                                                                        |
|                                                                |config.openshift.io   |consoles                  |get;list;watch                            |                                                                                                                                                        |
|                                                                |marketplace.redhat.com|marketplaceconfigs        |get;list;watch                            |                                                                                                                                                        |
|                                                                |authentication.k8s.io |tokenreviews              |create                                    |authchecker                                                                                                                                             |
|                                                                |                      |                          |                                          |                                                                                                                                                        |
|ServiceAccount: redhat-marketplace-servicemonitor-metrics-reader|                      |                          |                                          |                                                                                                                                                        |
|ClusterRole: redhat-marketplace-servicemonitor-metrics-reader   |                      |                          |                                          |                                                                                                                                                        |
|                                                                |authentication.k8s.io |tokenreviews              |create                                    |Servicemonitor kube-rbac-proxy protect metrics endpoint for operator and metric-state                                                                   |
|                                                                |authorization.k8s.io  |subjectaccessreviews      |create                                    |Servicemonitor kube-rbac-proxy protect metrics endpoint for operator and metric-state                                                                   |



### Documentation

[Red Hat Marketplace](https://marketplace.redhat.com/en-us/documentation)

[Wiki](https://github.com/redhat-marketplace/redhat-marketplace-operator/wiki/Home)
 