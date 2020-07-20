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
DOCKER_EXEC ?= $(shell command -v docker)

SERVICE_ACCOUNT := redhat-marketplace-operator
SECRETS_NAME := my-docker-secrets

OPERATOR_IMAGE ?= $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_IMAGE_TAG)

PULL_POLICY ?= IfNotPresent
.DEFAULT_GOAL := help

# cluster server to test the rhm operator
CLUSTER_SERVER ?= https://api.crc.testing:6443
# The namespace where the operator watches for changes. Set "" for AllNamespaces, set "ns1,ns2" for MultiNamespace
OPERATOR_WATCH_NAMESPACE ?= ""

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

.PHONY: build
build: ## Build the operator executable
	DOCKER_EXEC=$(DOCKER_EXEC) VERSION=$(VERSION) PUSH_IMAGE=false IMAGE=$(OPERATOR_IMAGE) ./scripts/skaffold_build.sh

.PHONY: push
push: push ## Push the operator image
	$(DOCKER_EXEC) push $(OPERATOR_IMAGE)

helm: ## build helm base charts
	. ./scripts/package_helm.sh $(VERSION) deploy ./deploy/chart/values.yaml --set image=$(OPERATOR_IMAGE) --set namespace=$(NAMESPACE)

MANIFEST_CSV_FILE := ./deploy/olm-catalog/redhat-marketplace-operator/manifests/redhat-marketplace-operator.clusterserviceversion.yaml
VERSION_CSV_FILE := ./deploy/olm-catalog/redhat-marketplace-operator/$(VERSION)/redhat-marketplace-operator.v$(VERSION).clusterserviceversion.yaml
CSV_CHANNEL ?= beta # change to stable for release
CSV_DEFAULT_CHANNEL ?= false # change to true for release
CHANNELS ?= beta

generate-bundle: ## Generate the csv
	make helm
	operator-sdk bundle create --generate-only \
		--package redhat-marketplace-operator \
		--default-channel=$(CSV_DEFAULT_CHANNEl) \
		--channels $(CHANNELS)
	@go run github.com/mikefarah/yq/v3 w -i $(MANIFEST_CSV_FILE) 'metadata.annotations.containerImage' $(OPERATOR_IMAGE)
	@go run github.com/mikefarah/yq/v3 w -i $(MANIFEST_CSV_FILE) 'metadata.annotations.createdAt' $(CREATED_TIME)
	@go run github.com/mikefarah/yq/v3 d -i $(MANIFEST_CSV_FILE) 'spec.install.spec.deployments[*].spec.template.spec.containers[*].env(name==WATCH_NAMESPACE).valueFrom'
	@go run github.com/mikefarah/yq/v3 w -i $(MANIFEST_CSV_FILE) 'spec.install.spec.deployments[*].spec.template.spec.containers[*].env(name==WATCH_NAMESPACE).value' ''

INTERNAL_CRDS='["razeedeployments.marketplace.redhat.com","meterbases.marketplace.redhat.com","meterdefinitions.marketplace.redhat.com","remoteresources3s.marketplace.redhat.com"]'

generate-csv: ## Generate the csv
	make helm
	operator-sdk generate csv \
		--from-version=$(FROM_VERSION) \
		--csv-version=$(VERSION) \
		--csv-channel=$(CSV_CHANNEL) \
		--default-channel=$(CSV_DEFAULT_CHANNEL) \
		--operator-name=redhat-marketplace-operator \
		--update-crds \
		--make-manifests=false
	@go run github.com/mikefarah/yq/v3 w -i $(VERSION_CSV_FILE) 'metadata.annotations.containerImage' $(OPERATOR_IMAGE)
	@go run github.com/mikefarah/yq/v3 w -i $(VERSION_CSV_FILE) 'metadata.annotations.createdAt' $(CREATED_TIME)
	@go run github.com/mikefarah/yq/v3 w -i $(VERSION_CSV_FILE) 'metadata.annotations."operators.operatorframework.io/internal-objects"' $(INTERNAL_CRDS)
	@go run github.com/mikefarah/yq/v3 d -i $(VERSION_CSV_FILE) 'spec.install.spec.deployments[*].spec.template.spec.containers[*].env(name==WATCH_NAMESPACE).valueFrom'
	@go run github.com/mikefarah/yq/v3 w -i $(VERSION_CSV_FILE) 'spec.install.spec.deployments[*].spec.template.spec.containers[*].env(name==WATCH_NAMESPACE).value' ''

