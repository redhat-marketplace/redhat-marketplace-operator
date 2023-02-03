include ../../utils.Makefile

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium
IMAGE_NAME = redhat-marketplace-authcheck
IMAGE_TAG ?= $(TAG)
QUAY_EXPIRATION?=never

IMG ?= $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
export IMG
IMAGE ?= $(IMG)
export IMAGE

all: svu generate vet fmt

build:
	$(call multiarch-build,redhat-marketplace-authchecker,./cmd/)

# Build the docker image
docker-build:
	$(call docker-templated-build,$(IMAGE),Authcheck,./authchecker/v2/,./cmd/,redhat-marketplace-authchecker,../..)

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

# Generate
generate:
	go generate ./...

# Run the test
.PHONY: test
test: ginkgo
	- $(GINKGO) -r -coverprofile=cover-unit.out --randomize-all --randomize-suites --cover --race --show-node-events --v --trace ./...
