#!/bin/sh
# odin Keenetic port-knocking rule graph; RouterForge-managed lifecycle.
# Original: https://forum.keenetic.ru/topic/22137-port-knoking-для-пробрасываемого-порта/
# Intended ONLY for explicit operations by the Port Access Manager.
set -eu
CFG=/opt/etc/routerforge/port-access-manager/odin.conf
STATE=/opt/etc/routerforge/port-access-manager/odin.enabled
HOOK=/opt/etc/ndm/netfilter.d/100routerforge-odin.sh
CMD=${1:-status}
IPT=/opt/sbin/iptables
[ -x "$IPT" ] || IPT=/usr/sbin/iptables
[ -x "$IPT" ] || IPT=iptables
# Not trusting the config as shell source; read normalized keys line by line.
load() {
    [ -f "$CFG" ] || return 1
    WAN=; TIP=; TPORT=; K1=; K2=; K3=; WINDOW=; TTL=
    while IFS='=' read -r k v; do
       case "$k" in
        WAN) WAN=$v;; TIP) TIP=$v;; TPORT) TPORT=$v;;
        K1) K1=$v;; K2) K2=$v;; K3) K3=$v;; WINDOW) WINDOW=$v;; TTL) TTL=$v;;
       esac
    done < "$CFG"
    case "$WAN" in *[!a-zA-Z0-9_.:-]*|'') return 1;; esac
    case "$TIP" in *[!0-9.]*|'') return 1;; esac
    for p in "$K1" "$K2" "$K3" "$TPORT" "$WINDOW" "$TTL"; do
      case "$p" in ''|*[!0-9]*) return 1;; esac
    done
}
exists() { "$IPT" -t filter -n -L "$1" >/dev/null 2>&1; }
# An explicit, two-rule ownership signature uses only core tcp/udp matches;
# this works on Keenetic kernels with no xt_comment module.
# Port zero is never configured as a knock or protected service port.
owned() {
  "$IPT" -t filter -C "$1" -p tcp --dport 0 -j RETURN >/dev/null 2>&1 &&
  "$IPT" -t filter -C "$1" -p udp --dport 0 -j RETURN >/dev/null 2>&1
}
mark_owned() {
  a "$1" -p tcp --dport 0 -j RETURN
  a "$1" -p udp --dport 0 -j RETURN
}
# Follow odin's original NDM hook placement: inspect every NEW WAN TCP
# connection in _NDM_INPUT so unrelated ports reset the knock sequence.
check_hook() { "$IPT" -t filter -C "$1" -i "$WAN" -p tcp -m state --state NEW -j "$2" >/dev/null 2>&1; }
add_knock_hook() { check_hook _NDM_INPUT KNOCKING-FIRST || "$IPT" -t filter -I _NDM_INPUT -i "$WAN" -p tcp -m state --state NEW -j KNOCKING-FIRST; }
add_service_hook() { check_hook _NDM_FORWARD KNOCKING-LAST || "$IPT" -t filter -I _NDM_FORWARD -i "$WAN" -p tcp -m state --state NEW -j KNOCKING-LAST; }
drop_hooks() {
  "$IPT" -t filter -D _NDM_INPUT -i "$WAN" -p tcp -m state --state NEW -j KNOCKING-FIRST 2>/dev/null || :
  "$IPT" -t filter -D _NDM_FORWARD -i "$WAN" -p tcp -m state --state NEW -j KNOCKING-LAST 2>/dev/null || :
  "$IPT" -t filter -D INPUT -i "$WAN" -p tcp -m multiport --dports "$K1,$K2,$K3" -m conntrack --ctstate NEW -j KNOCKING-FIRST 2>/dev/null || :
  "$IPT" -t filter -D _NDM_INPUT -i "$WAN" -p tcp -m conntrack --ctstate NEW -j KNOCKING-FIRST 2>/dev/null || :
  "$IPT" -t filter -D _NDM_FORWARD -i "$WAN" -p tcp -m conntrack --ctstate NEW -j KNOCKING-LAST 2>/dev/null || :
  "$IPT" -t filter -D FORWARD -i "$WAN" -d "$TIP" -p tcp --dport "$TPORT" -m conntrack --ctstate NEW -j KNOCKING-LAST 2>/dev/null || :
}
remove_chains() {
  # Only module-owned chains; never flush or delete NDM or unrelated chains.
  for n in KNOCKING-FIRST KNOCKING-LAST STATE0 STATE1 STATE2 STATE3 SUB_STATE1 SUB_STATE2 SUB_STATE3; do
     exists "$n" && "$IPT" -t filter -F "$n" || :
  done
  for n in KNOCKING-FIRST KNOCKING-LAST STATE0 STATE1 STATE2 STATE3 SUB_STATE1 SUB_STATE2 SUB_STATE3; do
     exists "$n" && "$IPT" -t filter -X "$n" || :
  done
}
a() {
  if "$IPT" -t filter -A "$@"; then return 0; fi
  echo "ODIN_RULE_APPEND_FAILED: chain=$1 args=$*" >&2
  return 1
}
apply() {
    load || { echo 'CONFIG_INVALID'; return 1; }
    "$IPT" -t filter -n -L _NDM_FORWARD >/dev/null 2>&1 || return 1
    "$IPT" -t filter -n -L _NDM_INPUT >/dev/null 2>&1 || return 1
    # This operation is explicitly idempotent for NDM netfilter rebuilds.
    if exists KNOCKING-FIRST; then
      exists KNOCKING-LAST || { echo 'PARTIAL_CHAIN_STATE'; return 1; }
      owned KNOCKING-FIRST && owned KNOCKING-LAST || { echo 'FOREIGN_CHAIN_COLLISION_OR_LEGACY_SIGNATURE'; return 1; }
      # Add original NDM hooks first, then remove legacy early hooks.
      add_knock_hook
      add_service_hook
      "$IPT" -t filter -D INPUT -i "$WAN" -p tcp -m multiport --dports "$K1,$K2,$K3" -m conntrack --ctstate NEW -j KNOCKING-FIRST 2>/dev/null || :
      "$IPT" -t filter -D FORWARD -i "$WAN" -d "$TIP" -p tcp --dport "$TPORT" -m conntrack --ctstate NEW -j KNOCKING-LAST 2>/dev/null || :
      return 0
    fi
    # Do not run two odin engines simultaneously. Stop previous revision first.
    for old in RF_ODIN_FIRST RF_ODIN_LAST RF_ODIN_S0 RF_ODIN_S1 RF_ODIN_S2 RF_ODIN_S3 RF_ODIN_SUB1 RF_ODIN_SUB2 RF_ODIN_SUB3; do
      exists "$old" && { echo 'LEGACY_ODIN_ACTIVE_DISABLE_OLD_FIRST'; return 1; }
    done
    # Refuse any partial/foreign chain state before owning anything.
    for n in KNOCKING-FIRST KNOCKING-LAST STATE0 STATE1 STATE2 STATE3 SUB_STATE1 SUB_STATE2 SUB_STATE3; do
      exists "$n" && { echo 'CHAIN_COLLISION'; return 1; }
    done
    # Stage the full graph BEFORE connecting to traffic; abort/undo on partial failure.
    trap 'drop_hooks; remove_chains' 0
    for n in KNOCKING-FIRST KNOCKING-LAST STATE0 STATE1 STATE2 STATE3 SUB_STATE1 SUB_STATE2 SUB_STATE3; do
      "$IPT" -t filter -N "$n"
    done
    # Original odin rule order and invalid-port reset policy (unchanged).
    a KNOCKING-LAST -d "$TIP" -p tcp --dport "$TPORT" -m recent --rcheck --seconds "$TTL" --name KKD --rsource -j ACCEPT
    a KNOCKING-LAST -d "$TIP" -p tcp --dport "$TPORT" -j STATE3
    a KNOCKING-LAST -m recent --remove --name KK1 --rsource
    a KNOCKING-LAST -m recent --remove --name KK2 --rsource
    a KNOCKING-LAST -m recent --remove --name KK3 --rsource
    a KNOCKING-LAST -m recent --remove --name KKD --rsource
    mark_owned KNOCKING-LAST
    a KNOCKING-FIRST -p tcp --dport "$K3" -j STATE2
    a KNOCKING-FIRST -p tcp --dport "$K2" -j STATE1
    a KNOCKING-FIRST -p tcp --dport "$K1" -j STATE0
    a KNOCKING-FIRST -m recent --remove --name KK1 --rsource
    a KNOCKING-FIRST -m recent --remove --name KK2 --rsource
    a KNOCKING-FIRST -m recent --remove --name KK3 --rsource
    mark_owned KNOCKING-FIRST
    a STATE0 -m recent --update --seconds "$WINDOW" --name KK1 --rsource -j DROP
    a STATE0 -m recent --set --name KK1 --rsource
    a STATE0 -m recent --rcheck --seconds "$WINDOW" --name KK1 --rsource -j DROP
    a STATE1 -m recent --update --seconds "$WINDOW" --name KK2 --rsource -j DROP
    a STATE1 -m recent --rcheck --seconds "$WINDOW" --name KK1 --rsource -j SUB_STATE1
    a STATE1 -j RETURN
    a SUB_STATE1 -m recent --remove --name KK1 --rsource
    a SUB_STATE1 -m recent --set --name KK2 --rsource
    a SUB_STATE1 -m recent --rcheck --seconds "$WINDOW" --name KK2 --rsource -j DROP
    a STATE2 -m recent --update --seconds "$WINDOW" --name KK3 --rsource -j DROP
    a STATE2 -m recent --rcheck --seconds "$WINDOW" --name KK2 --rsource -j SUB_STATE2
    a STATE2 -j RETURN
    a SUB_STATE2 -m recent --remove --name KK2 --rsource
    a SUB_STATE2 -m recent --set --name KK3 --rsource
    a SUB_STATE2 -m recent --rcheck --seconds "$WINDOW" --name KK3 --rsource -j DROP
    a STATE3 -m recent --rcheck --seconds "$WINDOW" --name KK3 --rsource -j SUB_STATE3
    a STATE3 -j DROP
    a SUB_STATE3 -m recent --remove --name KK3 --rsource
    a SUB_STATE3 -m recent --set --name KKD --rsource
    a SUB_STATE3 -m recent --rcheck --seconds "$WINDOW" --name KKD --rsource -j ACCEPT
    add_knock_hook
    add_service_hook
    trap - 0
}
case "$CMD" in
  status)
    if [ -f "$STATE" ]; then echo enabled; else echo disabled; fi
    ;;
  apply)
    [ -f "$STATE" ] || exit 0
    [ "${type:-iptables}" = ip6tables ] && exit 0
    [ "${table:-filter}" = filter ] || exit 0
    apply
    ;;
  enable)
    load || exit 1
    [ ! -f "$STATE" ] || exit 0
    # The caller installs NDM hook after successful transaction.
    apply
    ;;
  disable)
    load || exit 1
    drop_hooks
    remove_chains
    ;;
  *) exit 2;;
esac
