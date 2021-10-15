QUAY_EXPIRATION?=never
IMAGE_PUSH ?= true
BUILDX ?= true
ARCH ?= amd64

include ../utils.Makefile

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium

# Push the docker image
docker-push:
	docker push $(IMG)

docker-build: docker-build-1-16

16ARGS = --build-arg GO_VERSION=$(GO_VERSION)
16IMG = $(IMAGE_REGISTRY)/golang-base:1.16
docker-build-1-16:
	$(call docker-build,$(16IMG),$(16ARGS))
