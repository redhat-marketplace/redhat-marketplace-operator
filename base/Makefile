GO_VERSION ?= 1.16
QUAY_EXPIRATION?=never
IMAGE_PUSH ?= true
BUILDX ?= true
ARCH ?= amd64
BASE_TAG ?= $(GO_VERSION)

include ../utils.Makefile

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium

# Push the docker image
docker-push:
	docker push $(IMG)

docker-build: docker-build-1-16

16ARGS = --build-arg VERSION=1.16.6
16IMG = $(IMAGE_REGISTRY)/golang-base:1.16
docker-build-1-16:
	$(call docker-build,$(16IMG),$(16ARGS))

ARGS = --build-arg VERSION=1.15.11
IMG = $(IMAGE_REGISTRY)/golang-base:1.15
docker-build-1-15:
	$(call docker-build,$(IMG),$(ARGS))
