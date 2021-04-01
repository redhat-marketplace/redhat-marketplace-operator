SRC_DIR=$(shell pwd)
DST_DIR=$(SRC_DIR)
UNAME := $(shell echo `uname` | tr '[:upper:]' '[:lower:]')

ARCH ?= amd64

build:
	docker buildx build --load -t airgap:test .

demo: build
	docker-compose up

generate: protoc
	@{ TMP_DIR=$$(mktemp -d) ;\
		PATH=$(PATH):$(PROTOC_GEN_GO) $(PROTOC) -I=$(SRC_DIR) --go_out=$$TMP_DIR --go-grpc_out=$$TMP_DIR $(SRC_DIR)/apis/model/v1/modelv1.proto $(SRC_DIR)/apis/fileserver/filesender/filesender.proto $(SRC_DIR)/apis/fileretreiver/fileretreiver.proto $(SRC_DIR)/apis/adminserver/adminserver.proto ;\
		cp -r $$TMP_DIR/github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis ./ ;\
		rm -rf $$TMP_DIR ; \
	}

PROTOC_GEN_GO=$(shell pwd)/bin/protoc-gen-go
protoc-gen-go:
	$(call go-get-tool,$(PROTOC),google.golang.org/protobuf/cmd/protoc-gen-go)

ifeq ($(UNAME),darwin)
PROTOC_UNAME=osx
else
PROTOC_UNAME=$(UNAME)
endif

ifeq ($(ARCH),amd64)
PROTOC_ARCH=x86_64
else
PROTOC_ARCH=$(ARCH)
endif

PROTOC=$(shell pwd)/bin/protoc
protoc: protoc-gen-go
	$(call install-zip,https://github.com/protocolbuffers/protobuf/releases/download/v3.15.6,protoc-3.15.6-$(PROTOC_UNAME)-$(PROTOC_ARCH).zip,$(PROTOC),$(shell pwd)/bin,./bin/protoc ./include)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
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

define install-zip
@[ -f $(3) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
echo "Downloading $(1)/$(2)" ;\
curl --fail -LO $(1)/$(2) ;\
unzip $(2) $(foreach a,$(5),&& mv $(a) $(4)) ;\
rm -rf $$TMP_DIR ;\
}
endef
