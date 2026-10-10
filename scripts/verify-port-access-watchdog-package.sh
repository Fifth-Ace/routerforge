#!/bin/sh
set -eu
# K3I: Linux CI only, no router, iptables or NDM modifications.
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT"
echo '=== K3I: BUILD IPK ==='
ROUTERFORGE_TARGET=aarch64-3.10 sh scripts/build-port-access-manager-opkg.sh 0.0.0-k3i
PKG=dist/routerforge-port-access-manager_0.0.0-k3i_aarch64-3.10.ipk
test -s "$PKG"
tar -tzf "$PKG" | grep -qx './data.tar.gz'
tar -tzf "$PKG" | grep -qx './control.tar.gz'
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
tar -xzOf "$PKG" ./data.tar.gz > "$TMP/data.tar.gz"
for binary in routerforge-port-access-manager routerforge-port-access-watchdog; do
  tar -tzf "$TMP/data.tar.gz" | grep -qx "./opt/bin/$binary"
  tar -xzOf "$TMP/data.tar.gz" "./opt/bin/$binary" > "$TMP/$binary"
  test -s "$TMP/$binary"
  # ELF machine=183 (AArch64); EI_CLASS=2 (64-bit); verified without executing ARM binaries.
  machine="$(od -An -tu1 -j18 -N2 "$TMP/$binary" | tr -s ' ' | sed 's/^ //')"
  test "$machine" = '183 0' || { echo "INVALID_AARCH64_ELF: $binary ($machine)" >&2; exit 1; }
  magic="$(od -An -tx1 -N4 "$TMP/$binary" | tr -d ' \n')"
  test "$magic" = '7f454c46' || { echo "NOT_ELF: $binary" >&2; exit 1; }
  echo "IPK_BINARY: $binary AARCH64_PASS"
done

echo '=== K3I: NATIVE WATCHDOG SMOKE ==='
go build -o "$TMP/watchdog-native" ./modules/port-access-manager/watchdog
# Test expires after 2 seconds; rollback is a harmless marker file in temp.
mkdir -m 700 "$TMP/expired" "$TMP/confirmed"
printf '#!/bin/sh\nprintf rollback-ok > "%s"\n' "$TMP/rolled-back" > "$TMP/expired/rollback.sh"
chmod 700 "$TMP/expired/rollback.sh"
deadline="$(($(date +%s) + 2))"
"$TMP/watchdog-native" -transaction-dir "$TMP/expired" -deadline-unix "$deadline"
test "$(cat "$TMP/rolled-back")" = rollback-ok || { echo 'ROLLBACK_MARKER_FAIL' >&2; exit 1; }
echo 'TIMEOUT_ROLLBACK: PASS'
printf 'ok\n' > "$TMP/confirmed/confirmed"
printf '#!/bin/sh\nexit 97\n' > "$TMP/confirmed/rollback.sh"
chmod 700 "$TMP/confirmed/rollback.sh"
"$TMP/watchdog-native" -transaction-dir "$TMP/confirmed" -deadline-unix "$(($(date +%s) + 2))"
echo 'CONFIRMATION_SKIPS_ROLLBACK: PASS'
echo 'K3I_IPK_AND_WATCHDOG_SMOKE: PASS'
