#!/bin/sh
set -eu

INIT_FLAG="/data/.initialized"

test -f "$INIT_FLAG"
/usr/local/bin/minio-ready.sh
