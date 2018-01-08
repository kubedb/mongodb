#!/bin/bash
set -xeou pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT=$GOPATH/src/github.com/kubedb/mongodb
source "$REPO_ROOT/hack/libbuild/common/lib.sh"
source "$REPO_ROOT/hack/libbuild/common/kubedb_image.sh"

IMG=mongo-tools
TAG=3.4

pushd "$REPO_ROOT/hack/docker/mongo-tools/$TAG"

binary_repo $@
