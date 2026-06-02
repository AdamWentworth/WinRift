#!/usr/bin/env bash
set -euo pipefail

exec "${WINRIFT_REFRESH_RIOT_KEY:-/srv/winrift/refresh-riot-key}" "$@"
