#!/bin/sh
# RouterForge-owned and scoped FORWARD gate. Never manages NDM/third-party chains.
set -eu
BASE=/opt/etc/routerforge/port-access-manager
IPT=/opt/sbin/iptables
[ -x "$IPT" ] || IPT=/usr/sbin/iptables
[ -x "$IPT" ] || exit 1
ACTION=${1:-}
MODE=${2:-}
# Validate IPv4 before using user-substituted %IP% from knockd.
valid_ip() {
  case "$1" in ''|*[!0-9.]*|*..*|.*|*.) return 1;; esac
  printf '%s\n' "$1" | awk -F. 'NF==4 {for (i=1;i<=4;i++) if ($i=="" || $i>255 || $i<0 || length($i)>3) exit 1; exit 0} {exit 1}'
}
authorize() {
  [ "$MODE" = knockd ] || return 1
  [ -f "$BASE/knockd.gate" ] || return 1
  valid_ip "${3:-}" || return 1
  [ -f /proc/net/xt_recent/RFPA_K_AUTH ] || return 1
  printf '+%s\n' "$3" > /proc/net/xt_recent/RFPA_K_AUTH
}
chain() { case "$1" in knockd) echo RFPA_K_GATE;; fwknopd) echo RFPA_F_GATE;; *) return 1;; esac; }
list() { "$IPT" -t filter -n -L "$1" >/dev/null 2>&1; }
load() {
  CONF="$BASE/$MODE.gate"
  [ -f "$CONF" ] || return 1
  WAN=; TTL=; SERVICES=
  while IFS='=' read -r k v; do
    case "$k" in WAN) WAN=$v;; TTL) TTL=$v;; SERVICE) SERVICES="$SERVICES $v";; *) return 1;; esac
  done < "$CONF"
  case "$WAN" in ''|*[!a-zA-Z0-9_.-]*) return 1;; esac
  case "$TTL" in ''|*[!0-9]*) return 1;; esac
  [ "$TTL" -ge 30 ] && [ "$TTL" -le 86400 ] || return 1
  [ -n "$SERVICES" ] || return 1
}
service_args() {
  SERVICE_IP=${1%:*}; SERVICE_PORT=${1##*:}
  valid_ip "$SERVICE_IP" || return 1
  case "$SERVICE_PORT" in ''|*[!0-9]*) return 1;; esac
  [ "$SERVICE_PORT" -ge 1 ] && [ "$SERVICE_PORT" -le 65535 ] || return 1
}
remove_hooks() {
  CHAIN=$(chain "$MODE")
  for svc in $SERVICES; do
    service_args "$svc" || return 1
    "$IPT" -t filter -D FORWARD -i "$WAN" -d "$SERVICE_IP" -p tcp --dport "$SERVICE_PORT" -m conntrack --ctstate NEW -j "$CHAIN" 2>/dev/null || :
  done
}
apply() {
  CHAIN=$(chain "$MODE")
  "$IPT" -t filter -n -L FORWARD >/dev/null
  if list "$CHAIN"; then
    "$IPT" -t filter -C "$CHAIN" -m comment --comment RFPA_MANAGED -j RETURN >/dev/null 2>&1 || { echo FOREIGN_CHAIN >&2; return 1; }
  else
    "$IPT" -t filter -N "$CHAIN"
    if [ "$MODE" = knockd ]; then
      "$IPT" -t filter -A "$CHAIN" -m recent --rcheck --seconds "$TTL" --name RFPA_K_AUTH --rsource -j ACCEPT
    else
      # fwknopd itself owns its dynamic jump from RFPA_F_GATE to FWKNOP_FORWARD.
      # It must never install a broad hook straight into FORWARD.
      :
    fi
    "$IPT" -t filter -A "$CHAIN" -j DROP
    "$IPT" -t filter -A "$CHAIN" -m comment --comment RFPA_MANAGED -j RETURN
  fi
  for svc in $SERVICES; do
    service_args "$svc" || return 1
    "$IPT" -t filter -C FORWARD -i "$WAN" -d "$SERVICE_IP" -p tcp --dport "$SERVICE_PORT" -m conntrack --ctstate NEW -j "$CHAIN" >/dev/null 2>&1 || \
      "$IPT" -t filter -I FORWARD 1 -i "$WAN" -d "$SERVICE_IP" -p tcp --dport "$SERVICE_PORT" -m conntrack --ctstate NEW -j "$CHAIN"
  done
}
disable() {
  CHAIN=$(chain "$MODE")
  list "$CHAIN" || return 0
  "$IPT" -t filter -C "$CHAIN" -m comment --comment RFPA_MANAGED -j RETURN >/dev/null 2>&1 || { echo FOREIGN_CHAIN >&2; return 1; }
  remove_hooks
  "$IPT" -t filter -F "$CHAIN"
  "$IPT" -t filter -X "$CHAIN"
}
case "$ACTION" in
  authorize) authorize;;
  apply|disable) load; "$ACTION";;
  *) exit 2;;
esac
