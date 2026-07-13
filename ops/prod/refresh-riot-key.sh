#!/usr/bin/env bash
set -euo pipefail

DEPLOY_ROOT="${WINRIFT_DEPLOY_ROOT:-/srv/winrift}"
ENV_FILE="${WINRIFT_ENV_FILE:-${DEPLOY_ROOT}/.env}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${WINRIFT_COMPOSE_FILE:-}"
COMPOSE_FILE_EXPLICIT=0
PROJECT_DIRECTORY="${WINRIFT_PROJECT_DIRECTORY:-${DEPLOY_ROOT}}"
HEALTH_URL="${WINRIFT_HEALTH_URL:-http://127.0.0.1:8000/api/health}"
READ_FROM_STDIN=0
KEY_FILE=""
START_WORKER=1
RESTART_SERVICES=1
SHOW_KEY=1
CONFIRM_KEY=1
WORKER_START_ATTEMPTS="${WINRIFT_WORKER_START_ATTEMPTS:-5}"
WORKER_START_CHECK_SECONDS="${WINRIFT_WORKER_START_CHECK_SECONDS:-8}"
WORKER_START_RETRY_DELAY="${WINRIFT_WORKER_START_RETRY_DELAY:-12}"
AUTO_PATCH_ROLLOVER="${WINRIFT_AUTO_PATCH_ROLLOVER:-1}"
PATCH_ROLLOVER_RETAIN_DAYS="${WINRIFT_PATCH_ROLLOVER_RETAIN_DAYS:-0}"
PATCH_ROLLOVER_QUEUE_ID="${WINRIFT_PATCH_ROLLOVER_QUEUE_ID:-420}"
PATCHCTL_CLICKHOUSE_MAX_THREADS="${WINRIFT_PATCHCTL_CLICKHOUSE_MAX_THREADS:-2}"
PATCHCTL_CLICKHOUSE_MAX_MEMORY_MB="${WINRIFT_PATCHCTL_CLICKHOUSE_MAX_MEMORY_MB:-2048}"
PATCHCTL_CLICKHOUSE_MAX_OPEN_CONNS="${WINRIFT_PATCHCTL_CLICKHOUSE_MAX_OPEN_CONNS:-2}"
PATCHCTL_CLICKHOUSE_MAX_IDLE_CONNS="${WINRIFT_PATCHCTL_CLICKHOUSE_MAX_IDLE_CONNS:-1}"
PATCHCTL_CLICKHOUSE_MAX_EXECUTION_TIME_SECONDS="${WINRIFT_PATCHCTL_CLICKHOUSE_MAX_EXECUTION_TIME_SECONDS:-1800}"
PATCHCTL_MAX_LOAD_1M="${WINRIFT_PATCHCTL_MAX_LOAD_1M:-}"
PATCHCTL_MIN_AVAILABLE_MEMORY_MB="${WINRIFT_PATCHCTL_MIN_AVAILABLE_MEMORY_MB:-1024}"
PATCHCTL_PRESSURE_CHECK_ATTEMPTS="${WINRIFT_PATCHCTL_PRESSURE_CHECK_ATTEMPTS:-60}"
PATCHCTL_PRESSURE_CHECK_SLEEP_SECONDS="${WINRIFT_PATCHCTL_PRESSURE_CHECK_SLEEP_SECONDS:-10}"
MONITOR_STOPPED_FOR_MAINTENANCE=0

