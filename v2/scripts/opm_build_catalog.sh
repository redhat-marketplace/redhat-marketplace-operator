#!/usr/bin/env bash

fail_exit() 
{
  echo "ERROR: $1"
  [ -n "$2" ] && exit $2
  exit 1
}

# main code starts here

if [ $# -ne 2 ];
then
  echo "Usage: $0 <bundle image> <path to README file>"
  exit 1
fi

BUNDLE_IMAGE=$1
README_PATH=$2

echo "Running with $*"

OPERATOR_NAME=`echo $BUNDLE_IMAGE | rev | cut -f 1 -d '/' | rev | cut -f 1 -d ':' | sed -e 's/-manifest//'`  
OPERATOR_VERSION=`echo $BUNDLE_IMAGE | cut -f 2 -d ':'` 

echo "Operator name: $OPERATOR_NAME"
echo "Operator version: $OPERATOR_VERSION"

# check if opm tool is available

$OPM &>/dev/null
[ $? -ne 0 ] && fail_exit "opm tool not installed"

catalog_dir="catalog-$OPERATOR_NAME"

echo "Creating catalog directory $catalog_dir"
mkdir -p "$catalog_dir"
[ $? -ne 0 ] && fail_exit "Unable to create catalog directory"

echo "Generate Dockerfile"
$OPM generate dockerfile "$catalog_dir" -i registry.redhat.io/openshift4/ose-operator-registry:v4.14

echo "Populate catalog"
$OPM init ibm-metrics-operator --default-channel=stable --description=$README_PATH  --output yaml > "$catalog_dir"/index.yaml 

echo "Adding bundle"
$OPM render $BUNDLE_IMAGE --output=yaml >> "$catalog_dir"/index.yaml 

cat <<EOT >> "$catalog_dir"/index.yaml
---
schema: olm.channel
package: $OPERATOR_NAME
name: stable
entries:
  - name: $OPERATOR_NAME.v$OPERATOR_VERSION
EOT

echo "Validate catalog"
$OPM validate "$catalog_dir"
[ $? -ne 0 ] && fail_exit "Catalog validation failed"

catalog_image=`echo $BUNDLE_IMAGE | sed -e 's/manifest/catalog/'`
echo "Builing catalog image $catalog_image"
docker build -f $catalog_dir.Dockerfile -t $catalog_image .
[ $? -ne 0 ] && fail_exit "Image build failed"

echo "Pushing catalog image $catalog_image"
docker push $catalog_image
[ $? -ne 0 ] && fail_exit "Image push failed"

echo "Removing catalog dir $catalog_dir"
rm -rf "$catalog_dir"