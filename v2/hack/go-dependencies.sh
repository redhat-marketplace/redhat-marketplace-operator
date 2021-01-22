#!/usr/bin/env bash
set -euo pipefail

path=$1
toroot=$2

cd $path

files=$(go list -f '{{$d := .Dir}}{{range $f := .GoFiles}}{{printf "%s/%s\n" $d $f}}{{end}}{{range $f := .CgoFiles}}{{printf "%s/%s\n" $d $f}}{{end}}' ./... \
    | xargs \
    | uniq \
    | xargs jq --arg pwd `pwd` --arg path $path -Menc '[$ARGS.positional | .[] | gsub($pwd;$path)]' --args)

github_url="github.com/redhat-marketplace/redhat-marketplace-operator/"

dependencies=$(go list -f '{{range $v := .Imports}}{{printf "%s \n" $v}}{{end}}'  ./... | grep "github.com/redhat-marketplace/redhat-marketplace-operator" \
    | xargs \
    | uniq \
    | xargs jq --arg github "$github_url" --arg path "$path/$toroot" -Menc '[$ARGS.positional | .[] | gsub($github;$path + "/") + "/*.go"]' --args)

jq --argjson arr1 "$files" --argjson arr2 "$dependencies" -n -r -c '$arr1 + $arr2'
