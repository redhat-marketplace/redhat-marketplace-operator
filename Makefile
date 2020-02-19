.DEFAULT_GOAL:=help
SHELL:=/bin/bash
NAMESPACE=marketplace-operator
OPERATOR_IMAGE=marketplace-operator:latest

##@ Application

install: ## Install all resources (CR/CRD's, RBAC and Operator)
	@echo ....... Creating namespace .......
	- kubectl create namespace ${NAMESPACE}
	@echo ....... Applying CRDs .......
	- kubectl apply -f deploy/crds/marketplace.redhat.com_marketplaceconfigs_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_meterbases_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_meterings_crd.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_razeedeployments_crd.yaml -n ${NAMESPACE}
	@echo ....... Applying serivce accounts and role ........
	- kubectl apply -f deploy/role.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/role_binding.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/service_account.yaml -n ${NAMESPACE}
	@echo ....... Applying Operator .......
	- kubectl apply -f deploy/operator.yaml -n ${NAMESPACE}
	@echo ....... Applying Rules and Service Account .......
	- kubectl apply -f deploy/crds/marketplace.redhat.com_v1alpha1_marketplaceconfig_cr.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_v1alpha1_meterbase_cr.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_v1alpha1_razeedeployment_cr.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/crds/marketplace.redhat.com_v1alpha1_metering_cr.yaml -n ${NAMESPACE}

uninstall: ## Uninstall all that all performed in the $ make install
	@echo ....... Uninstalling .......
	@echo ....... Deleting CRDs.......
	- kubectl delete -f deploy/crds/marketplace.redhat.com_marketplaceconfigs_crd.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_meterbases_crd.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_meterings_crd.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/crds/marketplace.redhat.com_razeedeployments_crd.yaml -n ${NAMESPACE}
	@echo ....... Deleting Rules and Service Account .......
	- kubectl delete -f deploy/role.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/role_binding.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/service_account.yaml -n ${NAMESPACE}
	@echo ....... Deleting Operator .......
	- kubectl delete -f deploy/operator.yaml -n ${NAMESPACE}
	@echo ....... Deleting namespace ${NAMESPACE}.......
	- kubectl delete namespace ${NAMESPACE}

##@ Build

.PHONY: build
build: ## Build the operator executable
	@echo Building the operator exec with image name ${OPERATOR_IMAGE}
	mkdir -p build/_output
	[ -d "build/_output/assets" ] && rm -rf build/_output/assets
	cp -r ./assets build/_output
	operator-sdk build ${OPERATOR_IMAGE}

##@ Development

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
	@echo Updating the deep copy files with the changes in the API
	operator-sdk generate k8s
	@echo Updating the CRD files with the OpenAPI validations
	operator-sdk generate crds

##@ Manual Testing

create: ##creates the required crds for this deployment
	@echo creating crds
	- kubectl create -f deploy/crds/marketplace.redhat.com_marketplaceconfigs_crd.yaml
	- kubectl create -f deploy/crds/marketplace.redhat.com_razeedeployments_crd.yaml
	- kubectl create -f deploy/crds/marketplace.redhat.com_meterings_crd.yaml

deploys: ##deploys the resources for deployment
	@echo creating service_account
	- kubectl create -f deploy/service_account.yaml
	@echo creating role
	- kubectl create -f deploy/role.yaml
	@echo creating role_binding
	- kubectl create -f deploy/role_binding.yaml
	@echo creating operator
	- kubectl create -f deploy/operator.yaml

apply: ##applies changes to crds
	- kubectl apply -f deploy/crds/marketplace.redhat.com_v1alpha1_marketplaceconfig_cr.yaml
	- kubectl apply -f deploy/crds/marketplace.redhat.com_v1alpha1_razeedeployment_cr.yaml
	- kubectl apply -f deploy/crds/marketplace.redhat.com_v1alpha1_metering_cr.yaml
	
clean: ##delete the contents created in cr-crd
	@echo deleting resources
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_marketplaceconfig_cr.yaml
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_razeedeployment_cr.yaml
	- kubectl delete -f deploy/crds/marketplace.redhat.com_v1alpha1_metering_cr.yaml
	- kubectl delete -f deploy/operator.yaml
	- kubectl delete -f deploy/role_binding.yaml
	- kubectl delete -f deploy/role.yaml
	- kubectl delete -f deploy/service_account.yaml
	- kubectl delete -f deploy/crds/marketplace.redhat.com_marketplaceconfigs_crd.yaml
	- kubectl delete -f deploy/crds/marketplace.redhat.com_razeedeployments_crd.yaml
	- kubectl delete -f deploy/crds/marketplace.redhat.com_meterings_crd.yaml

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

.PHONY: test-e2e
test-e2e: ## Run integration e2e tests with different options.
	@echo ... Making image for e2e ...
	- make image
	@echo ... Running the same e2e tests with different args ...
	@echo ... Running locally ...
	- kubectl create namespace ${NAMESPACE} || true
	- operator-sdk test local ./test/e2e --namespace=${NAMESPACE} --go-test-flags="-tags e2e"

##@ General

.PHONY: help
help: ## Display this help
	@echo -e "Usage:\n  make \033[36m<target>\033[0m"
	@awk 'BEGIN {FS = ":.*##"}; \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
