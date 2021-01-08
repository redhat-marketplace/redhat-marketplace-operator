#!/usr/bin/env bash

if [[ ${#@} -lt 3 ]]; then
    echo "Usage: $0 semver chart values"
    echo "* semver: semver-formatted version for this package"
    echo "* chart: the directory to output the chart"
    echo "* values: the values file"
    exit 1
fi

version=$1
chartdir=$2
values=$3
shift 3
extra=$@

charttmpdir=`mktemp -d 2>/dev/null || mktemp -d -t 'charttmpdir'`
charttmpdir=${charttmpdir}/chart
rolesyaml=${charttmpdir}/role-values.yaml

cp -R deploy/chart/ ${charttmpdir}
echo "version: $version" >> ${charttmpdir}/Chart.yaml

#go run yq r ./deploy/role.yaml rules > ${rolesyaml}
#go run yq p -i ${rolesyaml} operator.rules

mkdir -p ${chartdir}

helm template \
    ${charttmpdir} \
    -f ${values} \
    --output-dir ${charttmpdir} \
    ${extra}

cp -R ${charttmpdir}/redhat-marketplace-operator-template-chart/templates/* ${chartdir}
echo "Copying from ${charttmpdir} to ${chartdir}"
