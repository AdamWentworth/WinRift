#!/usr/bin/env python3
import argparse
import re
import subprocess
import sys


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Refresh WinRift's Riot API key over SSH.",
        allow_abbrev=False,
    )
    parser.add_argument("target", help="Trusted SSH host or alias, optionally prefixed with user@")
    parser.add_argument(
        "--rollover",
        action="store_true",
        help="Also advance the collector patch if needed and archive data outside the retention window",
    )
    args = parser.parse_args(argv)

    if args.target.startswith("-") or not re.fullmatch(
        r"(?:[a-zA-Z0-9_.-]+@)?[a-zA-Z0-9_.:\[\]-]+", args.target
    ):
        parser.error("target must be an SSH host, alias, or user@host")
    if not sys.stdin.isatty() or not sys.stdout.isatty():
        parser.error("run this command in an interactive terminal")

    command = [
        "ssh", "-t", "-o", "StrictHostKeyChecking=yes",
        args.target, "/srv/winrift/refresh-riot-key",
    ]
    if not args.rollover:
        command.append("--no-patch-rollover")

    try:
        return subprocess.run(command, check=False).returncode
    except FileNotFoundError:
        parser.exit(1, "OpenSSH client (ssh) is required on PATH.\n")
    except KeyboardInterrupt:
        return 130


if __name__ == "__main__":
    sys.exit(main())
