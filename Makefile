SHELL:=/bin/bash
NAMESPACE ?= openshift-redhat-marketplace
OPSRC_NAMESPACE = marketplace-operator
OPERATOR_SOURCE = redhat-marketplace-operators
IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium
OPERATOR_IMAGE_NAME ?= redhat-marketplace-operator
VERSION ?= $(shell go run scripts/version/main.go)
FROM_VERSION ?= $(shell go run scripts/version/main.go last)
OPERATOR_IMAGE_TAG ?= $(VERSION)
CREATED_TIME ?= $(shell date +"%FT%H:%M:%SZ")
DEVPOSTFIX ?= ""
QUAY_EXPIRATION ?= never

# set these variables to the tag or SHA for the ubi image used in the Dockerfile.
# use '$(docker) manifest inspect registry.access.redhat.com/ubi8/ubi-minimal:<tag>' to get the SHA values
UBI_IMAGE_SHA_PPC=2507309a69f786388f4aad70cfd27a4582f3bd2df19a4608166845a1ba4d6f36
UBI_IMAGE_SHA_390=8782de8892bd10bbfa0220442fc71d45e660f0e8a811000f0f5d729ebffff645

SERVICE_ACCOUNT := redhat-marketplace-operator
SECRETS_NAME := my-docker-secrets

OPERATOR_IMAGE ?= $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_IMAGE_TAG)

REPORTER_IMAGE_NAME ?= redhat-marketplace-reporter
REPORTER_IMAGE_TAG ?= $(OPERATOR_IMAGE_TAG)
REPORTER_IMAGE := $(IMAGE_REGISTRY)/$(REPORTER_IMAGE_NAME):$(REPORTER_IMAGE_TAG)

METRIC_STATE_IMAGE_NAME ?= redhat-marketplace-metric-state
METRIC_STATE_IMAGE_TAG ?= $(OPERATOR_IMAGE_TAG)
METRIC_STATE_IMAGE := $(IMAGE_REGISTRY)/$(METRIC_STATE_IMAGE_NAME):$(METRIC_STATE_IMAGE_TAG)

AUTHCHECK_IMAGE_NAME ?= redhat-marketplace-authcheck
AUTHCHECK_IMAGE_TAG ?= $(OPERATOR_IMAGE_TAG)
AUTHCHECK_IMAGE := $(IMAGE_REGISTRY)/$(AUTHCHECK_IMAGE_NAME):$(AUTHCHECK_IMAGE_TAG)

PUSH_IMAGE ?= false
PULL_POLICY ?= IfNotPresent
.DEFAULT_GOAL := help

# cluster server to test the rhm operator
CLUSTER_SERVER ?= https://api.crc.testing:6443
# The namespace where the operator watches for changes. Set "" for AllNamespaces, set "ns1,ns2" for MultiNamespace
OPERATOR_WATCH_NAMESPACE ?= ""

## Tool paths
pwd = $(shell pwd)
docker ?= $(shell command -v docker)
jq ?= $(shell command -v jq)
operator-sdk = $(pwd)/testbin/operator-sdk
skaffold = $(pwd)/testbin/skaffold
cfssl = $(pwd)/testbin/cfssl
cfssljson = $(pwd)/testbin/cfssljson
cfssl-certinfo = $(pwd)/testbin/cfssl-certinfo
opm =  $(pwd)/testbin/opm
kind = $(shell go env GOPATH)/bin/kind
gocovmerge = $(shell go env GOPATH)/bin/gocovmerge
yq = $(shell go env GOPATH)/bin/yq

##@ Application

install: ## Install all resources (CR/CRD's, RBAC and Operator)
	@echo ....... Creating namespace .......
	- kubectl create namespace ${NAMESPACE}
	make helm
	make create
	make deploys
	make apply

uninstall: ## Uninstall all that all performed in the $ make install
	@echo ....... Uninstalling .......
	@make delete

##@ Build

.PHONY: clean
clean: ## Clean up generated files
	- rm ./deploy/role.yaml ./deploy/operator.yaml ./deploy/role_binding.yaml ./deploy/service_account.yaml

.PHONY: clean-tools
clean-tools: ## Clean up generated files that are emphemeral
	- rm -rf testbin


.PHONY: build-base
build-base: $(skaffold)
	$(skaffold) build --tag="1.15" -p base --default-repo quay.io/rh-marketplace

.PHONY: build
build: ## Build all operators
	@VERSION=$(VERSION) $(skaffold) build --tag $(OPERATOR_IMAGE_TAG) --default-repo $(IMAGE_REGISTRY) --namespace $(NAMESPACE)

