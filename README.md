# IBM&reg; IBM Software Central & Red Hat Marketplace Operators

| Branch  |                                                                                                            Builds                                                                                                             |
| :-----: | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------: |
| Develop |        [![Test](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml/badge.svg)](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml)        |
| Master  | [![Test](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml/badge.svg?branch=master)](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml) |

## Description

The operators provide functionality for reporting workload metrics and integrating product install lifecycle through [https://swc.saas.ibm.com](https://swc.saas.ibm.com).

### [IBM Metrics Operator](v2/README.md)

The [IBM Metrics Operator](v2/README.md) is used to meter workloads and report usage from Prometheus on an OpenShift cluster, and reports it to IBM Software Central.

### [IBM Data Reporter Operator](datareporter/v2/README.md)

The [IBM Data Reporter Operator](datareporter/v2/README.md) provides a push event interface into the IBM Metrics Operator's data-service, for reporting metrics to IBM Software Central.

### Red Hat Marketplace Deployment Operator by IBM

The Red Hat Marketplace Deployment Operator is deprecated, with final release v2.15.0.

### Note

Usage metrics may be monitored through [https://swc.saas.ibm.com](https://swc.saas.ibm.com) with only IBM Metrics Operator and a Red Hat Marketplace account.

## Installation

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


| Prometheus Provider  | Memory (GB) | CPU (cores) | Disk (GB) | Nodes |
| --------- | ----------- | ----------- | --------- | ----- |
| **[Openshift User Workload Monitoring](https://docs.openshift.com/container-platform/latest/monitoring/enabling-monitoring-for-user-defined-projects.html)** |          1  |     0.1       | 2x40        |   2    |

Multiple nodes are required to provide pod scheduling for high availability for IBM Metrics Operator Data Service and Prometheus.

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
- Azure Disk Storage (disk.csi.azure.com)

#### Known Problematic Storage Providers

- Azure File Storage (file.csi.azure.com)

### Access Modes required

 - ReadWriteOnce (RWO)

### Provisioning Options supported

Choose one of the following options to provision storage for the ibm-metrics-operator data-service

The precedence for determining the storageClassName is as follows:
- If the data-service StatefulSet already exists, the reconciler will use the existing immutable value
- If the PersistentVolumeClaims are already created, the reconciler will use the PersistentVolumeClaim storageClassName
- If MarketplaceConfig sets the `spec.storageClassName` at creation, the reconciler will use the MarketplaceConfig value (via MeterBase)
- If a defaultStorageClass is available, the reconciler will use the defaultStorageClass storageClassName

#### MarketplaceConfig spec.storageClassName
The StorageClass may be manually specified by setting the `spec.storageClassName` on the `markeplaceconfig`, either when the `markeplaceconfig` is created, or before setting `spec.license.accept=true`. The `spec.storageClassName` is an immutable field, as the data-service `spec.volumeClaimTemplates.[].storageClassName` is also immutable.

---
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
  namespace: ibm-software-central
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
    namespace: ibm-software-central
    name: rhm-data-service-rhm-data-service-0
```

### Kyverno Policies

The Operators make a best effort to comply with the Kyverno policies defined by [IBM MAS](https://github.com/ibm-mas/kyverno-policies)

Exceptions:
- Operators are not replicated
  - Service uptime is not considered critial, and can be rescheduled
- metric-state is not replicated
  - Service uptime is not considered critial, and can be rescheduled

### Getting Started with the platform

To get started with the platform, see the [documentation](https://swc.saas.ibm.com/en-us/documentation/getting-started/).


## Additional information

### SecurityContextConstraints requirements

The Operators and their components support running under the OpenShift Container Platform default restricted and restricted-v2 security context constraints.

### Installation Namespace and ClusterRoleBinding requirements

The IBM Metrics Operator components require specific ClusterRoleBindings.
- The metric-state component requires a ClusterRoleBinding for the the `view` ClusterRole.
  - Ability to read non-sensitive CustomResources. 
  - The `view` ClusterRole dynamically adds new CustomResourceDefinitions.
- The operator & reporter component requires a ClusterRoleBinding for the the `cluster-monitoring-view` ClusterRole.
  - Ability to view Prometheus metrics. 
  - The underlying ClusterRole RBAC often updated between OpenShift versions.

Due to limitations of Operator Lifecycle Manager (OLM), this ClusterRoleBinding can not be provided dynamically for arbitrary installation target namespaces.

A static ClusterRoleBinding is included for installation to the default namespace of `ibm-software-central`, and namespaces `openshift-redhat-marketplace`,  `redhat-marketplace`, `ibm-common-services`.

To create the ClusterRoleBindings for installation to an alternate namespace
```
oc project INSTALL-NAMESPACE
oc adm policy add-cluster-role-to-user view -z ibm-metrics-operator-metric-state
oc adm policy add-cluster-role-to-user cluster-monitoring-view -z ibm-metrics-operator-controller-manager,ibm-metrics-operator-reporter
```

### Metric State scoping requirements
The metric-state Deployment obtains `get/list/watch` access to metered resources via the `view` ClusterRole. For operators deployed using Operator Lifecycle Manager (OLM), permissions are added to `clusterrole/view` dynamically via a generated and annotated `-view` ClusterRole. If you wish to meter an operator, and its Custom Resource Definitions (CRDs) are not deployed through OLM, one of two options are required
1. Add the following label to a clusterrole that has get/list/watch access to your CRD: `rbac.authorization.k8s.io/aggregate-to-view: "true"`, thereby dynamically adding it to `clusterrole/view`
2. Create a ClusterRole that has get/list/watch access to your CRD, and create a ClusterRoleBinding for the metric-state ServiceAccount

Attempting to meter a resource with a MeterDefinition without the required permissions will log an `AccessDeniedError` in metric-state.

### Disaster Recovery

To plan for disaster recovery, note the PhysicalVolumeClaims `rhm-data-service-rhm-data-service-N`. 
- In connected environments, MeterReport data upload attempts occur hourly, and are then removed from data-service. There is a low risk of losing much unreported data.
- In an airgap environment, MeterReport data must be pulled from data-service and uploaded manually using [datactl](https://github.com/redhat-marketplace/datactl/). To prevent data loss in a disaster scenario, the data-service volumes should be considered in a recovery plan.

### Subscription Config

It is possible to [configure](https://github.com/operator-framework/operator-lifecycle-manager/blob/master/doc/design/subscription-config.md) how OLM deploys an Operator via the `config` field in the [Subscription](https://github.com/operator-framework/olm-book/blob/master/docs/subscriptions.md) object.

The IBM Metrics Operator will also read the `config` and append it to the operands. The primary use case is to control scheduling using [Tolerations](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) and [NodeSelectors](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/).

A limitation is that the `config` elements are only appended to the operands. The elements in the operands are not removed if the `config` is removed from the `Subscripion`. The operand must be modified manually, or deleted and recreated by the controller.

### Operators Visual Diagram

This is a high level visual representation of how the operators interact with Prometheus, CustomResources, with each other, and with IBM Software Central.

![Operators Visual Diagram](diagram.png)

### Documentation

[IBM Software Central](https://swc.saas.ibm.com/en-us/documentation)

[Wiki](https://github.com/redhat-marketplace/redhat-marketplace-operator/wiki/Home)
 