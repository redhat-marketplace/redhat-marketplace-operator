QUAY_EXPIRATION?=never
IMAGE_PUSH ?= true
BUILDX ?= true
ARCH ?= amd64

include ../utils.Makefile

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium

# Push the docker image
docker-push:
	docker push $(19IMG)

docker-build: data-service

DSIMGARGS = --build-arg GO_VERSION=$(GO_VERSION)
DSIMG = $(IMAGE_REGISTRY)/data-service-base:1.19

data-service:
	$(call docker-build,dataservice.Dockerfile,$(DSIMG),$(19ARGS))
