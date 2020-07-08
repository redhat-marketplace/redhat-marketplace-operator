# Getting Started

Getting Started is designed to give you base level information about the project, and quickly get you up and running.
For an overview of the different components and their purpose, checkout [High Level Overview](high-level-overview.md).

---
## Table of Contents
- [Learning Topics](#learning-topics)
  - [Core Learning](#core-learning)
  - [Relative Learning](#relative-learning)
  - [Additional Resources](#additional-resources)
- [Setup](#setup)
  - [Prerequisites](#prerequisites)
  - [Setup-Steps](#setup-steps)
- [Building and Running](#building-and-running)
  - [Building](#building)
  - [Running](#running-locally-manually)
- [Local Development](#local-development)
  - [Skaffold](#developing-with-skaffold-automatic)
  - [Operator-sdk](#developing-with-operator-sdk-automatic)
- [Testing](#testing)

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
  - [Razee](https://github.com/razee-io/Razee/blob/master/README.md)
- OLM
  - [Understanding the OLM](https://docs.openshift.com/container-platform/4.4/operators/understanding_olm/olm-understanding-olm.html)

### Additional Resources
Some extra resources that could be useful.
- [Openshift Documentation](https://docs.openshift.com/)
- [Kubernetes Operators: Automating the Container Orchestration Platform](https://learning.oreilly.com/library/view/kubernetes-operators/9781492048039/)

---

## Setup

### Prerequisites

These are the required tools (and coding languages) to get the project up and running.
1. Install [golang](https://golang.org/doc/install)
1. Install [operator-sdk](https://github.com/operator-framework/operator-sdk)
1. Install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/) or Install [CRC](https://developers.redhat.com/products/codeready-containers) (**Recommended**)
1. Install [docker-for-mac](https://docs.docker.com/docker-for-mac/install)

_Note:_ You can use minikube, but we'll need to install additional resources.

### Setup steps

1. Start your local kubernetes cluster. 

   For crc:
   ```sh
   # It's advised to create a new crc install with more cpu and more memory. 
   # This gives crc 8 cpu and 12288 MB of ram. (Suggested to use 12288 MB for Prometheus).
   # Small crc clusters are hard to work with and if your computer can handle it, run with these settings

   crc start -c 8 -m 12288
   
   # At this point you will be asked to input your secret.
   # Secret available on: https://cloud.redhat.com/openshift/install/crc/installer-provisioned
   <paste_secret>

   # After setup is complete, ensure commands are available using eval().
   eval $(crc oc-env)

   # Login to your local kubernetes cluster.
   # This information is available on terminal
   # after the 'crc start' command was completed.
    oc login -u kubeadmin -p <your_login_info> <your_cluster_info>
    
    # Optional: to enable Monitoring/Prometheus, refer to your version of CRC: https://access.redhat.com/documentation/en-us/red_hat_codeready_containers/1.12/html/getting_started_guide/administrative-tasks_gsg#starting-monitoring-alerting-telemetry_gsg
   ```

   For minikube:
   ```sh
   # Minikube is lightweight compared to crc but we still require a cerain amount of cpu's and ram.
   # Small clusters are hard to work with and if your computer can handle it, run with these settings

    minikube config set cpus 2
    minikube config set memory 4000

    # Start minikube
    minikube start

    # Minikube requires a few additional resources. 
    # This command will install: operator-framework, prometheus operator, and kube-state
    make setup-minikube

    # Ensure commands are available using eval().
    eval $(minikube docker-env)
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

### Building and pushing

```sh
# Builds the executable and packages the docker image
 make build

 # Pushes the image to your registry.
 make push
```

### Running locally (manually)

```sh
# Once the image is made and available you can run these commands

# make uninstall ensures there is no prior installation on your system
make uninstall

# make install will install all the crds and the operator image with an example CR.
make install

## Note: you can chain together make commands --so you can run this one command instead:
make build uninstall install

```

---

## Local Development

### Developing with Skaffold (automatic)

For a faster dev experience you may enjoy skaffold.

Development flows can be improved a bit using [skaffold](https://skaffold.dev/). To use these steps you'll need to install skaffold.
The skaffold script is made to redeploy resources as needed.

When you install skaffold, you can then run these commands:

```sh
# Run skaffold in dev mode, will watch file changes and auto restart pods
make skaffold-dev

# Run skaffold in run mode, will not watch changes
make skaffold-run
```

### Developing with operator-sdk (automatic)

This is a very fast method of rebuilding locally. It uses the same roles as the real operator by supplying a kubeconfig to the operator-sdk run command.

This is recommended if you do not want to pull and push images to remote registries.

```sh
make setup-operator-sdk run-operator-sdk
```

---

## Testing

Testing has been simplified thanks to the Makefile.

```sh
# Run unit tests
make test

# Results in amount of code coverage provided via unit tests
make test-cover

# Run end to end - uses your current kubectl context
# end to end will run the controller and test to make sure it installed correctly
make test-e2e

```

Unit tests are available in the same folder of the file being tested.
I.E. MarketplaceConfig unit tests are available under `/pkg/controller/marketplaceconfig`. 

_Note:_ the testing suite is undergoing updates,
as such, this may undergo changes.