.PHONY: build-json
build-json: ## Build all operators with json output
	@VERSION=$(VERSION) $(skaffold) build --tag $(OPERATOR_IMAGE_TAG) --default-repo $(IMAGE_REGISTRY) --namespace $(NAMESPACE) --output="{{ json . }}" --quiet

KIND_NAME?=test

.PHONY: build-and-load-kind
build-and-load-kind: ## Build all operators with json output
	- make build-json | $(jq) -r '.builds[].tag' | xargs -n 1 -t kind load docker-image --name=$(KIND_NAME)

BUILD_IMAGE ?= redhat-marketplace-operator

.PHONY: build
build-image: $(skaffold) ## Build the operator executable
	VERSION=$(VERSION) $(skaffold) build --tag $(OPERATOR_IMAGE_TAG) --default-repo $(IMAGE_REGISTRY) --namespace $(NAMESPACE) --cache-artifacts=false -b $(BUILD_IMAGE)

# build architecture-specific images
.PHONY: build-amd
build-amd: $(skaffold) ## Build the operator executable for amd64
	@echo Building amd64 image
	VERSION=$(VERSION) $(skaffold) build --profile amd --tag $(OPERATOR_IMAGE_TAG) --default-repo $(IMAGE_REGISTRY) --namespace $(NAMESPACE) --cache-artifacts=false

.PHONY: build-ppc
build-ppc: $(skaffold) ## Build the operator executable for ppc64le
	@echo Building ppc64le image
	@$(docker) run --rm --privileged multiarch/qemu-user-static:register --reset
	VERSION=$(VERSION) ARCH=ppc64le UBI_IMAGE_SHA=$(UBI_IMAGE_SHA_PPC) $(skaffold) build --profile ppc --tag $(OPERATOR_IMAGE_TAG) --default-repo $(IMAGE_REGISTRY) --namespace $(NAMESPACE) --cache-artifacts=false

.PHONY: build-390
build-390: $(skaffold) ## Build the operator executable for s390x
	@echo Building s390x image
	@$(docker) run --rm --privileged multiarch/qemu-user-static:register --reset
	VERSION=$(VERSION) ARCH=s390x UBI_IMAGE_SHA=$(UBI_IMAGE_SHA_390) $(skaffold) build --profile s390 --tag $(OPERATOR_IMAGE_TAG) --default-repo $(IMAGE_REGISTRY) --namespace $(NAMESPACE) --cache-artifacts=false

# build multiarch images
.PHONY: build-multiarch
build-multiarch:
	@echo Building multiarch images
	@curl -L -o /tmp/manifest-tool https://github.com/estesp/manifest-tool/releases/download/v1.0.0/manifest-tool-linux-amd64
	@chmod +x /tmp/manifest-tool
	/tmp/manifest-tool push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME)-ARCH:$(OPERATOR_IMAGE_TAG) --target $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME) --ignore-missing
	/tmp/manifest-tool push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME)-ARCH:$(OPERATOR_IMAGE_TAG) --target $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_IMAGE_TAG) --ignore-missing
	/tmp/manifest-tool push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(IMAGE_REGISTRY)/$(REPORTER_IMAGE_NAME)-ARCH:$(REPORTER_IMAGE_TAG) --target $(IMAGE_REGISTRY)/$(REPORTER_IMAGE_NAME) --ignore-missing
	/tmp/manifest-tool push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(IMAGE_REGISTRY)/$(REPORTER_IMAGE_NAME)-ARCH:$(REPORTER_IMAGE_TAG) --target $(IMAGE_REGISTRY)/$(REPORTER_IMAGE_NAME):$(REPORTER_IMAGE_TAG) --ignore-missing
	/tmp/manifest-tool push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(IMAGE_REGISTRY)/$(METRIC_STATE_IMAGE_NAME)-ARCH:$(METRIC_STATE_IMAGE_TAG) --target $(IMAGE_REGISTRY)/$(METRIC_STATE_IMAGE_NAME) --ignore-missing
	/tmp/manifest-tool push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(IMAGE_REGISTRY)/$(METRIC_STATE_IMAGE_NAME)-ARCH:$(METRIC_STATE_IMAGE_TAG) --target $(IMAGE_REGISTRY)/$(METRIC_STATE_IMAGE_NAME):$(METRIC_STATE_IMAGE_TAG) --ignore-missing
	/tmp/manifest-tool push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(IMAGE_REGISTRY)/$(AUTHCHECK_IMAGE_NAME)-ARCH:$(AUTHCHECK_IMAGE_TAG) --target $(IMAGE_REGISTRY)/$(AUTHCHECK_IMAGE_NAME) --ignore-missing
	/tmp/manifest-tool push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(IMAGE_REGISTRY)/$(AUTHCHECK_IMAGE_NAME)-ARCH:$(AUTHCHECK_IMAGE_TAG) --target $(IMAGE_REGISTRY)/$(AUTHCHECK_IMAGE_NAME):$(AUTHCHECK_IMAGE_TAG) --ignore-missing

