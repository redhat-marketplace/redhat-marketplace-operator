#!/usr/bin/env bash
set -euo pipefail

results=$1

pushed=$(echo "$results" | jq '[. | length == 12] | first')
table="No images pushed"

if [ "$pushed" = "true" ] ; then
  table=$(cat <<EOF
| Image | Certification Status | Publish Status |
|:--:|:--:|:--:|
$(echo "$results" | jq -r '[.[] | "|[" + .name + ":" + .tags[0] + "](" + .url + ")|" + .certification_status + "|" + .publish_status + "|"] | join("\n")' 2> /dev/null)
EOF
)
  all_published=$(echo "$results" | jq -r '[.[] | select(.publish_status != "Published")] | length == 0' 2> /dev/null)
  all_passed=$(echo $results | jq -r '[.[] | select(.certification_status != "Passed")] | length == 0' 2> /dev/null)
fi

#echo "::set-output name=all_published::${all_published:-false}"
#echo "::set-output name=all_passed::${all_passed:-false}"
#echo "::set-output name=pushed::${pushed:-false}"
echo "all_published=${all_published:-false}" >> $GITHUB_OUTPUT
echo "all_passed=${all_passed:-false}" >> $GITHUB_OUTPUT
echo "name=pushed=${pushed:-false}" >> $GITHUB_OUTPUT

echo "MD_TABLE<<EOF" >> $GITHUB_ENV
echo "$table" >> $GITHUB_ENV
echo "EOF" >> $GITHUB_ENV
