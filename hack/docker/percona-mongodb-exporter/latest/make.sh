#!/bin/bash

# Copyright The KubeDB Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eou pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT=$GOPATH/src/kubedb.dev/mongodb

source "$REPO_ROOT/hack/libbuild/common/lib.sh"
source "$REPO_ROOT/hack/libbuild/common/kubedb_image.sh"

DOCKER_REGISTRY=${DOCKER_REGISTRY:-kubedb}
IMG=percona-mongodb-exporter
IMG_REGISTRY=percona
IMG_REPOSITORY=mongodb_exporter
TAG=latest

# Take 1st 8 letters of hash as a shorten hash. Get hash without cloning: https://stackoverflow.com/a/24750310/4628962
COMMIT_HASH=`git ls-remote https://github.com/${IMG_REGISTRY}/${IMG_REPOSITORY}.git | grep HEAD | awk '{ print substr($1,1,8)}'`

build() {
  pushd "$REPO_ROOT/hack/docker/percona-mongodb-exporter/$TAG"

  local cmd="docker build --pull -t $DOCKER_REGISTRY/$IMG:$COMMIT_HASH ."
  echo $cmd; $cmd

  local cmd="docker tag $DOCKER_REGISTRY/$IMG:$COMMIT_HASH $DOCKER_REGISTRY/$IMG:$TAG"
  echo $cmd; $cmd

  popd
}

docker_push() {
  local cmd="docker push $DOCKER_REGISTRY/$IMG:$COMMIT_HASH"
  echo $cmd; $cmd

  local cmd="docker push $DOCKER_REGISTRY/$IMG:$TAG"
  echo $cmd; $cmd
}

binary_repo $@