helm: ## build helm base charts
	. ./scripts/package_helm.sh $(VERSION) deploy ./deploy/chart/values.yaml --set image=$(OPERATOR_IMAGE),metricStateImage=$(METRIC_STATE_IMAGE),reporterImage=$(REPORTER_IMAGE),authCheckImage=$(AUTHCHECK_IMAGE) --set namespace=$(NAMESPACE)

MANIFEST_CSV_FILE := ./deploy/olm-catalog/redhat-marketplace-operator/manifests/redhat-marketplace-operator.clusterserviceversion.yaml
VERSION_CSV_FILE := ./deploy/olm-catalog/redhat-marketplace-operator/$(VERSION)/redhat-marketplace-operator.v$(VERSION).clusterserviceversion.yaml
CSV_CHANNEL ?= beta # change to stable for release
CSV_DEFAULT_CHANNEL ?= false # change to true for release
CHANNELS ?= beta

generate-bundle: ## Generate the csv
	make helm
	$(operator-sdk) bundle create --generate-only \
		--package redhat-marketplace-operator \
		--default-channel=$(CSV_DEFAULT_CHANNEl) \
		--channels $(CHANNELS)
	$(yq) w -i $(MANIFEST_CSV_FILE) 'metadata.annotations.containerImage' $(OPERATOR_IMAGE)
	$(yq) w -i $(MANIFEST_CSV_FILE) 'metadata.annotations.createdAt' $(CREATED_TIME)
	$(yq) d -i $(MANIFEST_CSV_FILE) 'spec.install.spec.deployments[*].spec.template.spec.containers[*].env(name==WATCH_NAMESPACE).valueFrom'
	$(yq) w -i $(MANIFEST_CSV_FILE) 'spec.install.spec.deployments[*].spec.template.spec.containers[*].env(name==WATCH_NAMESPACE).value' ''

INTERNAL_CRDS='["razeedeployments.marketplace.redhat.com","remoteresources3s.marketplace.redhat.com"]'

generate-csv: ## Generate the csv
	make helm
	$(operator-sdk) generate csv \
		--from-version=$(FROM_VERSION) \
		--csv-version=$(VERSION) \
		--csv-channel=$(CSV_CHANNEL) \
		--default-channel=$(CSV_DEFAULT_CHANNEL) \
		--operator-name=redhat-marketplace-operator \
		--update-crds \
		--make-manifests=true
	$(yq) w -i $(MANIFEST_CSV_FILE) 'metadata.annotations.containerImage' $(OPERATOR_IMAGE)
	$(yq) w -i $(MANIFEST_CSV_FILE) 'metadata.annotations.createdAt' $(CREATED_TIME)
	$(yq) w -i $(MANIFEST_CSV_FILE) 'metadata.annotations.capabilities' "Full Lifecycle"
	$(yq) w -i $(MANIFEST_CSV_FILE) 'metadata.annotations."operators.operatorframework.io/internal-objects"' $(INTERNAL_CRDS)
	$(yq) d -i $(MANIFEST_CSV_FILE) 'spec.install.spec.deployments[*].spec.template.spec.containers[*].env(name==WATCH_NAMESPACE).valueFrom'
	$(yq) w -i $(MANIFEST_CSV_FILE) 'spec.install.spec.deployments[*].spec.template.spec.containers[*].env(name==WATCH_NAMESPACE).value' ''

PACKAGE_FILE ?= ./deploy/olm-catalog/redhat-marketplace-operator/redhat-marketplace-operator.package.yaml

manifest-package-beta: # Make sure we have the right versions
	$(yq) w -i $(PACKAGE_FILE) 'channels.(name==stable).currentCSV' redhat-marketplace-operator.v$(FROM_VERSION)
	$(yq) w -i $(PACKAGE_FILE) 'channels.(name==beta).currentCSV' redhat-marketplace-operator.v$(VERSION)