PACKAGE_FILE ?= ./deploy/olm-catalog/redhat-marketplace-operator/redhat-marketplace-operator.package.yaml

manifest-package-beta: # Make sure we have the right versions
	@go run github.com/mikefarah/yq/v3 w -i $(PACKAGE_FILE) 'channels.(name==stable).currentCSV' redhat-marketplace-operator.v$(FROM_VERSION)
	@go run github.com/mikefarah/yq/v3 w -i $(PACKAGE_FILE) 'channels.(name==beta).currentCSV' redhat-marketplace-operator.v$(VERSION)

manifest-package-stable: # Make sure we have the right versions
	@go run github.com/mikefarah/yq/v3 w -i $(PACKAGE_FILE) 'channels.(name==stable).currentCSV' redhat-marketplace-operator.v$(VERSION)
	@go run github.com/mikefarah/yq/v3 w -i $(PACKAGE_FILE) 'channels.(name==beta).currentCSV' redhat-marketplace-operator.v$(VERSION)

REGISTRY ?= quay.io

docker-login: ## Log into docker using env $DOCKER_USER and $DOCKER_PASSWORD
	@$(DOCKER_EXEC) login -u="$(DOCKER_USER)" -p="$(DOCKER_PASSWORD)" $(REGISTRY)

##@ Development

skaffold-dev: ## Run skaffold dev. Will unique tag the operator and rebuild.
	make helm
	make create
	. ./scripts/package_helm.sh $(VERSION) deploy ./deploy/chart/values.yaml --set image=redhat-marketplace-operator --set pullPolicy=IfNotPresent
	DOCKER_EXEC=$(DOCKER_EXEC) skaffold dev --tail --default-repo $(IMAGE_REGISTRY)

skaffold-run: ## Run skaffold run. Will uniquely tag the operator.
	make helm
	make create
	. ./scripts/package_helm.sh $(VERSION) deploy ./deploy/chart/values.yaml --set image=redhat-marketplace-operator --set pullPolicy=IfNotPresent
	DOCKER_EXEC=$(DOCKER_EXEC) skaffold run --tail --default-repo $(IMAGE_REGISTRY) --cleanup=false

code-vet: ## Run go vet for this project. More info: https://golang.org/cmd/vet/
	@echo go vet
	go vet $$(go list ./... )

code-fmt: ## Run go fmt for this project
	@echo go fmt
	go fmt $$(go list ./... )

code-dev: ## Run the default dev commands which are the go fmt and vet then execute the $ make code-gen
	@echo Running the common required commands for developments purposes
	- make code-fmt
	- make code-vet
	- make code-gen

code-gen: ## Run the operator-sdk commands to generated code (k8s and crds)
	@echo Generating k8s
	operator-sdk generate k8s
	@echo Updating the CRD files with the OpenAPI validations
	operator-sdk generate crds --crd-version=v1beta1
	@echo Generating the yamls for deployment
	- make helm
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

setup-operator-sdk-run: ## Create ns, crds, sa, role, and rolebinding before operator-sdk run
	- make helm
	- make create
	- make deploy-services
	- . ./scripts/operator_sdk_sa_kubeconfig.sh $(CLUSTER_SERVER) $(NAMESPACE) $(SERVICE_ACCOUNT)

operator-sdk-run: ## Run operator locally outside the cluster during development cycle
	operator-sdk run local --watch-namespace="" --kubeconfig=./sa.kubeconfig

##@ Manual Testing

create: ##creates the required crds for this deployment
	@echo creating crds
	- kubectl create namespace ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_marketplaceconfigs_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_razeedeployments_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_meterbases_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_meterdefinitions_crd.yaml -n ${NAMESPACE}
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
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_meterbase_cr.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_meterdefinition_cr.yaml -n ${NAMESPACE}
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
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run

.PHONY: test
test: ## Run go tests
	@echo ... Run tests
	go test ./...

.PHONY: test-cover
test-cover: ## Run coverage on code
	@echo Running coverage
	go test -coverprofile cover.out ./...
	go tool cover -func=cover.out

test-cover-html: test-cover ## Run coverage and display as html
	go tool cover -html=cover.out

