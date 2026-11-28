#!/usr/bin/env python3
"""CI-only package and maintainer-script fixtures. Never use /opt on the host."""
import argparse
import io
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
LEASE = "RF_APP_TRANSACTION_LEASE_FD"
INTENT = "RF_APP_TRANSACTION_PROFILING_INTENT"


def require(ok, message):
    if not ok:
        raise RuntimeError(message)


def run_script(script, lock, env):
    return subprocess.run(["sh", "-c", 'exec 3>"$1"; exec sh "$2"', "fixture", str(lock), str(script)],
                          env=env, capture_output=True, text=True, timeout=10)


def fixtures():
    services = sorted(p for pattern in ("components/*/packaging/S*routerforge*", "modules/*/packaging/S*routerforge*") for p in ROOT.glob(pattern) if p.is_file())
    require(len(services) == 11, f"unexpected RouterForge init script count: {len(services)}")
    with tempfile.TemporaryDirectory(prefix="rf-c7-") as tmp:
        d = Path(tmp)
        env = {"PATH": os.environ.get("PATH", "/usr/bin:/bin"), LEASE: "3", INTENT: "off"}
        for service in services:
            body = service.read_text(encoding="utf-8")
            require(body.startswith("#!/bin/sh\n# RF cooperative OPKG:"), f"FD close prefix missing: {service}")
            start = body.index('if [ "${RF_APP_TRANSACTION_LEASE_FD:-}" = 3 ]; then\n')
            end = body.index("\nfi\n", start) + len("\nfi\n")
            require(start < 200, f"FD close must be before service work: {service}")
            subprocess.run(["sh", "-n", str(service)], check=True)
            prefix = d / "prefix.sh"
            prefix.write_text("#!/bin/sh\n" + body[start:end] +
                              'test ! -e /proc/$$/fd/3 && test -z "${RF_APP_TRANSACTION_LEASE_FD:-}${RF_APP_TRANSACTION_PROFILING_INTENT:-}"\n', encoding="utf-8")
            require(run_script(prefix, d / "lock", env).returncode == 0, f"FD or environment leaked: {service}")
        print("C7_INIT_FD_CLOSE_FIXTURES=PASS", flush=True)
        opt = d / "opt"
        config_dir = opt / "etc/routerforge"
        init_dir = opt / "etc/init.d"
        config_dir.mkdir(parents=True)
        init_dir.mkdir(parents=True)
        marker = config_dir / "profiling.enabled"
        config = config_dir / "profiling.conf"
        config.write_bytes(b"listen=127.0.0.1:6061\nslow_ms=900\n")
        original_config = config.read_bytes()
        log = d / "service.log"
        service = init_dir / "S90routerforge"
        service.write_text('''#!/bin/sh
set -eu
if [ "${RF_APP_TRANSACTION_LEASE_FD:-}" = 3 ]; then
    test ! -e /proc/$$/fd/3
fi
printf '%s\\n' "$1" >> "$RF_FIXTURE_LOG"
if [ "$1" = start ]; then
    case "$RF_EXPECT_INTENT" in
        on) test -f "$RF_FIXTURE_MARKER" && test ! -s "$RF_FIXTURE_MARKER" ;;
        off) test ! -e "$RF_FIXTURE_MARKER" ;;
        *) exit 2 ;;
    esac
    test "${RF_FAIL_START:-0}" != 1
fi
''', encoding="utf-8")
        service.chmod(0o700)
        post = d / "postinst"
        raw = (ROOT / "modules/profiling/packaging/postinst").read_text(encoding="utf-8")
        post.write_text(raw.replace("/opt/", str(opt) + "/"), encoding="utf-8")
        subprocess.run(["sh", "-n", str(post)], check=True)
        for mode, fail, wanted in [("on", False, 0), ("off", False, 0), ("off", True, 1), ("invalid", False, 1), ("", False, 1)]:
            # Model the destructive OLD prerm followed by the NEW package payload.
            marker.unlink(missing_ok=True)
            marker.write_bytes(b"")
            log.unlink(missing_ok=True)
            env2 = dict(env, RF_FIXTURE_LOG=str(log), RF_FIXTURE_MARKER=str(marker), RF_EXPECT_INTENT=mode,
                        RF_FAIL_START="1" if fail else "0")
            env2[INTENT] = mode
            p = run_script(post, d / "lock", env2)
            require(p.returncode == wanted, f"postinst result {mode}/{fail}: {p.returncode} {p.stderr}")
            require(config.read_bytes() == original_config, "profiling configuration changed")
            if mode in ("on", "off"):
                require(marker.exists() == (mode == "on"), "marker intent lost")
                require(log.read_text(encoding="utf-8") == "stop\nstart\n", "service not restarted")
            else:
                require(not log.exists(), "invalid intent restarted service")
        # Unmanaged/legacy branch remains unchanged, not a migration proof.
        marker.write_bytes(b"")
        log.unlink(missing_ok=True)
        env2 = {"PATH": env["PATH"], "RF_FIXTURE_LOG": str(log), "RF_FIXTURE_MARKER": str(marker), "RF_EXPECT_INTENT": "on"}
        require(run_script(post, d / "lock", env2).returncode == 0 and marker.exists(), "legacy behavior changed")
        print("C7_PROFILING_MIGRATION_FIXTURES=PASS / NO REAL OPKG", flush=True)


def members(data):
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as tar:
        result = {}
        for member in tar.getmembers():
            name = member.name.removeprefix("./")
            if member.isfile():
                require(name not in result, f"duplicate tar member {name}")
                result[name] = tar.extractfile(member).read()
        return result


def verify_ipks(target):
    endian, machine = {"aarch64-3.10": (1, 183), "mips-3.4": (2, 8), "mipsel-3.4": (1, 8)}[target]
    for package in ("routerforge-core", "routerforge-profiling"):
        path = ROOT / "dist" / f"{package}_0.0.0-c7ci_{target}.ipk"
        outer = members(path.read_bytes())
        require(outer.get("debian-binary") == b"2.0\n", "invalid IPK version")
        control = members(outer["control.tar.gz"])
        fields = control["control"].decode("utf-8").splitlines()
        for expected in (f"Package: {package}", f"Architecture: {target}", "Version: 0.0.0-c7ci"):
            require(fields.count(expected) == 1, f"IPK metadata mismatch: {expected}")
        payload = members(outer["data.tar.gz"])
        if package == "routerforge-core":
            binary = payload["opt/bin/routerforge"]
            require(len(binary) > 20 and binary[:4] == b"\x7fELF" and binary[5] == endian and
                    int.from_bytes(binary[18:20], "little" if endian == 1 else "big") == machine,
                    f"wrong Core ELF target: {target}")
            init = ROOT / "components/core/packaging/S90routerforge"
            require(payload["opt/etc/init.d/S90routerforge"] == init.read_bytes(), "Core init script mismatch")
        else:
            post = ROOT / "modules/profiling/packaging/postinst"
            require(control["postinst"] == post.read_bytes(), "Profiling postinst mismatch")
            require(payload["opt/etc/routerforge/profiling.enabled"] == b"", "Profiling package marker mismatch")
        print(f"C7_IPK_PAYLOAD=PASS {package} {target}", flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--ipk-target", choices=("aarch64-3.10", "mips-3.4", "mipsel-3.4"))
    args = parser.parse_args()
    if args.ipk_target:
        verify_ipks(args.ipk_target)
    else:
        fixtures()
