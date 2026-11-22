import ipaddress
import json
import re
import shlex
import subprocess
import tempfile
from dataclasses import asdict, dataclass
from pathlib import Path


SETTINGS_PATH = Path.home() / ".winrift" / "riot-key-target.json"
HELPER = "/srv/winrift/refresh-riot-key"


class UpdaterError(Exception):
    pass


@dataclass(frozen=True)
class Target:
    address: str
    username: str
    hostname: str

    @classmethod
    def parse(cls, address, username, hostname):
        address, username, hostname = address.strip(), username.strip(), hostname.strip()
        try:
            if "%" in address:
                raise ValueError
            address = str(ipaddress.ip_address(address))
        except ValueError:
            raise UpdaterError("Enter the server’s full IP address.") from None
        if not re.fullmatch(r"[A-Za-z0-9_][A-Za-z0-9_.-]*", username):
            raise UpdaterError("Enter your SSH username on the WinRift server.")
        if not re.fullmatch(r"[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?", hostname):
            raise UpdaterError("Enter the server’s short machine name, such as adam-ubuntu.")
        return cls(address, username, hostname)


def load_target(path=SETTINGS_PATH):
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        return None
    if not isinstance(data, dict) or not all(
        isinstance(data.get(field), str) for field in ("address", "username", "hostname")
    ):
        raise ValueError("Invalid server settings")
    return Target.parse(data["address"], data["username"], data["hostname"])


def save_target(target, path=SETTINGS_PATH):
    path.parent.mkdir(parents=True, exist_ok=True)
    file = tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=path.parent, delete=False)
    temp_path = Path(file.name)
    try:
        with file:
            json.dump(asdict(target), file, indent=2)
            file.write("\n")
        temp_path.replace(path)
    finally:
        temp_path.unlink(missing_ok=True)


def validate_key(key):
    key = key.strip()
    if not re.fullmatch(r"RGAPI-[A-Za-z0-9_-]{14,250}", key):
        raise UpdaterError("Paste a complete Riot API key beginning with RGAPI-.")
    return key


def run_ssh(target, command, *, key=None):
    args = [
        "ssh", "-T", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes",
        "-o", "ConnectTimeout=8", "-o", "ServerAliveInterval=10", "-o", "ServerAliveCountMax=3",
        "-o", "ClearAllForwardings=yes", "-a", "-x", "-l", target.username, target.address, command,
    ]
    input_options = {"stdin": subprocess.DEVNULL, "timeout": 15} if key is None else {"input": key + "\n"}
    try:
        return subprocess.run(
            args, capture_output=True, text=True, encoding="utf-8", errors="replace", check=False,
            creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0), **input_options,
        )
    except FileNotFoundError:
        raise UpdaterError("Install the OpenSSH client and make sure ssh is on PATH.") from None
    except subprocess.TimeoutExpired:
        raise UpdaterError("The server took too long to respond. Check its IP address and network access.") from None
    except OSError:
        raise UpdaterError("The app couldn’t start SSH. Check your OpenSSH installation.") from None


def verify_target(target):
    result = run_ssh(target, f"hostname -s && {{ test -x {HELPER} || exit 72; }}")
    if result.returncode == 255:
        error = result.stderr.lower()
        if "host key verification failed" in error or "host identification has changed" in error:
            raise UpdaterError("SSH could not verify the server’s identity. Check its host key before connecting.")
        if "permission denied" in error:
            raise UpdaterError("SSH sign-in failed. Check the SSH username and your SSH key or agent.")
        raise UpdaterError("SSH couldn’t reach the server. Check its IP address and network access.")
    if result.stdout.strip() != target.hostname:
        raise UpdaterError(f"The server did not identify itself as {target.hostname}. Check the IP and machine name.")
    if result.returncode == 72:
        raise UpdaterError("The WinRift key updater is missing or cannot run on that server.")
    if result.returncode:
        raise UpdaterError("The server could not verify the WinRift key updater.")


def update_key(target, key, *, rollover=False):
    key = validate_key(key)
    verify_target(target)
    expected_host = shlex.quote(target.hostname)
    command = f'[ "$(hostname -s)" = {expected_host} ] || exit 73; exec {HELPER} --stdin --yes'
    if not rollover:
        command += " --no-patch-rollover"
    result = run_ssh(target, command, key=key)
    if result.returncode == 73:
        raise UpdaterError("The server’s machine name changed. Verify the target again before retrying.")
    if result.returncode == 255:
        raise UpdaterError("SSH disconnected during the update. Check WinRift on the server before retrying.")
    if result.returncode or "Riot key refresh complete." not in result.stdout.splitlines():
        raise UpdaterError("The server didn’t finish the update. Check WinRift and the Riot key before retrying.")
