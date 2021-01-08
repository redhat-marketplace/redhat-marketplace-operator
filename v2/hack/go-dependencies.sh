#!/usr/bin/env bash
set -euo pipefail

cd $1

go list -f '{{$d := .Dir}}{{range $f := .GoFiles}}{{printf "%s/%s\n" $d $f}}{{end}}{{range $f := .CgoFiles}}{{printf "%s/%s\n" $d $f}}{{end}}' ./... | xargs | uniq | xargs jq -Menc '$ARGS.positional' --args
