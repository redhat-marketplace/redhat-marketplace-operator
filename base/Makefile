GO_VERSION ?= 1.15
QUAY_EXPIRATION?=never
IMAGE_PUSH ?= true
BUILDX ?= true
ARCH ?= amd64
BASE_TAG ?= $(GO_VERSION)-$(ARCH)

ifeq ($(BUILDX),true)
DOCKERCMD=docker buildx
ifeq ($(IMAGE_PUSH),true)
ARGS=--push
else
ARGS=--load
endif
else
DOCKERCMD=docker
endif

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium
IMG ?= $(IMAGE_REGISTRY)/golang-base:$(BASE_TAG)

docker-build:
	$(DOCKERCMD) build -t "$(IMG)" \
	--build-arg ARCH=$(ARCH) \
	-f base.Dockerfile $(ARGS) .

# Push the docker image
docker-push:
	docker push $(IMG)
