#!/bin/bash

exec 1> >(logger -s -p daemon.info -t ms)
exec 2> >(logger -s -p daemon.error -t ms)

RETVAL=0

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

process=$1
shift
case "$process" in
    backup)
        backup "$@"
        ;;
    restore)
        restore "$@"
        ;;
    push)
        push "$@"
        ;;
    pull)
        pull "$@"
        ;;
    *)	(10)
        echo $"Unknown process!"
        RETVAL=1
esac
exit "$RETVAL"
