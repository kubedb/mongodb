#!/bin/bash

# ref: https://stackoverflow.com/a/7069755/244009
# ref: https://jonalmeida.com/posts/2013/05/26/different-ways-to-implement-flags-in-bash/
# ref: http://tldp.org/LDP/abs/html/comparison-ops.html

show_help() {
    echo "docker-entrypoint.sh - run tools"
    echo " "
    echo "docker-entrypoint.sh COMMAND [options]"
    echo " "
    echo "options:"
    echo "-h, --help                         show brief help"
    echo "    --host=HOST                    database host"
    echo "    --user=USERNAME                database username"
    echo "    --bucket=BUCKET                name of bucket"
    echo "    --folder=FOLDER                name of folder in bucket"
    echo "    --snapshot=SNAPSHOT            name of snapshot"
}

RETVAL=0
DEBUG=${DEBUG:-}

MONGO_HOST=${MONGO_HOST:-}
MONGO_USER=${MONGO_USER:-}
MONGO_PASSWORD=${MONGO_PASSWORD:-}
MONGO_BUCKET=${MONGO_BUCKET:-}
MONGO_FOLDER=${MONGO_FOLDER:-}
MONGO_SNAPSHOT=${MONGO_SNAPSHOT:-}

cmd=$1
shift

while test $# -gt 0; do
    case "$1" in
        -h|--help)
            show_help
            exit 0
            ;;
        --host*)
            export MONGO_HOST=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        --user*)
            export MONGO_USER=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        --bucket*)
            export MONGO_BUCKET=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        --folder*)
            export MONGO_FOLDER=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        --snapshot*)
            export MONGO_SNAPSHOT=`echo $1 | sed -e 's/^[^=]*=//g'`
            shift
            ;;
        *)
            show_help
            exit 1
            ;;
    esac
done

if [ -n "$DEBUG" ]
    env | sort | grep MONGO_*
    echo ""
fi

try_until_success() {
    $1
    while [ $? -ne 0 ]; do
        sleep 2
        $1
    done
}

backup() {
    # 1 - host
    # 2 - username
    # 3 - password

    path=/var/dump-backup
    mkdir -p "$path"
    cd "$path"
    rm -rf "$path"/*

    # Wait for mongodb to start
    # ref: http://unix.stackexchange.com/a/5279
    while ! nc -q 1 $1 27017 </dev/null; do echo "Waiting... Master pod is not ready yet"; sleep 5; done

    mongodump --host $1 --port 27017 --username $2 --password "$3" --out $path
    retval=$?
    if [ "$retval" -ne 0 ]; then
        echo "Fail to take backup"
        exit 1
    fi
    exit 0
}

restore() {
    # 1 - Host
    # 2 - username
    # 3 - password

    path=/var/dump-restore/
    mkdir -p "$path"
    cd "$path"

    # Wait for mongodb to start
    # ref: http://unix.stackexchange.com/a/5279
    while ! nc -q 1 $1 27017 </dev/null; do echo "Waiting... Master pod is not ready yet"; sleep 5; done

    mongorestore --host $1 --port 27017 --username $2 --password "$3"  $path
    retval=$?
    if [ "$retval" -ne 0 ]; then
        echo "Fail to restore"
        exit 1
    fi
    exit 0
}

push() {
    # 1 - bucket
    # 2 - folder
    # 3 - snapshot-name

    src_path=/var/dump-backup
    osm push --osmconfig=/etc/osm/config -c "$1" "$src_path" "$2/$3"
    retval=$?
    if [ "$retval" -ne 0 ]; then
        echo "Fail to push data to cloud"
        exit 1
    fi

    exit 0
}

pull() {
    # 1 - bucket
    # 2 - folder
    # 3 - snapshot-name

    dst_path=/var/dump-restore/
    mkdir -p "$dst_path"
    rm -rf "$dst_path"

    osm pull --osmconfig=/etc/osm/config -c "$1" "$2/$3" "$dst_path"
    retval=$?
    if [ "$retval" -ne 0 ]; then
        echo "Fail to pull data from cloud"
        exit 1
    fi

    exit 0
}

case "$cmd" in
    backup)
        try_until_success "backup $MONGO_HOST $MONGO_USER $MONGO_PASSWORD"
        push "$MONGO_BUCKET" "$MONGO_FOLDER" "$MONGO_SNAPSHOT"
        ;;
    restore)
        pull "$MONGO_HOST" "$MONGO_USER" "$MONGO_PASSWORD"
        try_until_success "restore $MONGO_BUCKET $MONGO_FOLDER $MONGO_SNAPSHOT"
        ;;
    *)  (10)
        echo $"Unknown cmd!"
        RETVAL=1
esac
exit "$RETVAL"
