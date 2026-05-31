#!/usr/bin/env bash
set -euo pipefail

base_url="${1:-${WINRIFT_PERF_BASE_URL:-http://127.0.0.1:8000}}"
base_url="${base_url%/}"
timeout_seconds="${WINRIFT_PERF_TIMEOUT_SECONDS:-15}"
warmups="${WINRIFT_PERF_WARMUPS:-1}"
strict_thresholds="${WINRIFT_PERF_STRICT:-0}"

failures=0
warnings=0

check_endpoint() {
  local name="$1"
  local path="$2"
  local max_total_ms="$3"
  local min_bytes="$4"
  local url="${base_url}${path}"
  local body_file metrics status ttfb total size total_ms ttfb_ms

  body_file="$(mktemp)"
  trap 'rm -f "${body_file}"' RETURN

  for _ in $(seq 1 "${warmups}"); do
    curl -fsS --max-time "${timeout_seconds}" -o /dev/null "${url}" >/dev/null || true
  done

  if ! metrics="$(curl -sS --max-time "${timeout_seconds}" -o "${body_file}" -w '%{http_code} %{time_starttransfer} %{time_total} %{size_download}' "${url}")"; then
    printf 'FAIL %-28s request failed url=%s\n' "${name}" "${url}" >&2
    failures=$((failures + 1))
    return
  fi

  read -r status ttfb total size <<<"${metrics}"
  total_ms="$(awk -v seconds="${total}" 'BEGIN { printf "%.0f", seconds * 1000 }')"
  ttfb_ms="$(awk -v seconds="${ttfb}" 'BEGIN { printf "%.0f", seconds * 1000 }')"

  if [[ "${status}" != "200" ]]; then
    printf 'FAIL %-28s status=%s total=%sms ttfb=%sms bytes=%s url=%s\n' "${name}" "${status}" "${total_ms}" "${ttfb_ms}" "${size}" "${url}" >&2
    failures=$((failures + 1))
    return
  fi

  if (( size < min_bytes )); then
    printf 'FAIL %-28s tiny-response status=%s total=%sms ttfb=%sms bytes=%s min_bytes=%s url=%s\n' "${name}" "${status}" "${total_ms}" "${ttfb_ms}" "${size}" "${min_bytes}" "${url}" >&2
    failures=$((failures + 1))
    return
  fi

  if (( total_ms > max_total_ms )); then
    printf 'WARN %-28s status=%s total=%sms ttfb=%sms threshold=%sms bytes=%s\n' "${name}" "${status}" "${total_ms}" "${ttfb_ms}" "${max_total_ms}" "${size}"
    warnings=$((warnings + 1))
    return
  fi

  printf 'OK   %-28s status=%s total=%sms ttfb=%sms threshold=%sms bytes=%s\n' "${name}" "${status}" "${total_ms}" "${ttfb_ms}" "${max_total_ms}" "${size}"
}

printf 'WinRift perf smoke base_url=%s warmups=%s strict_thresholds=%s\n' "${base_url}" "${warmups}" "${strict_thresholds}"

while IFS='|' read -r name path max_total_ms min_bytes; do
  [[ -z "${name}" || "${name}" =~ ^# ]] && continue
  check_endpoint "${name}" "${path}" "${max_total_ms}" "${min_bytes}"
done <<'CHECKS'
Health|/api/health|250|2
Summoner leaderboard|/api/summoners/leaderboard?platform=NA1&limit=50|750|1000
Champion guide index|/api/analytics/champion-guides?patch=16.10&rankBucket=ALL&minGames=1|1000|1000
Aatrox champion page|/api/analytics/champion-page?championId=266&role=TOP&patch=16.10&rankBucket=ALL&minGames=5&championMinGames=5&guideMinGames=5&guideLimit=25&indexMinGames=1&indexLimit=200&queueId=420|500|1000
CHECKS

if (( failures > 0 )); then
  printf 'Perf smoke failed: failures=%d warnings=%d\n' "${failures}" "${warnings}" >&2
  exit 1
fi

if (( warnings > 0 )); then
  printf 'Perf smoke completed with threshold warnings=%d\n' "${warnings}"
  if [[ "${strict_thresholds}" == "1" || "${strict_thresholds}" == "true" ]]; then
    exit 1
  fi
fi

printf 'Perf smoke passed: failures=0 warnings=%d\n' "${warnings}"
