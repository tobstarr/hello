#!/bin/bash
set -e

d=$(mktemp -d /tmp/go-bin-XXXX)
trap "rm -Rf $d" EXIT

export REPO=github.com/tobstarr/hello
export BIN_NAME=$(basename $REPO)
export GO_VERSION=1.7.1
export REVISION=${REVISION:-HEAD}

cat > $d/Dockerfile <<EOF
FROM docker.io/debian:jessie

RUN apt-get update && apt-get upgrade -y && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY ${BIN_NAME} /usr/local/bin
WORKDIR /app

ARG VERSION=v1

RUN echo -n \$VERSION > /etc/version

ENTRYPOINT ["/usr/local/bin/${BIN_NAME}"]
EOF

tar cz --exclude-vcs --exclude=docs . | docker run -i golang:${GO_VERSION} sh -ce "mkdir -p /go/src/$REPO; cd /go/src/$REPO; tar xz; go build -o /dst/${BIN_NAME} -ldflags '-s -w -X main.REVISION=$REVISION' ${REPO}; cat /dst/${BIN_NAME} | gzip" | gunzip > $d/${BIN_NAME}

chmod a+x $d/${BIN_NAME}

versions=$(grep '^\s*"v' main.go | cut -d '"' -f 2 | xargs)

pushd $d > /dev/null

IMAGE_REPO=${IMAGE_REPO:-quay.io/tobstarr/hello}

for v in ${versions}; do
    image_tag=${IMAGE_REPO}:${v}
    echo "building $image_tag"
    docker build -t ${image_tag} --build-arg VERSION=${v} .
    docker push ${image_tag}
done
