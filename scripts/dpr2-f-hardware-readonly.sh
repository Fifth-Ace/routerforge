#!/bin/sh
# DPR2-F: hardware evidence ONLY. No writes, installs, service changes, or DNS queries.
set -u
printf '=== PRECHECK ===\nMODE: READ_ONLY\nDNS_CHANGES: FORBIDDEN\nFIREWALL_CHANGES: FORBIDDEN\n'
printf '\n=== HOST / ARCH ===\n'
uname -srm 2>&1 || :
printf '\n=== MEMORY ===\n'
if [ -r /proc/meminfo ]; then grep -E '^(MemTotal|MemAvailable|MemFree|SwapTotal):' /proc/meminfo || :; else echo PROC_MEMINFO_UNAVAILABLE; fi
printf '\n=== STORAGE (READ_ONLY) ===\n'
df -k /opt 2>&1 || :
printf '\n=== /etc/resolv.conf (NAMESERVERS ONLY) ===\n'
if [ -r /etc/resolv.conf ]; then grep -E '^[[:space:]]*nameserver[[:space:]]' /etc/resolv.conf || :; else echo RESOLV_CONF_UNAVAILABLE; fi
printf '\n=== PORT 53 KERNEL ENTRIES (NOT OWNERSHIP PROOF) ===\n'
for p in /proc/net/udp /proc/net/udp6 /proc/net/tcp /proc/net/tcp6; do
  if [ -r "$p" ]; then
    awk 'NR>1 { split($2,a,":"); if (toupper(a[2])=="0035" && (FILENAME ~ /udp/ || $4=="0A")) count++ } END { printf "%s: %d\n", FILENAME, count+0 }' "$p"
  else
    echo "$p: unavailable"
  fi
done
printf '\n=== DNS/ROUTERFORGE PROCESS RSS (KIB) ===\n'
for d in /proc/[0-9]*; do
  [ -r "$d/comm" ] || continue
  IFS= read -r name < "$d/comm" || continue
  case "$name" in
    *routerforge*|*ndnproxy*|*stubby*|*dnsmasq*|*smartdns*)
      rss=unavailable
      if [ -r "$d/status" ]; then rss=$(awk '$1=="VmRSS:" {print $2}' "$d/status"); [ -n "$rss" ] || rss=unavailable; fi
      printf '%s %s RSS_KIB=%s\n' "${d##*/}" "$name" "$rss"
      ;;
  esac
done
printf '\n=== OPTIONAL NDMC SHOW-ONLY ===\n'
if command -v ndmc >/dev/null 2>&1 && command -v timeout >/dev/null 2>&1; then
  # No configuration commands; cap each observational invocation to 5 seconds.
  timeout 5 ndmc -c 'show version' 2>&1 || echo NDMC_VERSION_UNAVAILABLE_OR_TIMEOUT
  timeout 5 ndmc -c 'show ip name-server' 2>&1 || echo NDMC_NAME_SERVER_UNAVAILABLE_OR_TIMEOUT
else
  echo NDMC_OR_TIMEOUT_UNAVAILABLE_SHOW_SKIPPED
fi
printf '\n=== RESULT ===\nSTATE: READ_ONLY_COLLECTED\nHARDWARE_ACCEPTANCE: NOT_PERFORMED\nROUTER_CHANGES: NONE\n'
