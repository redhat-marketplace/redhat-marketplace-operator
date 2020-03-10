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
    
## Set up your local dev

### Install

1. Install [golang](https://golang.org/doc/install)
1. Install [operator-sdk](https://github.com/operator-framework/operator-sdk)
1. Install [CRC](https://developers.redhat.com/products/codeready-containers)
1. Install [docker-for-mac](https://docs.docker.com/docker-for-mac/install)

*Note:* You can use minikube, but we'll be using the coreos prometheus operator
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
   the sourcing automatically for you.

``` sh
export IMAGE_REGISTRY=your.registry.url.here/username
```

1. Build and push your images for agent and operator. 

1. Start developing! Re-push and iterate.


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

## Extra

### Private registry on CRC

CRC ships with a private registry we can expose. This involves creating a CA,
having the CRC VM trust that CA, create certificates from that CA, and adding
that cert to your local machine.

Once all those steps are complete you can push to the private registry.

``` sh
# generate key - stores it in the a .crc folder in the work directory
make registry-generate-key
# add the ca to the crc server
make registry-add-ca
# add the cert to your local machine
make registry-add-cert
# sets up the remote route for the registry
make registry-setup

# now - restart your docker daemon

# login to registry aka docker login
make registry-login 

# add the docker login to the crc server
make registry-add-docker-auth 
```

You can now tag and push to public-image-registry.apps-crc.testing/symposium
registry. The default of the operator build and agent build repos.

#### Troubleshooting

You may find yourself in a state where your image cannot be pulled. Verify these
things:

``` sh
# Re-run add-docker-auth. This will re-add auth
# to your service account
make registry-add-docker-auth
```

If add-docker-auth doesn't fix it. Try to run:

```sh
export SERVICE_ACCOUNT=marketplace-operator
oc secrets link $SERVICE_ACCOUNT my-docker-secrets --for=pull
```

This will give your service account access to the registry.

*Pro tip:* deleting and recreeating the pod is the easiest method of requeing it.