usage() {
  cat <<'EOF'
Usage: refresh-riot-key [options]

Updates RIOT_API_KEY in the server-local WinRift env file, clears the Riot auth
failure marker, advances the collector patch when Data Dragon has moved, recreates
the API, and starts the collector worker.

Options:
  --env-file PATH      Env file to update. Default: /srv/winrift/.env
  --deploy-root PATH   Compose project/deploy root. Default: /srv/winrift
  --compose-file PATH  Production docker-compose.yml path.
  --health-url URL     API health URL. Default: http://127.0.0.1:8000/api/health
  --key-file PATH      Read the Riot key from a file.
  --stdin              Read the Riot key from stdin.
  --hide-key           Hide interactive key input and show only a masked preview.
  --yes                Skip the interactive confirmation prompt.
  --no-worker          Restart API/monitor but do not start the worker.
  --no-restart         Only update env and clear auth marker.
  --no-patch-rollover  Do not check Data Dragon or advance COLLECTOR_CURRENT_PATCH.
  --patch-retain-days N
                       Raw retention days for patches archived during rollover. Default: 0
  --patchctl-max-threads N
                       Max ClickHouse query threads for rollover maintenance. Default: 2
  --patchctl-max-memory-mb N
                       Max ClickHouse query memory for rollover maintenance. Default: 2048
  --worker-attempts N  Worker start attempts for transient Riot 401s. Default: 5
  -h, --help           Show this help.

Normal use:
  refresh-riot-key

Interactive use shows the pasted key back to you and requires typing YES before
the env file is updated. The value is not written to shell history by this script,
but it will be visible in terminal scrollback unless --hide-key is used.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      ENV_FILE="${2:?missing value for --env-file}"
      shift 2
      ;;
    --deploy-root)
      DEPLOY_ROOT="${2:?missing value for --deploy-root}"
      PROJECT_DIRECTORY="${WINRIFT_PROJECT_DIRECTORY:-${DEPLOY_ROOT}}"
      shift 2
      ;;
    --compose-file)
      COMPOSE_FILE="${2:?missing value for --compose-file}"
      COMPOSE_FILE_EXPLICIT=1
      shift 2
      ;;
    --health-url)
      HEALTH_URL="${2:?missing value for --health-url}"
      shift 2
      ;;
    --key-file)
      KEY_FILE="${2:?missing value for --key-file}"
      shift 2
      ;;
    --stdin)
      READ_FROM_STDIN=1
      shift
      ;;
    --hide-key)
      SHOW_KEY=0
      shift
      ;;
    --yes)
      CONFIRM_KEY=0
      shift
      ;;
    --no-worker)
      START_WORKER=0
      shift
      ;;
    --no-restart)
      RESTART_SERVICES=0
      shift
      ;;
    --no-patch-rollover)
      AUTO_PATCH_ROLLOVER=0
      shift
      ;;
    --patch-retain-days)
      PATCH_ROLLOVER_RETAIN_DAYS="${2:?missing value for --patch-retain-days}"
      shift 2
      ;;
    --patchctl-max-threads)
      PATCHCTL_CLICKHOUSE_MAX_THREADS="${2:?missing value for --patchctl-max-threads}"
      shift 2
      ;;
    --patchctl-max-memory-mb)
      PATCHCTL_CLICKHOUSE_MAX_MEMORY_MB="${2:?missing value for --patchctl-max-memory-mb}"
      shift 2
      ;;
    --worker-attempts)
      WORKER_START_ATTEMPTS="${2:?missing value for --worker-attempts}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "${COMPOSE_FILE}" ]]; then
  if [[ -f "${DEPLOY_ROOT}/docker-compose.yml" ]]; then
    COMPOSE_FILE="${DEPLOY_ROOT}/docker-compose.yml"
  else
    COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"
  fi
fi

trim_key() {
  local value="$1"
  value="${value//$'\r'/}"
  value="${value//$'\n'/}"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "${value}"
}

normalize_key_input() {
  local value="$1"
  local line=""
  local fallback=""

  value="${value//$'\r'/$'\n'}"
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="$(trim_key "${line}")"
    line="${line#RIOT_API_KEY=}"
    if [[ "${line}" == RGAPI-* ]]; then
      printf '%s' "${line}"
      return 0
    fi
    if [[ -z "${fallback}" && -n "${line}" ]]; then
      fallback="${line}"
    fi
  done <<<"${value}"

  printf '%s' "${fallback}"
}

read_new_key() {
  local value=""
  if [[ -n "${KEY_FILE}" ]]; then
    if [[ ! -f "${KEY_FILE}" ]]; then
      echo "Key file not found: ${KEY_FILE}" >&2
      exit 1
    fi
    value="$(<"${KEY_FILE}")"
  elif [[ "${READ_FROM_STDIN}" -eq 1 ]]; then
    value="$(cat)"
  else
    if [[ ! -t 0 ]]; then
      echo "No TTY available. Use --stdin or --key-file." >&2
      exit 1
    fi
    if [[ "${SHOW_KEY}" -eq 1 ]]; then
      read -r -p "Paste new Riot API key (input visible): " value
    else
      read -r -s -p "Paste new Riot API key: " value
      echo
    fi
  fi
  normalize_key_input "${value}"
}

