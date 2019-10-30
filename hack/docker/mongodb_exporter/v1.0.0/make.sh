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
IMG=mongodb_exporter
TAG=v1.0.0

build() {
  pushd "$REPO_ROOT/hack/docker/mongodb_exporter/$TAG"

  # Download mongodb_exporter. github repo: https://github.com/dcu/mongodb_exporter
  # Prometheus Exporters link: https://prometheus.io/docs/instrumenting/exporters/
  wget -O mongodb_exporter https://github.com/dcu/mongodb_exporter/releases/download/$TAG/mongodb_exporter-linux-amd64
  chmod +x mongodb_exporter

  local cmd="docker build --pull -t $DOCKER_REGISTRY/$IMG:$TAG ."
  echo $cmd; $cmd

  rm mongodb_exporter
  popd
}

binary_repo $@
