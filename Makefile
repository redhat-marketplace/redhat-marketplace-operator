PROJECTS = operator authchecker metering reporter airgap watcher
PROJECT_FOLDERS = . authchecker metering reporter airgap watcher

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export

include utils.Makefile

.DEFAULT_GOAL := all

.PHONY: all
all: svu fmt vet generate docker-build

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

TIDY_TARGETS=authchecker/v2 cue.mod metering/v2 reporter/v2 tests/v2 v2 v2/scripts v2/tools/connect v2/tools/skaffold-tdd-tool v2/tools/version watcher/v2

.PHONY: tidy-all
tidy-all:
	current_dir=`pwd` ; \
	for project in $(TIDY_TARGETS) ; do \
		echo "go mod $$curent_dir/$$project" && cd $$current_dir/$$project && go mod tidy ; \
	done
	cd ./airgap/v2/ && $(BUF) mod update

.PHONY: download-all
download-all:
	$(shell cd v2/tools/version && go mod download)
	$(shell cd v2/tools/connect && go mod download)
	$(MAKE) $(addsuffix /download,$(PROJECTS))

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
	@cd ./airgap/v2 && rm -rf ./vendor && go mod tidy && go mod vendor && wicked-cli -p redhat-marketplace-airgap -s ./vendor -o ../../.wicked-report
	@cd ./watcher/v2 && rm -rf ./vendor && go mod tidy && go mod vendor && wicked-cli -p redhat-marketplace-watcher -s ./vendor -o ../../.wicked-report

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

airgap/%:
	@cd ./airgap/v2 && $(MAKE) $(@F)

watcher/%:
	@cd ./watcher/v2 && $(MAKE) $(@F)

tests/%:
	@cd ./tests/v2 && $(MAKE) $(@F)

base/%:
	cd ./base && $(MAKE) $(@F)
