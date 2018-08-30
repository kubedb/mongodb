#!/bin/bash
set -eou pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT=$GOPATH/src/github.com/kubedb/mongodb

source "$REPO_ROOT/hack/libbuild/common/lib.sh"
source "$REPO_ROOT/hack/libbuild/common/kubedb_image.sh"

DOCKER_REGISTRY=${DOCKER_REGISTRY:-kubedb}
IMG=mongodb_exporter
TAG=v1.0.0

build() {
  pushd "$REPO_ROOT/hack/docker/mongodb_exporter/$TAG"
  local cmd="docker build -t $DOCKER_REGISTRY/$IMG:$TAG ."
  echo $cmd; $cmd

  popd
}

binary_repo $@


#MONGODB_EXPORTER_VER=${MONGODB_EXPORTER_VER:-SOURCE}
#MONGODB_EXPORTER_BRANCH=${MONGODB_EXPORTER_BRANCH:-master}
#
#DIST=$REPO_ROOT/dist
#mkdir -p $DIST
#
#build_binary() {
#  if [ $MONGODB_EXPORTER_VER = 'SOURCE' ]; then
#    rm -rf $DIST/mongodb_exporter
#    cd $DIST
#    git clone https://github.com/dcu/mongodb_exporter.git
#    cd mongodb_exporter
#    checkout $MONGODB_EXPORTER_BRANCH
#    glide install
#    echo "Build binary using golang docker image"
#    docker run --rm -ti \
#      -v $(pwd):/go/src/github.com/dcu/mongodb_exporter \
#      -w /go/src/github.com/dcu/mongodb_exporter golang:1.9-alpine make release
#    mv mongodb_exporter $DIST/mongodb_exporter-bin
#    ls -la # delete
#   # rm -rf *
#    mv $DIST/mongodb_exporter-bin $DIST/dcu/mongodb_exporter
#  else
#    # Download mongodb_exporter
#    rm -rf $DIST/mongodb_exporter
#    mkdir $DIST/mongodb_exporter
#    cd $DIST/mongodb_exporter
#   wget -O mongodb_exporter https://github.com/dcu/mongodb_exporter/releases/download/$TAG/mongodb_exporter-linux-amd64
#   chmod +x mongodb_exporter
#  fi
#}
#
#build_docker() {
#  pushd $REPO_ROOT/hack/docker
#
#  # Download mongodb_exporter
#  cp $DIST/stash/stash-alpine-amd64 stash
#  chmod 755 stash
#
#  cp $DIST/dcu/mongodb_exporter mongodb_exporter
#  chmod 755 mongodb_exporter
#
#  cat >Dockerfile <<EOL
#FROM busybox
#COPY mongodb_exporter /bin/mongodb_exporter
#EXPOSE 9001
#ENTRYPOINT  [ "/bin/mongodb_exporter" ]
#EOL
#  local cmd="docker build -t $DOCKER_REGISTRY/$IMG:$TAG ."
#  echo $cmd
#  $cmd
#
#  rm stash Dockerfile mongodb_exporter
#  popd
#}
#
#build() {
#  build_binary
#  build_docker
#}