masked_key() {
  local key="$1"
  local length="${#key}"
  if [[ "${length}" -le 16 ]]; then
    printf '%s' "${key}"
    return 0
  fi
  printf '%s...%s' "${key:0:10}" "${key: -6}"
}

confirm_new_key() {
  local key="$1"
  local answer=""

  if [[ "${CONFIRM_KEY}" -eq 0 ]]; then
    return 0
  fi

  if [[ "${READ_FROM_STDIN}" -eq 1 || -n "${KEY_FILE}" ]]; then
    echo "Riot API key accepted from non-interactive input: $(masked_key "${key}")"
    return 0
  fi

  echo
  if [[ "${SHOW_KEY}" -eq 1 ]]; then
    echo "You entered this Riot API key:"
    printf '  %s\n' "${key}"
  else
    echo "Key preview: $(masked_key "${key}")"
  fi
  read -r -p "Type YES to update ${ENV_FILE} and restart WinRift services: " answer
  if [[ "${answer}" != "YES" ]]; then
    echo "Cancelled. No changes made."
    exit 1
  fi
}

env_value() {
  local key="$1"
  if [[ ! -f "${ENV_FILE}" ]]; then
    return 0
  fi
  grep -E "^${key}=" "${ENV_FILE}" | tail -1 | cut -d= -f2- || true
}

write_env_key() {
  local key="$1"
  local value="$2"
  local tmp=""
  local replaced=0

  tmp="$(mktemp "${ENV_FILE}.tmp.XXXXXX")"
  while IFS= read -r line || [[ -n "${line}" ]]; do
    if [[ "${line}" == "${key}="* ]]; then
      printf '%s=%s\n' "${key}" "${value}" >>"${tmp}"
      replaced=1
    else
      printf '%s\n' "${line}" >>"${tmp}"
    fi
  done <"${ENV_FILE}"

  if [[ "${replaced}" -eq 0 ]]; then
    printf '\n%s=%s\n' "${key}" "${value}" >>"${tmp}"
  fi

  chmod --reference="${ENV_FILE}" "${tmp}" 2>/dev/null || chmod 600 "${tmp}"
  mv "${tmp}" "${ENV_FILE}"
}

