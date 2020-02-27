# Marketplace Operator

This repo is a temporary home. It will be open-sourced when repo is available.

## Getting started

### Requirements

1. Install [golang](https://golang.org/doc/install)
1. Install [operator-sdk](https://github.com/operator-framework/operator-sdk)
1. Install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)
1. Install [CRC](https://developers.redhat.com/products/codeready-containers)

## New to golang? Operators? Kubernetes?

- Golang
  - [Golang tour](https://tour.golang.org/welcome/1)
  - [Pointers](https://tour.golang.org/moretypes/1) **Recommended**
  - [Concurrency](https://tour.golang.org/concurrency/1) **Recommended**
  - [Channels](https://tour.golang.org/concurrency/2) **Recommended**
- Operators
  - [Read the intro blog ](https://coreos.com/blog/introducing-operators.html)
  - Walk through the [operator-sdk getting
    started](https://github.com/operator-framework/getting-started)
- Kubernetes
  - [Basics](https://kubernetes.io/docs/tutorials/kubernetes-basics/)
  - [Deploy a
    deploymentset](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)

## Building

```sh
# Builds the executable
 make build
```

## Running locally

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

## Implementation

### CRDS

WIP

### Lifecycle

WIP

## IBMers - how to contribute

https://w3.ibm.com/developer/opensource/
