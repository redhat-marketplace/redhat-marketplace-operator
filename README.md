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

### Resources Required

Minimum system resources required:

| Software  | Memory (GB) | CPU (cores) | Disk (GB) | Nodes |
| --------- | ----------- | ----------- | --------- | ----- |
| **Total** |          7  |     2       | 40        |       |

### Storage

The RedHat Marketplace Operator creates 2 dynamic persistent volumes to store monitoring data used for telemetry, both with _ReadWriteOnce_ access mode.

### Installing

For installation and configuration see the [RedHat Marketplace documentation](https://marketplace.redhat.com/en-us/documentation/getting-started/).


## Additional information

### SecurityContextConstraints requirements

The Redhat Marketplace Operator and its components support running under the OpenShift Container Platform default restricted security context constraints except for the Razee deployment controller which requires the anyuid security context constraint.

### Documentation

[RedHat Marketplace](https://marketplace.redhat.com/en-us/documentation)

[Wiki](https://github.com/redhat-marketplace/redhat-marketplace-operator/wiki/Home)

