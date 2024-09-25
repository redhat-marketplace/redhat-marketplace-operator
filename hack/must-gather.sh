#!/bin/bash
set -x

# IBM Metrics Operator / Data Reporter Operator Namespace
IMO_NAMESPACE="${IMO_NAMESPACE:-ibm-software-central}"

# IBM Licensing Service Namespace
LS_NAMESPACE="${LS_NAMESPACE:-ibm-licensing}"

# OpenShift Monitoring Namespace
OM_NAMESPACE="${OM_NAMESPACE:-openshift-monitoring}"

# OpenShift User Workload Monitoring Namespace
OUWM_NAMESPACE="${OUWM_NAMESPACE:-openshift-user-workload-monitoring}"

BASEPATH="${BASEPATH:-ibm-metrics-operator-must-gather}"


IMO_K8S_OBJS="pod serviceaccount persistentvolumeclaim deployment statefulset replicaset cronjob job configmap marketplaceconfig meterbase meterreport razeedeployment route servicemonitor subscription clusterserviceversion"

LS_K8S_OBJS="pod serviceaccount deployment replicaset configmap route servicemonitor"

OM_K8S_OBJS="pod serviceaccount persistentvolumeclaim deployment statefulset replicaset configmap route servicemonitor"


# Check which CLI tool is available

if which oc > /dev/null; then
    KCMD=oc
elif which kubectl > /dev/null; then
    KCMD=kubectl
else
    echo "no oc or kubectl in PATH."
    exit
fi

### Functions ###

# $1 BASEPATH
# $2 NAMESPACE
# $3 OBJS
getobjs () {
  local BASEPATH="$1"
  local NAMESPACE="$2"
  local OBJS=("$3")

    mkdir -p $BASEPATH/$NAMESPACE

    for OBJ in ${OBJS[@]}; do
        # Get 
        $KCMD -n $NAMESPACE get $OBJ > $BASEPATH/$NAMESPACE/$OBJ-get.txt

        # Describe
        $KCMD -n $NAMESPACE describe $OBJ > $BASEPATH/$NAMESPACE/$OBJ-describe.txt

        # Get YAML
        $KCMD -n $NAMESPACE get $OBJ -o=yaml > $BASEPATH/$NAMESPACE/$OBJ-get.yaml
    done

    # Pod Logs
    for POD in $($KCMD -n $NAMESPACE get pod --output=jsonpath={.items..metadata.name}); do $KCMD -n $NAMESPACE logs --all-containers=true $POD > $BASEPATH/$NAMESPACE/$POD.log ; done

    # Pod Previous Logs
    for POD in $($KCMD -n $NAMESPACE get pod --output=jsonpath={.items..metadata.name}); do $KCMD -n $NAMESPACE logs -p --all-containers=true $POD > $BASEPATH/$NAMESPACE/$POD-prev.log ; done
}



### IBM Metrics Operator ###

# Get namespaced object details
getobjs "${BASEPATH}" "${IMO_NAMESPACE}" "${IMO_K8S_OBJS}"

# List namespaced Secrets
$KCMD -n $IMO_NAMESPACE get secret > $BASEPATH/$IMO_NAMESPACE/secret-get.txt

# All MeterDefinitions
$KCMD get meterdefinitions --all-namespaces -o=yaml > $BASEPATH/meterdefinitions-get.yaml

# Prometheus metric-state data
set +x
TOKEN=$($KCMD whoami -t) && \
HOST=$($KCMD -n $OM_NAMESPACE get route thanos-querier -ojsonpath={.spec.host}) && \
curl -H "Authorization: Bearer $TOKEN" -k "https://$HOST/api/v1/query?" --data-urlencode 'query=meterdef_metric_label_info' > $BASEPATH/$IMO_NAMESPACE/prom-metric-state.txt
set -x


### IBM Licensing Service ###

# Get namespaced object details
getobjs "${BASEPATH}" "${LS_NAMESPACE}" "${LS_K8S_OBJS}"

# Prometheus licensing-service product_license_usage data
set +x
TOKEN=$($KCMD whoami -t) && \
HOST=$($KCMD -n $OM_NAMESPACE get route thanos-querier -ojsonpath={.spec.host}) && \
curl -H "Authorization: Bearer $TOKEN" -k "https://$HOST/api/v1/query?" --data-urlencode 'query=avg_over_time(product_license_usage{}[1d])' > $BASEPATH/$LS_NAMESPACE/prom-licensing-service.txt
set -x


### Openshift Monitoring ###

# Get namespaced object details
getobjs "${BASEPATH}" "${OM_NAMESPACE}" "${OM_K8S_OBJS}"

# Active monitoring targets
set +x
TOKEN=$($KCMD whoami -t) && \
HOST=$($KCMD -n $OM_NAMESPACE get route thanos-querier -ojsonpath={.spec.host}) && \ 
curl -H "Authorization: Bearer $TOKEN" -k "https://$HOST/api/v1/targets?state=active" > $BASEPATH/$OM_NAMESPACE/active-targets.log
set -x

### Openshift User Workload Monitoring ###

# Get namespaced object details
getobjs "${BASEPATH}" "${OUWM_NAMESPACE}" "${OM_K8S_OBJS}"


### Archive ###
tar -czvf $BASEPATH.tar.gz $BASEPATH