runtime_host_path_for_marker() {
  local marker="$1"
  local runtime_dir="$2"
  if [[ "${marker}" == /run/winrift/* ]]; then
    printf '%s/%s' "${runtime_dir%/}" "${marker#/run/winrift/}"
  else
    printf '%s' "${marker}"
  fi
}

compose() {
  docker compose \
    --project-directory "${PROJECT_DIRECTORY}" \
    -f "${COMPOSE_FILE}" \
    --env-file "${ENV_FILE}" \
    "$@"
}

patchctl() {
  compose run \
    --rm \
    --no-deps \
    -e CLICKHOUSE_MAX_OPEN_CONNS="${PATCHCTL_CLICKHOUSE_MAX_OPEN_CONNS}" \
    -e CLICKHOUSE_MAX_IDLE_CONNS="${PATCHCTL_CLICKHOUSE_MAX_IDLE_CONNS}" \
    -e CLICKHOUSE_MAX_THREADS="${PATCHCTL_CLICKHOUSE_MAX_THREADS}" \
    -e CLICKHOUSE_MAX_MEMORY_MB="${PATCHCTL_CLICKHOUSE_MAX_MEMORY_MB}" \
    -e CLICKHOUSE_MAX_EXECUTION_TIME_SECONDS="${PATCHCTL_CLICKHOUSE_MAX_EXECUTION_TIME_SECONDS}" \
    -e PATCHCTL_MAX_LOAD_1M="${PATCHCTL_MAX_LOAD_1M}" \
    -e PATCHCTL_MIN_AVAILABLE_MEMORY_MB="${PATCHCTL_MIN_AVAILABLE_MEMORY_MB}" \
    -e PATCHCTL_PRESSURE_CHECK_ATTEMPTS="${PATCHCTL_PRESSURE_CHECK_ATTEMPTS}" \
    -e PATCHCTL_PRESSURE_CHECK_SLEEP_SECONDS="${PATCHCTL_PRESSURE_CHECK_SLEEP_SECONDS}" \
    api \
    /winrift-patchctl "$@"
}

wait_for_health() {
  local status=""
  local body_file="/tmp/winrift_refresh_health.json"
  rm -f "${body_file}"
  for _ in $(seq 1 30); do
    status="$(curl -sS -o "${body_file}" -w '%{http_code}' "${HEALTH_URL}" || true)"
    if [[ "${status}" == "200" ]] && ! grep -q '"riotApi"[[:space:]]*:[[:space:]]*"auth_failed"' "${body_file}" 2>/dev/null; then
      rm -f "${body_file}"
      return 0
    fi
    sleep 2
  done
  echo "API health did not recover. Last status: ${status}" >&2
  cat "${body_file}" >&2 2>/dev/null || true
  return 1
}

positive_integer() {
  local value="$1"
  [[ "${value}" =~ ^[1-9][0-9]*$ ]]
}

non_negative_integer() {
  local value="$1"
  [[ "${value}" =~ ^[0-9]+$ ]]
}

positive_number() {
  local value="$1"
  [[ "${value}" =~ ^[0-9]+([.][0-9]+)?$ ]] && awk -v value="${value}" 'BEGIN { exit !(value > 0) }'
}

truthy() {
  case "${1,,}" in
    1|true|yes|y|on)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

default_patchctl_max_load() {
  if [[ -n "${PATCHCTL_MAX_LOAD_1M}" ]]; then
    return 0
  fi
  PATCHCTL_MAX_LOAD_1M="$(nproc 2>/dev/null || printf '4')"
}

patch_bucket_parts() {
  local patch="$1"
  if [[ ! "${patch}" =~ ^([0-9]+)\.([0-9]+)$ ]]; then
    return 1
  fi
  printf '%s %s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}"
}

patch_is_newer() {
  local target="$1"
  local current="$2"
  local target_major target_minor current_major current_minor

  read -r target_major target_minor < <(patch_bucket_parts "${target}") || return 1
  read -r current_major current_minor < <(patch_bucket_parts "${current}") || return 1

  if (( target_major > current_major )); then
    return 0
  fi
  if (( target_major < current_major )); then
    return 1
  fi
  (( target_minor > current_minor ))
}

clear_auth_marker() {
  if [[ -n "${host_marker_path:-}" ]]; then
    echo "Clearing Riot auth marker: ${host_marker_path}"
    rm -f "${host_marker_path}"
  fi
}

worker_failure_is_retryable() {
  local logs="$1"
  grep -Eiq 'status=401|riot auth failure|auth-failed|Riot API key is missing, expired, or not authorized' <<<"${logs}"
}

start_worker_with_retries() {
  local attempt=1
  local worker_status=""
  local worker_logs=""

  while (( attempt <= WORKER_START_ATTEMPTS )); do
    if (( WORKER_START_ATTEMPTS == 1 )); then
      echo "Starting worker."
    else
      echo "Starting worker. Attempt ${attempt}/${WORKER_START_ATTEMPTS}."
    fi

    compose up -d --no-deps --force-recreate --no-build worker
    sleep "${WORKER_START_CHECK_SECONDS}"
    worker_status="$(docker inspect --format '{{.State.Status}}' winrift_worker 2>/dev/null || true)"
    if [[ "${worker_status}" == "running" ]]; then
      return 0
    fi

    worker_logs="$(docker logs --tail 120 winrift_worker 2>&1 || true)"
    if ! worker_failure_is_retryable "${worker_logs}" || (( attempt == WORKER_START_ATTEMPTS )); then
      echo "Worker did not stay running. Status: ${worker_status}" >&2
      printf '%s\n' "${worker_logs}" >&2
      return 1
    fi

    echo "Worker hit a transient Riot auth response while the refreshed key propagates; retrying in ${WORKER_START_RETRY_DELAY}s." >&2
    clear_auth_marker
    sleep "${WORKER_START_RETRY_DELAY}"
    attempt=$((attempt + 1))
  done
}

restore_monitor_after_failure() {
  local status=$?
  trap - EXIT
  if [[ "${status}" -ne 0 && "${MONITOR_STOPPED_FOR_MAINTENANCE}" -eq 1 ]]; then
    echo "Refresh failed during maintenance; restarting monitor before exiting." >&2
    compose up -d --no-deps --no-build monitor >/dev/null 2>&1 || true
  fi
  exit "${status}"
}

rollover_patch_window_if_needed() {
  local current_patch=""
  local latest_patch=""

  if ! truthy "${AUTO_PATCH_ROLLOVER}"; then
    echo "Patch rollover skipped by configuration."
    return 0
  fi

  current_patch="$(env_value COLLECTOR_CURRENT_PATCH)"
  if [[ -z "${current_patch}" ]]; then
    echo "Patch rollover skipped because COLLECTOR_CURRENT_PATCH is empty."
    return 0
  fi

  echo "Checking latest Riot patch from Data Dragon."
  latest_patch="$(patchctl -action latest-patch | tr -d '\r' | tail -n 1)"
  if [[ -z "${latest_patch}" ]]; then
    echo "Latest Riot patch lookup returned an empty value." >&2
    return 1
  fi

  if patch_is_newer "${latest_patch}" "${current_patch}"; then
    echo "Detected newer Riot patch: ${current_patch} -> ${latest_patch}."
    echo "Archiving and pruning raw data that falls outside the new retention window."
    patchctl \
      -action rollover \
      -patch "${latest_patch}" \
      -platform ALL \
      -queue "${PATCH_ROLLOVER_QUEUE_ID}" \
      -retain-days "${PATCH_ROLLOVER_RETAIN_DAYS}" \
      -prune-raw=true
    write_env_key "COLLECTOR_CURRENT_PATCH" "${latest_patch}"
    echo "Updated COLLECTOR_CURRENT_PATCH=${latest_patch} in ${ENV_FILE}."
  else
    echo "Collector patch ${current_patch} is already current for latest Riot patch ${latest_patch}."
  fi
}

require_file() {
  local path="$1"
  local label="$2"
  if [[ ! -f "${path}" ]]; then
    echo "${label} not found: ${path}" >&2
    exit 1
  fi
}

require_file "${ENV_FILE}" "Env file"

if ! positive_integer "${WORKER_START_ATTEMPTS}"; then
  echo "Worker start attempts must be a positive integer: ${WORKER_START_ATTEMPTS}" >&2
  exit 1
fi
if ! positive_integer "${WORKER_START_CHECK_SECONDS}"; then
  echo "Worker start check seconds must be a positive integer: ${WORKER_START_CHECK_SECONDS}" >&2
  exit 1
fi
if ! positive_integer "${WORKER_START_RETRY_DELAY}"; then
  echo "Worker start retry delay must be a positive integer: ${WORKER_START_RETRY_DELAY}" >&2
  exit 1
fi
default_patchctl_max_load
if ! non_negative_integer "${PATCH_ROLLOVER_RETAIN_DAYS}"; then
  echo "Patch rollover retain days must be a non-negative integer: ${PATCH_ROLLOVER_RETAIN_DAYS}" >&2
  exit 1
fi
if ! positive_integer "${PATCH_ROLLOVER_QUEUE_ID}"; then
  echo "Patch rollover queue id must be a positive integer: ${PATCH_ROLLOVER_QUEUE_ID}" >&2
  exit 1
fi
if ! positive_integer "${PATCHCTL_CLICKHOUSE_MAX_THREADS}"; then
  echo "Patchctl ClickHouse max threads must be a positive integer: ${PATCHCTL_CLICKHOUSE_MAX_THREADS}" >&2
  exit 1
fi
if ! positive_integer "${PATCHCTL_CLICKHOUSE_MAX_MEMORY_MB}"; then
  echo "Patchctl ClickHouse max memory must be a positive integer MB value: ${PATCHCTL_CLICKHOUSE_MAX_MEMORY_MB}" >&2
  exit 1
fi
if ! positive_integer "${PATCHCTL_CLICKHOUSE_MAX_OPEN_CONNS}"; then
  echo "Patchctl ClickHouse max open connections must be a positive integer: ${PATCHCTL_CLICKHOUSE_MAX_OPEN_CONNS}" >&2
  exit 1
fi
if ! positive_integer "${PATCHCTL_CLICKHOUSE_MAX_IDLE_CONNS}"; then
  echo "Patchctl ClickHouse max idle connections must be a positive integer: ${PATCHCTL_CLICKHOUSE_MAX_IDLE_CONNS}" >&2
  exit 1
fi
if ! positive_integer "${PATCHCTL_CLICKHOUSE_MAX_EXECUTION_TIME_SECONDS}"; then
  echo "Patchctl ClickHouse max execution time must be a positive integer: ${PATCHCTL_CLICKHOUSE_MAX_EXECUTION_TIME_SECONDS}" >&2
  exit 1
fi
if ! positive_number "${PATCHCTL_MAX_LOAD_1M}"; then
  echo "Patchctl max load must be a positive number: ${PATCHCTL_MAX_LOAD_1M}" >&2
  exit 1
fi
if ! positive_integer "${PATCHCTL_MIN_AVAILABLE_MEMORY_MB}"; then
  echo "Patchctl minimum available memory must be a positive integer MB value: ${PATCHCTL_MIN_AVAILABLE_MEMORY_MB}" >&2
  exit 1
fi
if ! positive_integer "${PATCHCTL_PRESSURE_CHECK_ATTEMPTS}"; then
  echo "Patchctl pressure check attempts must be a positive integer: ${PATCHCTL_PRESSURE_CHECK_ATTEMPTS}" >&2
  exit 1
fi
if ! positive_integer "${PATCHCTL_PRESSURE_CHECK_SLEEP_SECONDS}"; then
  echo "Patchctl pressure check sleep seconds must be a positive integer: ${PATCHCTL_PRESSURE_CHECK_SLEEP_SECONDS}" >&2
  exit 1
fi

if [[ "${RESTART_SERVICES}" -eq 1 && "${COMPOSE_FILE_EXPLICIT}" -eq 0 && "${COMPOSE_FILE}" != "${DEPLOY_ROOT}/docker-compose.yml" ]]; then
  if [[ -f "${SCRIPT_DIR}/docker-compose.yml" && -d "${DEPLOY_ROOT}" ]]; then
    echo "Installing production Compose file to ${DEPLOY_ROOT}/docker-compose.yml."
    install -m 0644 "${SCRIPT_DIR}/docker-compose.yml" "${DEPLOY_ROOT}/docker-compose.yml"
    COMPOSE_FILE="${DEPLOY_ROOT}/docker-compose.yml"
  fi
fi

NEW_KEY="$(read_new_key)"
if [[ -z "${NEW_KEY}" ]]; then
  echo "Riot API key cannot be empty." >&2
  exit 1
fi
if [[ "${#NEW_KEY}" -lt 20 ]]; then
  echo "Riot API key looks too short. Aborting." >&2
  exit 1
fi
if [[ "${NEW_KEY}" != RGAPI-* ]]; then
  echo "Riot API key must start with RGAPI-. Aborting before updating ${ENV_FILE}." >&2
  exit 1
fi

confirm_new_key "${NEW_KEY}"

echo "Updating ${ENV_FILE}."
write_env_key "RIOT_API_KEY" "${NEW_KEY}"
unset NEW_KEY

runtime_dir="$(env_value WINRIFT_RUNTIME_STATE_DIR)"
runtime_dir="${runtime_dir:-${DEPLOY_ROOT}/runtime}"
marker_path="$(env_value RIOT_AUTH_FAILURE_MARKER_PATH)"
marker_path="${marker_path:-/run/winrift/riot-auth-failed}"
host_marker_path="$(runtime_host_path_for_marker "${marker_path}" "${runtime_dir}")"

clear_auth_marker

if [[ "${RESTART_SERVICES}" -eq 0 ]]; then
  echo "Updated key and cleared marker. Service restart skipped by --no-restart."
  exit 0
fi

require_file "${COMPOSE_FILE}" "Compose file"

trap restore_monitor_after_failure EXIT

echo "Stopping monitor during intentional key-refresh maintenance."
compose stop monitor >/dev/null 2>&1 || true
MONITOR_STOPPED_FOR_MAINTENANCE=1

echo "Stopping worker before recreating API."
compose stop worker >/dev/null 2>&1 || true

echo "Starting ClickHouse if needed."
compose up -d --no-build clickhouse

rollover_patch_window_if_needed

echo "Recreating API with refreshed environment."
compose up -d --no-deps --force-recreate --no-build api
wait_for_health

echo "Ensuring monitor is running."
compose up -d --no-deps --no-build monitor
MONITOR_STOPPED_FOR_MAINTENANCE=0

if [[ "${START_WORKER}" -eq 1 ]]; then
  start_worker_with_retries
else
  echo "Worker start skipped by --no-worker."
fi

echo "Riot key refresh complete."
