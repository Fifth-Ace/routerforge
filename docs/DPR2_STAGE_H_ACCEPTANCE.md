# RouterForge DPR2-H — DSTest read-only hardware inventory and acceptance gate

**Base:** DPR2-G exact commit `f9afcc25a9af842e122eb47a3f51e4abfda7a7f1`.

## Scope

This substantial stage packages a bounded hardware evidence collector, strict offline parser/CLI, reproducible tests and three experimental IPKs. It does **not** install, restart or run an experimental binary on DSTest. The read-only collector runs from SSH stdin without transferring files. It does not execute DNS queries, open listeners, touch port 53, restart services or inspect/modify NFQUEUE_300. It does not persist a file on the router.

## Strict separation of gates

- SOFTWARE_CI: Go test / race / vet, strict parser and negative forged-evidence tests, experimental three-architecture IPK matrix.
- HARDWARE_INVENTORY: SSH to *verified* DSTest `192.168.10.1`, observed ARM64 architecture, basic memory/flash, bounded NDMC show command status and passive socket-table counts. No raw config or credentials are included in the collector output.
- SHADOW_HARDWARE_ACCEPTANCE: **NOT_TESTED** until separately authorized. This needs a live experimental process, which would conflict with the established strict read-only/no-runtime-mutations boundary. This stage must not claim that running a collector is shadow acceptance.
- SYSTEM_DNS_ACTIVATION: **FORBIDDEN**. No Keenetic RCI setters, dns-override, iptables/nftables mutation or interception.

## What the data can and cannot prove

The collector can identify which Linux architecture and LAN address are present, report passive resource/socket observations and indicate that bounded `show` commands work. Socket-table entries do not establish owner identity, firmware DNS semantics, split-DNS preservation or the ability to bind. Even a perfect text report cannot authorize any live mutation.

## Exact-result contract

`CI_CODE=PASS` does not imply `DSTEST_HARDWARE=PASS`. If no SSH or known host key is available, return `HARDWARE=NOT_TESTED` explicitly. After a successful trusted SSH collector and parser, return `HARDWARE_INVENTORY=PASS` and `SHADOW_HARDWARE=NOT_TESTED` — never full hardware acceptance.

The next authorized stage will be a narrowly scoped temporary shadow-only binary on DSTest, under separate explicit approval, verified rollback/cleanup and no access to production/router runtime.
