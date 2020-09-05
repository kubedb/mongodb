#!/usr/bin/env bash

# Copyright AppsCode Inc. and Contributors
#
# Licensed under the AppsCode Community License 1.0.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.





set -x

# uninstall operator
./hack/deploy/setup.sh --uninstall --purge
./hack/deploy/setup.sh --uninstall --purge

# remove creds
rm -rf /gcs.json
rm -rf hack/config/.env

# remove docker images
source "hack/libbuild/common/lib.sh"
detect_tag ''

# delete docker image on exit
./hack/libbuild/docker.py del_tag $DOCKER_REGISTRY mg-operator $TAG