manifest-package-stable: # Make sure we have the right versions
	$(yq) w -i $(PACKAGE_FILE) 'channels.(name==stable).currentCSV' redhat-marketplace-operator.v$(VERSION)
	$(yq) w -i $(PACKAGE_FILE) 'channels.(name==beta).currentCSV' redhat-marketplace-operator.v$(VERSION)

REGISTRY ?= quay.io

docker-login: ## Log into $(docker) using env $DOCKER_USER and $DOCKER_PASSWORD
	@$(docker) login -u="$(DOCKER_USER)" -p="$(DOCKER_PASSWORD)" $(REGISTRY)

##@ Development

ARGS ?=

skaffold-dev: $(skaffold) ## Run skaffold dev. Will unique tag the operator and rebuild.
	make create
	DEVPOSTFIX=$(DEVPOSTFIX) docker=$(DOCKER_EXEC) $(skaffold) dev --tail --port-forward --default-repo $(IMAGE_REGISTRY) --namespace $(NAMESPACE) $(ARGS)

skaffold-run: $(skaffold) ## Run skaffold run. Will uniquely tag the operator.
	make helm
	make create
	. ./scripts/package_helm.sh $(VERSION) deploy ./deploy/chart/values.yaml --set image=redhat-marketplace-operator --set pullPolicy=IfNotPresent
	docker=$(DOCKER_EXEC) $(skaffold) run --tail --default-repo $(IMAGE_REGISTRY) --cleanup=false $(ARGS)

code-vet: ## Run go vet for this project. More info: https://golang.org/cmd/vet/
	@echo go vet
	go vet $$(go list ./... )

code-fmt: ## Run go fmt for this project
	@echo go fmt
	go fmt $$(go list ./... )

code-dev: ## Run the default dev commands which are the go fmt and vet then execute the $ make code-gen
	@echo Running the common required commands for developments purposes
	- make code-gen
	- make code-fmt
	- make code-vet

.PHONY: k8s-gen
k8s-gen:
	. ./scripts/update-codegen.sh

code-gen: ## Run the operator-sdk commands to generated code (k8s and crds)
ifndef GOROOT
	$(error GOROOT is undefined)
endif
	@echo Generating k8s
	$(operator-sdk) generate k8s
	@echo Updating the CRD files with the OpenAPI validations
	$(operator-sdk) generate crds --crd-version=v1beta1
	@echo Go generating
	- go generate ./...

setup-minikube: ## Setup minikube for full operator dev
	@echo Installing operatorframework
	kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.15.1/crds.yaml
	kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.15.1/olm.yaml
	@echo Applying prometheus operator
	kubectl apply -f https://raw.githubusercontent.com/coreos/prometheus-operator/master/bundle.yaml
	@echo Apply kube-state
	for item in cluster-role.yaml service-account.yaml cluster-role-binding.yaml deployment.yaml service.yaml ; do \
		kubectl apply -f https://raw.githubusercontent.com/kubernetes/kube-state-metrics/master/examples/standard/$$item ; \
	done

setup-operator-sdk: ## Create ns, crds, sa, role, and rolebinding before operator-sdk run
	- make helm
	- make create
	- make deploy-services
	- . ./scripts/operator_sdk_sa_kubeconfig.sh $(CLUSTER_SERVER) $(NAMESPACE) $(SERVICE_ACCOUNT)

run-operator-sdk: ## Run operator locally outside the cluster during development cycle
	$(operator-sdk) run local --watch-namespace="" --kubeconfig=./sa.kubeconfig

##@ Manual Testing

create: ##creates the required crds for this deployment
	@echo creating crds
	- kubectl create namespace ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_marketplaceconfigs_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_razeedeployments_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_meterbases_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_meterdefinitions_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_meterreports_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_remoteresources3s_crd.yaml -n ${NAMESPACE}

deploys: ##deploys the resources for deployment
	@echo deploying services and operators
	- make deploy-services
	- kubectl create -f deploy/operator.yaml --namespace=${NAMESPACE}

deploy-services: ##deploys the service acconts, roles, and role bindings
	- kubectl create -f deploy/service_account.yaml --namespace=${NAMESPACE}
	- kubectl create -f deploy/role.yaml --namespace=${NAMESPACE}
	- kubectl create -f deploy/role_binding.yaml --namespace=${NAMESPACE}

migrate: ##used to simulate migrating to the latest version of the operator
	- make helm
	- kubectl apply -f deploy/operator.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/role_binding.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/role.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/service_account.yaml -n ${NAMESPACE}
	- make create
	- kubectl apply -f deploy/operator.yaml --namespace=${NAMESPACE}

