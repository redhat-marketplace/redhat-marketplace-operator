# IBM&reg; RedHat Marketplace Operator

| Branch  |                                                                                                            Builds                                                                                                             |
| :-----: | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------: |
| Develop |        [![Test](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml/badge.svg)](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml)        |
| Master  | [![Test](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml/badge.svg?branch=master)](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions/workflows/test.yml) |

## Description

The Red Hat Marketplace operator is the Openshift client side tool for the Red Hat Marketplace. It is used to register an Openshift cluster with the Red Hat Marketplace. Please visit [https://marketplace.redhat.com](https://marketplace.redhat.com) for more info.



## Installation

### Prerequisites
* User with **Cluster Admin** role
* OpenShift Container Platform, major version 4 with any available supported minor version
* It is recommended to [enable monitoring for user-defined projects](https://docs.openshift.com/container-platform/4.10/monitoring/enabling-monitoring-for-user-defined-projects.html) as the Prometheus provider.
  * A minimum retention time of 168h and minimum storage capacity of 40Gi per volume.

### Resources Required

Minimum system resources required:

| Operator  | Memory (GB) | CPU (cores) | Disk (GB) | Nodes |
| --------- | ----------- | ----------- | --------- | ----- |
| **Operator** |          1  |     0.5       | 3x1        |    3   |

| Prometheus Provider  | Memory (GB) | CPU (cores) | Disk (GB) | Nodes |
| --------- | ----------- | ----------- | --------- | ----- |
| **[Openshift User Workload Monitoring](https://docs.openshift.com/container-platform/4.10/monitoring/enabling-monitoring-for-user-defined-projects.html)** |          1  |     0.1       | 2x40        |   2    |
| **RedHat Marketplace Prometheus** |          2.5  |     0.5       | 2x20        |    2   |

Multiple nodes are required to provide pod scheduling for high availability for RedHat Marketplace Data Service and Prometheus.

### Storage

The RedHat Marketplace Operator creates 3 x 1GB dynamic persistent volumes to store reports as part of the data service, with _ReadWriteOnce_ access mode.

If using Openshift User Workload Monitoring as the Prometheus provider, the RedHat Marketplace Operator requires User Workload Monitoring to be configured with 40Gi persistent volumes at minimum.

If using RedHat Marketplace Operator to configure a Prometheus provider, the RedHat Marketplace Operator creates 2 x 20Gi dynamic persistent volumes to store monitoring data used for telemetry, both with _ReadWriteOnce_ access mode.

### Installing

For installation and configuration see the [RedHat Marketplace documentation](https://marketplace.redhat.com/en-us/documentation/getting-started/).


## Additional information

### SecurityContextConstraints requirements

The Redhat Marketplace Operator and its components support running under the OpenShift Container Platform default restricted security context constraints.

### Metric State scoping requirements
The metric-state Deployment obtains `get/list/watch` access to metered resources via the `view` ClusterRole. For operators deployed using Openshift Lifecycle Manager (OLM), permissions are added to `clusterrole/view` dynamically via a generated and annotated `-view` ClusterRole. If you wish to meter an operator, and its Custom Resource Definitions (CRDs) are not deployed through OLM, one of two options are required
1. Add the following label to a clusterrole that has get/list/watch access to your CRD: `rbac.authorization.k8s.io/aggregate-to-view: "true"`, thereby dynamically adding it to `clusterrole/view`
2. Create a ClusterRole that has get/list/watch access to your CRD, and create a ClusterRoleBinding for the metric-state ServiceAccount

Attempting to meter a resource with a MeterDefinition without the required permissions will log an `AccessDeniedError` in metric-state.
### Documentation

[RedHat Marketplace](https://marketplace.redhat.com/en-us/documentation)

[Wiki](https://github.com/redhat-marketplace/redhat-marketplace-operator/wiki/Home)
 
