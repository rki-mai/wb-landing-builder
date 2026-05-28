#!/usr/bin/env bash
# Сквозной smoke-тест: auth → storage → publishing.
# Требует: curl, jq, запущенный docker compose (make up).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURES_DIR="${ROOT_DIR}/scripts/fixtures/mutations"

BASE_URL="${BASE_URL:-http://localhost:8080}"
PROJECT_ID="${PROJECT_ID:-smoke-$(date +%s)}"
EMAIL="${SMOKE_EMAIL:-smoke-$(date +%s)@example.com}"
PASSWORD="${SMOKE_PASSWORD:-SmokeTest123!}"
VERBOSE="${SMOKE_VERBOSE:-0}"
SMOKE_SKIP_CLEANUP="${SMOKE_SKIP_CLEANUP:-0}"
MONGO_CONTAINER="${SMOKE_MONGO_CONTAINER:-landing-mongo}"
MONGO_DB="${SMOKE_MONGO_DB:-storage}"

CREATED_EMAILS=()
CREATED_USER_IDS=()
CREATED_PUBLICATION_IDS=()
STORAGE_TOUCHED=0
CLEANUP_RAN=0

RED='\033[0;31m'
GREEN='\033[0;32m'
DIM='\033[2m'
NC='\033[0m'

step=0

log() {
  step=$((step + 1))
  printf "${GREEN}[%02d]${NC} %s\n" "$step" "$1"
}

log_ok() {
  printf "       ${DIM}→${NC} %s\n" "$1"
}

fail() {
  printf "${RED}FAIL:${NC} %s\n" "$1" >&2
  exit 1
}

track_email() {
  CREATED_EMAILS+=("$1")
}

track_user() {
  local user_id="$1"
  [[ -n "$user_id" && "$user_id" != "null" ]] && CREATED_USER_IDS+=("$user_id")
}

track_publication() {
  local publication_id="$1"
  [[ -n "$publication_id" && "$publication_id" != "null" ]] && CREATED_PUBLICATION_IDS+=("$publication_id")
}

register_user() {
  local email="$1"
  local response user_id

  track_email "$email"
  response="$(json_post '/api/v1/auth/register' \
    "{\"email\":\"${email}\",\"password\":\"${PASSWORD}\"}" \
    201)"
  user_id="$(echo "$response" | jq -r '.id')"
  track_user "$user_id"
  echo "$user_id"
}

