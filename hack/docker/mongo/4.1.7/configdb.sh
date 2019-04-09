#!/bin/bash

# ref: https://github.com/kubernetes/charts/blob/master/stable/mongodb-replicaset/init/on-start.sh

replica_set="$REPLICA_SET"
script_name=${0##*/}

if [[ "$AUTH" == "true" ]]; then
  admin_user="$MONGO_INITDB_ROOT_USERNAME"
  admin_password="$MONGO_INITDB_ROOT_PASSWORD"
  admin_creds=(-u "$admin_user" -p "$admin_password")
  auth_args=(--auth --keyFile=/data/configdb/key.txt)
fi

function log() {
  local msg="$1"
  local timestamp
  timestamp=$(date --iso-8601=ns)
  echo "[$timestamp] [$script_name] $msg" | tee -a /work-dir/log.txt
}

function shutdown_mongo() {
  if [[ $# -eq 1 ]]; then
    args="timeoutSecs: $1"
  else
    args='force: true'
  fi
  log "Shutting down MongoDB ($args)..."
  mongo admin "${admin_creds[@]}" --eval "db.shutdownServer({$args})"
}

my_hostname=$(hostname)
log "Bootstrapping MongoDB replica set member: $my_hostname"

log "Reading standard input..."
while read -ra line; do
  if [[ "${line}" == *"${my_hostname}"* ]]; then
    service_name="$line"
    continue
  fi
  peers=("${peers[@]}" "$line")
done

log "Peers: ${peers[*]}"

log "Starting a MongoDB instance..."
mongod --config /data/configdb/mongod.conf --dbpath=/data/db --configsvr --replSet="$replica_set" --port=27017 "${auth_args[@]}" --bind_ip=0.0.0.0 >>/work-dir/log.txt 2>&1 &

log "Waiting for MongoDB to be ready..."
until mongo --eval "db.adminCommand('ping')"; do
  log "Retrying..."
  sleep 2
done

log "Initialized."

# try to find a master and add yourself to its replica set.
for peer in "${peers[@]}"; do
  if mongo admin --host "$peer" "${admin_creds[@]}" --eval "rs.isMaster()" | grep '"ismaster" : true'; then
    log "Found master: $peer"
    log "Adding myself ($service_name) to replica set..."
    mongo admin --host "$peer" "${admin_creds[@]}" --eval "rs.add('$service_name')"

    sleep 3

    log 'Waiting for replica to reach SECONDARY state...'
    until printf '.' && [[ $(mongo admin "${admin_creds[@]}" --quiet --eval "rs.status().myState") == '2' ]]; do
      sleep 1
    done

    log '✓ Replica reached SECONDARY state.'

    shutdown_mongo "60"
    log "Good bye."
    exit 0
  fi
done

# else initiate a replica set with yourself.
if mongo --eval "rs.status()" | grep "no replset config has been received"; then
  log "Initiating a new replica set with myself ($service_name)..."
  mongo --eval "rs.initiate({'_id': '$replica_set', 'members': [{'_id': 0, 'host': '$service_name'}]})"

  sleep 3

  log 'Waiting for replica to reach PRIMARY state...'
  until printf '.' && [[ $(mongo --quiet --eval "rs.status().myState") == '1' ]]; do
    sleep 1
  done

  log '✓ Replica reached PRIMARY state.'

  if [[ "$AUTH" == "true" ]]; then
    log "Creating admin user..."
    mongo admin --eval "db.createUser({user: '$admin_user', pwd: '$admin_password', roles: [{role: 'root', db: 'admin'}]})"
  fi

  log "Done."
fi

shutdown_mongo
log "Good bye."
