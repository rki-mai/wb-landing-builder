#!/bin/sh
set -eu

# shellcheck source=/usr/local/lib/minio/common.sh
. /usr/local/lib/minio/common.sh

trap cleanup EXIT INT TERM

main() {
  start_minio "$@"
  wait_for_minio
  initialize_minio
  ensure_public_read_policy
  wait_minio
}

main "$@"
