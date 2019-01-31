#!/bin/bash
set -xeou pipefail

DOCKER_REGISTRY=${DOCKER_REGISTRY:-kubedb}
IMG=mongo
DB_VERSION=4.0
TAG="$DB_VERSION"

IMG=mongo
DB_VERSION=4.0
PATCH=4.0.5

TAG="$DB_VERSION" # suffix will be added in later versions
BASE_TAG="$PATCH" # suffix will be added in later versions


docker pull "$DOCKER_REGISTRY/$IMG:$BASE_TAG"

docker tag "$DOCKER_REGISTRY/$IMG:$BASE_TAG" "$DOCKER_REGISTRY/$IMG:$TAG"
docker push "$DOCKER_REGISTRY/$IMG:$TAG"
