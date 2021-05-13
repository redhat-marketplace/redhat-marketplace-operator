#/bin/sh

#set -x

CDIR=`dirname $0`
CDIR=`cd ${CDIR};pwd -P`

CSV_FILE="$CDIR/redhat-marketplace-operator.clusterserviceversion.yaml"
BUNDLE_FILE="$CDIR/../../../bundle/manifests/redhat-marketplace-operator.clusterserviceversion.yaml"
PATCH_FILE="$CDIR/patch-related-images.yaml"

# export passed cli arguments as variables
for param in $@
do
 eval "export $param"
done 

# make backup of patch file and update version
/bin/cp -f $PATCH_FILE $PATCH_FILE.bak
sed "s/name: redhat-marketplace-operator.v.*/name: redhat-marketplace-operator.v${VERSION}/g" $PATCH_FILE > $PATCH_FILE.$$
/bin/mv -f $PATCH_FILE.$$ $PATCH_FILE

# update version in patch file

# copy csv file to cdir before patching
/bin/cp -f $BUNDLE_FILE $CSV_FILE

echo "Patching csv file"
${KUSTOMIZE} build $CDIR -o $CSV_FILE

# replace rhmp related images
sed "s%METRIC_STATE_IMAGE_MARKER%${METRIC_STATE_IMAGE}%g;s%REPORTER_IMAGE_MARKER%${REPORTER_IMAGE}%g;s%AUTHCHECK_IMAGE_MARKER%${AUTHCHECK_IMAGE}%g" $CSV_FILE > $CSV_FILE.$$
/bin/mv -f $CSV_FILE.$$ $BUNDLE_FILE

# remove csv file
/bin/rm -f $CSV_FILE

# restore patch from backup
/bin/mv -f $PATCH_FILE.bak $PATCH_FILE
