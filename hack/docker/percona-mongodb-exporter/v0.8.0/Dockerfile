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

# ref: https://github.com/percona/mongodb_exporter/blob/master/Dockerfile

FROM alpine as builder

WORKDIR /percona/

RUN wget https://github.com/percona/mongodb_exporter/releases/download/v0.8.0/mongodb_exporter-0.8.0.linux-amd64.tar.gz
RUN tar -zxvf mongodb_exporter-0.8.0.linux-amd64.tar.gz

FROM       alpine
EXPOSE     9216

RUN apk add --update ca-certificates
COPY --from=builder /percona/mongodb_exporter /bin/mongodb_exporter

ENTRYPOINT  [ "/bin/mongodb_exporter" ]