cleanup_test_data() {
  [[ "$CLEANUP_RAN" == "1" ]] && return 0
  CLEANUP_RAN=1

  if [[ "$SMOKE_SKIP_CLEANUP" == "1" ]]; then
    printf "${DIM}       → cleanup skipped (SMOKE_SKIP_CLEANUP=1)${NC}\n"
    return 0
  fi

  if [[ ${#CREATED_USER_IDS[@]} -eq 0 && "$STORAGE_TOUCHED" != "1" && ${#CREATED_PUBLICATION_IDS[@]} -eq 0 ]]; then
    printf "${DIM}       → nothing to cleanup (no resources created in this run)${NC}\n"
    return 0
  fi

  if ! command -v docker >/dev/null 2>&1; then
    printf "${RED}WARN:${NC} docker not found, cannot cleanup MongoDB\n" >&2
    return 0
  fi

  if ! docker ps --format '{{.Names}}' | grep -qx "$MONGO_CONTAINER"; then
    printf "${DIM}       → mongo container '${MONGO_CONTAINER}' not running, skip DB cleanup${NC}\n"
    return 0
  fi

  printf "${DIM}       → cleaning only resources created in this smoke run${NC}\n"

  local user_ids_json publication_ids_json storage_touched_js result
  user_ids_json="$(printf '%s\n' "${CREATED_USER_IDS[@]}" | jq -R . | jq -s -c .)"
  publication_ids_json="$(printf '%s\n' "${CREATED_PUBLICATION_IDS[@]}" | jq -R . | jq -s -c .)"
  storage_touched_js=$([[ "$STORAGE_TOUCHED" == "1" ]] && echo true || echo false)

  result="$(
    docker exec "$MONGO_CONTAINER" mongosh --quiet "$MONGO_DB" --eval "
const projectId = $(jq -nc --arg id "$PROJECT_ID" '$id');
const userIds = ${user_ids_json};
const publicationIds = ${publication_ids_json};
const storageTouched = ${storage_touched_js};

let mutations = { deletedCount: 0 };
let drafts = { deletedCount: 0 };
if (storageTouched && userIds.length > 0) {
  mutations = db.mutations.deleteMany({ project_id: projectId, owner_id: { \$in: userIds } });
  drafts = db.drafts.deleteMany({ project_id: projectId, owner_id: { \$in: userIds } });
}

let publications = { deletedCount: 0 };
if (publicationIds.length > 0) {
  publications = db.publications.deleteMany({ _id: { \$in: publicationIds } });
}

let refreshTokens = { deletedCount: 0 };
let deletedUsers = { deletedCount: 0 };
if (userIds.length > 0) {
  refreshTokens = db.refresh_tokens.deleteMany({ user_id: { \$in: userIds } });
  deletedUsers = db.users.deleteMany({ _id: { \$in: userIds } });
}

print(JSON.stringify({
  mutations: mutations.deletedCount,
  drafts: drafts.deletedCount,
  publications: publications.deletedCount,
  refresh_tokens: refreshTokens.deletedCount,
  users: deletedUsers.deletedCount
}));
"
  )"

  result="$(echo "$result" | tr -d '\r' | grep -E '^\{' | tail -1)"
  if [[ -z "$result" ]] || ! echo "$result" | jq . >/dev/null 2>&1; then
    printf "${RED}WARN:${NC} cleanup finished but mongo response was unexpected: ${result:-<empty>}\n" >&2
    return 0
  fi

  log_ok "$(echo "$result" | jq -r '
    "removed: mutations=\(.mutations) (draft state), draft_snapshots=\(.drafts) (collapse checkpoints only), " +
    "publications=\(.publications) (0 if already deleted via API in step 10), " +
    "refresh_tokens=\(.refresh_tokens), users=\(.users)"
  ')"
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "Need '$1' in PATH"
}

format_json() {
  local body="$1"
  if [[ -z "$body" ]]; then
    return 0
  fi
  if echo "$body" | jq . >/dev/null 2>&1; then
    echo "$body" | jq .
  else
    echo "$body"
  fi
}

dump_response() {
  local label="$1"
  local body="$2"
  printf "${RED}%s:${NC}\n" "$label" >&2
  format_json "$body" >&2
}

maybe_verbose() {
  local body="$1"
  if [[ "$VERBOSE" == "1" && -n "$body" ]]; then
    format_json "$body" | sed 's/^/       /'
  fi
}

json_post() {
  local path="$1"
  local payload="$2"
  local expected="${3:-200}"
  local auth_header="${4:-}"

  local args=(
    -sS -X POST "${BASE_URL}${path}"
    -H 'Content-Type: application/json'
    -H 'Accept: application/json'
    -d "$payload"
    -w $'\n__HTTP_CODE__:%{http_code}'
  )
  if [[ -n "$auth_header" ]]; then
    args+=(-H "Authorization: Bearer ${auth_header}")
  fi

  local response body code
  response="$(curl "${args[@]}")"
  body="${response%%$'\n'__HTTP_CODE__:*}"
  code="${response##*$'\n'__HTTP_CODE__:}"

  if [[ "$code" != "$expected" ]]; then
    dump_response "POST ${path} (HTTP ${code}, expected ${expected})" "$body"
    fail "POST ${path} expected HTTP ${expected}, got ${code}"
  fi
  echo "$body"
}

json_get() {
  local path="$1"
  local expected="${2:-200}"
  local token="$3"

  local response body code
  response="$(
    curl -sS "${BASE_URL}${path}" \
      -H 'Accept: application/json' \
      -H "Authorization: Bearer ${token}" \
      -w $'\n__HTTP_CODE__:%{http_code}'
  )"
  body="${response%%$'\n'__HTTP_CODE__:*}"
  code="${response##*$'\n'__HTTP_CODE__:}"

  if [[ "$code" != "$expected" ]]; then
    dump_response "GET ${path} (HTTP ${code}, expected ${expected})" "$body"
    fail "GET ${path} expected HTTP ${expected}, got ${code}"
  fi
  echo "$body"
}

json_delete() {
  local path="$1"
  local expected="${2:-204}"
  local token="$3"

  local code
  code="$(
    curl -sS -o /dev/null -w '%{http_code}' -X DELETE "${BASE_URL}${path}" \
      -H "Authorization: Bearer ${token}"
  )"
  if [[ "$code" != "$expected" ]]; then
    fail "DELETE ${path} expected HTTP ${expected}, got ${code}"
  fi
}

