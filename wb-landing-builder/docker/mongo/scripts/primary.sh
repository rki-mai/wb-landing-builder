#!/bin/bash
set -euo pipefail

mongosh --quiet --eval 'try { quit(db.hello().isWritablePrimary ? 0 : 1) } catch (e) { quit(1) }'