.PHONY: test-integration
test-integration:
	@echo Test integration

.PHONY: test-e2e
test-e2e: ## Run integration e2e tests with different options.
	@echo ... Making build for e2e ...
	@echo ... Applying code templates for e2e ...
	- make code-templates
	@echo ... Running the same e2e tests with different args ...
	@echo ... Running locally ...
	- kubectl create namespace ${NAMESPACE} || true
	- operator-sdk test local ./test/e2e --namespace=${NAMESPACE} --go-test-flags="-tags e2e"

##@ Misc

.PHONY: deploy-test-prometheus
deploy-test-prometheus: ## Helper to setup minikube
	. ./scripts/deploy_test_prometheus.sh

.PHONY: check-licenses
check-licenses: ## Check if all files have licenses
	go run github.com/google/addlicense -check -c "IBM Corp." **/*.go

.PHONY: add-licenses
add-licenses: ## Add licenses to the go file
	go run github.com/google/addlicense -c "IBM Corp." **/*.go

scorecard: ## Run scorecard tests
	operator-sdk scorecard -b ./deploy/olm-catalog/redhat-marketplace-operator

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
	$(DOCKER_EXEC) tag $(OPERATOR_IMAGE) $(REDHAT_OPERATOR_IMAGE)
	$(DOCKER_EXEC) push $(OPERATOR_IMAGE)
	$(DOCKER_EXEC) push $(REDHAT_OPERATOR_IMAGE)

IMAGE ?= $(OPERATOR_IMAGE)

tag-and-push: ## Tag and push operator-image
	$(DOCKER_EXEC) tag $(IMAGE) $(TAG)
	$(DOCKER_EXEC) push $(TAG)

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

OLM_REPO ?= quay.io/rh-marketplace/operator-manifest
OLM_BUNDLE_REPO ?= quay.io/rh-marketplace/operator-manifest-bundle
OLM_PACKAGE_NAME ?= redhat-marketplace-operator-test
TAG ?= latest

opm-bundle-all: # used to bundle all the versions available
	./scripts/opm_bundle_all.sh $(OLM_REPO) $(OLM_PACKAGE_NAME)

opm-bundle-last-edge: ## Bundle latest for edge
	operator-sdk bundle create -g --directory "./deploy/olm-catalog/redhat-marketplace-operator/$(VERSION)" -c stable,beta --default-channel stable --package $(OLM_PACKAGE_NAME)
	@go run github.com/mikefarah/yq/v3 w -i deploy/olm-catalog/redhat-marketplace-operator/metadata/annotations.yaml 'annotations."operators.operatorframework.io.bundle.channels.v1"' edge
	docker build -f bundle.Dockerfile -t "$(OLM_REPO):$(TAG)" .
	docker push "$(OLM_REPO):$(TAG)"

opm-bundle-last-beta: ## Bundle latest for beta
	operator-sdk bundle create -g --directory "./deploy/olm-catalog/redhat-marketplace-operator/$(VERSION)" -c stable,beta --default-channel stable --package $(OLM_PACKAGE_NAME)
	@go run github.com/mikefarah/yq/v3 w -i deploy/olm-catalog/redhat-marketplace-operator/metadata/annotations.yaml 'annotations."operators.operatorframework.io.bundle.channels.v1"' beta
	docker build -f bundle.Dockerfile -t "$(OLM_REPO):$(TAG)" .
	docker push "$(OLM_REPO):$(TAG)"

olm-bundle-last-stable: ## Bundle latest for stable
	operator-sdk bundle create "$(OLM_REPO):$(TAG)" --directory "./deploy/olm-catalog/redhat-marketplace-operator/$(VERSION)" -c stable,beta --default-channel stable --package $(OLM_PACKAGE_NAME)

opm-index-base: ## Create an index base
	./scripts/opm_build_index.sh $(OLM_REPO) $(OLM_BUNDLE_REPO) $(TAG) $(VERSION)

install-test-registry: ## Install the test registry
	kubectl apply -f ./deploy/olm-catalog/test-registry.yaml

##@ Help

.PHONY: help
help: ## Display this help
	@echo -e "Usage:\n  make \033[36m<target>\033[0m"
	@echo Targets:
	@awk 'BEGIN {FS = ":.*##"}; \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
