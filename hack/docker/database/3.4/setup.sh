#!/bin/bash

set -e
set -o errexit
set -o nounset
set -o pipefail

GOPATH=$(go env GOPATH)
REPO_ROOT=$GOPATH/src/github.com/kubedb/mongodb

source "$REPO_ROOT/hack/libbuild/common/kubedb_image.sh"

DOCKER_REGISTRY=${DOCKER_REGISTRY:-kubedb}
TAG=3.4

docker_names=( \
    "mongo" \
    "mongo-tools" \
)

build() {
    pushd "$REPO_ROOT/hack/docker/database/$TAG"
    for NAME in "${docker_names[@]}"
    do
        echo "Building $DOCKER_REGISTRY/$NAME:$TAG"
        cd $NAME
        docker build -t "$DOCKER_REGISTRY/$NAME:$TAG" .
        cd ..
        echo
    done
    popd
}

docker_push() {
    for NAME in "${docker_names[@]}"
    do
        docker push "$DOCKER_REGISTRY/$NAME:$TAG"
    done
}

docker_release() {
    for NAME in "${docker_names[@]}"
    do
        docker push "$DOCKER_REGISTRY/$NAME:$TAG"
    done
}

binary_repo $@
