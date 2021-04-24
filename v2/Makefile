# Current Operator version
VERSION ?= $(shell cd ./tools/version && go run ./main.go)
UNAME_S := $(shell uname -s)
UNAME := $(shell echo `uname` | tr '[:upper:]' '[:lower:]')

export VERSION

include ../hack/utils.Makefile

# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium

TAG ?= $(VERSION)

OPERATOR_IMAGE_NAME = redhat-marketplace-operator
OPERATOR_IMAGE_TAG ?= $(TAG)
OPERATOR_IMAGE ?= $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_IMAGE_TAG)

REPORTER_IMAGE_NAME ?= redhat-marketplace-reporter
REPORTER_IMAGE_TAG ?= $(TAG)
REPORTER_IMAGE := $(IMAGE_REGISTRY)/$(REPORTER_IMAGE_NAME):$(REPORTER_IMAGE_TAG)

METRIC_STATE_IMAGE_NAME ?= redhat-marketplace-metric-state
METRIC_STATE_IMAGE_TAG ?= $(TAG)
METRIC_STATE_IMAGE := $(IMAGE_REGISTRY)/$(METRIC_STATE_IMAGE_NAME):$(METRIC_STATE_IMAGE_TAG)

AUTHCHECK_IMAGE_NAME ?= redhat-marketplace-authcheck
AUTHCHECK_IMAGE_TAG ?= $(TAG)
AUTHCHECK_IMAGE := $(IMAGE_REGISTRY)/$(AUTHCHECK_IMAGE_NAME):$(AUTHCHECK_IMAGE_TAG)

BUNDLE_NAME ?= redhat-marketplace-operator-manifest
BUNDLE_IMG ?= $(IMAGE_REGISTRY)/$(BUNDLE_NAME):$(VERSION)

DEV_INDEX_NAME ?= redhat-marketplace-operator-dev-index
DEV_INDEX_IMG ?= $(IMAGE_REGISTRY)/$(DEV_INDEX_NAME)

namespace ?= openshift-redhat-marketplace

# Image URL to use all building/pushing image targets
IMG ?= $(OPERATOR_IMAGE)
IMAGE ?= $(OPERATOR_IMAGE)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
# CRD_OPTIONS ?= "crd:trivialVersions=true"
CRD_OPTIONS ?= "crd:crdVersions={v1beta1},trivialVersions=false,preserveUnknownFields=false"

GOPATH=$(shell go env GOPATH)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.DEFAULT_GOAL := all

all: manager

