#!/bin/bash


oc apply -f datareporterconfig.yaml

oc apply -f ../role/role.yaml

NAMESPACE=$(oc config view --minify -o jsonpath='{..namespace}')
DRHOST=$(oc get route ibm-data-reporter --template='{{ .spec.host }}')

# Create set of ServiceAccounts, SA Secrets, Bindings
for i in {1..5}
do

oc apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ibm-data-reporter-operator-api-$i
  labels:
    redhat.marketplace.com/name: ibm-data-reporter-operator
---
apiVersion: v1
kind: Secret
metadata:
  name: ibm-data-reporter-operator-api-$i
  labels:
    name: ibm-data-reporter-operator
  annotations:
    kubernetes.io/service-account.name: ibm-data-reporter-operator-api-$i
type: kubernetes.io/service-account-token
EOF

  oc create clusterrolebinding ibm-data-reporter-operator-api-$i --clusterrole=ibm-data-reporter-operator-api --serviceaccount=${NAMESPACE}:ibm-data-reporter-operator-api-$i
  oc label clusterrolebinding/ibm-data-reporter-operator-api redhat.marketplace.com/name=ibm-data-reporter-operator
done


# For each user, send 10 events
for (( n=1 ; ; n++ ))
do
    for i in {1..5}
    do
        DRTOKEN=$(oc get secret/ibm-data-reporter-operator-api-$i -o jsonpath='{.data.token}' | base64 --decode)
        for j in {1..10}
          do
          EVENT='{"event":"n-'$n'-apikey'$i'"}'
          echo $EVENT
          curl -k -H "Authorization: Bearer ${DRTOKEN}" -X POST -d "${EVENT}" https://${DRHOST}/v1/event &
        done
    done
    wait
done





