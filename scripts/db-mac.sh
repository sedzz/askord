#!/usr/bin/env bash
# Starts Postgres on macOS with Apple `container` (an alternative to `docker compose up -d`).
# It is idempotent: you can always run it the same way, whether or not the container exists.
set -euo pipefail

NAME=askord-postgres
VOLUME=pgdata
IMAGE=postgres:17

if ! command -v container >/dev/null; then
  echo "The 'container' command was not found (https://github.com/apple/container)." >&2
  exit 1
fi

container system start

if ! container volume list | grep -qw "$VOLUME"; then
  container volume create "$VOLUME"
fi

if container list --all | grep -qw "$NAME"; then
  container start "$NAME" >/dev/null 2>&1 || true
else
  # PGDATA in a subdirectory: a new volume comes with lost+found and initdb does not accept a non-empty directory.
  container run -d --name "$NAME" \
    -e PGDATA=/var/lib/postgresql/data/pgdata \
    -e POSTGRES_USER=askord -e POSTGRES_PASSWORD=askord -e POSTGRES_DB=askord \
    -p 127.0.0.1:5432:5432 \
    -v "$VOLUME":/var/lib/postgresql/data \
    "$IMAGE"
fi

echo "Postgres at localhost:5432 (container $NAME)"
