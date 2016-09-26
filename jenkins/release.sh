#!/bin/bash
set -e

function cleanup {
  exit_code=$?
  if [[ $exit_code != 0 ]]; then
    echo "rolling back deployment"
    kubectl rollout undo deployments/hello
    exit $exit_code
  fi
  echo "finished deployment"
}


image_tag=$(cat image_tag)
echo "releasing ${image_tag}"

kubectl set image deployments/hello '*='${image_tag}

trap cleanup EXIT

timeout 60 kubectl rollout status deployments/hello
