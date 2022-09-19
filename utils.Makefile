comma := ,
space := $(subst ,, )

UNAME_S := $(shell uname -s)
UNAME := $(shell echo `uname` | tr '[:upper:]' '[:lower:]')

VERSION ?= $(shell $(SVU) next --prefix "")
TAG ?= $(VERSION)
export VERSION
export TAG

BINDIR ?= ./bin
# GO_VERSION can be major version only, latest stable minor version will be retrieved by base.Dockerfile
GO_VERSION ?= 1.19
ARCHS ?= amd64 ppc64le s390x arm64
BUILDX ?= true
ARCH ?= amd64
IMAGE_PUSH ?= true
DOCKER_BUILD := docker build
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
DOCKERBUILDXCACHE ?=

clean-bin:
	rm -rf $(PROJECT_DIR)/bin

# --TOOLS--
#
# find or download controller-gen
# download controller-gen if necessary
CONTROLLER_GEN_VERSION=v0.7.0
CONTROLLER_GEN=$(PROJECT_DIR)/bin/controller-gen
controller-gen:
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION),$(CONTROLLER_GEN_VERSION))

CODEGEN_VERSION=kubernetes-1.22.3

CODEGEN_PKG=$(GOPATH)/src/k8s.io/code-generator
code-generator:
	@[ -d $(CODEGEN_PKG) ] && { \
		cd $(CODEGEN_PKG) && \
			if [ "$$(git describe --tags)" != "$(CODEGEN_VERSION)" ]; \
				then git fetch --tags && git checkout tags/$(CODEGEN_VERSION) ; \
			fi;
	}
	@[ -d $(CODEGEN_PKG) ] || { \
		git clone -b tags/$(CODEGEN_VERSION) git@github.com:kubernetes/code-generator $(GOPATH)/k8s.io/code-generator ;\
	}

KUSTOMIZE_VERSION=v4.4.0
KUSTOMIZE=$(PROJECT_DIR)/bin/kustomize
kustomize:
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@$(KUSTOMIZE_VERSION),$(KUSTOMIZE_VERSION))

OMT_VERSION=v0.1.6
OMT=$(PROJECT_DIR)/bin/operator-manifest-tools
omt:
	$(call go-get-tool,$(OMT),github.com/operator-framework/operator-manifest-tools@$(OMT_VERSION),$(OMT_VERSION))

export KUSTOMIZE

OPENAPI_GEN=$(PROJECT_DIR)/bin/openapi-gen
openapi-gen:
	$(call go-get-tool,$(OPENAPI_GEN),github.com/kubernetes/kube-openapi/cmd/openapi-gen@690f563a49b523b7e87ea117b6bf448aead23b09,690f563)

HELM_VERSION=v3.8.2
HELM=$(PROJECT_DIR)/bin/helm
helm:
	@[ -f $(HELM)-$(HELM_VERSION) ] || { \
		HELM_INSTALL_DIR=$$(dirname $(HELM)) $(PROJECT_DIR)/hack/get_helm.sh --version $(HELM_VERSION) && touch $(HELM)-$(HELM_VERSION) ;\
	}

GINKGO_VERSION=v1.16.5
GINKGO=$(PROJECT_DIR)/bin/ginkgo
ginkgo:
	$(call go-get-tool,$(GINKGO),github.com/onsi/ginkgo/ginkgo@$(GINKGO_VERSION),$(GINKGO_VERSION))

LICENSE=$(PROJECT_DIR)/bin/addlicense
addlicense:
	$(call go-install-tool,$(LICENSE),github.com/google/addlicense@v1.0.0,v1.0.0)

GO_LICENSES=$(PROJECT_DIR)/bin/go-licenses
golicense:
	$(call go-get-tool,$(GO_LICENSES),github.com/google/go-licenses@v1.0.0,v1.0.0)

YQ_VERSION=v4.8.0

YQ=$(PROJECT_DIR)/bin/yq
yq:
	$(call go-get-tool,$(YQ),github.com/mikefarah/yq/v4@$(YQ_VERSION),$(YQ_VERSION))

OPERATOR_SDK_VERSION=v1.14.0

OPERATOR_SDK=$(PROJECT_DIR)/bin/operator-sdk
operator-sdk:
	$(call install-binary,https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION),operator-sdk_$(UNAME)_$(ARCH),$(OPERATOR_SDK),$(OPERATOR_SDK_VERSION))

OPM_VERSION=v1.19.1

OPM=$(PROJECT_DIR)/bin/opm
opm:
	$(call install-binary,https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION),$(UNAME)-$(ARCH)-opm,$(OPM),$(OPM_VERSION))

SVU_VERSION=v1.8.0

.SILENT: svu
SVU=$(PROJECT_DIR)/bin/svu
svu:
	$(call go-get-tool,$(SVU),github.com/caarlos0/svu@$(SVU_VERSION),$(SVU_VERSION))

VERSION_TOOL=$(PROJECT_DIR)/bin/version-tool
version-tool:
	@[ -f $(VERSION_TOOL) ] || { \
	cd v2/tools/version && go build -o $(VERSION_TOOL) ./main.go ; \
	}

