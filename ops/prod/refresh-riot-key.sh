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

usage() {
  cat <<'EOF'
Usage: refresh-riot-key [options]

Updates RIOT_API_KEY in the server-local WinRift env file, clears the Riot auth
failure marker, recreates the API, and starts the collector worker.

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

echo "Stopping worker before recreating API."
compose stop worker >/dev/null 2>&1 || true

echo "Starting ClickHouse if needed."
compose up -d --no-build clickhouse

echo "Recreating API with refreshed environment."
compose up -d --no-deps --force-recreate --no-build api
wait_for_health

echo "Ensuring monitor is running."
compose up -d --no-deps --no-build monitor

if [[ "${START_WORKER}" -eq 1 ]]; then
  start_worker_with_retries
else
  echo "Worker start skipped by --no-worker."
fi

echo "Riot key refresh complete."
