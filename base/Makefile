QUAY_EXPIRATION?=never
IMAGE_PUSH ?= true
BUILDX ?= true
ARCH ?= amd64

include ../utils.Makefile

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium

# Push the docker image
docker-push:
	docker push $(IMG)

docker-build: base data-service

16ARGS = --build-arg GO_VERSION=$(GO_VERSION)
16IMG = $(IMAGE_REGISTRY)/golang-base:1.16
base:
	$(call docker-build,base.Dockerfile,$(16IMG),$(16ARGS))

DSIMGARGS = --build-arg GO_VERSION=$(GO_VERSION)
DSIMG = $(IMAGE_REGISTRY)/data-service-base:1.16

data-service:
	$(call docker-build,dataservice.Dockerfile,$(DSIMG),$(16ARGS))
