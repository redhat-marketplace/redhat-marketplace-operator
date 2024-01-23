# IBM&reg; Red Hat Marketplace Operators

| Branch  |                                                                                                            Builds                                                                                                             |
| :-----: | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------: |
| Develop |        [![Test](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml/badge.svg)](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml)        |
| Master  | [![Test](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml/badge.svg?branch=master)](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml) |

## Description

The Red Hat Marketplace Operators are a collection of operators for reporting workload metrics and integrating product install lifecycle through [https://marketplace.redhat.com](https://marketplace.redhat.com).

### [IBM Metrics Operator](v2/README.md)

The [IBM Metrics Operator](v2/README.md) is used to meter workloads and report usage from Prometheus on an OpenShift cluster, and reports it to Red Hat Marketplace.

### [IBM Data Reporter Operator](datareporter/v2/README.md)

The [IBM Data Reporter Operator](datareporter/v2/README.md) is used as a push event interface into the IBM Metrics Operator's data-service, for reporting to Red Hat Marketplace.

### [Red Hat Marketplace Deployment Operator by IBM](deployer/v2/README.md)

The [Red Hat Marketplace Deployment Operator by IBM](deployer/v2/README.md) is used for integrating cluster and operator subscription management on an OpenShift cluster via [https://marketplace.redhat.com](https://marketplace.redhat.com).


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

### Upgrade Policy

The operator releases adhere to semantic versioning and provides a seamless upgrade path for minor and patch releases within the current stable channel.

### Prerequisites
* User with **Cluster Admin** role
* OpenShift Container Platform, major version 4 with any available supported minor version
* It is required to [enable monitoring for user-defined projects](https://docs.openshift.com/container-platform/latest/monitoring/enabling-monitoring-for-user-defined-projects.html) as the Prometheus provider.
  * A minimum retention time of 168h and minimum storage capacity of 40Gi per volume.

### Resources Required

Minimum system resources required:

| Operator                | Memory (MB) | CPU (cores) | Disk (GB) | Nodes |
| ----------------------- | ----------- | ----------- | --------- | ----- |
| **Metrics**   |        750  |     0.25    | 3x1       |    3  |
| **Data Reporter** |        85  |     0.1    | -         |    1  |
| **Deployment** |        250  |     0.25    | -         |    1  |


| Prometheus Provider  | Memory (GB) | CPU (cores) | Disk (GB) | Nodes |
| --------- | ----------- | ----------- | --------- | ----- |
| **[Openshift User Workload Monitoring](https://docs.openshift.com/container-platform/latest/monitoring/enabling-monitoring-for-user-defined-projects.html)** |          1  |     0.1       | 2x40        |   2    |

Multiple nodes are required to provide pod scheduling for high availability for Red Hat Marketplace Data Service and Prometheus.

The IBM Metrics Operator automatically creates 3 x 1Gi PersistentVolumeClaims to store reports as part of the data service, with _ReadWriteOnce_ access mode. Te PersistentVolumeClaims are automatically created by the ibm-metrics-operator after creating a `redhat-marketplace-pull-secret` and accepting the license in `marketplaceconfig`.

| NAME                                | CAPACITY | ACCESS MODES |
| ----------------------------------- | -------- | ------------ |
| rhm-data-service-rhm-data-service-0 | 1Gi | RWO |
| rhm-data-service-rhm-data-service-1 | 1Gi | RWO |
| rhm-data-service-rhm-data-service-2 | 1Gi | RWO |

### Supported Storage Providers

- OpenShift Container Storage / OpenShift Data Foundation version 4.x, from version 4.2 or higher
- IBM Cloud Block storage and IBM Cloud File storage
- IBM Storage Suite for IBM Cloud Paks:
  - File storage from IBM Spectrum Fusion/Scale 
  - Block storage from IBM Spectrum Virtualize, FlashSystem or DS8K
- Portworx Storage, version 2.5.5 or above
- Amazon Elastic File Storage

### Access Modes required

 - ReadWriteOnce (RWO)

### Provisioning Options supported

Choose one of the following options to provision storage for the ibm-metrics-operator data-service

#### Dynamic provisioning using a default StorageClass
   - A StorageClass is defined with a `metadata.annotations: storageclass.kubernetes.io/is-default-class: "true"`
   - PersistentVolumes will be provisioned automatically for the generated PersistentVolumeClaims
--- 
#### Manually create each PersistentVolumeClaim with a specific StorageClass
   - Must be performed before creating a `redhat-marketplace-pull-secret` or accepting the license in `marketplaceconfig`. Otherwise, the automatically generated PersistentVolumeClaims are immutable.
```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: rhm-data-service
  name: rhm-data-service-rhm-data-service-0
  namespace: redhat-marketplace
spec:
  storageClassName: rook-cephfs
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```
---
#### Manually provision each PersistentVolume for the generated PersistentVolumeClaims with a specific StorageClass
  - May be performed before or after creating a `redhat-marketplace-pull-secret` or accepting the license in `marketplaceconfig`.  
```
apiVersion: v1
kind: PersistentVolume
metadata:
  name: rhm-data-service-rhm-data-service-0
spec:
  csi:
    driver: rook-ceph.cephfs.csi.ceph.com
    volumeHandle: rhm-data-service-rhm-data-service-0
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Delete
  storageClassName: rook-cephfs
  volumeMode: Filesystem
  claimRef:
    kind: PersistentVolumeClaim
    namespace: redhat-marketplace
    name: rhm-data-service-rhm-data-service-0
```

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

### Disaster Recovery

To plan for disaster recovery, note the PhysicalVolumeClaims `rhm-data-service-rhm-data-service-N`. 
- In connected environments, MeterReport data upload attempts occur hourly, and are then removed from data-service. There is a low risk of losing much unreported data.
- In an airgap environment, MeterReport data must be pulled from data-service and uploaded manually using `datactl`. To prevent data loss in a disaster scenario, the data-service volumes should be considered in a recovery plan.

### Subscription Config

It is possible to [configure](https://github.com/operator-framework/operator-lifecycle-manager/blob/master/doc/design/subscription-config.md) how OLM deploys an Operator via the `config` field in the [Subscription](https://github.com/operator-framework/olm-book/blob/master/docs/subscriptions.md) object.

The IBM Metrics Operator and Red Hat Marketplace Deployment Operator will also read the `config` and append it to the operands. The primary use case is to control scheduling using [Tolerations](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) and [NodeSelectors](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/).

A limitation is that the `config` elements are only appended to the operands. The elements in the operands are not removed if the `config` is removed from the `Subscripion`. The operand must be modified manually, or deleted and recreated by the controller.

### Operators Visual Diagram

This is a high level visual representation of how the operators interact with Prometheus, CustomResources, with each other, and with Red Hat Marketplace.

![Operators Visual Diagram](diagram.png)

### Documentation

[Red Hat Marketplace](https://marketplace.redhat.com/en-us/documentation)

[Wiki](https://github.com/redhat-marketplace/redhat-marketplace-operator/wiki/Home)
 
