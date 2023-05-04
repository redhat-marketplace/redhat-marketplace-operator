#!/bin/bash

for i in {1..10}
do
    oc create secret generic ibm-data-reporter-secret-$i --from-literal=X-API-KEY=apikey$i
done

oc apply -f datareporterconfig.yaml

oc apply -f ../role/role.yaml

NAMESPACE=$(oc config view --minify -o jsonpath='{..namespace}')
oc create clusterrolebinding ibm-data-reporter-operator-api --clusterrole=ibm-data-reporter-operator-api --serviceaccount=${NAMESPACE}:ibm-data-reporter-operator-api
oc label clusterrolebinding/ibm-data-reporter-operator-api redhat.marketplace.com/name=ibm-data-reporter-operator


DRHOST=$(oc get route ibm-data-reporter --template='{{ .spec.host }}')
DRTOKEN=$(oc get secret/ibm-data-reporter-operator-api -o jsonpath='{.data.token}' | base64 --decode)
curl -k -H "Authorization: Bearer ${DRTOKEN}" https://${DRHOST}/v1/status 


for (( n=1 ; ; n++ ))
do
    for i in {1..10}
    do
        EVENT='{"event":"n-'$n'-apikey'$i'"}'
        echo $EVENT
        curl -k -H "Authorization: Bearer ${DRTOKEN}" -H "X-API-KEY: apikey$i" -X POST -d "${EVENT}" https://${DRHOST}/v1/event &
    done
    wait
done





