#!/bin/sh
# DPR2-H: streamed via SSH stdin. No packages, writes, service operations,
# DNS requests, firewall changes, NFQUEUE access, sockets opened, or process launches.
# The observed port tables are a snapshot, not a proof of listener ownership.
set -u
printf 'RF_DPR2H_EVIDENCE_BEGIN\nSCHEMA=1\nMODE=READ_ONLY\n'
a=$(uname -m 2>/dev/null || echo UNKNOWN)
case "$a" in *[!A-Za-z0-9_-]*|'') a=UNKNOWN;; esac
printf 'ARCH=%s\n' "$a"
ipmatch=UNKNOWN
if command -v ip >/dev/null 2>&1; then
  ipmatch=NO
  ip -4 addr show 2>/dev/null | grep -Eq 'inet[[:space:]]+192\.168\.10\.1(/|[[:space:]])' && ipmatch=YES
fi
printf 'DSTEST_IP_MATCH=%s\n' "$ipmatch"
if [ -d /opt ]; then echo OPT_DIR=YES; else echo OPT_DIR=NO; fi
if [ -x /opt/bin/opkg ]; then echo ENTWARE_OPKG=YES; else echo ENTWARE_OPKG=NO; fi
avail=$(awk '$1=="MemAvailable:" {print $2; found=1; exit} END {if(!found) print "UNKNOWN"}' /proc/meminfo 2>/dev/null)
case "$avail" in ''|*[!0-9]*) avail=UNKNOWN;; esac
printf 'MEM_AVAILABLE_KIB=%s\n' "$avail"
free=UNKNOWN
if command -v df >/dev/null 2>&1; then
  free=$(df -k /opt 2>/dev/null | awk 'NR==2 {print $4}')
fi
case "$free" in ''|*[!0-9]*) free=UNKNOWN;; esac
printf 'OPT_FREE_KIB=%s\n' "$free"
for spec in udp tcp; do
  n=0
  for fam in '' 6; do
    file="/proc/net/${spec}${fam}"
    [ -r "$file" ] || continue
    c=$(awk -v protocol="$spec" 'NR>1 {split($2,a,":"); if(toupper(a[2])=="0035" && (protocol=="udp" || $4=="0A")) n++} END {print n+0}' "$file" 2>/dev/null)
    case "$c" in ''|*[!0-9]*) c=0;; esac
    n=$((n+c))
  done
  printf 'PORT53_%s_ENTRIES=%s\n' "$(printf '%s' "$spec" | tr 'a-z' 'A-Z')" "$n"
done
# Evidence that a high port is unused is not proof it can be opened later.
count=0
for f in /proc/net/udp /proc/net/udp6 /proc/net/tcp /proc/net/tcp6; do
  [ -r "$f" ] || continue
  v=$(awk 'NR>1 {split($2,a,":"); if(toupper(a[2])=="D839") n++} END {print n+0}' "$f" 2>/dev/null)
  case "$v" in ''|*[!0-9]*) v=0;; esac
  count=$((count+v))
done
printf 'SHADOW_55353_ENTRIES=%s\n' "$count"
# Only bounded show commands. Never print router configuration or credentials.
ns=UNAVAILABLE; ver=UNAVAILABLE
if command -v ndmc >/dev/null 2>&1 && command -v timeout >/dev/null 2>&1; then
  timeout 5 ndmc -c 'show version' >/dev/null 2>&1 && ver=OK
  timeout 5 ndmc -c 'show ip name-server' >/dev/null 2>&1 && ns=OK
fi
printf 'NDMC_VERSION=%s\nNDMC_DNS=%s\n' "$ver" "$ns"
cat <<'MARKERS'
DNS_OVERRIDE=UNKNOWN
LOCAL_NAMES=NOT_TESTED
SHADOW_UDP_TCP=NOT_TESTED
LOOP_DETECTION=NOT_TESTED
ROLLBACK=NOT_TESTED
DNS_INTERCEPTION=NONE
NFQUEUE_300=NOT_TOUCHED
RF_DPR2H_EVIDENCE_END
MARKERS
