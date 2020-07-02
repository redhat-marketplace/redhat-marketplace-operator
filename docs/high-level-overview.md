# High Level Overview

This document is designed to provide a high level understanding for different components of the operator. Content covers: what specific controllers do, why specific controllers are important, basic architecture of the project (how different components interact with each other) and .

## Table of Contents
- [CRDs & Their Controllers](#crds--their-controllers)
  - [MarketplaceConfig](#marketplaceconfig)
  - [MeterBase](#meterbase)
  - [MeterDefinition](#meterdefinition)
  - [RazeeDeployment](#razeedeployment)
- [Other Controllers](#other-controllers)
  - [Node Controller](#node-controller)
  - [Subscription Controller](#subscription-controller)

### CRDs & Their Controllers
---

#### MarketplaceConfig
MarketplaceConfig is the first CR created. Its prime responsibility is to deploy the remaining CRs. Currently (based on flags) MarketplaceConfig can create:
* RazeeDeployment
* MeterBase
* The Operator Source
* The IBM Catalog Source

MarketplaceConfig is valuable because it is a single point of origin that ensures the correct resources are installed on the cluster.

#### MeterBase
MeterBase is responsible for setting up prometheus. MeterBase currently creates:
* The Prometheus Operator
* Service for Prometheus
* Persistant Volume Storage

MeterBase is valuable because it ensures we can track metrics via prometheus.

#### MeterDefinition
WIP

#### RazeeDeployment
WIP

### Other Controllers
---
#### Node Controller
WIP

#### Subscription Controller
WIP