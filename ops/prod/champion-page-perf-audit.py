#!/usr/bin/env python3
"""Strict production audit for every WinRift canonical champion page."""

from __future__ import annotations

import argparse
import concurrent.futures
import json
import math
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any


DEFAULT_MAX_MS = 500
DEFAULT_CONCURRENCY = 4
DEFAULT_TIMEOUT_SECONDS = 5.0


@dataclass(frozen=True)
class Champion:
    champion_id: int
    name: str


@dataclass
class AuditResult:
    champion_id: int
    champion_name: str
    data_patch: str
    request_kind: str
    duration_ms: float
    status: int
    response_bytes: int
    cache_status: str
    guide_games: int
    role: str
    error: str = ""


def env_int(name: str, default: int) -> int:
    value = os.getenv(name, "").strip()
    return int(value) if value else default


def env_float(name: str, default: float) -> float:
    value = os.getenv(name, "").strip()
    return float(value) if value else default


def fetch_json(url: str, timeout_seconds: float) -> tuple[dict[str, Any], dict[str, str]]:
    request = urllib.request.Request(url, headers={"Accept": "application/json", "User-Agent": "WinRift-Perf-Audit/1"})
    with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
        body = response.read()
        payload = json.loads(body)
        headers = {key.lower(): value for key, value in response.headers.items()}
        return payload, headers


def load_champions(base_url: str, timeout_seconds: float) -> list[Champion]:
    payload, _ = fetch_json(f"{base_url}/api/static/champions", timeout_seconds)
    rows = payload.get("data", {}).get("data", {})
    champions = []
    for row in rows.values():
        try:
            champion_id = int(row.get("key", 0))
        except (TypeError, ValueError):
            continue
        name = str(row.get("name", "")).strip()
        if champion_id > 0 and name:
            champions.append(Champion(champion_id=champion_id, name=name))
    champions.sort(key=lambda champion: champion.name.casefold())
    if not champions:
        raise RuntimeError("static champion metadata returned no champions")
    return champions


def load_patch_scope(base_url: str, timeout_seconds: float, requested_patch: str) -> tuple[str, str, list[str]]:
    payload, _ = fetch_json(f"{base_url}/api/analytics/patches?queueId=420", timeout_seconds)
    current_patch = requested_patch.strip() or str(payload.get("currentPatch", "")).strip()
    if not current_patch:
        raise RuntimeError("analytics patch endpoint returned no current patch")

    previous = []
    for row in payload.get("results", []):
        patch = str(row.get("patch", "")).strip()
        matches = int(row.get("matches", 0) or 0)
        if patch and patch_key(patch) < patch_key(current_patch) and matches > 0:
            previous.append((patch, matches))
    previous.sort(key=lambda row: patch_key(row[0]), reverse=True)
    fallback_patch = next((patch for patch, matches in previous if matches >= 5000), "")
    if not fallback_patch and previous:
        fallback_patch = previous[0][0]
    selectable_patches = [current_patch]
    selectable_patches.extend(patch for patch, _ in previous if patch != current_patch)
    return current_patch, fallback_patch, selectable_patches


def patch_key(patch: str) -> tuple[int, int]:
    parts = patch.split(".")
    try:
        return int(parts[0]), int(parts[1])
    except (IndexError, ValueError):
        return 0, 0


def champion_page_url(base_url: str, champion_id: int, patch: str, role: str = "") -> str:
    params = {
        "championId": str(champion_id),
        "patch": patch,
        "minGames": "5",
        "championMinGames": "10",
        "limit": "4",
        "guideMinGames": "5",
        "guideLimit": "12",
        "indexMinGames": "1",
        "indexLimit": "250",
        "queueId": "420",
    }
    role = role.strip().upper()
    if role:
        params["role"] = role
        if role == "JUNGLE":
            params["itemContext"] = "JUNGLE"
        elif role == "UTILITY":
            params["itemContext"] = "SUPPORT"
    return f"{base_url}/api/analytics/champion-page?{urllib.parse.urlencode(params)}"


def audit_targets(champions: list[Champion], patches: list[str], current_patch: str) -> list[tuple[Champion, str, str]]:
    return [
        (champion, patch, "current" if patch == current_patch else "archived")
        for patch in patches
        for champion in champions
    ]


