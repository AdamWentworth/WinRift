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
        result = subprocess.CompletedProcess([], 0, "wrong-host\n", "")
        with patch("subprocess.run", return_value=result), patch("subprocess.Popen") as open_ssh:
            with self.assertRaisesRegex(UpdaterError, "did not identify itself"):
                client["update_key"](TARGET, TEST_KEY)
        open_ssh.assert_not_called()

    def test_untrusted_server_is_rejected_before_sending_key(self):
        result = subprocess.CompletedProcess([], 255, "", "Host key verification failed.")
        with patch("subprocess.run", return_value=result), patch("subprocess.Popen") as open_ssh:
            with self.assertRaisesRegex(UpdaterError, "could not verify"):
                client["update_key"](TARGET, TEST_KEY)
        open_ssh.assert_not_called()

    def test_key_is_delivered_on_stdin_after_host_verification(self):
        verified = subprocess.CompletedProcess([], 0, "winrift-host\n", "")
        for rollover in (False, True):
            with self.subTest(rollover=rollover), patch("subprocess.run", return_value=verified) as check:
                with patch("subprocess.Popen") as open_ssh:
                    process = open_ssh.return_value
                    process.stdout = iter(["Riot key refresh complete.\n"])
                    process.wait.return_value = 0
                    client["update_key"](TARGET, TEST_KEY, rollover=rollover)
                self.assertEqual(check.call_args.kwargs["stdin"], subprocess.DEVNULL)
                args = open_ssh.call_args.args[0]
                self.assertEqual(args[-4:-1], ["-l", "operator", "192.0.2.77"])
                self.assertIn("StrictHostKeyChecking=yes", args)
                self.assertIn("hostname -s", args[-1])
                self.assertIn("--stdin --yes", args[-1])
                self.assertEqual("--no-patch-rollover" in args[-1], not rollover)
                self.assertNotIn(TEST_KEY, " ".join(args))
                self.assertEqual(open_ssh.call_args.kwargs["stdin"], subprocess.PIPE)
                self.assertEqual(open_ssh.call_args.kwargs["stderr"], subprocess.STDOUT)
                process.stdin.write.assert_called_once_with(TEST_KEY + "\n")

    def test_streams_redacted_output_before_waiting_for_exit(self):
        verified = subprocess.CompletedProcess([], 0, "winrift-host\n", "")
        with patch("subprocess.run", return_value=verified), patch("subprocess.Popen") as open_ssh:
            process = open_ssh.return_value
            process.stdout = iter([
                "\x1b[32mRestarting API\x1b[0m\n",
                f"Riot key {TEST_KEY} rejected. Previous: RGAPI-another-key-for-tests\n",
            ])
            process.wait.return_value = 1
            logs = []

            def record(line):
                process.wait.assert_not_called()
                logs.append(line)

            with self.assertRaises(UpdaterError) as error:
                client["update_key"](TARGET, TEST_KEY, on_output=record)
        self.assertEqual(logs, ["Restarting API\n", "Riot key [Riot key hidden] rejected. Previous: [Riot key hidden]\n"])
        self.assertNotIn(TEST_KEY, str(error.exception))

    def test_ssh_exit_without_confirmation_is_not_success(self):
        verified = subprocess.CompletedProcess([], 0, "winrift-host\n", "")
        with patch("subprocess.run", return_value=verified), patch("subprocess.Popen") as open_ssh:
            process = open_ssh.return_value
            process.stdout = iter(["Restarting API\n"])
            process.wait.return_value = 0
            with self.assertRaisesRegex(UpdaterError, "didn’t finish"):
                client["update_key"](TARGET, TEST_KEY)