apply: ##applies changes to crds
	- kubectl apply -f deploy/crds/marketplace.redhat.com_v1alpha1_marketplaceconfig_cr.yaml --namespace=${NAMESPACE}

delete-resources: ## delete-resources
	- kubectl delete -n ${NAMESPACE} razeedeployments.marketplace.redhat.com --all

delete: ##delete the contents created in 'make create'
	@echo deleting resources
	- kubectl delete opsrc ${OPERATOR_SOURCE} -n openshift-marketplace
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_marketplaceconfig_cr.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_razeedeployment_cr.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_meterdefinition_cr.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_meterbase_cr.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/operator.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/role_binding.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/role.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/service_account.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_marketplaceconfigs_crd.yaml
	- kubectl patch Razeedeployment rhm-marketplaceconfig-razeedeployment -p '{"metadata":{"finalizers":[]}}' --type=merge
	- kubectl delete -f deploy/crds/marketplace.redhat.com_razeedeployments_crd.yaml
	- kubectl delete -f deploy/crds/marketplace.redhat.com_meterbases_crd.yaml
	- kubectl delete -f deploy/crds/marketplace.redhat.com_meterdefinitions_crd.yaml
	- kubectl patch remoteresources3s.marketplace.redhat.com parent -p '{"metadata":{"finalizers":[]}}' --type=merge
	- kubectl patch remoteresources3s.marketplace.redhat.com child -p '{"metadata":{"finalizers":[]}}' --type=merge
	- kubectl patch customresourcedefinition.apiextensions.k8s.io remoteresources3s.marketplace.redhat.com -p '{"metadata":{"finalizers":[]}}' --type=merge
	- kubectl delete -f deploy/crds/marketplace.redhat.com_remoteresources3s_crd.yaml -n ${NAMESPACE}
	- kubectl delete namespace ${NAMESPACE}

delete-razee: ##delete the razee CR
	@echo deleting razee CR
	- kubectl delete -f  deploy/crds/marketplace.redhat.com_v1alpha1_razeedeployment_cr.yaml -n ${NAMESPACE}

create-razee: ##create the razee CR
	@echo creating razee CR
	- kubectl create -f  deploy/crds/marketplace.redhat.com_v1alpha1_razeedeployment_cr.yaml -n ${NAMESPACE}

##@ Tests

.PHONY: lint
lint: ## lint the repo
	golangci-lint run

.PHONY: test
test: testbin ## test-ci runs all tests for CI builds
	@echo "testing"
	make test-ci-unit

KIND_CLUSTER_NAME ?= test
KIND_CONTROL_PLANE_NODE ?= $(KIND_CLUSTER_NAME)-control-plane

setup-kind: ## setup the kind cluster for integration test; requires .docker/config.json to house the passwords for auth to registry.redhat.io
	@[[ "$(cat ~/.docker/config.json | jq -r '.credStore')" != "" ]] && echo "remove credStore from .docker/config.json and relog into docker login registry.redhat.io" && exit 1 || echo "looking good"
	@- $(kind) create cluster --name $(KIND_CLUSTER_NAME) --config ./kind-cluster.yaml
	@- $(kind) export kubeconfig --name  $(KIND_CLUSTER_NAME)
	@- make kind-certs
	@- operator-sdk olm install
	@- echo "openshift-monitoring openshift-config-managed openshift-config openshift-redhat-marketplace openshift-marketplace" | xargs -n 1 kubectl create ns
	@- find | grep test/testdata | grep monitoring | xargs -n 1 kubectl apply -f
	@- helm repo add bitnami https://charts.bitnami.com/bitnami --force-update
	@- helm install kube-state-metrics bitnami/kube-state-metrics -n openshift-monitoring --set namespace=openshift-monitoring,serviceMonitor.enabled=true,serviceMonitor.namespace=openshift-monitoring,fullnameOverride=kube-state-metrics
	@- kubectl apply -f ./test/testdata/kind-metrics-server.yaml
	@- kubectl create secret generic regcred  --namespace $$NAMESPACE --from-file=.dockerconfigjson=$$HOME/.docker/config.json --type=kubernetes.io/dockerconfigjson
	@- ./test/mockcontrollers/deploy.sh

