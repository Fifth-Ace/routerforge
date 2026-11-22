# RouterForge DNS Router — DPR2-K acceptance and handoff

Status: experimental, feature branch only. No Stable/Beta/Main/Dev deployment.

## Exact source and evidence

- Base branch: `feature/dns-router-r7-stage-h`, exact commit `3fe2808caadea206d21cadb95ae7618264a79916` (DPR2-H software CI `40577407228`, success).
- DPR2-I: hardware UDP/TCP/DoT/DoH, guarded lifecycle, explicit failover, TLS negative scenarios and 200-query benchmark passed on DSTest ARM64. Measured average UDP 1423 us, TCP 2045 us; live VmRSS 12484 KiB, VmHWM 12804 KiB.
- DPR2-J: domain-scoped routing through native Keenetic `ndnproxy` via temporary relay, UDP/TCP passed; DNS inventory exact rollback passed.
- DPR2-K: native private-LAN listener (without relay) tested on `DSTest`, `192.168.10.1`, ARM64. Direct UDP/TCP to `192.168.10.1:55358` passed, and UDP/TCP via `ndnproxy` on `127.0.0.1:53` and `192.168.10.1:53` passed. One temporary zone `dpr2k.test` registered via `ndmc` and removed. `DNS_INVENTORY_EXACT_RESTORED: PASS`; `NDNPROXY_RUNTIME: ONLINE`; temporary processes/config removed, no persistent configuration save, firewall or NFQUEUE mutation.
- Negative unauthorized-source UDP/TCP scenarios were passed during local DPR2-K development; `TestDPR2KExperimentalLANRequiresExactACL` now included in CI.

## Change contract

1. Existing loopback shadow mode remains the default; no additional flags change previous behavior.
2. LAN bind only on explicit **private IPv4 literal address**, with `--serve --shadow-forward --shadow-lan-experimental --shadow-allow-clients=...`. No wildcard/IPv6/public or port 53 bind.
3. Source allowlist contains 1..8 exact private/loopback IPv4 addresses; reject unlisted UDP and TCP clients before resolver policy evaluation.
4. New IPK remains experimental, no autostart, no public publish, no `ndnproxy`/Keenetic integration side effects by installation.
5. Do not claim production readiness. Remaining work includes persistent UI/API config for the listener, managed lifecycle/rollback, stronger security and load soak; production activation requires separate review.

## CI acceptance

- `go test ./modules/dns-router-runtime/...`, `go test -race`, `go vet`, and explicit `TestDPR2K` negative cases.
- Experimental package matrix aarch64/mips/mipsel at `0.14.0~dev.dpr2k`; no publish.
- Confirm immutable `main`, `dev`, Beta, Stable and v0.14.0 refs; no router runtime change.
