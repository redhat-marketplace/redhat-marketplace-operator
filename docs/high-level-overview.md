# High Level Overview

This document is designed to provide a high level understanding for different components of the operator. Content covers: what specific controllers do, why specific controllers are important, and basic architecture of the project (how different components interact with each other).

---
## Table of Contents
- [Project Structure](#project-structure)
- [CRDs & Their Controllers](#crds--their-controllers)
  - [MarketplaceConfig](#marketplaceconfig)
  - [MeterBase](#meterbase)
  - [MeterDefinition](#meterdefinition)
  - [RazeeDeployment](#razeedeployment)
- [Other Controllers](#other-controllers)
  - [Node Controller](#node-controller)
  - [Subscription Controller](#subscription-controller)

---
## Project Structure

| Folder  |  Purpose  |
|:--|:--|
| assets  | Stores static assets used in the operator  |
| build  | Build output. |
| bundle  | Temporary directory to use for building the CSV bundle. |
| cmd  | Commands to run the operator |
| deploy  |  Deploy specific code included CSV and generated CRDs  |
| docs  | Documentation  |
| /Makefile | Make commands to simplify development |  
| pkg  | The business logic for the operator |
| pkg/apis  | API files to generate CRDs for the operator  |
| pkg/controller  |  Controller codes  |
| pkg/managers  | Code to bootstrap new controller managers  |
| pkg/reporter  | Montioring reporter specific code  |
| pkg/utils  | Helpful utilitly functions  |
| reports | Temporary directory for reporting |
| scripts | Contains scripts to help build or use the operator  |
| test | All our integration or end-to-end tests. Also includes testing tools |
| test/e2e | End to end testing |
| test/rectest | Contains custom and generated code to simply unit testing|
| version | Version file contains the operator version |

---
## CRDs & Their Controllers

### MarketplaceConfig
MarketplaceConfig is the first CR created. Its prime responsibility is to deploy the remaining CRs. Currently (based on flags) MarketplaceConfig can create:
* RazeeDeployment
* MeterBase
* The Operator Source
* The IBM Catalog Source

MarketplaceConfig is valuable because it is a single point of origin that ensures the correct resources are installed on the cluster.

### MeterBase
MeterBase is responsible for setting up prometheus. MeterBase currently creates:
* The Prometheus Operator
* Service for Prometheus
* Persistant Volume Storage

MeterBase is valuable because it ensures we can track metrics via prometheus.

### MeterDefinition
WIP

### RazeeDeployment
WIP

---
## Other Controllers

### Node Controller
WIP

### Subscription Controller
WIP