PC_TOOL=$(PROJECT_DIR)/bin/partner-connect-tool
pc-tool:
	@[ -f $(VERSION_TOOL) ] || { \
	cd v2/tools/connect && go build -o $(PC_TOOL) ./main.go ; \
	}

SKAFFOLD=$(PROJECT_DIR)/bin/skaffold
SKAFFOLD_VERSION=v1.38.0
skaffold:
	$(call install-binary,https://storage.googleapis.com/skaffold/releases/$(SKAFFOLD_VERSION),skaffold-$(UNAME)-amd64,$(SKAFFOLD),$(SKAFFOLD_VERSION))

BUF=$(PROJECT_DIR)/bin/buf
BUF_VERSION=v1.0.0-rc8
ifeq ($(UNAME_S),Linux)
WILDCARDS=--wildcards
endif
buf:
	$(call install-targz,https://github.com/bufbuild/buf/releases/download/$(BUF_VERSION)/buf-$(shell uname -s)-$(shell uname -m).tar.gz,$(BUF),$(BUF_VERSION),$(PROJECT_DIR)/bin,--strip-components 2 $(WILDCARDS) "*/bin/*")

ENVTEST=$(PROJECT_DIR)/bin/setup-envtest
envtest:
	$(shell GOBIN=$(PROJECT_DIR)/bin go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

.PHONY: source-envtest
source-envtest:
	@echo export KUBEBUILDER_ASSETS="'$(shell $(ENVTEST) use -p path 1.19.x)'"

# --COMMON--

# Run go mod tidy against code
tidy:
	go mod tidy

# Run go mod tidy against code
download:
	go mod download

# func(name)
define multiarch-build
{ \
	mkdir -p $(BINDIR) && \
	for arch in $(ARCHS); do \
	  GOOS=linux GOARCH=$${arch} go build -o $(BINDIR)/$(1)-linux-$${arch} $(2) ; \
  done \
}
endef

# func(ARCHS, OS) TARGETS
define build-targets
$(subst $(space),$(comma),$(addprefix $(2)/,$(1)))
endef

ifeq ($(BUILDX),true)
DOCKER_BUILD := docker buildx build --platform $(call build-targets,$(ARCHS),linux)
endif

ifneq ($(DOCKERBUILDXCACHE),)
DOCKER_EXTRA_ARGS = --cache-from "type=local,src=$(DOCKERBUILDXCACHE)" --cache-to "type=local,dest=$(DOCKERBUILDXCACHE)" --output "type=image,push=$(IMAGE_PUSH)"
else
DOCKER_EXTRA_ARGS =
ifneq ($(PODMAN),true)
ifeq ($(IMAGE_PUSH),true)
DOCKER_BUILD += --push
else
DOCKER_BUILD += --load
endif
endif
endif

# func(image,args)
define docker-build
$(DOCKER_BUILD) \
-t "$(2)" \
$(DOCKER_EXTRA_ARGS) \
-f $(1) $(3) .
endef

# func(image,name,path,exec,bin,args)
define docker-templated-build
$(DOCKER_BUILD) \
-f ./Dockerfile \
--tag $(1) \
$(DOCKER_EXTRA_ARGS) \
--build-arg ARCHS='$(ARCHS)' \
--build-arg REGISTRY=$(IMAGE_REGISTRY) \
--build-arg GO_VERSION=$(GO_VERSION) \
--build-arg name=$(2) \
--build-arg path=$(3) \
--build-arg exec=$(4) \
--build-arg bin=$(5) \
--build-arg app_version=\"$(VERSION)\" \
--build-arg quay_expiration=\"$(QUAY_EXPIRATION)\" \
$(6)
endef

# go-get-tool will 'go get' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1)-$(3) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo $(1) ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
mkdir -p $$(dirname $(1)) ;\
touch $(1)-$(3) ;\
}
endef

# go-install-tool will 'go install' any package $2 and install it to $1.
define go-install-tool
@[ -f $(1)-$(3) ] || { \
set -e ;\
GOBIN=$$(dirname $(1)) go install $(2) ;\
touch $(1)-$(3) ;\
}
endef

# install-binary will 'curl' any package url $1 with file $2 and install it to $3
define install-binary
@[ -f $(3)-$(4) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
echo "Downloading $(1)" ;\
curl -o $(3) -LO $(1)/$(2) ;\
chmod +x $(3) ;\
mkdir -p $$(dirname $(1)) ;\
rm -rf $$TMP_DIR ;\
touch $(3)-$(4) ;\
}
endef

# $1 url $2 bin path $3 version $4 is the bin path to extract to $5 is optional extract args
define install-targz
[ -f $(2)-$(3) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
echo "Downloading $(1)"; \
mkdir -p $$(dirname $(1)) ;\
curl -sSL $(1) | tar -xvzf - -C "$(4)" $(5) ;\
rm -rf $$TMP_DIR ;\
touch $(2)-$(3); \
}
endef


# $1 is the image
define get-image-sha
$$(IMAGE=$(1); echo $${IMAGE%:*}@sha256:$$(skopeo inspect --raw docker://$(1) | sha256sum | cut -d " " -f 1))
endef
