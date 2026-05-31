#!/usr/bin/env bash
set -euo pipefail

base_url="${1:-${WINRIFT_PERF_BASE_URL:-http://127.0.0.1:8000}}"
base_url="${base_url%/}"
timeout_seconds="${WINRIFT_PERF_TIMEOUT_SECONDS:-15}"
warmups="${WINRIFT_PERF_WARMUPS:-1}"
runs="${WINRIFT_PERF_RUNS:-3}"
strict_thresholds="${WINRIFT_PERF_STRICT:-0}"
perf_patch="${WINRIFT_PERF_PATCH:-16.10}"
jsonl_path="${WINRIFT_PERF_JSONL:-}"

failures=0
warnings=0

if [[ -n "${jsonl_path}" ]]; then
  : >"${jsonl_path}"
fi

check_endpoint() {
  local name="$1"
  local path="$2"
  local max_total_ms="$3"
  local min_bytes="$4"
  local url="${base_url}${path}"
  local body_file metrics status ttfb total size total_ms ttfb_ms
  local total_values=()
  local ttfb_values=()
  local size_values=()
  local max_observed_ms=0
  local min_observed_bytes=0
  local avg_total_ms avg_ttfb_ms max_ttfb_ms

  body_file="$(mktemp)"
  trap 'rm -f "${body_file}"' RETURN

  for _ in $(seq 1 "${warmups}"); do
    curl -fsS --max-time "${timeout_seconds}" -o /dev/null "${url}" >/dev/null || true
  done

  for _ in $(seq 1 "${runs}"); do
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

    total_values+=("${total_ms}")
    ttfb_values+=("${ttfb_ms}")
    size_values+=("${size}")
    if (( total_ms > max_observed_ms )); then
      max_observed_ms="${total_ms}"
    fi
    if (( min_observed_bytes == 0 || size < min_observed_bytes )); then
      min_observed_bytes="${size}"
    fi
  done

  avg_total_ms="$(printf '%s\n' "${total_values[@]}" | awk '{ sum += $1 } END { printf "%.0f", sum / NR }')"
  avg_ttfb_ms="$(printf '%s\n' "${ttfb_values[@]}" | awk '{ sum += $1 } END { printf "%.0f", sum / NR }')"
  max_ttfb_ms="$(printf '%s\n' "${ttfb_values[@]}" | sort -n | tail -1)"

  if [[ -n "${jsonl_path}" ]]; then
    printf '{"name":"%s","url":"%s","runs":%s,"avgTotalMs":%s,"maxTotalMs":%s,"avgTtfbMs":%s,"maxTtfbMs":%s,"minBytes":%s,"thresholdMs":%s}\n' \
      "${name}" "${url}" "${runs}" "${avg_total_ms}" "${max_observed_ms}" "${avg_ttfb_ms}" "${max_ttfb_ms}" "${min_observed_bytes}" "${max_total_ms}" >>"${jsonl_path}"
  fi

  if (( max_observed_ms > max_total_ms )); then
    printf 'WARN %-28s runs=%s avg=%sms max=%sms avg_ttfb=%sms max_ttfb=%sms threshold=%sms min_bytes=%s\n' "${name}" "${runs}" "${avg_total_ms}" "${max_observed_ms}" "${avg_ttfb_ms}" "${max_ttfb_ms}" "${max_total_ms}" "${min_observed_bytes}"
    warnings=$((warnings + 1))
    return
  fi

  printf 'OK   %-28s runs=%s avg=%sms max=%sms avg_ttfb=%sms max_ttfb=%sms threshold=%sms min_bytes=%s\n' "${name}" "${runs}" "${avg_total_ms}" "${max_observed_ms}" "${avg_ttfb_ms}" "${max_ttfb_ms}" "${max_total_ms}" "${min_observed_bytes}"
}

printf 'WinRift perf smoke base_url=%s patch=%s warmups=%s runs=%s strict_thresholds=%s\n' "${base_url}" "${perf_patch}" "${warmups}" "${runs}" "${strict_thresholds}"

while IFS='|' read -r name path max_total_ms min_bytes; do
  [[ -z "${name}" || "${name}" =~ ^# ]] && continue
  check_endpoint "${name}" "${path}" "${max_total_ms}" "${min_bytes}"
done <<CHECKS
Health|/api/health|250|2
Patch list|/api/analytics/patches?queueId=420|500|100
Summoner leaderboard|/api/summoners/leaderboard?platform=NA1&limit=50|750|1000
Champion role rates|/api/analytics/champion-roles?championIds=266,62,103,64,421&queueId=420|500|100
Champion guide index|/api/analytics/champion-guides?patch=${perf_patch}&rankBucket=ALL&minGames=1|1000|1000
Aatrox champion page|/api/analytics/champion-page?championId=266&role=TOP&patch=${perf_patch}&rankBucket=ALL&minGames=5&championMinGames=5&guideMinGames=5&guideLimit=25&indexMinGames=1&indexLimit=200&queueId=420|500|1000
Kled matchup page|/api/analytics/champion-page?championId=240&role=TOP&opponentChampionId=122&patch=${perf_patch}&minGames=3&championMinGames=10&guideMinGames=5&guideLimit=12&indexMinGames=1&indexLimit=250&queueId=420&limit=4|750|1000
Lee Sin matchup page|/api/analytics/champion-page?championId=64&role=JUNGLE&opponentChampionId=200&patch=${perf_patch}&minGames=3&championMinGames=10&guideMinGames=5&guideLimit=12&indexMinGames=1&indexLimit=250&queueId=420&limit=4|750|1000
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
