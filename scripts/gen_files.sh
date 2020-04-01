#!/usr/bin/env bash

if [ ! -x "$(command -v gucci)" ]; then
    echo "gucci doesn't exist, installing it"
    go get github.com/noqcks/gucci
fi

gucci ./deploy/operator.yaml.tpl > ./deploy/operator.yaml
gucci ./deploy/role_binding.yaml.tpl > ./deploy/role_binding.yaml
