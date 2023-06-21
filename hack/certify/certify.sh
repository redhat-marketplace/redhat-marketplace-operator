#!/bin/bash
set -Eeox pipefail

SLEEP_LONG="${SLEEP_LONG:-5}"
SLEEP_SHORT="${SLEEP_SHORT:-2}"
CERT_NAMESPACE="${CERT_NAMESPACE:-rhm-certification}"
KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"
SUBMIT="${SUBMIT:-false}"

# certified operator name: redhat-marketplace-operator, ibm-metrics-operator
OP_NAME=$1

BRANCH="${OP_NAME}-${VERSION}"

# Check Subscriptions: subscription-name, namespace
checksub () {
	echo "Waiting for Subscription $1 InstallPlan to complete."

	# Wait 2 resync periods for OLM to emit new installplan
	sleep 60

	# Wait for the InstallPlan to be generated and available on status
	unset INSTALL_PLAN
	until oc get subscription $1 -n $2 --output=jsonpath={.status.installPlanRef.name}
	do
		sleep $SLEEP_SHORT
	done

	# Get the InstallPlan
	until [ -n "$INSTALL_PLAN" ]
	do
		sleep $SLEEP_SHORT
		INSTALL_PLAN=$(oc get subscription $1 -n $2 --output=jsonpath={.status.installPlanRef.name})
	done

	# Wait for the InstallPlan to Complete
	unset PHASE
	until [ "$PHASE" == "Complete" ]
	do
		PHASE=$(oc get installplan $INSTALL_PLAN -n $2 --output=jsonpath={.status.phase})
    if [ "$PHASE" == "Failed" ]; then
      set +x
      sleep 3
      echo "InstallPlan $INSTALL_PLAN for subscription $1 failed."
      echo "To investigate the reason of the InstallPlan failure run:"
      echo "oc describe installplan $INSTALL_PLAN -n $2"
      exit 1
    fi
		sleep $SLEEP_SHORT
	done
	
	# Get installed CluserServiceVersion
	unset CSV
	until [ -n "$CSV" ]
	do
		sleep $SLEEP_SHORT
		CSV=$(oc get subscription $1 -n $2 --output=jsonpath={.status.installedCSV})
	done
	
	# Wait for the CSV
	unset PHASE
	until [ "$PHASE" == "Succeeded" ]
	do
		PHASE=$(oc get clusterserviceversion $CSV -n $2 --output=jsonpath={.status.phase})
    if [ "$PHASE" == "Failed" ]; then
      set +x
      sleep 3
      echo "ClusterServiceVersion $CSV for subscription $1 failed."
      echo "To investigate the reason of the ClusterServiceVersion failure run:"
      echo "oc describe clusterserviceversion $CSV -n $2"
      exit 1
    fi
		sleep $SLEEP_SHORT
	done
}


if [ -z ${PYXIS_API_KEY+x} ]; then echo "PYXIS_API_KEY is unset"; exit 1; fi

if [ ${SUBMIT} == "true" ] && [ -z ${GITHUB_TOKEN+x} ]; then echo "GITHUB_TOKEN is unset for SUBMIT=true"; exit 1; fi

# Install Subscription
cat <<EOF | oc apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: openshift-redhat-marketplace
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: openshift-pipelines-operator-rh
  namespace: openshift-operators
spec:
  channel: latest
  installPlanApproval: Automatic
  name: openshift-pipelines-operator-rh
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF


# Verify Subscriptions
checksub openshift-pipelines-operator-rh openshift-operators

# Switch to certification namespace
oc delete ns $CERT_NAMESPACE --ignore-not-found
oc adm new-project $CERT_NAMESPACE
oc project $CERT_NAMESPACE

# Wait for the tekton serviceaccount to generate
echo "Waiting for ServiceAccount pipeline in namespace $CERT_NAMESPACE to generate."
until oc -n $CERT_NAMESPACE get serviceaccount pipeline
do
  sleep $SLEEP_LONG
done

oc create secret generic pyxis-api-secret --from-literal pyxis_api_key=$PYXIS_API_KEY

# Create the kubeconfig used by the certification pipeline
oc delete secret kubeconfig --ignore-not-found
oc create secret generic kubeconfig --from-file=kubeconfig=$KUBECONFIG

# Import redhat catalogs
oc import-image certified-operator-index \
  --request-timeout=5m \
  --from=registry.redhat.io/redhat/certified-operator-index \
  --reference-policy local \
  --scheduled \
  --confirm \
  --all

