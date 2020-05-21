# Getting started

## Requirements

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

## Set up your local dev

### Install

1. Install [golang](https://golang.org/doc/install)
1. Install [operator-sdk](https://github.com/operator-framework/operator-sdk)
1. Install [CRC](https://developers.redhat.com/products/codeready-containers)
1. Install [docker-for-mac](https://docs.docker.com/docker-for-mac/install)

_Note:_ You can use minikube, but we'll be using the coreos prometheus operator
for installs and you'll need to install that operator by hand.

### Setup

1. Create a new crc instance.

   ```sh
   # it's advised to create a new crc install with more
   # cpu and more memory. This gives crc 8 cpu and 10G of ram.
   # Small crc clusters are hard to work with and if your computer
   # can handle it run with these settings
   crc start -c 8 -m 10000
   ```

1. Create a docker Setup a private registry on Openshift CRC or use quay.io.

1. Add your registry url to a local .env file. And source it. Some tools will do
   the sourcing automatically for you. Creating a [quay.io](https://quay.io) account is the easiest
   registry to start development.

   ```sh
   export IMAGE_REGISTRY=your.registry.url.here/username
   ```

1. Build and push your images for agent and operator.

1. Start developing! Re-push and iterate.

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
| test | All our integration or end-to-end tests. Also includes testing tools. |
| test/controller | A helper controller test framework. Make it easier to unit test reconcilers.  |
| test/e2e |  End to end tests. |
| test/minikube |  Minikube setup code to provide feature functionality with CRC. |
| version | Version file contains the operator version. |

### Building

```sh
# Builds the executable
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

### Testing

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


## Developing with Skaffold (automatic)

For a faster dev experience you may enjoy skaffold.

Development flows can be improved a bit using [skaffold](https://skaffold.dev/). To use these steps you'll need to install skaffold. The skaffold file is made to deploy

When you install skaffold, you can then run these commands:

```sh
# Run skaffold in dev mode, will watch file changes and auto restart pods
make skaffold-dev

# Run skaffold in run mode, will not watch changes
make skaffold-run
```
