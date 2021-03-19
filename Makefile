PROJECTS = operator authchecker metering reporter
PROJECT_FOLDERS = . authchecker metering reporter

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif


export

.DEFAULT_GOAL := all

.PHONY: all
all: fmt vet generate build

skaffold-dev:
	make operator/skaffold-dev

skaffold-run:
	make operator/skaffold-run

skaffold-build: vet fmt
	make operator/skaffold-build

.PHONY: build
build:
	$(MAKE) docker-build

.PHONY: vet
vet:
	$(MAKE) $(addsuffix /vet,$(PROJECTS))

.PHONY: fmt
fmt:
	$(MAKE) $(addsuffix /fmt,$(PROJECTS))

.PHONY: test
test:
	$(MAKE) $(addsuffix /test,$(PROJECTS))

generate:
	$(MAKE) $(addsuffix /generate,$(PROJECTS))

docker-build:
	$(MAKE) base/docker-build
	$(MAKE) $(addsuffix /docker-build,$(PROJECTS))

docker-push:
	$(MAKE) base/docker-push
	$(MAKE) $(addsuffix /docker-push,$(PROJECTS))

docker-manifest:
	$(MAKE) $(addsuffix /docker-manifest,$(PROJECTS))

.PHONY: check-licenses
check-licenses: addlicense ## Check if all files have licenses
	 find . -type f -name "*.go" | xargs $(LICENSE) -check -c "IBM Corp." -v

add-licenses: addlicense
	 find . -type f -name "*.go" | xargs $(LICENSE) -c "IBM Corp."

save-licenses: golicense
	for folder in $(addsuffix /v2,$(PROJECT_FOLDERS)) ; do \
		[ ! -d "licenses" ] && sh -c "cd $$folder && $(GO_LICENSES) save --save_path licenses --force ./... && chmod -R +w licenses" ; \
	done

cicd:
	go generate .
	cd .github/workflows && go generate .

LICENSE=$(shell pwd)/v2/bin/addlicense
addlicense:
	$(call go-get-tool,$(LICENSE),github.com/google/addlicense)

GO_LICENSES=$(shell pwd)/v2/bin/go-licenses
golicense:
	$(call go-get-tool,$(GO_LICENSES),github.com/google/go-licenses)

export GO_LICENSES

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))/v2
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

clean-vendor:
	rm -rf $(addsuffix /v2/vendor,$(PROJECT_FOLDERS))

clean-licenses:
	-chmod -R +w $(addsuffix /v2/licenses,$(PROJECT_FOLDERS))
	-rm -rf $(addsuffix /v2/licenses,$(PROJECT_FOLDERS))
	-mkdir -p $(addsuffix /v2/licenses,$(PROJECT_FOLDERS))
	touch $(addsuffix /v2/licenses/.gitkeep,$(PROJECT_FOLDERS))

wicked:
	mkdir -p .wicked-report
	@cd ./v2 && rm -rf ./vendor && go mod tidy && go mod vendor && wicked-cli -p redhat-marketplace-operator -s ./vendor -o ../.wicked-report
	@cd ./reporter/v2 && rm -rf ./vendor && go mod tidy && go mod vendor && wicked-cli -p redhat-marketplace-reporter -s ./vendor -o ../../.wicked-report
	@cd ./metering/v2 && rm -rf ./vendor && go mod tidy && go mod vendor && wicked-cli -p redhat-marketplace-metering -s ./vendor -o ../../.wicked-report
	@cd ./authchecker/v2 && rm -rf ./vendor && go mod tidy && go mod vendor && wicked-cli -p redhat-marketplace-authchecker -s ./vendor -o ../../.wicked-report

operator/%:
	@cd ./v2 && $(MAKE) $(@F)

reporter/%:
	@cd ./reporter/v2 && $(MAKE) $(@F)

metering/%:
	@cd ./metering/v2 && $(MAKE) $(@F)

authchecker/%:
	@cd ./authchecker/v2 && $(MAKE) $(@F)

base/%:
	cd ./base && $(MAKE) $(@F)
