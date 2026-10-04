#!/usr/bin/env python3
"""Run the wodin/go-qrcode `qrcode` tool, installing it first if need be.

    qrcode.py [the tool's own flags] CONTENT
    qrcode.py --locate

Every argument is handed to the tool unchanged, and its output and exit status
come back unchanged. --locate prints the path of the binary that would run.

The tool is looked for, in order: where $QRCODE_BIN points, on PATH, among the
binaries bundled with the skill, in the cache, and in the GitHub release. An
upstream skip2/go-qrcode binary is passed over wherever it is found: it shares
the name and has no logo support. Only the standard library is used.
"""

import hashlib
import io
import os
import platform
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
import zipfile
from pathlib import Path

VERSION = "v0.1.0"
RELEASE = "https://github.com/wodin/go-qrcode/releases/download/" + VERSION

# Exit status when there is no tool to run: the shell's "command not found",
# which the tool itself never returns.
NOT_INSTALLED = 127

SYSTEMS = {"Linux": "linux", "Darwin": "darwin", "Windows": "windows"}
MACHINES = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}


def target():
    """The release's name for this platform, such as linux-amd64, or None."""
    system = SYSTEMS.get(platform.system())
    machine = MACHINES.get(platform.machine().lower())

    return "%s-%s" % (system, machine) if system and machine else None


def executable_name():
    return "qrcode.exe" if platform.system() == "Windows" else "qrcode"


def supports_logos(binary):
    """Whether binary runs here and is this fork's tool, not upstream's."""
    try:
        usage = subprocess.run([str(binary), "-h"], capture_output=True, timeout=30)
    except (OSError, subprocess.SubprocessError):
        return False

    return b"logo-clearing" in usage.stdout + usage.stderr


def cache_directory():
    """Somewhere writable that survives the run, or failing that the run's own."""
    home = os.environ.get("XDG_CACHE_HOME") or os.path.join(Path.home(), ".cache")

    for directory in (Path(home, "go-qrcode", VERSION),
                      Path(tempfile.gettempdir(), "go-qrcode-" + VERSION)):
        try:
            directory.mkdir(parents=True, exist_ok=True)
        except OSError:
            continue
        if os.access(directory, os.W_OK):
            return directory

    return None


def bundled(cache):
    """The binary shipped in the skill's bin/ for this platform, runnable.

    A skill is often unpacked without its execute bits, or onto a read-only
    mount, so a binary that does not run where it lies is copied to the cache.
    """
    name = "qrcode-%s%s" % (target(), Path(executable_name()).suffix)
    shipped = Path(__file__).resolve().parent.parent / "bin" / name

    if not shipped.is_file():
        return None
    if supports_logos(shipped):
        return shipped
    if cache is None:
        return None

    copy = cache / executable_name()
    shutil.copyfile(shipped, copy)
    copy.chmod(copy.stat().st_mode | stat.S_IXUSR)

    return copy if supports_logos(copy) else None


def fetch(url):
    with urllib.request.urlopen(url, timeout=60) as response:
        return response.read()


def downloaded(cache):
    """The release's binary for this platform, verified and unpacked."""
    windows = platform.system() == "Windows"
    archive = "qrcode-%s-%s.%s" % (VERSION, target(), "zip" if windows else "tar.gz")

    sums = fetch(RELEASE + "/SHA256SUMS").decode()
    expected = {name: digest for digest, name in
                (line.split() for line in sums.splitlines() if line.strip())}

    data = fetch(RELEASE + "/" + archive)
    if hashlib.sha256(data).hexdigest() != expected.get(archive):
        raise OSError("%s does not match its entry in SHA256SUMS" % archive)

    member = "%s/%s" % (archive.rsplit(".zip" if windows else ".tar.gz", 1)[0],
                        executable_name())
    if windows:
        binary = zipfile.ZipFile(io.BytesIO(data)).read(member)
    else:
        binary = tarfile.open(fileobj=io.BytesIO(data)).extractfile(member).read()

    path = cache / executable_name()
    path.write_bytes(binary)
    path.chmod(path.stat().st_mode | stat.S_IXUSR)

    return path


def locate():
    """The tool's path, or an explanation of why there is none."""
    named = os.environ.get("QRCODE_BIN")
    if named:
        if supports_logos(named):
            return named, None
        return None, ("QRCODE_BIN is %s, which does not run here or is not the "
                      "wodin/go-qrcode tool" % named)

    on_path = shutil.which("qrcode")
    if on_path and supports_logos(on_path):
        return on_path, None

    if target() is None:
        return None, "no binary is published for %s %s" % (
            platform.system(), platform.machine())

    cache = cache_directory()

    shipped = bundled(cache)
    if shipped:
        return str(shipped), None

    if cache is None:
        return None, "nowhere writable to install the tool to"

    cached = cache / executable_name()
    if cached.is_file() and supports_logos(cached):
        return str(cached), None

    try:
        return str(downloaded(cache)), None
    except Exception as err:  # no network is the usual one, and any is final
        return None, "downloading the release failed: %s" % err


def explain(reason):
    """Says how to install the tool by hand, for the platform it is needed on."""
    name = target() or "<os>-<arch>"
    archive = "qrcode-%s-%s.%s" % (
        VERSION, name, "zip" if name.startswith("windows") else "tar.gz")

    print("""The qrcode tool is not installed, and could not be installed: %s.

To install it by hand, any one of:

  1. Download %s from
       https://github.com/wodin/go-qrcode/releases/tag/%s
     unpack it, and point QRCODE_BIN at the qrcode binary inside. Where this
     machine has no network access, ask the user to download that file and
     upload it to the conversation, then unpack it here.

  2. Build it, with Go installed:
       git clone https://github.com/wodin/go-qrcode
       cd go-qrcode && go build -o qrcode ./qrcode
     and point QRCODE_BIN at the result.

`go install github.com/skip2/go-qrcode/...` is not a way: that is upstream,
which has no logo support.""" % (reason, archive, VERSION), file=sys.stderr)


def main():
    binary, reason = locate()
    if binary is None:
        explain(reason)
        return NOT_INSTALLED

    if sys.argv[1:] == ["--locate"]:
        print(binary)
        return 0

    return subprocess.run([binary] + sys.argv[1:]).returncode


if __name__ == "__main__":
    sys.exit(main())
