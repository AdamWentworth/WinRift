import importlib.util
import sys
import unittest
import urllib.parse
from pathlib import Path
from unittest import mock


SCRIPT_PATH = Path(__file__).resolve().parents[1] / "champion-page-perf-audit.py"
SPEC = importlib.util.spec_from_file_location("champion_page_perf_audit", SCRIPT_PATH)
AUDIT = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = AUDIT
SPEC.loader.exec_module(AUDIT)


class ChampionPagePerfAuditTests(unittest.TestCase):
    def test_selects_newest_mature_previous_patch(self):
        patch_payload = {
            "currentPatch": "16.17",
            "results": [
                {"patch": "16.17", "matches": 2426},
                {"patch": "16.16", "matches": 62495},
                {"patch": "16.15", "matches": 3898},
                {"patch": "16.13", "matches": 350705},
            ],
        }
        with mock.patch.object(AUDIT, "fetch_json", return_value=(patch_payload, {})):
            current, fallback, selectable = AUDIT.load_patch_scope("http://api", 1, "")

        self.assertEqual(current, "16.17")
        self.assertEqual(fallback, "16.16")
        self.assertEqual(selectable, ["16.17", "16.16", "16.15", "16.13"])

    def test_builds_the_exact_frontend_canonical_cache_key_request(self):
        url = AUDIT.champion_page_url("http://api", 62, "16.17", "JUNGLE")
        query = urllib.parse.urlparse(url).query
        params = urllib.parse.parse_qs(query)

        self.assertEqual(params["championId"], ["62"])
        self.assertEqual(params["patch"], ["16.17"])
        self.assertEqual(params["role"], ["JUNGLE"])
        self.assertEqual(params["itemContext"], ["JUNGLE"])
        self.assertEqual(params["championMinGames"], ["10"])
        self.assertEqual(params["guideLimit"], ["12"])
        self.assertEqual(params["indexLimit"], ["250"])

    def test_all_patch_targets_cover_every_champion_patch_pair(self):
        champions = [AUDIT.Champion(62, "Wukong"), AUDIT.Champion(103, "Ahri")]
        targets = AUDIT.audit_targets(champions, ["16.17", "16.16", "16.15"], "16.17")

        self.assertEqual(len(targets), 6)
        self.assertEqual(
            {(champion.champion_id, patch, kind) for champion, patch, kind in targets},
            {
                (62, "16.17", "current"),
                (103, "16.17", "current"),
                (62, "16.16", "archived"),
                (103, "16.16", "archived"),
                (62, "16.15", "archived"),
                (103, "16.15", "archived"),
            },
        )

    def test_fails_slow_or_uncached_pages(self):
        result = AUDIT.AuditResult(
            champion_id=62,
            champion_name="Wukong",
            data_patch="16.17",
            request_kind="current",
            duration_ms=650,
            status=200,
            response_bytes=10000,
            cache_status="miss",
            guide_games=100,
            role="JUNGLE",
        )

        self.assertEqual(AUDIT.result_failure(result, 500, True, False), "cache=miss, expected hit")
        result.cache_status = "hit"
        self.assertEqual(AUDIT.result_failure(result, 500, True, False), "650.0ms exceeds 500ms")

    def test_requires_fallback_pages_to_contain_guide_data(self):
        result = AUDIT.AuditResult(
            champion_id=103,
            champion_name="Ahri",
            data_patch="16.16",
            request_kind="fallback",
            duration_ms=25,
            status=200,
            response_bytes=10000,
            cache_status="hit",
            guide_games=0,
            role="MIDDLE",
        )

        self.assertEqual(
            AUDIT.result_failure(result, 500, True, True),
            "fallback patch still has no guide games",
        )


if __name__ == "__main__":
    unittest.main()
