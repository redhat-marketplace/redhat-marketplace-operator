PROJECTS = operator authchecker metering reporter
PROJECTS_ALL = $(PROJECTS) tests
PROJECT_FOLDERS = . authchecker metering reporter

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export

include utils.Makefile

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
	$(MAKE) $(addsuffix /vet,$(PROJECTS_ALL))

.PHONY: fmt
fmt:
	$(MAKE) $(addsuffix /fmt,$(PROJECTS_ALL))

.PHONY: tidy-all
tidy-all:
	$(MAKE) $(addsuffix /tidy,$(PROJECTS_ALL))

.PHONY: download-all
download-all:
	$(MAKE) $(addsuffix /download,$(PROJECTS_ALL))

.PHONY: test
test:
	$(MAKE) $(addsuffix /test,$(PROJECTS) tests)

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
		[ ! -d "_licenses" ] && sh -c "cd $$folder && $(GO_LICENSES) save --save_path _licenses --force ./... && chmod -R +w _licenses" ; \
	done

cicd:
	go generate ./gen.go
	cd .github/workflows && go generate ./gen.go

export GO_LICENSES

clean-vendor:
	rm -rf $(addsuffix /v2/vendor,$(PROJECT_FOLDERS))

clean-licenses:
	-chmod -R +w $(addsuffix /v2/_licenses,$(PROJECT_FOLDERS))
	-rm -rf $(addsuffix /v2/_licenses,$(PROJECT_FOLDERS))
	-mkdir -p $(addsuffix /v2/_licenses,$(PROJECT_FOLDERS))
	touch $(addsuffix /v2/_licenses/.gitkeep,$(PROJECT_FOLDERS))

wicked:
	mkdir -p .wicked-report
	@cd ./v2 && rm -rf ./vendor && go mod tidy && go mod vendor && wicked-cli -p redhat-marketplace-operator -s ./vendor -o ../.wicked-report
	@cd ./reporter/v2 && rm -rf ./vendor && go mod tidy && go mod vendor && wicked-cli -p redhat-marketplace-reporter -s ./vendor -o ../../.wicked-report
	@cd ./metering/v2 && rm -rf ./vendor && go mod tidy && go mod vendor && wicked-cli -p redhat-marketplace-metering -s ./vendor -o ../../.wicked-report
	@cd ./authchecker/v2 && rm -rf ./vendor && go mod tidy && go mod vendor && wicked-cli -p redhat-marketplace-authchecker -s ./vendor -o ../../.wicked-report

# -- Release

create-next-release: svu
	git checkout develop
	git pull
	git checkout -b release/$(shell $(SVU) next)

create-next-hotfix: svu
	git checkout master
	git pull
	git checkout -b release/$(shell $(SVU) next)

# --

operator/%:
	@cd ./v2 && $(MAKE) $(@F)

reporter/%:
	@cd ./reporter/v2 && $(MAKE) $(@F)

metering/%:
	@cd ./metering/v2 && $(MAKE) $(@F)

authchecker/%:
	@cd ./authchecker/v2 && $(MAKE) $(@F)

tests/%:
	@cd ./tests/v2 && $(MAKE) $(@F)



base/%:
	cd ./base && $(MAKE) $(@F)
