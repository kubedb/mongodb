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

#https://github.com/karlkfi/concourse-dcind
FROM golang:1.10-stretch

ENV DOCKER_CHANNEL=stable \
    DOCKER_VERSION=18.03.1-ce

# Install Docker, Docker Compose, Docker Squash
RUN apt-get update && apt-get install -y \
    curl \
    python-pip \
    libdevmapper-dev \
    iptables \
    ca-certificates \
    && \
    curl -fL "https://download.docker.com/linux/static/${DOCKER_CHANNEL}/x86_64/docker-${DOCKER_VERSION}.tgz" | tar zx && \
    mv docker/* /bin/ && chmod +x /bin/docker* && \
    rm -rf /var/cache/apt/* && \
    rm -rf /root/.cache

COPY entrypoint.sh /bin/entrypoint.sh
RUN curl -LO https://gist.githubusercontent.com/tahsinrahman/db0626153488f88ceac544404f33cc0f/raw/f9ba010b5dd194dbbf96f1431e5d8a46966ed79a/entrypoint.sh && \
    chmod +x entrypoint.sh && \
    mv entrypoint.sh /bin/

ENTRYPOINT ["entrypoint.sh"]
