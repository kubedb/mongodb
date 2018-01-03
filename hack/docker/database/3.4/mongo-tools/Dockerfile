#####################################
#      Image for database dump      #
#  Image: kubedb/mongodb:util         #
#####################################

FROM appscode/python:2.7

ENV MONGO_MAJOR 3.4
ENV MONGO_VERSION 3.4.10
## osm version
ENV OSM_VERSION 0.6.2

RUN groupadd -r mongodb && useradd -r -g mongodb mongodb
RUN apt-key adv --keyserver ha.pool.sks-keyservers.net --recv-keys 0C49F3730359A14518585931BC711F9BA15703C6

ARG MONGO_PACKAGE=mongodb-org
ARG MONGO_REPO=repo.mongodb.org
ENV MONGO_PACKAGE=${MONGO_PACKAGE} MONGO_REPO=${MONGO_REPO}



RUN echo "deb http://$MONGO_REPO/apt/debian jessie/${MONGO_PACKAGE%-unstable}/$MONGO_MAJOR main" | tee "/etc/apt/sources.list.d/${MONGO_PACKAGE%-unstable}.list"


RUN set -x \
  && apt-get update \
  && apt-get install -y curl ca-certificates netcat \
  && apt-get install -y --no-install-recommends  mongodb-org-tools --force-yes \
  && pip install --upgrade pip \
  && pip install pyyaml \
  && rm -rf /var/lib/apt/lists/* /usr/share/doc /usr/share/man /tmp/*

RUN set -x \
  && curl -L "https://cdn.appscode.com/binaries/osm/$OSM_VERSION/osm-linux-amd64" -o /usr/local/bin/osm \
  && chmod +x /usr/local/bin/osm

COPY execute.py /execute.py
COPY utils.sh /utils.sh
RUN chmod +x /utils.sh

ENTRYPOINT ["python2.7", "execute.py"]