def audit_page(
    base_url: str,
    champion: Champion,
    patch: str,
    request_kind: str,
    timeout_seconds: float,
    role: str = "",
) -> AuditResult:
    url = champion_page_url(base_url, champion.champion_id, patch, role)
    started_at = time.perf_counter()
    try:
        request = urllib.request.Request(url, headers={"Accept": "application/json", "User-Agent": "WinRift-Perf-Audit/1"})
        with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
            body = response.read()
            duration_ms = (time.perf_counter() - started_at) * 1000
            status = response.status
            cache_status = response.headers.get("X-WinRift-Cache", "").strip().lower()
        payload = json.loads(body)
        filters = payload.get("filters", {})
        guide_games = int(payload.get("guide", {}).get("summary", {}).get("games", 0) or 0)
        resolved_role = str(filters.get("role", "")).strip().upper()
        required_sections = ("guide", "buildAdvice", "guideIndex", "roleRates")
        missing_sections = [section for section in required_sections if section not in payload]
        error = ""
        if int(filters.get("championId", 0) or 0) != champion.champion_id:
            error = "response championId does not match request"
        elif str(filters.get("patch", "")).strip() != patch:
            error = "response patch does not match request"
        elif missing_sections:
            error = f"missing response sections: {','.join(missing_sections)}"
        elif not resolved_role:
            error = "response did not resolve a champion role"
        return AuditResult(
            champion_id=champion.champion_id,
            champion_name=champion.name,
            data_patch=patch,
            request_kind=request_kind,
            duration_ms=round(duration_ms, 1),
            status=status,
            response_bytes=len(body),
            cache_status=cache_status,
            guide_games=guide_games,
            role=resolved_role,
            error=error,
        )
    except urllib.error.HTTPError as error:
        duration_ms = (time.perf_counter() - started_at) * 1000
        return AuditResult(champion.champion_id, champion.name, patch, request_kind, round(duration_ms, 1), error.code, 0, "", 0, role, f"HTTP {error.code}")
    except Exception as error:  # noqa: BLE001 - every request failure belongs in the audit report.
        duration_ms = (time.perf_counter() - started_at) * 1000
        return AuditResult(champion.champion_id, champion.name, patch, request_kind, round(duration_ms, 1), 0, 0, "", 0, role, str(error))


def result_failure(result: AuditResult, max_ms: int, require_cache_hit: bool, require_guide_data: bool) -> str:
    if result.error:
        return result.error
    if result.status != 200:
        return f"HTTP {result.status}"
    if result.response_bytes < 1000:
        return f"response too small ({result.response_bytes} bytes)"
    if require_cache_hit and result.cache_status != "hit":
        return f"cache={result.cache_status or 'missing'}, expected hit"
    if result.duration_ms > max_ms:
        return f"{result.duration_ms:.1f}ms exceeds {max_ms}ms"
    if require_guide_data and result.guide_games <= 0:
        return "fallback patch still has no guide games"
    return ""


