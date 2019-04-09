#!/bin/bash

set -eou pipefail

# ref: https://github.com/kubernetes/charts/blob/master/stable/mongodb-replicaset/init/on-start.sh

export CONFIGDB_REPSET=${CONFIGDB_REPSET:-}
export SHARD_REPSETS=${SHARD_REPSETS:-}
domain=$(awk -v s=search '{if($1 == s)print $3}' /etc/resolv.conf)
SHARD_REPSETS=${SHARD_REPSETS//svc/$domain} # replace svc with $domain. xref: https://stackoverflow.com/a/13210909/4628962
SHARD_REPSETS_LIST=(${SHARD_REPSETS// / })  # make array that splits by space. https://stackoverflow.com/a/15400047/4628962

# awk -v s=search '{if($1 == s)print $3}' /etc/resolv.conf

if [[ "$AUTH" == "true" ]]; then
  admin_user="$MONGO_INITDB_ROOT_USERNAME"
  admin_password="$MONGO_INITDB_ROOT_PASSWORD"
  admin_creds=(-u "$admin_user" -p "$admin_password")
  auth_args=(--keyFile=/data/configdb/key.txt)
fi

function log() {
  local msg="$1"
  local timestamp
  timestamp=$(date --iso-8601=ns)
  echo "[$timestamp] $msg" | tee -a /work-dir/log.txt
}

function shutdown_mongo() {
  if [[ $# -eq 1 ]]; then
    args="timeoutSecs: $1"
  else
    args='force: true'
  fi
  log "Shutting down mongos ($args)..."
  mongo admin "${admin_creds[@]}" --eval "db.shutdownServer({$args})"
}

log "Ping Config Server replicaset : $CONFIGDB_REPSET"
until mongo --quiet --host "$CONFIGDB_REPSET" "${admin_creds[@]}" --eval "db.adminCommand('ping')"; do
  sleep 1
  log "Ping to Config Server replicaset fails."
done

log "Check if Config Server primary node is UP!!"
until [[ $(mongo --quiet --host "$CONFIGDB_REPSET" "${admin_creds[@]}" --eval "rs.status().hasOwnProperty('myState') && rs.status().myState==1;" | tail -1) == true ]]; do
  log "Primary Node of Config Server replicaset is not up"
  sleep 1
done

log "Starting a mongos instance..."
mongos --config /data/configdb/mongod.conf --configdb="$CONFIGDB_REPSET" --port=27017 "${auth_args[@]}" --bind_ip=0.0.0.0 >>/work-dir/log.txt 2>&1 &

log "Waiting for mongos to be ready..."
until mongo --eval "db.adminCommand('ping')"; do
  log "Retrying..."
  sleep 2
done

log "Add shard instances"
total=${#SHARD_REPSETS_LIST[*]}

log "Shard list $total: ${SHARD_REPSETS_LIST[*]}"

for ((i = 0; i < $total; i++)); do
  repSet=${SHARD_REPSETS_LIST[$i]}
  log "Add shard: $repSet"
  mongo "${admin_creds[@]}" --eval "sh.addShard('$repSet');"
done

log "Ensure admin user credentials"
if [[ $(mongo admin "${admin_creds[@]}" --eval "db.system.users.find({user:'$admin_user'}).count()" | tail -1) == 0 ]]; then
  log "Creating admin user..."
  mongo admin "${ssl_args[@]}" --eval "db.createUser({user: '$admin_user', pwd: '$admin_password', roles: [{role: 'root', db: 'admin'}]})"
fi

# Initialize Part for KubeDB. ref: https://github.com/docker-library/mongo/blob/a499e81e743b05a5237e2fd700c0284b17d3d416/3.4/docker-entrypoint.sh#L302
# Start
log "Ensure Initializing init scripts"
if [[ $(mongo admin "${admin_creds[@]}" --eval "db.kubedb.find({'_id' : 'kubedb','kubedb' : 'initialized'}).count()" | tail -1) == 0 ]] &&
  [[ $(mongo admin "${ssl_args[@]}" "${admin_creds[@]}" --eval "db.kubedb.insert({'_id' : 'kubedb','kubedb' : 'initialized'});" |
    grep -c "E11000 duplicate key error collection: admin.kubedb") -eq 0 ]]; then

  export MONGO_INITDB_DATABASE="${MONGO_INITDB_DATABASE:-test}"
  log "Initialize init scripts"
  echo

  ls -la /docker-entrypoint-initdb.d
  for f in /docker-entrypoint-initdb.d/*; do
    case "$f" in
      *.sh)
        echo "$0: running $f"
        . "$f"
        ;;
      *.js)
        echo "$0: running $f 1"
        mongo "$MONGO_INITDB_DATABASE" "${admin_creds[@]}" --authenticationDatabase admin "$f"
        ;;
      *) echo "$0: ignoring $f" ;;
    esac
    echo
  done
  # END
fi

shutdown_mongo
log "Good bye."