kind-certs:
	@for file in ca.crt ca.key apiserver.crt; do \
    $(docker) cp $(KIND_CONTROL_PLANE_NODE):/etc/kubernetes/pki/$$file ./test/certs/$$file ; \
    done
	@cd test/certs && $(cfssl) certinfo -cert apiserver.crt | $(jq) -r ".sans" > sans.json
	@cd test/certs	&& $(jq) -n --argfile o1 server-csr.json --argfile o2 sans.json '$$o1 | .hosts = $$o2 | .hosts[.hosts | length] |= . + "*.openshift-redhat-marketplace.svc"' > marketplace-csr.json
	@cd test/certs && $(cfssl) gencert -ca=ca.crt -ca-key=ca.key -profile=kubernetes marketplace-csr.json | $(cfssljson) -bare server

 test-ci-int-kind: ## test integration using kind
	PULL_SECRET_NAME=regcred IS_KIND=true kubectl kuttl test --namespace openshift-redhat-marketplace --kind-context test --config ./kuttl-test-kind.yaml ./test/e2e --test ^register-test$

.PHONY: test-cover
test-cover: ## Run coverage on code
	@echo Running coverage
	make test-ci

CONTROLLERS=$(shell go list ./pkg/... ./cmd/... ./internal/... | grep -v 'pkg/generated' | xargs | sed -e 's/ /,/g')
#INTEGRATION_TESTS=$(shell go list ./test/... | xargs | sed -e 's/ /,/g')

.PHONY: test-ci
test-ci: test-ci test-ci-unit test-join

.PHONY: test-ci-unit
test-ci-unit: ## test-ci-unit runs all tests for CI builds
	ginkgo -r -coverprofile=cover-unit.out.tmp -outputdir=. --randomizeAllSpecs --randomizeSuites --cover --race --progress --trace ./pkg ./cmd ./internal
	cat cover-unit.out.tmp | grep -v "_generated.go|zz_generated|testbin.go|wire_gen.go" > cover-unit.out

.PHONY: test-ci-int
test-ci-int:  ## test-ci-int runs all tests for CI builds
	kubectl kuttl test --namespace openshift-redhat-marketplace --kind-context test --config ./kuttl-test-kind.yaml ./test/e2e --test ^register-test$

CLUSTER_TYPE ?= kind

.PHONY: tdd
tdd: ## tdd runs integration tests
ifeq ($(CLUSTER_TYPE),kind)
	cd ./scripts/skaffold-tdd-tool && NAMESPACE=$(NAMESPACE) go run .
else
	cd ./scripts/skaffold-tdd-tool && DISABLED_FEATURES="MockOpenShift" NAMESPACE=$(NAMESPACE) go run .
endif


test-join: $(gocovmerge)
	$(gocovmerge) cover-int.out cover-unit.out > cover.out

$(gocovmerge):
	GO111MODULE="on" go get "github.com/wadey/gocovmerge"

cover.out:
	make test-join

test-cover-text: cover.out ## Run coverage and display as html
	go tool cover -func=cover.out

test-cover-html: cover.out ## Run coverage and display as html
	go tool cover -html=cover.out

test-generate-certs:
	mkdir -p test/certs
	#cd test/certs && $(cfssl) gencert -initca ca-csr.json | ../../$(cfssljson) -bare ca
	cd test/certs && $(cfssl) gencert -ca=ca.crt -ca-key=ca.key -profile=kubernetes server-csr.json | ../../$(cfssljson) -bare server

##@ Misc

.PHONY: deploy-test-prometheus
deploy-test-prometheus: ## Helper to setup minikube
	. ./scripts/deploy_test_prometheus.sh

.PHONY: check-licenses
check-licenses: ## Check if all files have licenses
	addlicense -check -c "IBM Corp." ./pkg/**/*.go ./cmd/**/*.go ./internal/**/*.go ./test/**/*.go

.PHONY: add-licenses
add-licenses: ## Add licenses to the go file
	addlicense -c "IBM Corp." ./pkg/**/*.go ./cmd/**/*.go ./internal/**/*.go ./test/**/*.go

scorecard: ## Run scorecard tests
	$(operator-sdk) scorecard -b ./deploy/olm-catalog/redhat-marketplace-operator

##@ Publishing

