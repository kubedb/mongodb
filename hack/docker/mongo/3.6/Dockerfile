FROM mongo:3.6

COPY replicaset.sh /usr/local/bin/
COPY configdb.sh /usr/local/bin/
COPY sharding.sh /usr/local/bin/
COPY mongos.sh /usr/local/bin/
COPY peer-finder /usr/local/bin/

RUN chmod -c 755 /usr/local/bin/peer-finder \
 /usr/local/bin/replicaset.sh \
 /usr/local/bin/configdb.sh \
 /usr/local/bin/sharding.sh \
 /usr/local/bin/mongos.sh

# For starting mongodb container
# default entrypoint of parent mongo:3.6
# ENTRYPOINT ["docker-entrypoint.sh"]

# For starting bootstraper init container (for mongodb replicaset)
# ENTRYPOINT ["peer-finder"]
