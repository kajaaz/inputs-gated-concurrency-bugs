#!/bin/sh
# Clones free5gc/bsf at the given tag and places the harness inside the
# module, where it may import the internal packages.
set -eu
tag=${1:-v1.0.1}
dir=${2:-bsf-$tag}
here=$(cd "$(dirname "$0")" && pwd)

git -c advice.detachedHead=false clone --quiet --branch "$tag" https://github.com/free5gc/bsf "$dir"
mkdir -p "$dir/cmd/bombharness"
cp "$here/harness/main.go" "$dir/cmd/bombharness/main.go"
echo "Build with: cd $dir && go build -race -o free5gc-bsf-put ./cmd/bombharness"
