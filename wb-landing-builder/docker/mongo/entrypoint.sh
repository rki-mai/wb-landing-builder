#!/bin/bash
set -euo pipefail

# shellcheck source=/usr/local/lib/mongo/common.sh
source /usr/local/lib/mongo/common.sh

trap cleanup EXIT INT TERM

main() {
  start_mongod
  wait_for_mongo
  initialize_mongo
  wait_for_primary
  wait_mongod
}

main "$@"
