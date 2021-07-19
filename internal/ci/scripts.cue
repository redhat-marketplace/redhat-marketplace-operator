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