oc import-image redhat-marketplace-index \
  --request-timeout=5m \
  --from=registry.redhat.io/redhat/redhat-marketplace-index \
  --reference-policy local \
  --scheduled \
  --confirm \
  --all

CWD=$(pwd)
TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'cptmpdir')
OP_DIR=$CWD

# Install the Certification Pipeline
cd $TMP_DIR
git clone https://github.com/redhat-openshift-ecosystem/operator-pipelines
cd operator-pipelines

git checkout v1.0.83

# Create a new SCC
oc apply -f ansible/roles/operator-pipeline/templates/openshift/openshift-pipelines-custom-scc.yml
# Add SCC to a pipeline service account
oc adm policy add-scc-to-user pipelines-custom-scc -z pipeline

# Patch for TLSVERIFY=false to use internal registry
# yq eval -i '(.spec.params[] | select(.name == "TLSVERIFY") | .default) = "false"' ansible/roles/operator-pipeline/templates/openshift/tasks/buildah.yml

# Workaround some files that fail webhook validation
oc apply -R -f ansible/roles/operator-pipeline/templates/openshift/pipelines || true
oc apply -R -f ansible/roles/operator-pipeline/templates/openshift/tasks || true

# Add bundle to the fork on version branch
cd $TMP_DIR
git clone git@github.com:redhat-marketplace/certified-operators.git
cd certified-operators

# Keep main up to date before a new branch
git pull https://github.com/redhat-marketplace/certified-operators.git main
git push origin main

git checkout -B $BRANCH

# Cleanup previous manifests, metadata, and create version dir
rm -rf operators/${OP_NAME}/$VERSION/manifests
rm -rf operators/${OP_NAME}/$VERSION/metadata
mkdir -p operators/${OP_NAME}/$VERSION

# Copy the manifests to the branch
cp -r $OP_DIR/bundle/manifests operators/${OP_NAME}/$VERSION/
cp -r $OP_DIR/bundle/metadata operators/${OP_NAME}/$VERSION/

# The operator service account should be ommited in the bundle
# It will fail certification
# The service account will be created by OLM
# kustomize questionable capability to remove the service account
rm -Rf operators/${OP_NAME}/$VERSION/manifests/${OP_NAME}_v1_serviceaccount.yaml

# Set our organization, should be default
# echo "organization: certified-operators" > config.yaml

# This should automatically be present
# redhat-marketplace-operator
# echo "cert_project_id: 5f68c9457115dbd1183ccab6" > operators/${OP_NAME}/ci.yaml
# ibm-metrics-operator
# echo "cert_project_id: 6419c8987cadbe946d0e0594" > operators/${OP_NAME}/ci.yaml
# data-reporter-operator
# echo "cert_project_id: 646fa6b07840293fae4e3d17" > operators/${OP_NAME}/ci.yaml

# Commit and push the changes to the branch
git add --all
git commit -m $BRANCH
git push -f origin $BRANCH


# Run the Pipeline

cd $TMP_DIR/operator-pipelines

# latest client does not print log
# curl https://mirror.openshift.com/pub/openshift-v4/clients/pipeline/latest/tkn-linux-amd64.tar.gz | tar -xz 
curl https://mirror.openshift.com/pub/openshift-v4/clients/pipeline/0.23.1/tkn-linux-amd64-0.23.1.tar.gz | tar -xz

GIT_REPO_URL=https://github.com/redhat-marketplace/certified-operators.git
BUNDLE_PATH=operators/${OP_NAME}/$VERSION

if [ "$SUBMIT" == "true" ]; then
    oc create secret generic github-api-token --from-literal GITHUB_TOKEN=$GITHUB_TOKEN
    ./tkn pipeline start operator-ci-pipeline \
    --use-param-defaults \
    --param git_repo_url=$GIT_REPO_URL \
    --param git_branch=$BRANCH \
    --param bundle_path=$BUNDLE_PATH \
    --param upstream_repo_name=redhat-openshift-ecosystem/certified-operators \
    --param submit=true \
    --param env=prod \
    --workspace name=pipeline,volumeClaimTemplateFile=templates/workspace-template.yml \
    --workspace name=pyxis-api-key,secret=pyxis-api-secret \
    --showlog
else
    ./tkn pipeline start operator-ci-pipeline \
    --use-param-defaults \
    --param git_repo_url=$GIT_REPO_URL \
    --param git_branch=$BRANCH \
    --param bundle_path=$BUNDLE_PATH \
    --param env=prod \
    --workspace name=pipeline,volumeClaimTemplateFile=templates/workspace-template.yml \
    --showlog
fi