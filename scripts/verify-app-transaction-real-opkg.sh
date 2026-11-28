#!/bin/bash
# CI-only: real Entware OPKG, actual Profiling IPKs, synthetic Core service.
# All package mutations occur in a disposable networkless container.
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT="$PWD"
OLD=bf8c6d5899a9b5a9db2088adc7511279d0ac6e73
SHA="$(git rev-parse HEAD)"
[[ "$SHA" =~ ^[0-9a-f]{40}$ ]]
[ "${GITHUB_ACTIONS:-}" = true ] && [ "${RUNNER_OS:-}" = Linux ] && [ "${GITHUB_SHA:-}" = "$SHA" ] || { echo 'CI_DISPOSABLE_RUNNER_REQUIRED'; exit 1; }
[ "$(uname -m)" = x86_64 ]
command -v docker
WORK="$(mktemp -d "${RUNNER_TEMP:?}/rf-real-opkg.XXXXXXXX")"
LAB="$WORK/lab"
mkdir -m 0755 "$LAB" "$WORK/old" "$WORK/new"
NAME="rf-opkg-acceptance-${GITHUB_RUN_ID:?}-${GITHUB_RUN_ATTEMPT:?}"
cleanup() {
    docker rm -f "$NAME" >/dev/null 2>&1 || true
    rm -rf -- "$WORK"
}
trap cleanup EXIT
# The old package's own prerm/postinst are taken from the exact pre-C7 tree;
# the new one uses the actual candidate packager, not handcrafted controls.
for side in old new; do
    ref="$SHA"; version="1~dev.r2.${SHA:0:12}"
    if [ "$side" = old ]; then ref="$OLD"; version=1~dev.r1.bf8c6d5899a9; fi
    git archive "$ref" scripts/build-module-opkg.sh scripts/target-env.sh modules/profiling/packaging LICENSE | tar -x -C "$WORK/$side"
    ROUTERFORGE_TARGET=aarch64-3.10 sh "$WORK/$side/scripts/build-module-opkg.sh" profiling "$version"
    cp "$WORK/$side/dist/routerforge-profiling_${version}_aarch64-3.10.ipk" "$LAB/$side.ipk"
done
python3 - "$LAB" <<'PY'
from pathlib import Path
import io, tarfile, sys
p=Path(sys.argv[1])
def archive(files):
    out=io.BytesIO()
    with tarfile.open(fileobj=out,mode='w:gz') as t:
        for name,data in files.items():
            i=tarfile.TarInfo('./'+name);i.size=len(data);i.mode=0o644;i.uid=i.gid=0
            t.addfile(i,io.BytesIO(data))
    return out.getvalue()
control=b'Package: routerforge-core\nVersion: 1~dev.r1.bf8c6d5899a9\nArchitecture: aarch64-3.10\nSection: admin\nPriority: optional\nMaintainer: RouterForge CI fixture\nDescription: Dependency fixture only; not the real Core\n'
(p/'core-fixture.ipk').write_bytes(archive({'debian-binary':b'2.0\n','control.tar.gz':archive({'control':control}),'data.tar.gz':archive({'opt/share/rf-ci-core-fixture':b'not a Core binary\n'})}))
(p/'rf-isolated-lab-v1').write_text('fixture only\n')
PY
cat > "$LAB/service-fixture.sh" <<'SH'
#!/bin/sh
set -eu
root=/opt/var/rf-acceptance
case "$1" in
 stop)
    rm -f "$root/ready"
    printf 'stop fd3=%s\n' "$(test -e /proc/$$/fd/3 && echo open || echo closed)" >> "$root/service.log"
    ;;
 start)
    if [ "${RF_APP_TRANSACTION_LEASE_FD:-}" = 3 ] && [ ! -e /proc/$$/fd/3 ]; then
        # New postinst has closed the inherited descriptor before starting us.
        case "${RF_APP_TRANSACTION_PROFILING_INTENT:-}" in
            on) test -f /opt/etc/routerforge/profiling.enabled && test ! -s /opt/etc/routerforge/profiling.enabled ;;
            off) test ! -e /opt/etc/routerforge/profiling.enabled ;;
            *) exit 4 ;;
        esac
        printf 'new-postinst start fd3=closed intent=%s\n' "$RF_APP_TRANSACTION_PROFILING_INTENT" >> "$root/service.log"
        : > "$root/entered"
        n=0
        while [ ! -e "$root/continue" ]; do
            n=$((n+1)); [ "$n" -lt 60 ] || exit 5; sleep 1
        done
        [ "${RF_LAB_CASE:-}" != failed-start ] || exit 6
        # Real long-lived child, without explicitly closing fd3 here. The
        # production adapter's /proc audit must find no inherited lease.
        sleep 90 </dev/null >/dev/null 2>&1 &
        printf '%s\n' "$!" > "$root/daemon-pid"
    else
        # Old prerm or non-protocol invocation. Do not spawn a real legacy Core.
        printf 'legacy start fd3=%s protocol=%s\n' "$(test -e /proc/$$/fd/3 && echo open || echo closed)" "${RF_APP_TRANSACTION_LEASE_FD:-none}" >> "$root/service.log"
    fi
    printf 'ready\n' > "$root/ready"
    ;;
 *) exit 7 ;;
