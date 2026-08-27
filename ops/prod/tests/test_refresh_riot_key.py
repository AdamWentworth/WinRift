import json
import os
import stat
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).resolve().parents[1] / "refresh-riot-key.sh"


class RefreshRiotKeyTests(unittest.TestCase):
    def test_service_recreation_recovers_immutable_deployed_image(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            env_file = root / ".env"
            compose_file = root / "docker-compose.yml"
            deployment_dir = root / "deployments"
            fake_bin = root / "bin"
            docker_log = root / "docker.log"
            expected_image = "ghcr.io/adamwentworth/winrift-core@sha256:" + "a" * 64

            deployment_dir.mkdir()
            fake_bin.mkdir()
            env_file.write_text(
                "RIOT_API_KEY=RGAPI-old\n"
                "COLLECTOR_CURRENT_PATCH=16.17\n"
                "WINRIFT_RUNTIME_STATE_DIR=/tmp/winrift-test-runtime\n",
                encoding="utf-8",
            )
            compose_file.write_text("services: {}\n", encoding="utf-8")
            (deployment_dir / "core.json").write_text(
                json.dumps({"deployed_image": expected_image}),
                encoding="utf-8",
            )

            self._write_executable(
                fake_bin / "docker",
                """
                #!/usr/bin/env bash
                set -euo pipefail
                if [[ "${1:-}" == "inspect" ]]; then
                  if [[ "$*" == *".Config.Image"* ]]; then
                    printf '%s\n' 'ghcr.io/adamwentworth/winrift-core:latest'
                  fi
                  exit 0
                fi
                if [[ "${1:-}" == "compose" ]]; then
                  printf '%s\t%s\n' "${WINRIFT_CORE_IMAGE:-}" "$*" >>"${FAKE_DOCKER_LOG}"
                  exit 0
                fi
                exit 0
                """,
            )
            self._write_executable(
                fake_bin / "curl",
                """
                #!/usr/bin/env bash
                set -euo pipefail
                output_file=''
                while (($#)); do
                  if [[ "$1" == '-o' ]]; then
                    output_file="$2"
                    shift 2
                  else
                    shift
                  fi
                done
                printf '%s' '{"riotApi":"ok","status":"ok"}' >"${output_file}"
                printf '%s' '200'
                """,
            )

            command = [
                "bash",
                str(SCRIPT_PATH),
                "--env-file",
                str(env_file),
                "--deploy-root",
                str(root),
                "--compose-file",
                str(compose_file),
                "--health-url",
                "http://127.0.0.1:8000/api/health",
                "--stdin",
                "--yes",
                "--no-patch-rollover",
                "--no-worker",
            ]
            process_env = os.environ.copy()
            process_env.update(
                {
                    "PATH": f"{fake_bin}{os.pathsep}{process_env['PATH']}",
                    "FAKE_DOCKER_LOG": str(docker_log),
                    "WINRIFT_WORKER_START_CHECK_SECONDS": "1",
                    "WINRIFT_WORKER_START_RETRY_DELAY": "1",
                }
            )
            result = subprocess.run(
                command,
                input="RGAPI-refreshed-test-key-1234567890\n",
                text=True,
                capture_output=True,
                env=process_env,
                check=False,
            )

            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn(
                f"Preserving WinRift core image during service recreation: {expected_image}",
                result.stdout,
            )
            compose_calls = docker_log.read_text(encoding="utf-8").splitlines()
            self.assertGreaterEqual(len(compose_calls), 5)
            for call in compose_calls:
                selected_image, _ = call.split("\t", 1)
                self.assertEqual(selected_image, expected_image)

    @staticmethod
    def _write_executable(path, contents):
        path.write_text(textwrap.dedent(contents).lstrip(), encoding="utf-8", newline="\n")
        path.chmod(path.stat().st_mode | stat.S_IXUSR)


if __name__ == "__main__":
    unittest.main()
