include ../../utils.Makefile

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium
IMAGE_NAME = redhat-marketplace-data-service
IMAGE_TAG ?= $(TAG)
QUAY_EXPIRATION?=never

IMG ?= $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
export IMG
IMAGE ?= $(IMG)
export IMAGE

all: svu generate vet fmt

IMAGE_PUSH ?= true
BUILDX ?= true
ARCH ?= amd64

ifeq ($(BUILDX),true)
DOCKERCMD=docker buildx
ifeq ($(IMAGE_PUSH),true)
ARGS="--push"
else
ARGS="--load"
endif
else
DOCKERCMD=docker
endif

# Build the docker image
docker-build:
	$(call docker-templated-build,$(IMAGE),DataService,./airgap/v2/,,redhat-marketplace-data-service,../..)

# Push the docker image
docker-push:
	docker push ${IMG}


docker-manifest:
	docker manifest create $(IMAGE) $(addprefix $(IMAGE)-,$(ARCHS))
	docker manifest push $(IMAGE)

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

.PHONY: test
test: fmt vet ginkgo
	$(GINKGO) -r -coverprofile=cover-unit.out --randomize-all --randomize-suites --cover --race --show-node-events --trace ./...

buff-update:
	$(BUF) mod update
	$(BUF) mod update ./apis

# Generate
generate: buf buff-update
	$(BUF) generate
	go generate ./...

SRC_DIR=$(shell pwd)
DST_DIR=$(SRC_DIR)
UNAME := $(shell echo `uname` | tr '[:upper:]' '[:lower:]')

ARCH ?= amd64

build:
	go build -o $(BINDIR)/redhat-marketplace-data-service ./cmd/server/main.go

demo: build
	docker-compose up

# skaffold helper for ibm-metrics-operator
.PHONY: kustomize-set-operator
kustomize-set-operator:
	cd ../../v2/config/manager && \
	$(KUSTOMIZE) edit set annotation \
	dqLiteImage:$(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
