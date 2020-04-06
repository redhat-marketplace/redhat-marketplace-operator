SHELL:=/bin/bash
NAMESPACE ?= redhat-marketplace-operator
IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium
OPERATOR_IMAGE_NAME ?= redhat-marketplace-operator
OPERATOR_IMAGE_TAG ?= dev
AGENT_IMAGE_NAME ?= marketplace-agent
AGENT_IMAGE_TAG ?= latest
VERSION ?= $(shell go run scripts/version/main.go)

SERVICE_ACCOUNT := redhat-marketplace-operator
SECRETS_NAME := my-docker-secrets

OPERATOR_IMAGE := $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_IMAGE_TAG)
AGENT_IMAGE := $(IMAGE_REGISTRY)/$(AGENT_IMAGE_NAME):$(AGENT_IMAGE_TAG)

PULL_POLICY ?= IfNotPresent

.DEFAULT_GOAL := help

##@ Application

install: ## Install all resources (CR/CRD's, RBAC and Operator)
	@echo ....... Creating namespace .......
	- kubectl create namespace ${NAMESPACE}
	make create
	make deploys
	make apply

uninstall: ## Uninstall all that all performed in the $ make install
	@echo ....... Uninstalling .......
	@make clean

##@ Build

.PHONY: build
build: ## Build the operator executable
	PUSH_IMAGE=false IMAGE=$(OPERATOR_IMAGE) ./scripts/skaffold-build.sh

.PHONY: push
push: push ## Push the operator image
	docker push $(OPERATOR_IMAGE)

generate-csv: ## Generate the csv
	operator-sdk generate csv --csv-version $(VERSION) --csv-config=./deploy/olm-catalog/csv-config.yaml

##@ Development

skaffold-dev: ## Run skaffold dev. Will unique tag the operator and rebuild. Minikube only.
	make create
	PULL_POLICY=$(PULL_POLICY) RELATED_IMAGE_MARKETPLACE_OPERATOR=redhat-marketplace-operator RELATED_IMAGE_MARKETPLACE_AGENT=$(AGENT_IMAGE) scripts/gen_files.sh
	skaffold dev --tail --default-repo $(IMAGE_REGISTRY)

skaffold-run: ## Run skaffold run. Will uniquely tag the operator. Minikube only.
	make create
	PULL_POLICY=$(PULL_POLICY) RELATED_IMAGE_MARKETPLACE_OPERATOR=redhat-marketplace-operator RELATED_IMAGE_MARKETPLACE_AGENT=$(AGENT_IMAGE) scripts/gen_files.sh
	skaffold run --tail --default-repo $(IMAGE_REGISTRY)

code-vet: ## Run go vet for this project. More info: https://golang.org/cmd/vet/
	@echo go vet
	go vet $$(go list ./... )

code-fmt: ## Run go fmt for this project
	@echo go fmt
	go fmt $$(go list ./... )

code-templates: ## Gen templates
	@PULL_POLICY=$(PULL_POLICY) RELATED_IMAGE_MARKETPLACE_OPERATOR=$(OPERATOR_IMAGE) RELATED_IMAGE_MARKETPLACE_AGENT=$(AGENT_IMAGE) ./scripts/gen_files.sh

code-dev: ## Run the default dev commands which are the go fmt and vet then execute the $ make code-gen
	@echo Running the common required commands for developments purposes
	- make code-fmt
	- make code-vet
	- make code-gen

code-gen: ## Run the operator-sdk commands to generated code (k8s and crds)
	@echo Generating k8s
	operator-sdk generate k8s
	@echo Updating the CRD files with the OpenAPI validations
	operator-sdk generate crds
	@echo Generating the yamls for deployment
	- make code-templates
	@echo Go generatign
	- go generate ./...

setup-minikube: ## Setup minikube for full operator dev
	@echo Applying prometheus operator
	kubectl apply -f https://raw.githubusercontent.com/coreos/prometheus-operator/master/bundle.yaml
	@echo Applying operator marketplace
	for item in 01_namespace.yaml 02_catalogsourceconfig.crd.yaml 03_operatorsource.crd.yaml 04_service_account.yaml 05_role.yaml 06_role_binding.yaml 07_upstream_operatorsource.cr.yaml 08_operator.yaml ; do \
		kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/$$item ; \
	done
	@echo Applying olm
	kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-lifecycle-manager/master/deploy/upstream/quickstart/crds.yaml
	kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-lifecycle-manager/master/deploy/upstream/quickstart/olm.yaml
	@echo Apply kube-state
	for item in cluster-role.yaml service-account.yaml cluster-role-binding.yaml deployment.yaml service.yaml ; do \
		kubectl apply -f https://raw.githubusercontent.com/kubernetes/kube-state-metrics/master/examples/standard/$$item ; \
	done

##@ Manual Testing

create: ##creates the required crds for this deployment
	@echo creating crds
	- kubectl create namespace ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_marketplaceconfigs_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_razeedeployments_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_meterings_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_meterbases_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_meterdefinitions_crd.yaml -n ${NAMESPACE}

deploys: ##deploys the resources for deployment
	@echo deploying services and operators
	- kubectl create -f deploy/service_account.yaml -n ${NAMESPACE}
	- kubectl create -f deploy/role.yaml -n ${NAMESPACE}
	- kubectl create -f deploy/role_binding.yaml -n ${NAMESPACE}
	- kubectl create -f deploy/operator.yaml -n ${NAMESPACE}

apply: ##applies changes to crds
	- kubectl apply -f deploy/crds/marketplace.redhat.com_v1alpha1_marketplaceconfig_cr.yaml -n ${NAMESPACE}

clean: ##delete the contents created in 'make create'
	@echo deleting resources
	- kubectl delete opsrc ${OPERATOR_SOURCE} -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_marketplaceconfig_cr.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_razeedeployment_cr.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_metering_cr.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_meterbase_cr.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_meterdefinitions_cr.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/operator.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/role_binding.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/role.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/service_account.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_marketplaceconfigs_crd.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_razeedeployments_crd.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_meterings_crd.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_meterbases_crd.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_meterdefinitions_crd.yaml -n ${NAMESPACE}
	- kubectl delete namespace razee

##@ Tests

.PHONY: test
test: ## Run go tests
	@echo ... Run tests
	go test ./...

.PHONY: test-cover
test-cover: ## Run coverage on code
	@echo Running coverage
	go test -coverprofile cover.out ./...
	go tool cover -func=cover.out

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

deploy-test-prometheus:
	. ./scripts/deploy_test_prometheus.sh

##@ Help

.PHONY: help
help: ## Display this help
	@echo -e "Usage:\n  make \033[36m<target>\033[0m"
	@echo Targets:
	@awk 'BEGIN {FS = ":.*##"}; \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
