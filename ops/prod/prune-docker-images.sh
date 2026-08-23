#!/usr/bin/env bash

set -euo pipefail

MAX_ROOT_USE_PERCENT="${MAX_ROOT_USE_PERCENT:-65}"
IMAGE_RETENTION="${IMAGE_RETENTION:-168h}"

if ! [[ "${MAX_ROOT_USE_PERCENT}" =~ ^[0-9]+$ ]] \
  || ((MAX_ROOT_USE_PERCENT < 1 || MAX_ROOT_USE_PERCENT > 99)); then
  echo "MAX_ROOT_USE_PERCENT must be an integer between 1 and 99." >&2
  exit 2
fi

root_use="$(df -P / | awk 'NR == 2 {gsub(/%/, "", $5); print $5}')"
if ! [[ "${root_use}" =~ ^[0-9]+$ ]]; then
  echo "Could not determine root filesystem usage." >&2
  exit 1
fi

if ((root_use < MAX_ROOT_USE_PERCENT)); then
  echo "Root usage is ${root_use}%; Docker image cleanup starts at ${MAX_ROOT_USE_PERCENT}%."
  exit 0
fi

echo "Root usage is ${root_use}%; pruning images unused by containers and older than ${IMAGE_RETENTION}."
docker image prune --all --force --filter "until=${IMAGE_RETENTION}"