esac
SH
chmod 0755 "$LAB/service-fixture.sh"
# Official installer binary; HTTPS origin and published byte length checked.
# No independent upstream SHA256 is available to this runner. Record the
# observed hash; do not mislabel it as an independently pinned trust proof.
curl --proto '=https' --tlsv1.2 --fail --silent --show-error --max-time 90 \
    https://bin.entware.net/x64-k3.2/installer/opkg --output "$LAB/opkg"
[ "$(wc -c < "$LAB/opkg")" -eq 1147704 ] || { echo 'ENTWARE_BOOTSTRAP_OPKG_SIZE_DRIFT'; exit 1; }
readelf -h "$LAB/opkg" | grep -F 'Advanced Micro Devices X86-64'
if readelf -l "$LAB/opkg" | grep -q 'INTERP'; then echo 'ENTWARE_BOOTSTRAP_NOT_STATIC'; exit 1; fi
sha256sum "$LAB/opkg" "$LAB/old.ipk" "$LAB/new.ipk" "$LAB/core-fixture.ipk"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -tags rf_opkg_acceptance -o "$LAB/acceptance.test" ./internal/apptransaction
chmod 0755 "$LAB/acceptance.test"
# No source tree, credentials, docker socket, network, device or host /opt
# mounted in the execution container. All writable state is ephemeral tmpfs.
docker pull ubuntu:24.04
docker image inspect --format '{{.Id}} {{json .RepoDigests}}' ubuntu:24.04
for mode in on off worker-crash corrupt failed-start; do
    echo "=== REAL OPKG ACCEPTANCE / $mode ==="
    docker run --rm --name "$NAME" --init --pull never --network none \
        --read-only --cap-drop ALL --security-opt no-new-privileges \
        --pids-limit 128 --memory 512m --cpus 2 \
        --tmpfs /opt:rw,exec,nosuid,nodev,size=128m,mode=0755 \
        --tmpfs /tmp:rw,exec,nosuid,nodev,size=32m,mode=1777 \
        --tmpfs /run:rw,nosuid,nodev,size=16m,mode=0755 \
        --mount "type=bind,src=$LAB,dst=/lab,readonly" \
        -e RF_OPKG_ACCEPTANCE=isolated-ci-v1 -e "RF_LAB_COMMIT=$SHA" -e "RF_LAB_CASE=$mode" \
        -e PATH=/opt/bin:/usr/bin:/bin ubuntu:24.04 \
        /usr/bin/timeout -k 10s 140s /lab/acceptance.test -test.run='^TestRealOPKGAcceptance$' -test.v -test.timeout=120s
done
echo 'REAL_ENTWARE_OPKG_ACCEPTANCE=PASS / AMD64 CONTAINER / ACTUAL PROFILING IPKS / FIXTURE CORE'
