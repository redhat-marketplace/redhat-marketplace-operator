#/bin/sh

set -x

CDIR=`dirname $0`
CDIR=`cd ${CDIR};pwd -P`

BASES_DIR="$CDIR/bases"

CSV_FILE="$BASES_DIR/redhat-marketplace-operator.clusterserviceversion.yaml"

echo "Patching csv file $METRIC_STATE_IMAGE"
${KUSTOMIZE} build $BASES_DIR -o $CSV_FILE

for param in $@
do
 eval "export $param"
done 

# replace rhmp images
sed "s%METRIC_STATE_IMAGE_MARKER%${METRIC_STATE_IMAGE}%g;s%REPORTER_IMAGE_MARKER%${REPORTER_IMAGE}%g;s%AUTHCHECK_IMAGE_MARKER%${AUTHCHECK_IMAGE}%g" $CSV_FILE > $CSV_FILE.$$
mv $CSV_FILE.$$ $CSV_FILE
