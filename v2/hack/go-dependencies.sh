#!/usr/bin/env bash
set -euo pipefail

path=$1
cd $path

go list -f '{{$d := .Dir}}{{range $f := .GoFiles}}{{printf "%s/%s\n" $d $f}}{{end}}{{range $f := .CgoFiles}}{{printf "%s/%s\n" $d $f}}{{end}}' ./... \
    | xargs \
    | uniq \
    | xargs jq --arg pwd `pwd` --arg path $path -Menc '[$ARGS.positional | .[] | gsub($pwd;$path)]' --args
