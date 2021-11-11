IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium
IMAGE_NAME = redhat-marketplace-data-service
IMAGE_TAG ?= $(VERSION)
QUAY_EXPIRATION?=never

IMG ?= $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
export IMG
IMAGE ?= $(IMG)
export IMAGE

include ../../utils.Makefile

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
	$(GINKGO) -r -coverprofile=cover-unit.out.tmp -outputdir=. --randomizeAllSpecs --randomizeSuites --cover --race --progress --trace ./...

# Generate
generate:
	go generate ./...

SRC_DIR=$(shell pwd)
DST_DIR=$(SRC_DIR)
UNAME := $(shell echo `uname` | tr '[:upper:]' '[:lower:]')

ARCH ?= amd64

build:
	docker buildx build --load -t airgap:test .

demo: build
	docker-compose up

PROTOC_GEN_GO=$(shell pwd)/bin/protoc-gen-go
protoc-gen-go:
	$(call go-get-tool,$(PROTOC),google.golang.org/protobuf/cmd/protoc-gen-go,)

ifeq ($(UNAME),darwin)
PROTOC_UNAME=osx
else
PROTOC_UNAME=$(UNAME)
endif

ifeq ($(ARCH),amd64)
PROTOC_ARCH=x86_64
else
PROTOC_ARCH=$(ARCH)
endif

PROTOC=$(shell pwd)/bin/protoc
protoc: protoc-gen-go
	$(call install-zip,https://github.com/protocolbuffers/protobuf/releases/download/v3.15.6,protoc-3.15.6-$(PROTOC_UNAME)-$(PROTOC_ARCH).zip,$(PROTOC),$(shell pwd)/bin,./bin/protoc ./include)

# install-binary will 'curl' any package url $1 with file $2 and install it to $3
define install-binary
@[ -f $(3) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
echo "Downloading $(1)" ;\
curl -LO $(1)/$(2) ;\
chmod +x $(2) && mv $(2) $(3) ;\
rm -rf $$TMP_DIR ;\
}
endef

define install-zip
@[ -f $(3) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
echo "Downloading $(1)/$(2)" ;\
curl --fail -LO $(1)/$(2) ;\
unzip $(2) $(foreach a,$(5),&& mv $(a) $(4)) ;\
rm -rf $$TMP_DIR ;\
}
endef
