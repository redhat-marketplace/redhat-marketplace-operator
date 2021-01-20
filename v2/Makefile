# Current Operator version
VERSION ?= $(shell go run ./tools/version/main.go)
UNAME_S := $(shell uname -s)

export VERSION

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

.PHONY: test-ci-int
test-ci-int:  ## test-ci-int runs all tests for CI builds
	kubectl kuttl test --namespace openshift-redhat-marketplace --kind-context test --config ./kuttl-test.yaml ../tests/v2/e2e --test "(^register-test$$|^features-test$$)" --suppress-log "events"

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
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen manifests go-bindata
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
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

# Build the docker image
docker-build:
	$(DOCKERCMD) build \
	-f ./Dockerfile \
	--tag $(IMAGE) \
	--build-arg path=./v2/ \
	--build-arg name=Operator \
	--build-arg exec=./ \
	--build-arg bin=redhat-marketplace-operator \
	--build-arg app_version=\"$(VERSION)\" \
	--build-arg quay_expiration=\"$(QUAY_EXPIRATION)\" \
	$(ARGS) \
	..

# Push the docker image
docker-push:
	docker push ${IMG}

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: clean manifests kustomize helm
	operator-sdk generate kustomize manifests -q
	$(HELM) template rhm-template ./config/helm-wrapper -f ./config/helm-wrapper/values.yaml \
	--set operatorImage=$(OPERATOR_IMAGE) \
	--set metricStateImage=$(METRIC_STATE_IMAGE) \
	--set reporterImage=$(REPORTER_IMAGE) \
	--set authCheckImage=$(AUTHCHECK_IMAGE) \
	--post-renderer ./config/manifests/kustomize | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

bundle-deploy:
	docker push $(BUNDLE_IMG)
	operator-sdk run bundle $(BUNDLE_IMG)

bundle-stable:
	$(MAKE) bundle CHANNELS=stable DEFAULT_CHANNEL=stable

bundle-uninstall:
	- kubectl delete catalogsources.operators.coreos.com redhat-marketplace-operator-catalog
	#- kubectl delete marketplaceconfigs.marketplace.redhat.com --all
	- kubectl delete subscription -l operators.coreos.com/redhat-marketplace-operator.openshift-redhat-marketplace
	- kubectl delete csv -l operators.coreos.com/redhat-marketplace-operator.openshift-redhat-marketplace

skaffold-build:
	skaffold build --default-repo $(IMAGE_REGISTRY) --tag $(TAG)

INSTALL_DEBUG ?= false

helm:
	cd config && $(HELM) template rhm-operator ./helm-wrapper \
	   --namespace $(namespace) \
	   -f ./helm-wrapper/values.yaml \
	   --set operatorImage=$(OPERATOR_IMAGE) \
	   --set metricStateImage=$(METRIC_STATE_IMAGE) \
	   --set reporterImage=$(REPORTER_IMAGE) \
	   --set authCheckImage=$(AUTHCHECK_IMAGE) \
     --post-renderer ./helm-wrapper/kustomize

install:
	cd config && $(HELM) template rhm-operator ./helm-wrapper \
	   --namespace $(namespace) \
	   -f ./helm-wrapper/values.yaml \
	   --set operatorImage=$(OPERATOR_IMAGE) \
	   --set metricStateImage=$(METRIC_STATE_IMAGE) \
	   --set reporterImage=$(REPORTER_IMAGE) \
	   --set authCheckImage=$(AUTHCHECK_IMAGE) \
     --post-renderer ./helm-wrapper/kustomize --debug=$(INSTALL_DEBUG) | kubectl apply -f -

uninstall:
	cd config && $(HELM) template rhm-operator ./helm-wrapper \
	   --namespace $(namespace) \
	   -f ./helm-wrapper/values.yaml \
	   --set operatorImage=$(OPERATOR_IMAGE) \
	   --set metricStateImage=$(METRIC_STATE_IMAGE) \
	   --set reporterImage=$(REPORTER_IMAGE) \
	   --set authCheckImage=$(AUTHCHECK_IMAGE) \
     --post-renderer ./helm-wrapper/kustomize | kubectl delete -f -

skaffold-dev: install
	skaffold dev --default-repo $(IMAGE_REGISTRY) --cleanup=false

skaffold-run:
	skaffold run --default-repo $(IMAGE_REGISTRY) --tag $(TAG)

# --TOOLS--
#
# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

code-generator:
ifeq (, $(shell ls $(GOPATH)/src/k8s.io/code-generator))
	@{ \
	set -e ;\
	K8S_CODEGEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$K8S_CODEGEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/k8s.io/code-generator@v0.19.4 ;\
	rm -rf $$K8S_CODEGEN_TMP_DIR ;\
	}
CODEGEN_PKG=$(GOPATH)/src/k8s.io/code-generator
else
CODEGEN_PKG=$(GOPATH)/src/k8s.io/code-generator
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif
export KUSTOMIZE

openapi-gen:
ifeq (, $(shell which openapi-gen))
	@{ \
	set -e ;\
	OPENAPI_GEN_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$OPENAPI_GEN_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get github.com/kubernetes/kube-openapi/cmd/openapi-gen@690f563a49b523b7e87ea117b6bf448aead23b09 ;\
	rm -rf $$OPENAPI_GEN_GEN_TMP_DIR ;\
	}
OPENAPI_GEN=$(GOBIN)/openapi-gen
else
OPENAPI_GEN=$(shell which openapi-gen)
endif

helm:
ifeq (, $(shell which helm))
ifeq ($(UNAME_S),Linux)
	curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash
endif
ifeq ($(UNAME_S),Darwin)
	brew install skaffold
	brew install helm
endif
endif
HELM=$(shell which helm)

skaffold:
ifeq (, $(shell which skaffold))
ifeq ($(UNAME_S),Linux)
	curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-darwin-amd64 && \
	sudo install skaffold /usr/local/bin/
endif
ifeq ($(UNAME_S),Darwin)
	brew install skaffold
endif
SKAFFOLD=$(shell which skaffold)
else
SKAFFOLD=$(shell which skaffold)
endif

ginkgo:
ifeq (, $(shell which ginkgo))
	@{ \
	set -e ;\
	GINKGO_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$GINKGO_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get -u github.com/onsi/ginkgo/ginkgo ;\
	rm -rf $$GINKGO_GEN_TMP_DIR ;\
	}
GINKGO=$(GOBIN)/ginkgo
else
GINKGO=$(shell which ginkgo)
endif


go-bindata:
ifeq (, $(shell which go-bindata))
	@{ \
	set -e ;\
	GOBINDATA_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$GOBINDATA_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get -u github.com/kevinburke/go-bindata/... ;\
	rm -rf $$GO-BINDATA_GEN_TMP_DIR ;\
	}
GOBINDATA=$(GOBIN)/go-bindata
else
GOBINDATA=$(shell which go-bindata)
endif

addlicense:
ifeq (, $(shell which addlicense))
	@{ \
	set -e ;\
	GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get -u github.com/google/addlicense ;\
	rm -rf $$GEN_TMP_DIR ;\
	}
LICENSE=$(GOBIN)/addlicense
else
LICENSE=$(GOBIN)/addlicense
endif
