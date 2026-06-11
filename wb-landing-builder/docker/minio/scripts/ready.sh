#!/bin/sh
set -eu

MINIO_ENDPOINT="${MINIO_ENDPOINT:-http://127.0.0.1:9000}"

mc alias set local "$MINIO_ENDPOINT" "${MINIO_ROOT_USER}" "${MINIO_ROOT_PASSWORD}" >/dev/null 2>&1
mc ready local >/dev/null 2>&1
