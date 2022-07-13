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

### Documentation

[RedHat Marketplace](https://marketplace.redhat.com/en-us/documentation)

[Wiki](https://github.com/redhat-marketplace/redhat-marketplace-operator/wiki/Home)