def percentile(values: list[float], percentile_value: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    index = max(0, math.ceil((percentile_value / 100) * len(ordered)) - 1)
    return ordered[index]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default=os.getenv("WINRIFT_PERF_BASE_URL", "http://127.0.0.1:8000"))
    parser.add_argument("--patch", default=os.getenv("WINRIFT_PERF_PATCH", ""))
    parser.add_argument("--max-ms", type=int, default=env_int("WINRIFT_CHAMPION_PAGE_MAX_MS", DEFAULT_MAX_MS))
    parser.add_argument("--concurrency", type=int, default=env_int("WINRIFT_CHAMPION_PAGE_CONCURRENCY", DEFAULT_CONCURRENCY))
    parser.add_argument("--timeout-seconds", type=float, default=env_float("WINRIFT_PERF_TIMEOUT_SECONDS", DEFAULT_TIMEOUT_SECONDS))
    parser.add_argument("--json", default=os.getenv("WINRIFT_CHAMPION_PAGE_AUDIT_JSON", ""))
    parser.add_argument("--allow-cache-miss", action="store_true")
    parser.add_argument("--all-patches", action="store_true", help="Audit every selectable patch instead of only the current patch.")
    parser.add_argument("--champion-id", type=int, action="append", default=[], help="Audit only selected champion IDs; repeatable and intended for diagnostics.")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    base_url = args.base_url.rstrip("/")
    if args.max_ms <= 0 or args.concurrency <= 0 or args.timeout_seconds <= 0:
        raise SystemExit("max-ms, concurrency, and timeout-seconds must be positive")

    champions = load_champions(base_url, args.timeout_seconds)
    if args.champion_id:
        selected_ids = set(args.champion_id)
        champions = [champion for champion in champions if champion.champion_id in selected_ids]
        missing_ids = selected_ids.difference(champion.champion_id for champion in champions)
        if missing_ids:
            raise SystemExit(f"unknown champion IDs: {sorted(missing_ids)}")
    current_patch, fallback_patch, selectable_patches = load_patch_scope(base_url, args.timeout_seconds, args.patch)
    audited_patches = selectable_patches if args.all_patches else [current_patch]
    require_cache_hit = not args.allow_cache_miss
    print(
        f"WinRift champion-page audit base_url={base_url} current_patch={current_patch} "
        f"fallback_patch={fallback_patch or 'none'} patches={','.join(audited_patches)} "
        f"champions={len(champions)} max_ms={args.max_ms} "
        f"concurrency={args.concurrency} require_cache_hit={str(require_cache_hit).lower()}"
    )

    results: list[AuditResult] = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.concurrency) as executor:
        futures = [
            executor.submit(
                audit_page,
                base_url,
                champion,
                patch,
                request_kind,
                args.timeout_seconds,
            )
            for champion, patch, request_kind in audit_targets(champions, audited_patches, current_patch)
        ]
        for future in concurrent.futures.as_completed(futures):
            results.append(future.result())

    page_failures = [(result, result_failure(result, args.max_ms, require_cache_hit, False)) for result in results]
    page_failures = [(result, reason) for result, reason in page_failures if reason]

    fallback_candidates = [
        (champion, "")
        for champion in champions
        for result in results
        if result.data_patch == current_patch
        and result.champion_id == champion.champion_id
        and not result.error
        and result.guide_games <= 0
    ]
    if fallback_candidates and not fallback_patch:
        for champion, role in fallback_candidates:
            results.append(AuditResult(champion.champion_id, champion.name, "", "fallback", 0, 0, 0, "", 0, role, "no fallback patch available"))
    elif fallback_candidates:
        with concurrent.futures.ThreadPoolExecutor(max_workers=args.concurrency) as executor:
            futures = [
                executor.submit(audit_page, base_url, champion, fallback_patch, "fallback", args.timeout_seconds, role)
                for champion, role in fallback_candidates
            ]
            for future in concurrent.futures.as_completed(futures):
                results.append(future.result())

    failures = list(page_failures)
    for result in results:
        if result.request_kind != "fallback":
            continue
        reason = result_failure(result, args.max_ms, require_cache_hit, True)
        if reason:
            failures.append((result, reason))

    results.sort(key=lambda result: (result.request_kind, result.champion_name.casefold()))
    durations = [result.duration_ms for result in results if result.status == 200]
    cache_hits = sum(1 for result in results if result.cache_status == "hit")
    summary = {
        "baseUrl": base_url,
        "currentPatch": current_patch,
        "fallbackPatch": fallback_patch,
        "auditedPatches": audited_patches,
        "champions": len(champions),
        "requests": len(results),
        "fallbackRequests": sum(1 for result in results if result.request_kind == "fallback"),
        "cacheHits": cache_hits,
        "maxMs": max(durations, default=0),
        "p95Ms": percentile(durations, 95),
        "averageMs": sum(durations) / len(durations) if durations else 0,
        "thresholdMs": args.max_ms,
        "failures": len(failures),
        "results": [asdict(result) for result in results],
    }
    if args.json:
        report_path = Path(args.json)
        report_path.parent.mkdir(parents=True, exist_ok=True)
        report_path.write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")

    slowest = sorted(results, key=lambda result: result.duration_ms, reverse=True)[:10]
    print(
        f"requests={len(results)} fallback_requests={summary['fallbackRequests']} cache_hits={cache_hits} "
        f"avg={summary['averageMs']:.1f}ms p95={summary['p95Ms']:.1f}ms max={summary['maxMs']:.1f}ms"
    )
    print("Slowest champion pages:")
    for result in slowest:
        print(
            f"  {result.champion_name:<18} kind={result.request_kind:<8} patch={result.data_patch:<6} "
            f"role={result.role or '-':<7} total={result.duration_ms:>7.1f}ms cache={result.cache_status or '-'} bytes={result.response_bytes}"
        )

    if failures:
        print(f"Champion-page performance audit FAILED ({len(failures)} failures):", file=sys.stderr)
        for result, reason in sorted(failures, key=lambda failure: failure[0].champion_name.casefold()):
            print(
                f"  {result.champion_name} kind={result.request_kind} patch={result.data_patch or '-'}: {reason}",
                file=sys.stderr,
            )
        return 1

    print("Champion-page performance audit PASSED")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
