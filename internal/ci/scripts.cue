package ci

import (
	"strings"
)

scripts: [...{file: string, script: #Script & {}}]
scripts: [
	{
		file:   "scripts/get_images.sh"
		script: getImagesScript
	},
  {
    file: "scripts/scan_images.sh"
    script: scanImagesScript
  },
]

#Script: {
	result: string
}

getImagesScript: #Script & {
	result: """
TAG=$1
IMAGES=""

\(strings.Join([ for k, v in _#images {(_#defineImage & {#args: image: v}).res}], "\n\n"))

IMAGES="$IMAGES --images \(_#manifest.url),,^$TAG(-\\d+)*(-cert-\\d+)*$"

echo $IMAGES
"""
}


scanImagesScript: #Script & {
  result: """
if [[ -z "${TAG}" ]]; then
  echo "TAG isn't set"
  exit 1
fi

if [[ -z "${REDHAT_TOKEN}" ]]; then
  echo "REDHAT_TOKEN isn't set"
  exit 1
fi

\(_#scanCommand.res.#shell)
"""
}
