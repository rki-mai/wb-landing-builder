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

ensure_public_read_policy() {
  MINIO_ENDPOINT="${MINIO_ENDPOINT:-http://127.0.0.1:9000}"
  mc alias set local "$MINIO_ENDPOINT" "${MINIO_ROOT_USER}" "${MINIO_ROOT_PASSWORD}" >/dev/null 2>&1 || true
  mc anonymous set download "local/${BUCKET}" || true
  echo "MinIO bucket '${BUCKET}' public download policy applied"
}

wait_minio() {
  wait "$MINIO_PID"
}
