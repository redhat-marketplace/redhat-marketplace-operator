VERSION ?= 1.0.5
export VERSION

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium
IMAGE_NAME = redhat-marketplace-authcheck
IMAGE_TAG ?= $(VERSION)
QUAY_EXPIRATION?=never

IMG ?= $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
export IMG
IMAGE ?= $(IMG)
export IMAGE

all: generate vet fmt

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
	$(DOCKERCMD) build \
	-f ./Dockerfile \
	--tag $(IMAGE) \
  --build-arg ARCH=$(ARCH) \
	--build-arg REGISTRY=$(IMAGE_REGISTRY) \
	--build-arg path=./authchecker/v2/ \
	--build-arg name=\"AuthCheck\" \
	--build-arg exec=./cmd/ \
	--build-arg bin=redhat-marketplace-authchecker \
	--build-arg app_version=\"$(VERSION)\" \
	--build-arg quay_expiration=\"$(QUAY_EXPIRATION)\" \
	$(ARGS) \
	../..

# Push the docker image
docker-push:
	docker push ${IMG}

ARCHS ?= amd64 ppc64le s390x

docker-manifest:
	docker manifest create $(IMAGE) $(addprefix $(IMAGE)-,$(ARCHS))
	docker manifest push $(IMAGE)

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate
generate:
	go generate ./...

# Run the test
.PHONY: test
test: ginkgo
	- $(GINKGO) -r -coverprofile=cover-unit.out.tmp -outputdir=. --randomizeAllSpecs --randomizeSuites --cover --race --progress --trace ./...

ginkgo:
ifeq (, $(shell which ginkgo))
	@{ \
	set -e ;\
	GINKGO_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$GINKGO_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get -u github.com/onsi/ginkgo/ginkgo ;\
	rm -rf $$GINKGO_GEN_TMP_DIR ;\
	}
GINKGO=$(GOBIN)/ginkgo
else
GINKGO=$(shell which ginkgo)
endif
