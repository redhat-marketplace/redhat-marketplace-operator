VERSION ?= 1.0.5
export VERSION

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium
IMAGE_NAME = redhat-marketplace-metric-state
IMAGE_TAG ?= $(VERSION)
QUAY_EXPIRATION?=never

IMG ?= $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
export IMG
IMAGE ?= $(IMG)
export IMAGE

all: generate vet fmt

IMAGE_PUSH ?= true

ifeq ($(IMAGE_PUSH),true)
ARGS=--push
else
ARGS=--load
endif

# Build the docker image
docker-build:
	docker buildx build \
	-f ./Dockerfile \
	--tag $(IMAGE) \
	--build-arg path=./metering/v2/ \
	--build-arg name=MetricState \
	--build-arg exec=./cmd/ \
	--build-arg bin=redhat-marketplace-metric-state \
	--build-arg app_version=\"$(VERSION)\" \
	--build-arg quay_expiration=\"$(QUAY_EXPIRATION)\" \
	$(ARGS) \
	../..

# Push the docker image
docker-push:
	docker push ${IMG}

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
test:
	go test ./...
