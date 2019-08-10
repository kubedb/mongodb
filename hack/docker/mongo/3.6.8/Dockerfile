FROM mongo:3.6.8

COPY replicaset.sh /usr/local/bin/
COPY configdb.sh /usr/local/bin/
COPY sharding.sh /usr/local/bin/
COPY mongos.sh /usr/local/bin/
COPY peer-finder /usr/local/bin/

# Copy ssl-client-user to docker-entrypoint.d directory.
# xref: https://github.com/docker-library/mongo/issues/329#issuecomment-460858099
COPY 000-ssl-client-user.sh /docker-entrypoint-initdb.d/

RUN chmod -c 755 /usr/local/bin/peer-finder \
 /usr/local/bin/replicaset.sh \
 /usr/local/bin/configdb.sh \
 /usr/local/bin/sharding.sh \
 /usr/local/bin/mongos.sh

ENV SSL_MODE ""
ENV CLUSTER_AUTH_MODE ""

# For starting mongodb container
# default entrypoint of parent mongo:3.6.8
# ENTRYPOINT ["docker-entrypoint.sh"]

# For starting bootstraper init container (for mongodb replicaset)
# ENTRYPOINT ["peer-finder"]
