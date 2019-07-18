#!/usr/bin/env bash

pushd $GOPATH/src/kubedb.dev/mongodb/hack/gendocs
go run main.go
popd
