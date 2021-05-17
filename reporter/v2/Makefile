VERSION ?= $(shell cd ../../v2/tools/version && go run main.go)
export VERSION

include ../../utils.Makefile

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium
IMAGE_NAME = redhat-marketplace-reporter
IMAGE_TAG ?= $(VERSION)
QUAY_EXPIRATION?=never

IMG ?= $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
export IMG
IMAGE ?= $(IMG)
export IMAGE

all: generate vet fmt

build:
	$(call multiarch-build,redhat-marketplace-reporter,./cmd/reporter)

# Build the docker image
docker-build:
	$(call docker-templated-build,$(IMAGE),Reporter,./reporter/v2/,./cmd/reporter,redhat-marketplace-reporter,../..)

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
	$(GINKGO) -r -coverprofile=cover-unit.out.tmp -outputdir=. --randomizeAllSpecs --randomizeSuites --cover --race --progress --trace ./...
