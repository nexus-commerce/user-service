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

docker compose --profile kafka down -v
docker compose "${profile[@]}" up -d
docker compose logs -f user-service
