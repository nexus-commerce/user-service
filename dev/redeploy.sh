#!/bin/bash

set -e

usage() {
  echo "usage: $0 [--kafka]" >&2
  exit 1
}

profile=()
case "$1" in
  "") ;;
  --kafka) profile=(--profile kafka) ;;
  *) usage ;;
esac
[ $# -le 1 ] || usage

cd "$(dirname "$0")"

docker compose --env-file ../.env --profile kafka down -v
docker compose --env-file ../.env "${profile[@]}" up -d
docker compose --env-file ../.env logs -f user-service
