#!/bin/sh

INIT_FLAG="/data/.initialized"
BUCKET="${MINIO_DEFAULT_BUCKET:-publications}"
MINIO_PID=""

cleanup() {
  if [ -n "${MINIO_PID:-}" ] && kill -0 "$MINIO_PID" 2>/dev/null; then
    kill -TERM "$MINIO_PID" 2>/dev/null || true
    wait "$MINIO_PID" 2>/dev/null || true
  fi
}

start_minio() {
  minio "$@" &
  MINIO_PID=$!
}

wait_for_minio() {
  until /usr/local/bin/minio-ready.sh; do
    sleep 1
  done
}

initialize_minio() {
  if [ -f "$INIT_FLAG" ]; then
    echo "MinIO already initialized"
    return 0
  fi

  mc mb -p "local/${BUCKET}" || true
  touch "$INIT_FLAG"
  echo "MinIO bucket '${BUCKET}' ready"
}

wait_minio() {
  wait "$MINIO_PID"
}
