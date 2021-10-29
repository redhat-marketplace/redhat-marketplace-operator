#!/bin/bash
set -Eeox pipefail

SLEEP_LONG="${SLEEP_LONG:-5}"
SLEEP_SHORT="${SLEEP_SHORT:-2}"
CERT_NAMESPACE="rhm-certification"
KUBECONFIG=$HOME/.kube/config
VERSION="2.4.0"

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



# Install Subscription
oc apply -f sub.yaml

# Verify Subscriptions
checksub openshift-pipelines-operator-rh openshift-operators

# Switch to certification namespace
oc delete ns $CERT_NAMESPACE --ignore-not-found
oc adm new-project $CERT_NAMESPACE
oc project $CERT_NAMESPACE

oc create secret generic pyxis-api-secret --from-literal pyxis_api_key=$PYXIS_API_KEY

# Create the kubeconfig used by the certification pipeline
oc delete secret kubeconfig --ignore-not-found
oc create secret generic kubeconfig --from-file=kubeconfig=$KUBECONFIG

# Import redhat catalogs
oc import-image certified-operator-index \
  --from=registry.redhat.io/redhat/certified-operator-index \
  --reference-policy local \
  --scheduled \
  --confirm \
  --all

oc import-image redhat-marketplace-index \
  --from=registry.redhat.io/redhat/redhat-marketplace-index \
  --reference-policy local \
  --scheduled \
  --confirm \
  --all

CWD=$(pwd)
TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'cptmpdir')

# Install the Certification Pipeline
cd $TMP_DIR
git clone https://github.com/redhat-openshift-ecosystem/operator-pipelines
cd operator-pipelines
oc apply -R -f ansible/roles/operator-pipeline/templates/openshift/pipelines
oc apply -R -f ansible/roles/operator-pipeline/templates/openshift/tasks
cd $CWD


# Generate the bundle and add it to the fork
cd $TMP_DIR
git clone git@github.com:redhat-marketplace/certified-operators-preprod.git

cd $CWD
cd ../../

make bundle
rm -rf $TMP_DIR/certified-operators-preprod/operators/redhat-marketplace-operator/$VERSION
mkdir -p $TMP_DIR/certified-operators-preprod/operators/redhat-marketplace-operator/$VERSION
cp -r bundle/manifests $TMP_DIR/certified-operators-preprod/operators/redhat-marketplace-operator/$VERSION/
cp -r bundle/metadata $TMP_DIR/certified-operators-preprod/operators/redhat-marketplace-operator/$VERSION/

# remove sa duplicated in csv?
rm -Rf  $TMP_DIR/certified-operators-preprod/operators/redhat-marketplace-operator/$VERSION/manifests/redhat-marketplace-operator_v1_serviceaccount.yaml

echo "organization: redhat-marketplace" > $TMP_DIR/certified-operators-preprod/config.yaml
echo "cert_project_id: 5f68c9457115dbd1183ccab6" > $TMP_DIR/certified-operators-preprod/operators/redhat-marketplace-operator/ci.yaml

cd $TMP_DIR
curl -L https://github.com/mikefarah/yq/releases/download/v4.13.5/yq_linux_amd64.tar.gz | tar -xz 
./yq_linux_amd64 eval '
  .annotations."com.redhat.openshift.versions" = "v4.6-v4.9"
' -i $TMP_DIR/certified-operators-preprod/operators/redhat-marketplace-operator/$VERSION/metadata/annotations.yaml

cd $TMP_DIR/certified-operators-preprod
git add --all
git commit -m $VERSION
git push


# Run the Pipeline

cd $TMP_DIR/operator-pipelines
curl https://mirror.openshift.com/pub/openshift-v4/clients/pipeline/0.19.1/tkn-linux-amd64-0.19.1.tar.gz | tar -xz 

GIT_REPO_URL=https://github.com/redhat-marketplace/certified-operators-preprod.git
BUNDLE_PATH=operators/redhat-marketplace-operator/$VERSION

./tkn pipeline start operator-ci-pipeline \
  --param git_repo_url=$GIT_REPO_URL \
  --param git_branch=stage \
  --param bundle_path=$BUNDLE_PATH \
  --param env=stage \
  --workspace name=pipeline,volumeClaimTemplateFile=templates/workspace-template.yml \
  --workspace name=kubeconfig,secret=kubeconfig \
  --showlog