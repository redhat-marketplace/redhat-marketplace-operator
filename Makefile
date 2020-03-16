SHELL:=/bin/bash
NAMESPACE ?= redhat-marketplace-operator
IMAGE_REGISTRY ?= public-image-registry.apps-crc.testing/symposium
OPERATOR_IMAGE_NAME ?= redhat-marketplace-operator
OPERATOR_IMAGE_TAG ?= latest
AGENT_IMAGE_NAME ?= marketplace-agent
AGENT_IMAGE_TAG ?= latest

SERVICE_ACCOUNT := redhat-marketplace-operator

OPERATOR_IMAGE := $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_IMAGE_TAG)
AGENT_IMAGE := $(IMAGE_REGISTRY)/$(AGENT_IMAGE_NAME):$(AGENT_IMAGE_TAG)

include scripts/RegistryMakefile

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

hot-update: ##
	@echo ........ Building .........
	make build
	@echo ........ Pushing ..........
	make push
	@echo ........ Deleteing pods ...
	- kubectl delete pods -n $(NAMESPACE) -l name=$(OPERATOR_IMAGE_NAME)
	@echo ........ Recreating crs ..........
	make unapply
	make apply

##@ Build

.PHONY: build
build: ## Build the operator executable
	@echo Adding assets
	@mkdir -p build/_output
	- [ -d "build/_output/assets" ] && rm -rf build/_output/assets
	@cp -r ./assets build/_output
	@make code-templates
	@echo Building the operator exec with image name $(OPERATOR_IMAGE)
	operator-sdk build $(OPERATOR_IMAGE)

.PHONY: push
push: push ## Push the operator image
	docker push $(OPERATOR_IMAGE)

##@ Development

code-vet: ## Run go vet for this project. More info: https://golang.org/cmd/vet/
	@echo go vet
	go vet $$(go list ./... )

code-fmt: ## Run go fmt for this project
	@echo go fmt
	go fmt $$(go list ./... )

code-templates: ## Gen templates
	@RELATED_IMAGE_MARKETPLACE_OPERATOR=$(OPERATOR_IMAGE) RELATED_IMAGE_MARKETPLACE_AGENT=$(AGENT_IMAGE) scripts/gen_files.sh

code-dev: ## Run the default dev commands which are the go fmt and vet then execute the $ make code-gen
	@echo Running the common required commands for developments purposes
	- make code-fmt
	- make code-vet
	- make code-gen

code-gen: ## Run the operator-sdk commands to generated code (k8s and crds)
	@echo Updating the deep copy files with the changes in the API
	operator-sdk generate k8s
	@echo Updating the CRD files with the OpenAPI validations
	operator-sdk generate crds
	@echo Generating the yamls for deployment
	- make code-templates

setup-minikube: ## Setup minikube for full operator dev
	@echo Applying prometheus operator
	kubectl apply -f https://raw.githubusercontent.com/coreos/prometheus-operator/master/bundle.yaml
	@echo Applying olm
	kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-lifecycle-manager/master/deploy/upstream/quickstart/crds.yaml
	kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-lifecycle-manager/master/deploy/upstream/quickstart/olm.yaml
	@echo Applying operator marketplace
	for item in 01_namespace.yaml 02_catalogsourceconfig.crd.yaml 03_operatorsource.crd.yaml 04_service_account.yaml 05_role.yaml 06_role_binding.yaml 07_upstream_operatorsource.cr.yaml 08_operator.yaml ; do \
		kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-marketplace/master/deploy/upstream/$$item ; \
	done

##@ Manual Testing

create: ##creates the required crds for this deployment
	@echo creating crds
	- kubectl create -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_marketplaceconfigs_crd.yaml
	- kubectl create -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_razeedeployments_crd.yaml
	- kubectl create -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_meterings_crd.yaml
	- kubectl create -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_meterbases_crd.yaml

deploys: ##deploys the resources for deployment
	@echo creating service_account
	- kubectl create -n $(NAMESPACE) -f deploy/service_account.yaml
	@echo creating role
	- kubectl create -n $(NAMESPACE) -f deploy/role.yaml
	@echo creating role_binding
	- kubectl create -n $(NAMESPACE) -f deploy/role_binding.yaml
	@echo creating operator
	- kubectl create -n $(NAMESPACE) -f deploy/operator.yaml

apply: ##applies changes to crds
	- kubectl apply -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_v1alpha1_marketplaceconfig_cr.yaml
	- kubectl apply -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_v1alpha1_razeedeployment_cr.yaml
	- kubectl apply -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_v1alpha1_metering_cr.yaml

unapply: ##removes the crds
	- kubectl delete -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_v1alpha1_marketplaceconfig_cr.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_v1alpha1_razeedeployment_cr.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_v1alpha1_metering_cr.yaml

clean: ##delete the contents created in 'make create'
	@echo deleting resources
	- kubectl delete opsrc ${OPERATOR_SOURCE}
	- kubectl delete -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_v1alpha1_marketplaceconfig_cr.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_v1alpha1_razeedeployment_cr.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_v1alpha1_metering_cr.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_v1alpha1_meterbase_cr.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/operator.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/role_binding.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/role.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/service_account.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_marketplaceconfigs_crd.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_razeedeployments_crd.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_meterings_crd.yaml
	- kubectl delete -n $(NAMESPACE) -f deploy/crds/marketplace.redhat.com_meterbases_crd.yaml

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
	- make build
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
