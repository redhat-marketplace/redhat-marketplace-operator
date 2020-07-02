# Getting Started

Getting Started is designed to give you base level information about the project, and quickly get you up and running.
For an overview of the different components and their purpose, checkout [High Level Overview](../high-level-overview.md).

---
## Table of Contents
- [Learning Topics](#learning-topics)
  - [Core Learning](#core-learning)
  - [Relative Learning](#relative-learning)
  - [Additional Resources](#additional-resources)
  - [Project Structure](#project-structure)
- [Setup](#setup)
  - [Prerequisites](#prerequisites)
  - [Setup-Steps](#setup-steps)
- [Building and Running](#building-and-running)
  - [Building](#building)
  - [Running](#running-locally-manually)
- [Testing](#testing)
- [Local Development](#local-development)
  - [Skaffold](#developing-with-skaffold-automatic)
  - [Operator-sdk](#running-locally-using-operator-sdk-automatic)

---

## Learning Topics
New to golang? Operators? Kubernetes? Here is everything you need to know.

### Core learning
These are core skills that are always applicable, regardless of what aspect of the project you are working on.

- Git Flow
  - [Git Flow Tutorial](https://www.atlassian.com/git/tutorials/comparing-workflows/gitflow-workflow)
- Golang
  - [Golang tour](https://tour.golang.org/welcome/1)
  - [Pointers](https://tour.golang.org/moretypes/1) (**Recommended**)
  - [Concurrency](https://tour.golang.org/concurrency/1) (**Recommended**)
  - [Channels](https://tour.golang.org/concurrency/2) (**Recommended**)
- Operators
  - [Read the intro blog ](https://coreos.com/blog/introducing-operators.html)
  - Walk through the [operator-sdk getting started](https://github.com/operator-framework/getting-started)
- Kubernetes
  - [Basics](https://kubernetes.io/docs/tutorials/kubernetes-basics/)
  - [Deploy a deploymentset](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)

### Relative learning
These are additional skills that may or may not be applicable, depending on which aspect of the project you are working oon.

- Prometheus
  - [Getting started](https://prometheus.io/docs/prometheus/latest/getting_started/)
  - [Metric Types](https://prometheus.io/docs/concepts/metric_types/)
  - [Prometheus and Golang](https://prometheus.io/docs/guides/go-application/)
  - [Prometheus Operator](https://github.com/coreos/prometheus-operator/blob/master/Documentation/user-guides/getting-started.md) **Recommended**
- Razee
  - PlaceHolder
  - [Razee](https://github.com/razee-io/Razee)
- OLM
  - PlaceHolder

### Additional Resources
Some extra resources that could be useful.
- Example
  - Placeholder

### Project Structure

| Folder  |  Purpose  |
|:--|:--|
| assets  | Stores static assets used in the operator  |
| build  | Build output. |
| bundle  | Temporary directory to use for building the CSV bundle. |
| cmd  | Commands to run the operator |
| deploy  |  Deploy specific code included CSV and generated CRDs  |
| docs  | Documentation  |
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

## Setup

### Prerequisites

These are the required tools (and coding languages) to get the project up and running.
1. Install [golang](https://golang.org/doc/install)
1. Install [operator-sdk](https://github.com/operator-framework/operator-sdk)
1. Install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/) or Install [CRC](https://developers.redhat.com/products/codeready-containers) (**Recommended**)
1. Install [docker-for-mac](https://docs.docker.com/docker-for-mac/install)

_Note:_ You can use minikube, but we'll be using the coreos prometheus operator
for installs and you'll need to install that operator by hand.

### Setup steps

1. Create a new crc instance.

   ```sh
   # it's advised to create a new crc install with more
   # cpu and more memory. This gives crc 8 cpu and 10G of ram.
   # Small crc clusters are hard to work with and if your computer
   # can handle it run with these settings
   crc start -c 8 -m 10000
   ```

1. Create a docker Setup with a private registry on Openshift CRC or use quay.io. Creating a [quay.io](https://quay.io) account is the easiest
   registry to start development.

1. Add your registry url to a local .env file. And source it. Some tools will do
   the sourcing automatically for you.

   ```sh
   export IMAGE_REGISTRY=your.registry.url.here/username
   ```

1. Build and push your images for the operator.

    ```sh
    operator-sdk build your.registry.url.here/username/image-name
    docker push your.registry.url.here/username/image-name
    ```

1. Start developing! Re-push and iterate.

---

## Building and Running

### Building

```sh
# Builds the executable and packages the docker image
 make build
```

### Running locally (manually)

```sh
# To run locally you should create your image on your
# target env, either minikube or crc
make build

# Once the image is made you can run these commands

# make uninstall ensures there is no prior installation on your system
make uninstall

# make install will install all the crds and the operator image with
# an example CR.
make install

## --or you can run this one command:
make build uninstall install

```

---

## Testing

```sh
# Run unit tests
make test

# Run cover on unit tests
make test-cover

# Run end 2 end - uses your current
# kubectl context
make test-e2e

# end to end will run the controller and test
# to make sure it installed correctly
```


## Local Development

### Developing with Skaffold (automatic)

For a faster dev experience you may enjoy skaffold.

Development flows can be improved a bit using [skaffold](https://skaffold.dev/). To use these steps you'll need to install skaffold. The skaffold file is made to deploy

When you install skaffold, you can then run these commands:

```sh
# Run skaffold in dev mode, will watch file changes and auto restart pods
make skaffold-dev

# Run skaffold in run mode, will not watch changes
make skaffold-run
```

### Running locally using operator-sdk (automatic)

This is a very fast method of rebuilding locally. It uses the same roles as the real operator by supplying a kubeconfig to the operator-sdk run command.

This is recommended if you do not want to pull and push images to remote registries.

```sh
make setup-operator-sdk-run operator-sdk-run
```
