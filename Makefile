PROJECTS = operator authchecker metering reporter

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export

.DEFAULT_GOAL := all

all: fmt vet generate build

skaffold-dev:
	make operator/skaffold-dev

skaffold-run:
	make operator/skaffold-run

build: vet fmt
	make operator/skaffold-build

vet: $(addsuffix /vet,$(PROJECTS))

fmt: $(addsuffix /fmt,$(PROJECTS))

test: $(addsuffix /test,$(PROJECTS))

generate: $(addsuffix /generate,$(PROJECTS))

operator/%:
	cd ./v2 && $(MAKE) $(@F)

reporter/%:
	cd ./reporter/v2 && $(MAKE) $(@F)

metering/%:
	cd ./metering/v2 && $(MAKE) $(@F)

authchecker/%:
	cd ./authchecker/v2 && $(MAKE) $(@F)

.PHONY: check-licenses
check-licenses: install-tools ## Check if all files have licenses
	 find . -type f -name "*.go" | xargs $(LICENSE) -check -v

add-licenses:
	 find . -type f -name "*.go" | xargs $(LICENSE)


install-tools:
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

%:
	$(MAKE) operator/$@
