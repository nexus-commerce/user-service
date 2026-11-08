#!/bin/bash

set -e

cd "$(dirname "$0")"

[ $# -gt 0 ] || set -- ./...

docker compose --env-file ../.env run --rm --no-TTY user-service sh -c \
  'TEST_POSTGRES_DSN="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@$POSTGRES_HOST:$POSTGRES_PORT/$POSTGRES_DATABASE?sslmode=disable" exec go test -count=1 "$@"' \
  sh "$@"
