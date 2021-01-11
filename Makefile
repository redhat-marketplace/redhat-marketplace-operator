PROJECTS = operator authchecker metering reporter

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
	$(MAKE) $(addsuffix /docker-build,$(PROJECTS))

.PHONY: check-licenses
check-licenses: addlicense ## Check if all files have licenses
	 find . -type f -name "*.go" | xargs $(LICENSE) -check -c "IBM Corp." -v

add-licenses: addlicense
	 find . -type f -name "*.go" | xargs $(LICENSE) -c "IBM Corp."

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

operator/%:
	cd ./v2 && $(MAKE) $(@F)

reporter/%:
	cd ./reporter/v2 && $(MAKE) $(@F)

metering/%:
	cd ./metering/v2 && $(MAKE) $(@F)

authchecker/%:
	cd ./authchecker/v2 && $(MAKE) $(@F)
