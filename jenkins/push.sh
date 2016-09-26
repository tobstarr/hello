#!/bin/bash
set -e

image_tag=$(cat $WORKSPACE/image_tag)

docker push ${image_tag}

echo "pushed image ${image_tag}"

echo -n ${image_tag} > ${WORKSPACE}/image_tag
