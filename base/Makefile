QUAY_EXPIRATION?=never
IMAGE_PUSH ?= true
BUILDX ?= true
ARCH ?= amd64

include ../utils.Makefile

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium
REGISTRY ?= $(IMAGE_REGISTRY)
DSIMG = $(IMAGE_REGISTRY)/data-service-base:ubi8
DSUBIIMG = $(IMAGE_REGISTRY)/data-service-ubi-minimal:ubi8
ARGS = --build-arg REGISTRY=$(IMAGE_REGISTRY)

# Build the docker images
docker-build: data-service data-service-ubi-minimal

# Push the docker images
docker-push:
	docker push $(DSIMG)
	docker push $(DSUBIIMG)

# go-toolset dqlite-lib-builder
data-service:
	$(call docker-build,dataservice.Dockerfile,$(DSIMG),$(ARGS))

# ubi-minimal plus built prereqs
data-service-ubi-minimal:
	$(call docker-build,data-service-ubi-minimal.Dockerfile,$(DSUBIIMG),$(ARGS))
