#!/bin/bash
set -e

export GO_VERSION=1.7.1
export REPO=github.com/tobstarr/hello
export BIN_NAME=$(basename $REPO)

tar cz --exclude-vcs --exclude=docs . | docker run -i golang:${GO_VERSION} sh -ce "mkdir -p /go/src/$REPO; cd /go/src/$REPO; tar xz; go test -v"
