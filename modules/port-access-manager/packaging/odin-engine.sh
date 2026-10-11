#!/bin/sh
# RouterForge adaptation of odin's Keenetic port-knocking state machine.
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
check_hook() { "$IPT" -t filter -C "$1" -i "$WAN" -p tcp -m conntrack --ctstate NEW -j "$2" >/dev/null 2>&1; }
drop_hooks() {
  "$IPT" -t filter -D _NDM_INPUT -i "$WAN" -p tcp -m conntrack --ctstate NEW -j RF_ODIN_FIRST 2>/dev/null || :
  "$IPT" -t filter -D _NDM_FORWARD -i "$WAN" -p tcp -m conntrack --ctstate NEW -j RF_ODIN_LAST 2>/dev/null || :
}
remove_chains() {
  # Only module-owned chains; never flush or delete NDM or unrelated chains.
  for n in RF_ODIN_FIRST RF_ODIN_LAST RF_ODIN_S0 RF_ODIN_S1 RF_ODIN_S2 RF_ODIN_S3 RF_ODIN_SUB1 RF_ODIN_SUB2 RF_ODIN_SUB3; do
     exists "$n" && "$IPT" -t filter -F "$n" || :
  done
  for n in RF_ODIN_FIRST RF_ODIN_LAST RF_ODIN_S0 RF_ODIN_S1 RF_ODIN_S2 RF_ODIN_S3 RF_ODIN_SUB1 RF_ODIN_SUB2 RF_ODIN_SUB3; do
     exists "$n" && "$IPT" -t filter -X "$n" || :
  done
}
a() { "$IPT" -t filter -A "$@"; }
apply() {
    load || { echo 'CONFIG_INVALID'; return 1; }
    "$IPT" -t filter -n -L _NDM_FORWARD >/dev/null 2>&1 || return 1
    "$IPT" -t filter -n -L _NDM_INPUT >/dev/null 2>&1 || return 1
    # This operation is explicitly idempotent for NDM netfilter rebuilds.
    if exists RF_ODIN_FIRST; then
      exists RF_ODIN_LAST || { echo 'PARTIAL_CHAIN_STATE'; return 1; }
      "$IPT" -t filter -C RF_ODIN_FIRST -m comment --comment RF_ODIN_MANAGED -j RETURN >/dev/null 2>&1 || { echo 'FOREIGN_CHAIN_COLLISION'; return 1; }
      "$IPT" -t filter -C RF_ODIN_LAST -m comment --comment RF_ODIN_MANAGED -j RETURN >/dev/null 2>&1 || { echo 'FOREIGN_CHAIN_COLLISION'; return 1; }
      check_hook _NDM_INPUT RF_ODIN_FIRST || "$IPT" -t filter -I _NDM_INPUT -i "$WAN" -p tcp -m conntrack --ctstate NEW -j RF_ODIN_FIRST
      check_hook _NDM_FORWARD RF_ODIN_LAST || "$IPT" -t filter -I _NDM_FORWARD -i "$WAN" -p tcp -m conntrack --ctstate NEW -j RF_ODIN_LAST
      return 0
    fi
    # Refuse any partial/foreign chain state before owning anything.
    for n in RF_ODIN_FIRST RF_ODIN_LAST RF_ODIN_S0 RF_ODIN_S1 RF_ODIN_S2 RF_ODIN_S3 RF_ODIN_SUB1 RF_ODIN_SUB2 RF_ODIN_SUB3; do
      exists "$n" && { echo 'CHAIN_COLLISION'; return 1; }
    done
    # Stage the full graph BEFORE connecting to traffic; abort/undo on partial failure.
    trap 'drop_hooks; remove_chains' 0
    for n in RF_ODIN_FIRST RF_ODIN_LAST RF_ODIN_S0 RF_ODIN_S1 RF_ODIN_S2 RF_ODIN_S3 RF_ODIN_SUB1 RF_ODIN_SUB2 RF_ODIN_SUB3; do
      "$IPT" -t filter -N "$n"
    done
    # odin's original stage transitions and invalid-port reset policy.
    a RF_ODIN_LAST -d "$TIP" -p tcp --dport "$TPORT" -m recent --rcheck --seconds "$TTL" --name RFOD_D --rsource -j ACCEPT
    a RF_ODIN_LAST -d "$TIP" -p tcp --dport "$TPORT" -j RF_ODIN_S3
    a RF_ODIN_LAST -m recent --remove --name RFOD_1 --rsource
    a RF_ODIN_LAST -m recent --remove --name RFOD_2 --rsource
    a RF_ODIN_LAST -m recent --remove --name RFOD_3 --rsource
    a RF_ODIN_LAST -m recent --remove --name RFOD_D --rsource
    a RF_ODIN_LAST -m comment --comment RF_ODIN_MANAGED -j RETURN
    a RF_ODIN_FIRST -p tcp --dport "$K3" -j RF_ODIN_S2
    a RF_ODIN_FIRST -p tcp --dport "$K2" -j RF_ODIN_S1
    a RF_ODIN_FIRST -p tcp --dport "$K1" -j RF_ODIN_S0
    a RF_ODIN_FIRST -m recent --remove --name RFOD_1 --rsource
    a RF_ODIN_FIRST -m recent --remove --name RFOD_2 --rsource
    a RF_ODIN_FIRST -m recent --remove --name RFOD_3 --rsource
    a RF_ODIN_FIRST -m comment --comment RF_ODIN_MANAGED -j RETURN
    a RF_ODIN_S0 -m recent --update --seconds "$WINDOW" --name RFOD_1 --rsource -j DROP
    a RF_ODIN_S0 -m recent --set --name RFOD_1 --rsource
    a RF_ODIN_S0 -m recent --rcheck --seconds "$WINDOW" --name RFOD_1 --rsource -j DROP
    a RF_ODIN_S1 -m recent --update --seconds "$WINDOW" --name RFOD_2 --rsource -j DROP
    a RF_ODIN_S1 -m recent --rcheck --seconds "$WINDOW" --name RFOD_1 --rsource -j RF_ODIN_SUB1
    a RF_ODIN_S1 -j RETURN
    a RF_ODIN_SUB1 -m recent --remove --name RFOD_1 --rsource
    a RF_ODIN_SUB1 -m recent --set --name RFOD_2 --rsource
    a RF_ODIN_SUB1 -m recent --rcheck --seconds "$WINDOW" --name RFOD_2 --rsource -j DROP
    a RF_ODIN_S2 -m recent --update --seconds "$WINDOW" --name RFOD_3 --rsource -j DROP
    a RF_ODIN_S2 -m recent --rcheck --seconds "$WINDOW" --name RFOD_2 --rsource -j RF_ODIN_SUB2
    a RF_ODIN_S2 -j RETURN
    a RF_ODIN_SUB2 -m recent --remove --name RFOD_2 --rsource
    a RF_ODIN_SUB2 -m recent --set --name RFOD_3 --rsource
    a RF_ODIN_SUB2 -m recent --rcheck --seconds "$WINDOW" --name RFOD_3 --rsource -j DROP
    a RF_ODIN_S3 -m recent --rcheck --seconds "$WINDOW" --name RFOD_3 --rsource -j RF_ODIN_SUB3
    a RF_ODIN_S3 -j DROP
    a RF_ODIN_SUB3 -m recent --remove --name RFOD_3 --rsource
    a RF_ODIN_SUB3 -m recent --set --name RFOD_D --rsource
    a RF_ODIN_SUB3 -m recent --rcheck --seconds "$WINDOW" --name RFOD_D --rsource -j ACCEPT
    "$IPT" -t filter -I _NDM_INPUT -i "$WAN" -p tcp -m conntrack --ctstate NEW -j RF_ODIN_FIRST
    "$IPT" -t filter -I _NDM_FORWARD -i "$WAN" -p tcp -m conntrack --ctstate NEW -j RF_ODIN_LAST
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
