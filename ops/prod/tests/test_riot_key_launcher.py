import contextlib
import io
import runpy
import subprocess
import unittest
from pathlib import Path
from unittest.mock import patch


main = runpy.run_path(str(Path(__file__).resolve().parents[1] / "riot-key.py"))["main"]


class RiotKeyLauncherTests(unittest.TestCase):
    def test_key_only_is_default(self):
        command = ["ssh", "-t", "-o", "StrictHostKeyChecking=yes",
                   "winrift-prod", "/srv/winrift/refresh-riot-key", "--no-patch-rollover"]
        with patch("sys.stdin.isatty", return_value=True), patch("sys.stdout.isatty", return_value=True):
            with patch("subprocess.run", return_value=subprocess.CompletedProcess(command, 23)) as run:
                self.assertEqual(main(["winrift-prod"]), 23)
                run.assert_called_once_with(command, check=False)

    def test_rollover_uses_the_server_default(self):
        command = ["ssh", "-t", "-o", "StrictHostKeyChecking=yes",
                   "operator@winrift-prod", "/srv/winrift/refresh-riot-key"]
        with patch("sys.stdin.isatty", return_value=True), patch("sys.stdout.isatty", return_value=True):
            with patch("subprocess.run", return_value=subprocess.CompletedProcess(command, 0)) as run:
                self.assertEqual(main(["operator@winrift-prod", "--rollover"]), 0)
                run.assert_called_once_with(command, check=False)

    def test_rejects_ssh_options_and_abbreviated_rollover(self):
        for args in (["--", "-oProxyCommand=bad"], ["winrift-prod", "--roll"]):
            with self.subTest(args=args), contextlib.redirect_stderr(io.StringIO()), patch("subprocess.run") as run:
                with self.assertRaises(SystemExit) as error:
                    main(args)
                self.assertEqual(error.exception.code, 2)
                run.assert_not_called()

    def test_requires_interactive_input(self):
        with patch("sys.stdin.isatty", return_value=False), patch("subprocess.run") as run:
            with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as error:
                main(["winrift-prod"])
            self.assertEqual(error.exception.code, 2)
            run.assert_not_called()