wait_for_api() {
  log "Waiting for API at ${BASE_URL}"
  for _ in $(seq 1 60); do
    if curl -fsS "${BASE_URL}/swagger/doc.json" >/dev/null 2>&1; then
      log_ok "API is ready"
      return 0
    fi
    sleep 2
  done
  fail "API is not ready at ${BASE_URL}. Run: make up"
}

main() {
  require_cmd curl
  require_cmd jq
  trap cleanup_test_data EXIT

  wait_for_api

  log "Register user ${EMAIL}"
  register_user "$EMAIL" >/dev/null
  log_ok "registered"

  log "Login"
  local token
  token="$(json_post '/api/v1/auth/login' \
    "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" \
    200 | jq -r '.access_token')"
  [[ -n "$token" && "$token" != "null" ]] || fail "Empty access_token"
  log_ok "access_token received"

  log "Apply sample draft mutations to project ${PROJECT_ID}"
  local mutation_file payload version
  version=0
  for mutation_file in "${FIXTURES_DIR}"/*.json; do
    payload="$(cat "$mutation_file")"
    version="$(json_post "/api/v1/storage/${PROJECT_ID}/mutations" \
      "$payload" 200 "$token" | jq -r '.version')"
  done
  STORAGE_TOUCHED=1
  log_ok "6 mutations applied, latest version=${version}"

  log "Read draft snapshot"
  local draft elements_count
  draft="$(json_get "/api/v1/storage/${PROJECT_ID}" 200 "$token")"
  elements_count="$(echo "$draft" | jq '.elements | length')"
  [[ "$elements_count" -eq 6 ]] || fail "Expected 6 elements, got ${elements_count}"
  log_ok "$(echo "$draft" | jq -r '[.version, (.elements | length), ([.elements[].id] | join(", "))] | "version=\(.[0]), elements=\(.[1]), ids=\(.[2])"')"
  maybe_verbose "$draft"

  log "Create publication"
  local publication publication_id
  publication="$(json_post "/api/v1/storage/${PROJECT_ID}/publications" '{}' 201 "$token")"
  publication_id="$(echo "$publication" | jq -r '.id')"
  [[ -n "$publication_id" && "$publication_id" != "null" ]] || fail "Empty publication id"
  track_publication "$publication_id"
  log_ok "$(echo "$publication" | jq -r '["id=\(.id)", "status=\(.status)", "assets=\(.assets_path)"] | join(", ")')"
  maybe_verbose "$publication"

  log "List publication IDs"
  local ids
  ids="$(json_get "/api/v1/storage/${PROJECT_ID}/publications" 200 "$token")"
  echo "$ids" | jq -e --arg id "$publication_id" '.ids | index($id) != null' >/dev/null \
    || fail "Created publication not found in list"
  log_ok "$(echo "$ids" | jq -r '.ids | "ids=[\(join(", "))]"')"

  log "Get publication metadata"
  local pub_meta
  pub_meta="$(json_get "/api/v1/storage/${PROJECT_ID}/publications/${publication_id}" 200 "$token")"
  log_ok "$(echo "$pub_meta" | jq -r '"project_id=\(.project_id), version=\(.version)"')"

  log "Check project access control (foreign user gets 403)"
  local other_email other_token
  other_email="other-$(date +%s)@example.com"
  register_user "$other_email" >/dev/null
  other_token="$(json_post '/api/v1/auth/login' \
    "{\"email\":\"${other_email}\",\"password\":\"${PASSWORD}\"}" \
    200 | jq -r '.access_token')"
  json_get "/api/v1/storage/${PROJECT_ID}/publications" 403 "$other_token" >/dev/null
  log_ok "foreign user denied (HTTP 403)"

  log "Delete publication"
  json_delete "/api/v1/storage/${PROJECT_ID}/publications/${publication_id}" 204 "$token"
  log_ok "deleted ${publication_id}"

  log "Verify publication list is empty"
  ids="$(json_get "/api/v1/storage/${PROJECT_ID}/publications" 200 "$token")"
  local list_len
  list_len="$(echo "$ids" | jq '.ids | length')"
  [[ "$list_len" -eq 0 ]] || fail "Expected empty publication list, got ${list_len}"
  log_ok "publication list is empty"

  log "Cleanup test data"
  cleanup_test_data
  trap - EXIT

  printf "\n${GREEN}Smoke test passed${NC} (project: ${PROJECT_ID})\n"
}

main "$@"
