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
RUN curl -LO https://raw.githubusercontent.com/karlkfi/concourse-dcind/master/entrypoint.sh && \
    chmod +x entrypoint.sh && \
    mv entrypoint.sh /bin/

ENTRYPOINT ["entrypoint.sh"]
