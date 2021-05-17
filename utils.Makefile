comma := ,
space :=
space +=

BINDIR ?= ./bin
ARCHS ?= amd64 ppc64le s390x
BUILDX ?= true
ARCH ?= amd64
IMAGE_PUSH ?= true
DOCKER_BUILD := docker build
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))

# --TOOLS--
#
# find or download controller-gen
# download controller-gen if necessary
CONTROLLER_GEN=$(PROJECT_DIR)/bin/controller-gen
controller-gen:
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0)

CODEGEN_PKG=$(GOPATH)/src/k8s.io/code-generator
code-generator:
	@[ -d $(CODEGEN_PKG) ] || { \
	GO111MODULE=off go get k8s.io/code-generator ;\
	}

KUSTOMIZE=$(PROJECT_DIR)/bin/kustomize
kustomize:
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.1.0)

export KUSTOMIZE

OPENAPI_GEN=$(PROJECT_DIR)/bin/openapi-gen
openapi-gen:
	$(call go-get-tool,$(OPENAPI_GEN),github.com/kubernetes/kube-openapi/cmd/openapi-gen@690f563a49b523b7e87ea117b6bf448aead23b09)

helm:
ifeq (, $(shell which helm))
ifeq ($(UNAME_S),Linux)
	curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash
endif
ifeq ($(UNAME_S),Darwin)
	brew install helm
endif
endif
HELM=$(shell which helm)

skaffold:
ifeq (, $(shell which skaffold))
ifeq ($(UNAME_S),Linux)
	curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64 && \
	sudo install skaffold /usr/local/bin/
endif
ifeq ($(UNAME_S),Darwin)
	brew install skaffold
endif
SKAFFOLD=$(shell which skaffold)
else
SKAFFOLD=$(shell which skaffold)
endif

GINKGO=$(PROJECT_DIR)/bin/ginkgo
ginkgo:
	$(call go-get-tool,$(GINKGO),github.com/onsi/ginkgo/ginkgo)

GOBINDATA=$(PROJECT_DIR)/bin/go-bindata
go-bindata:
	$(call go-get-tool,$(GOBINDATA),github.com/kevinburke/go-bindata/...)

LICENSE=$(PROJECT_DIR)/bin/addlicense
addlicense:
	$(call go-get-tool,$(LICENSE),github.com/google/addlicense)

GO_LICENSES=$(PROJECT_DIR)/bin/go-licenses
golicense:
	$(call go-get-tool,$(GO_LICENSES),github.com/google/go-licenses)

YQ=$(PROJECT_DIR)/bin/yq
yq:
	$(call go-get-tool,$(YQ),github.com/mikefarah/yq/v4@v4.6.1)

OPERATOR_SDK=$(PROJECT_DIR)/bin/operator-sdk
operator-sdk:
	$(call install-binary,https://github.com/operator-framework/operator-sdk/releases/download/v1.3.2,operator-sdk_$(UNAME)_$(ARCH),$(OPERATOR_SDK))

OPM=$(PROJECT_DIR)/bin/opm
opm:
	$(call install-binary,https://github.com/operator-framework/operator-registry/releases/download/v1.13.7,$(UNAME)-$(ARCH)-opm,$(OPM))

.SILENT: svu
SVU=$(PROJECT_DIR)/bin/svu
svu:
	$(call go-get-tool,$(SVU),github.com/caarlos0/svu@v1.3.2)

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

# --COMMON--

# Run go mod tidy against code
tidy:
	go mod tidy

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
ifeq ($(IMAGE_PUSH),true)
DOCKER_BUILD += --push
else
DOCKER_BUILD += --load
endif
endif


# func(image,args)
define docker-build
$(DOCKER_BUILD) \
-t "$(1)" \
-f base.Dockerfile $(2) .
endef

# func(image,name,path,exec,bin,args)
define docker-templated-build
$(DOCKER_BUILD) \
-f ./Dockerfile \
--tag $(1) \
--build-arg ARCHS='$(ARCHS)' \
--build-arg REGISTRY=$(IMAGE_REGISTRY) \
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
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

# install-binary will 'curl' any package url $1 with file $2 and install it to $3
define install-binary
@[ -f $(3) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
echo "Downloading $(1)" ;\
curl -LO $(1)/$(2) ;\
chmod +x $(2) && mv $(2) $(3) ;\
rm -rf $$TMP_DIR ;\
}
endef
