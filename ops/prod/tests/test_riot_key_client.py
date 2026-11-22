import json
import runpy
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


client = runpy.run_path(str(Path(__file__).resolve().parents[1] / "riot_key_client.py"))
Target = client["Target"]
UpdaterError = client["UpdaterError"]
TARGET = Target.parse("192.0.2.77", "operator", "winrift-host")
TEST_KEY = "RGAPI-fake-key-for-transport-tests"


class RiotKeyClientTests(unittest.TestCase):
    def test_rejects_incomplete_addresses_and_shell_syntax(self):
        for fields in ((".77", "operator", "host"), ("fe80::1%bad", "operator", "host"),
                       ("192.0.2.77", "-a", "host"), ("192.0.2.77", "operator", "host; no")):
            with self.subTest(fields=fields), self.assertRaises(UpdaterError):
                Target.parse(*fields)

    def test_settings_store_only_the_target(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config" / "saved-server.json"
            self.assertIsNone(client["load_target"](path))
            client["save_target"](TARGET, path)
            self.assertEqual(client["load_target"](path), TARGET)
            self.assertEqual(json.loads(path.read_text()), {
                "address": "192.0.2.77", "username": "operator", "hostname": "winrift-host",
            })
            self.assertEqual(list(path.parent.iterdir()), [path])

    def test_wrong_server_is_rejected_before_sending_key(self):
        with patch("subprocess.run", return_value=subprocess.CompletedProcess([], 0, "wrong-host\n", "")) as run:
            with self.assertRaisesRegex(UpdaterError, "did not identify itself"):
                client["update_key"](TARGET, TEST_KEY)
        run.assert_called_once()
        self.assertNotIn("input", run.call_args.kwargs)

    def test_untrusted_server_is_rejected_before_sending_key(self):
        result = subprocess.CompletedProcess([], 255, "", "Host key verification failed.")
        with patch("subprocess.run", return_value=result) as run:
            with self.assertRaisesRegex(UpdaterError, "could not verify"):
                client["update_key"](TARGET, TEST_KEY)
        run.assert_called_once()
        self.assertNotIn("input", run.call_args.kwargs)

    def test_key_is_delivered_on_stdin_after_host_verification(self):
        for rollover in (False, True):
            responses = [
                subprocess.CompletedProcess([], 0, "winrift-host\n", ""),
                subprocess.CompletedProcess([], 0, "Riot key refresh complete.\n", ""),
            ]
            with self.subTest(rollover=rollover), patch("subprocess.run", side_effect=responses) as run:
                client["update_key"](TARGET, TEST_KEY, rollover=rollover)
                check, update = run.call_args_list
                self.assertEqual(check.kwargs["stdin"], subprocess.DEVNULL)
                args = update.args[0]
                self.assertEqual(args[-4:-1], ["-l", "operator", "192.0.2.77"])
                self.assertIn("StrictHostKeyChecking=yes", args)
                self.assertIn("hostname -s", args[-1])
                self.assertIn("--stdin --yes", args[-1])
                self.assertEqual("--no-patch-rollover" in args[-1], not rollover)
                self.assertNotIn(TEST_KEY, " ".join(args))
                self.assertEqual(update.kwargs["input"], TEST_KEY + "\n")

    def test_helper_output_is_not_exposed_on_failure(self):
        responses = [
            subprocess.CompletedProcess([], 0, "winrift-host\n", ""),
            subprocess.CompletedProcess([], 1, "", TEST_KEY),
        ]
        with patch("subprocess.run", side_effect=responses), self.assertRaises(UpdaterError) as error:
            client["update_key"](TARGET, TEST_KEY)
        self.assertNotIn(TEST_KEY, str(error.exception))