clean:
	rm -f bundle/manifests/*.yaml bundle/metadata/*.yaml bundle/tests/*.yaml

# Run tests
.PHONY: test
test: fmt vet manifests ginkgo
	$(GINKGO) -r -coverprofile=cover-unit.out.tmp -outputdir=. --randomizeAllSpecs --randomizeSuites --cover --race --progress --trace ./pkg ./cmd ./internal ./apis ./controllers

.PHONY: test-ci-unit
test-ci-unit: ginkgo ## test-ci-unit runs all tests for CI builds
	$(GINKGO) -r -coverprofile=cover-unit.out.tmp -outputdir=. --randomizeAllSpecs --randomizeSuites --cover --race --progress --trace ./pkg ./cmd ./internal ./apis ./controllers
	cat cover-unit.out.tmp | grep -v "_generated.go|zz_generated|testbin.go|wire_gen.go" > cover-unit.out

KUTTL_INT_TESTS ?= "(^register-test$$|^features-test$$)"

.PHONY: test-ci-int
test-ci-int:  ## test-ci-int runs all tests for CI builds
	SKIP_DEPLOY=true kubectl kuttl test --namespace openshift-redhat-marketplace --kind-context test --config ./kuttl-test.yaml ../tests/v2/e2e --test $(KUTTL_INT_TESTS) --suppress-log "events"

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize helm install
	cd config && $(HELM) template rhm-template ./helm-wrapper \
	-f ./helm-wrapper/values.yaml \
	--set operatorImage=$(OPERATOR_IMAGE) \
	--set metricStateImage=$(METRIC_STATE_IMAGE) \
	--set reporterImage=$(REPORTER_IMAGE) \
	--set authCheckImage=$(AUTHCHECK_IMAGE) | ./default/kustomize | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) object:headerFile="hack/boilerplate.go.txt" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen manifests go-bindata
	go generate ./...

# Generate API
generate-api: code-generator
	@{ \
	set -e ;\
	API_TEMP_DIR=$$(mktemp -d) ;\
	$(CODEGEN_PKG)/generate-groups.sh \
		"defaulter,client,lister" \
    github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated \
    github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis \
    "marketplace:v1alpha1,v1beta1" \
    --output-base $$API_TEMP_DIR \
    --go-header-file hack/boilerplate.go.txt ; \
	cp -r "$$API_TEMP_DIR/github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/generated" "apis/marketplace" ;\
	}

QUAY_EXPIRATION?=never
IMAGE_PUSH ?= true
BUILDX ?= true
ARCH ?= amd64

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

build:
	$(call multiarch-build,redhat-marketplace-operator)

# Build the docker image
docker-build:
	$(call docker-templated-build,$(IMAGE),Operator,./v2/,./,redhat-marketplace-operator,..)

# Push the docker image
docker-push:
	docker push ${IMG}

ARCHS ?= amd64 ppc64le s390x

docker-manifest:
	docker manifest create $(IMAGE) $(addprefix $(IMAGE)-,$(ARCHS))
	docker manifest push $(IMAGE)

CSV_YAML_BUNDLE_FILE=bundle/manifests/redhat-marketplace-operator.clusterserviceversion.yaml
CREATED_TIME ?= $(shell date +"%FT%H:%M:%SZ")

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: clean manifests kustomize helm operator-sdk yq
	$(OPERATOR_SDK) generate kustomize manifests -q
	$(HELM) template rhm-template ./config/helm-wrapper -f ./config/helm-wrapper/values.yaml \
	--set operatorImage=$(OPERATOR_IMAGE) \
	--set metricStateImage=$(METRIC_STATE_IMAGE) \
	--set reporterImage=$(REPORTER_IMAGE) \
	--set authCheckImage=$(AUTHCHECK_IMAGE) \
	--post-renderer ./config/manifests/kustomize | $(OPERATOR_SDK) generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	$(YQ) eval -i '.spec.webhookdefinitions[].targetPort = 9443' $(CSV_YAML_BUNDLE_FILE)
	$(YQ) eval -i '.spec.webhookdefinitions[].containerPort = 9443' $(CSV_YAML_BUNDLE_FILE)
	$(YQ) eval -i ".spec.metadata.annotations.containerImage = \"$(OPERATOR_IMAGE)\"" $(CSV_YAML_BUNDLE_FILE)
	$(YQ) eval -i ".spec.metadata.annotations.createdAt = \"$(CREATED_TIME)\"" $(CSV_YAML_BUNDLE_FILE)
	$(OPERATOR_SDK) bundle validate ./bundle
	#Generate a copy of the resultant yaml files to the deploy directory - this is only used for content certification linting
	rm -rf ./deploy && mkdir -p ./deploy ./deploy/crds && \
	cp -p ./config/samples/*v1alpha1*.yaml ./config/samples/*v1beta1*.yaml ./deploy/crds
	#generate all yaml for the deploy directory to be used in the bundle
	$(HELM) template rhm-template ./config/helm-wrapper -f ./config/helm-wrapper/values.yaml \
	--set operatorImage=$(OPERATOR_IMAGE) \
	--set metricStateImage=$(METRIC_STATE_IMAGE) \
	--set reporterImage=$(REPORTER_IMAGE) \
	--set authCheckImage=$(AUTHCHECK_IMAGE) \
	--post-renderer ./config/default/kustomize > ./deploy/manifestsNativeDeploy.yaml

BUILD_ARCHS = $(subst $(space),$(comma),$(addprefix linux/,$(ARCHS)))

# Build the bundle image for all architectures.
.PHONY: bundle-build-multiarch
bundle-build-multiarch:
	docker buildx build --push -f bundle.Dockerfile -t $(BUNDLE_IMG) \
	--platform $(call build-targets,$(ARCHS),linux) \
	--label com.redhat.openshift.versions="v4.5,v4.6,v4.7" \
  --label com.redhat.delivery.operator.bundle=true \
	--label com.redhat.delivery.backport=true \
	.

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) \
	--label com.redhat.openshift.versions="v4.5,v4.6,v4.7" \
  --label com.redhat.delivery.operator.bundle=true \
	--label com.redhat.delivery.backport=true \
	.

bundle-deploy:
	docker push $(BUNDLE_IMG)

bundle-run: operator-sdk
	$(OPERATOR_SDK) run bundle $(BUNDLE_IMG) --namespace openshift-redhat-marketplace

bundle-run-install:
	$(MAKE) skaffold-build bundle bundle-build bundle-deploy bundle-run

bundle-run-uninstall:
	for NAME in marketplaceconfigs.marketplace.redhat.com subscriptions.operators.coreos.com csv operatorgroup catalogsources.operators.coreos.com ; do \
		kubectl delete $$NAME --all -n openshift-redhat-marketplace ; \
	done


BUNDLE_IMAGE_REGISTRY ?= $(IMAGE_REGISTRY)

bundle-stable:
	$(MAKE) bundle CHANNELS=stable DEFAULT_CHANNEL=stable IMAGE_REGISTRY=$(BUNDLE_IMAGE_REGISTRY)

VERSIONS_LIST ?= $(BUNDLE_IMG)

.PHONY: bundle-build-index-multiarch
bundle-dev-index-multiarch:
	$(OPM) index add -u docker --generate --bundles "$(VERSIONS_LIST)" -d dev-index.Dockerfile
	docker buildx build --push -f dev-index.Dockerfile -t "$(DEV_INDEX_IMG):$(VERSION)" \
	--platform $(subst $(space),$(comma),$(addprefix linux/,$(ARCHS))) \
	--label com.redhat.openshift.versions="v4.5,v4.6,v4.7" \
  --label com.redhat.delivery.operator.bundle=true \
	--label com.redhat.delivery.backport=true \
	.

bundle-dev-index: opm
	$(OPM) index add -u docker --bundles "$(VERSIONS_LIST)" --tag "$(DEV_INDEX_IMG):$(VERSION)" --permissive
	docker push $(DEV_INDEX_IMG):$(VERSION)

bundle-uninstall:
	- kubectl delete catalogsources.operators.coreos.com redhat-marketplace-operator-catalog
	#- kubectl delete marketplaceconfigs.marketplace.redhat.com --all
	- kubectl delete subscription -l operators.coreos.com/redhat-marketplace-operator.openshift-redhat-marketplace
	- kubectl delete csv -l operators.coreos.com/redhat-marketplace-operator.openshift-redhat-marketplace

skaffold-build:
	cd .. && $(MAKE) save-licenses
	skaffold build --default-repo $(IMAGE_REGISTRY) --tag $(TAG)

INSTALL_DEBUG ?= false

compile-helm: helm
	cd config && $(HELM) template rhm-operator ./helm-wrapper \
	   --namespace $(namespace) \
	   -f ./helm-wrapper/values.yaml \
	   --set operatorImage=$(OPERATOR_IMAGE) \
	   --set metricStateImage=$(METRIC_STATE_IMAGE) \
	   --set reporterImage=$(REPORTER_IMAGE) \
	   --set authCheckImage=$(AUTHCHECK_IMAGE) \
     --post-renderer ./helm-wrapper/kustomize

install: kustomize compile-helm
	cd config && $(HELM) template rhm-operator ./helm-wrapper \
	   --namespace $(namespace) \
	   -f ./helm-wrapper/values.yaml \
	   --set operatorImage=$(OPERATOR_IMAGE) \
	   --set metricStateImage=$(METRIC_STATE_IMAGE) \
	   --set reporterImage=$(REPORTER_IMAGE) \
	   --set authCheckImage=$(AUTHCHECK_IMAGE) \
     --post-renderer ./helm-wrapper/kustomize --debug=$(INSTALL_DEBUG) | kubectl apply -f -

uninstall: helm yq
	@cd config && $(HELM) template rhm-operator ./helm-wrapper \
	   --namespace $(namespace) \
	   -f ./helm-wrapper/values.yaml \
	   --set operatorImage=$(OPERATOR_IMAGE) \
	   --set metricStateImage=$(METRIC_STATE_IMAGE) \
	   --set reporterImage=$(REPORTER_IMAGE) \
	   --set authCheckImage=$(AUTHCHECK_IMAGE) \
     --post-renderer ./helm-wrapper/kustomize | $(YQ) eval-all 'select(.kind != "Namespace") | select(.kind != "CustomResourceDefinition")' -  | kubectl delete -f -

skaffold-dev: install
	skaffold dev --default-repo $(IMAGE_REGISTRY) --cleanup=false

skaffold-run:
	skaffold run --default-repo $(IMAGE_REGISTRY) --tag $(TAG)

current-version:
	@echo $(VERSION)

TAG ?= ""
CREDS ?= ""
TIMEOUT ?= 15
PIDs ?=

wait-and-publish:
	cd tools/connect ; \
	go run main.go wait-and-publish --timeout $(TIMEOUT) --tag $(TAG) $(PIDS)
