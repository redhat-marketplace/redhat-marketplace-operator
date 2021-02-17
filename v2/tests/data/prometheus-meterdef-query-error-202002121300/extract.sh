#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
pushd $DIR &> /dev/null

rm -rf etc prometheus
tar -xzf data.tar.gz

popd &> /dev/null