OPERATOR_IMAGE ?= $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(VERSION)
REDHAT_IMAGE_REGISTRY := scan.connect.redhat.com/ospid-c93f69b6-cb04-437b-89d6-e5220ce643cd
REDHAT_OPERATOR_IMAGE := $(REDHAT_IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(VERSION)

.PHONY: bundle
bundle: ## Bundles the csv to submit
	. ./scripts/bundle_csv.sh `pwd` $(VERSION) $(OPERATOR_IMAGE)

DATETIME = $(shell date +"%FT%H%M%SZ")
REDHAT_PROJECT_ID ?= ospid-962ccd50-bf22-4663-a865-f539e2189f0e
REDHAT_API_KEY ?=

.PHONY: upload-bundle
upload-bundle: ## Uploads bundle to partner connect (use with caution and only on release branch)
	curl https://connect.redhat.com/api/v2/projects/$(REDHAT_PROJECT_ID)/operator-upload \
			-H "Authorization: Bearer $(REDHAT_API_KEY)" -H "Content-Type: application/json" \
			--data '{"file": "$(shell cat bundle/redhat-marketplace-operator-bundle-$(VERSION).zip | base64)", "filename": "redhat-marketplace-operator-bundle-$(VERSION)-$(DATETIME).zip", "filepath": "public://redhat-marketplace-operator-bundle-$(VERSION)-$(DATETIME).zip"}'

.PHONY: publish-image
publish-image: ## Publish image
	make build
	$(docker) tag $(OPERATOR_IMAGE) $(REDHAT_OPERATOR_IMAGE)
	$(docker) push $(OPERATOR_IMAGE)
	$(docker) push $(REDHAT_OPERATOR_IMAGE)

IMAGE ?= $(OPERATOR_IMAGE)

tag-and-push: ## Tag and push operator-image
	$(docker) tag $(IMAGE) $(TAG)
	$(docker) push $(TAG)

ARGS ?= "--patch"

.PHONY: bump-version
bump-version: ## Bump the version and add the file for a commit
	@go run scripts/version/main.go next $(ARGS)


REDHAT_PROJECT_ID ?= ospid-c93f69b6-cb04-437b-89d6-e5220ce643cd
SHA ?=
TAG ?= latest

publish-pc: ## Publish to partner connect
	curl -X POST https://connect.redhat.com/api/v2/projects/$(REDHAT_PROJECT_ID)/containers/$(SHA)/tags/$(TAG)/publish -H "Authorization: Bearer $(REDHAT_API_KEY)" -H "Content-type: application/json" --data "{}" | jq

publish-status-pc: ## Get publish status to partner connect
	@curl -X GET 'https://connect.redhat.com/api/v2/projects/$(REDHAT_PROJECT_ID)?tags=$(TAG)' -H "Authorization: Bearer $(REDHAT_API_KEY)" -H "Content-type: application/json" | jq

OS_PIDS ?= ""
REPOS ?= ""
TAG ?= ""
CREDS ?= ""
TIMEOUT ?= 15

wait-and-publish:
	RESULTS=""; \
	array=($(OS_PIDS)) ; \
	repos=($(REPOS)) ; \
	for i in "$${!array[@]}"; do \
			PID="$${array[$$i]}" ; \
			REPO="$${repos[$$i]}" ; \
			RESULT=`skopeo inspect docker://$$REPO:$$TAG` ; \
			if [ $$? -ne 0 ]; then echo "failed to get skopeo" && exit 1 ; fi ; \
			DIGEST=`echo $$RESULT | $(jq) -r '.Digest'` ; \
			RESULTS="--pid $$PID=$$DIGEST $$RESULTS" ; \
	done ; \
	echo $$RESULTS ; \
	cd scripts ; \
	go run main.go wait-and-publish --timeout $(TIMEOUT) --tag $(TAG) $$RESULTS

##@ Release

.PHONY: current-version
current-version: ## Get current version
	@echo $(VERSION)

.PHONY: release-start
release-start: ## Start a release
	git flow release start $(go run scripts/version/main.go version)

.PHONY: release-finish
release-finish: ## Start a release
	git flow release finish $(go run scripts/version/main.go version)

##@ OPM

OLM_REPO ?= quay.io/rh-marketplace/redhat-marketplace-operator-manifest
OLM_BUNDLE_REPO ?= quay.io/rh-marketplace/redhat-marketplace-operator-bundle
OLM_PACKAGE_NAME ?= redhat-marketplace-operator
TAG ?= $(VERSION)

opm-bundle-all: # used to bundle all the versions available
	./scripts/opm_bundle_all.sh $(OLM_REPO) $(OLM_PACKAGE_NAME) $(VERSION)

opm-bundle-last-beta: ## Bundle latest for beta
	$(operator-sdk) bundle create -g --directory "./deploy/olm-catalog/redhat-marketplace-operator/manifests" -c stable,beta --default-channel stable --package $(OLM_PACKAGE_NAME)
	$(docker) build -f custom-bundle.Dockerfile -t "$(OLM_REPO):$(TAG)" --build-arg channels=beta .
	$(docker) tag "$(OLM_REPO):$(TAG)" "$(OLM_REPO):$(VERSION)"
	$(docker) push "$(OLM_REPO):$(TAG)"
	$(docker) push "$(OLM_REPO):$(VERSION)"

opm-bundle-last-stable: ## Bundle latest for stable
	$(operator-sdk) bundle create -g --directory "./deploy/olm-catalog/redhat-marketplace-operator/manifests" -c stable,beta --default-channel stable --package $(OLM_PACKAGE_NAME)
	$(docker) build -f custom-bundle.Dockerfile -t "$(OLM_REPO):$(TAG)" --build-arg channels=stable,beta .
	$(docker) tag "$(OLM_REPO):$(TAG)" "$(OLM_REPO):$(VERSION)"
	$(docker) push "$(OLM_REPO):$(TAG)"
	$(docker) push "$(OLM_REPO):$(VERSION)"

opm-index-base: ## Create an index base
	git fetch --tags
	./scripts/opm_build_index.sh $(OLM_REPO) $(OLM_BUNDLE_REPO) $(TAG) $(VERSION)

install-test-registry: ## Install the test registry
	kubectl apply -f ./deploy/olm-catalog/test-registry.yaml


##@ Tools

.PHONY: install-tools
install-tools: testbin $(cfssl) $(cfssljson) $(cfssl-certinfo) $(skaffold) $(operator-sdk) $(opm) $(kind)
	@echo Installing tools from tools.go
	@$(shell cd ./scripts && GO111MODULE=off go get -tags tools)
	@cat scripts/tools.go | grep _ | awk -F'"' '{print $$2}' | xargs -tI % go install %

UNAME := $(shell echo `uname` | tr '[:upper:]' '[:lower:]')

testbin:
	mkdir -p testbin

testbin/etcd:
	make install-envtest

testbin/kubectl:
	make install-envtest

testbin/kube-apiserver:
	make install-envtest

K8S_VERSION = v1.18.2
ETCD_VERSION = v3.4.3
install-envtest: testbin
	/bin/bash ./scripts/setup_envtest.sh $(K8S_VERSION) $(ETCD_VERSION)
	chmod +x testbin/etcd testbin/kubectl testbin/kube-apiserver

$(cfssl): testbin
	cd testbin && curl -L https://github.com/cloudflare/cfssl/releases/download/v1.4.1/cfssl_1.4.1_$(UNAME)_amd64 -o cfssl
	chmod +x testbin/cfssl

$(cfssljson): testbin
	cd testbin && curl -L https://github.com/cloudflare/cfssl/releases/download/v1.4.1/cfssljson_1.4.1_$(UNAME)_amd64 -o cfssljson
	chmod +x testbin/cfssljson

$(cfssl-certinfo): testbin
	cd testbin && curl -L https://github.com/cloudflare/cfssl/releases/download/v1.4.1/cfssl-certinfo_1.4.1_$(UNAME)_amd64 -o cfssl-certinfo
	chmod +x testbin/cfssl-certinfo

$(kind): testbin
	GO111MODULE="on" go get sigs.k8s.io/kind

operator_sdk_version ?= v0.19.4

ifeq ($(UNAME),darwin)
	operator_sdk_uname = apple-darwin
else
	operator_sdk_uname = linux-gnu
endif


$(operator-sdk): testbin
	echo $(operator_sdk_uname)
	curl -LO https://github.com/operator-framework/operator-sdk/releases/download/$(operator_sdk_version)/operator-sdk-$(operator_sdk_version)-x86_64-$(operator_sdk_uname)
	chmod +x operator-sdk-$(operator_sdk_version)-x86_64-$(operator_sdk_uname) && mv operator-sdk-$(operator_sdk_version)-x86_64-$(operator_sdk_uname) testbin/operator-sdk

skaffold_version ?= v1.17.0

$(skaffold): testbin
	curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/$(skaffold_version)/skaffold-$(UNAME)-amd64
	chmod +x skaffold && mv skaffold testbin/skaffold

opm_version ?= v1.14.1

$(opm): testbin
	curl -LO https://github.com/operator-framework/operator-registry/releases/download/$(opm_version)/$(UNAME)-amd64-opm
	chmod +x $(UNAME)-amd64-opm && mv $(UNAME)-amd64-opm testbin/opm

.PHONY: help
help: ## Display this help
	@echo -e "Usage:\n  make \033[36m<target>\033[0m"
	@echo Targets:
	@awk 'BEGIN {FS = ":.*##"}; \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
