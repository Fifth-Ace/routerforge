#!/bin/sh
# DPR2-G: only nonsecret, structured observations. No writes, installs, probes,
# restarts, DNS queries, config updates, firewall, service or port changes.
set -u
printf 'RF_DPR2G_EVIDENCE_BEGIN\nSCHEMA=1\nMODE=READ_ONLY\n'
arch=$(uname -m 2>/dev/null || echo unknown)
case "$arch" in
  *[!A-Za-z0-9_-]*|'') arch=unknown ;;
esac
printf 'ARCH=%s\n' "$arch"
if [ -d /opt ]; then echo ENTWARE_OPT=AVAILABLE; else echo ENTWARE_OPT=UNAVAILABLE; fi
if [ -r /proc/meminfo ]; then echo PROC_MEMINFO=AVAILABLE; else echo PROC_MEMINFO=UNAVAILABLE; fi
port53=0
for p in /proc/net/udp /proc/net/udp6 /proc/net/tcp /proc/net/tcp6; do
  [ -r "$p" ] || continue
  count=$(awk 'NR>1 {split($2,a,":"); if(toupper(a[2])=="0035" && (FILENAME ~ /udp/ || $4=="0A")) n++} END{print n+0}' "$p")
  port53=$((port53 + count))
done
printf 'PORT53_ENTRIES=%s\n' "$port53"
ndmcversion=UNKNOWN
ndmcdns=UNKNOWN
if command -v ndmc >/dev/null 2>&1 && command -v timeout >/dev/null 2>&1; then
  if timeout 5 ndmc -c 'show version' >/dev/null 2>&1; then ndmcversion=OK; fi
  if timeout 5 ndmc -c 'show ip name-server' >/dev/null 2>&1; then ndmcdns=OK; fi
fi
printf 'NDMC_VERSION=%s\nNDMC_DNS_SHOW=%s\n' "$ndmcversion" "$ndmcdns"
# Do not invent capability evidence based on a successful show command.
cat <<'FACTS'
DNS_OVERRIDE=UNKNOWN
PORT53_OWNER=UNKNOWN
LOCAL_NAMES_BASELINE=NOT_TESTED
SHADOW_UDP_TCP=NOT_TESTED
RECURSION_TEST=NOT_TESTED
ROLLBACK_TEST=NOT_TESTED
RF_DPR2G_EVIDENCE_END
FACTS
