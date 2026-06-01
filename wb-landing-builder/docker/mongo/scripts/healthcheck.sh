#!/bin/bash
set -euo pipefail

INIT_FLAG="/data/db/.initialized"

test -f "$INIT_FLAG"
/usr/local/bin/mongo-ping.sh
/usr/local/bin/mongo-primary.sh
