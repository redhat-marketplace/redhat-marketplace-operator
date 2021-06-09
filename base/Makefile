GO_VERSION ?= 1.16
QUAY_EXPIRATION?=never
IMAGE_PUSH ?= true
BUILDX ?= true
ARCH ?= amd64
BASE_TAG ?= $(GO_VERSION)

include ../utils.Makefile

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium
IMG ?= $(IMAGE_REGISTRY)/golang-base:$(BASE_TAG)

# Push the docker image
docker-push:
	docker push $(IMG)

IMG ?= $(IMAGE_REGISTRY)/golang-base:$(GO_VERSION)

docker-build:
	$(call docker-build,$(IMG),$(ARGS))
