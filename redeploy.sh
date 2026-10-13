#!/bin/bash

set -e

docker compose down -v
docker compose up -d
docker compose logs -f user-service
