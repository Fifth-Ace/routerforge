# RouterForge DNS Router — DPR2-G: guarded DNS Proxy integration rehearsal

**Implementation branch:** `feature/dns-router-r7-stage-g` (one commit on exact DPR2-F).
**Code base:** `0e30c34ec971684bbd7aa64161e84f75c0edbe4f`.
**Scope:** offline hardware evidence parsing, strict read-only collector, in-memory transaction rehearsal, API/UI, experimental IPK/CI coverage.

## What this stage delivers

1. The test-device script `scripts/dpr2-g-hardware-readonly.sh` emits only a bounded set of safe observations, delimited by `RF_DPR2G_EVIDENCE_BEGIN` / `END`; it executes read-only proc checks, and *optional* `ndmc -c 'show ...'` if available.
2. `ParseHardwareReadOnlyEvidence` validates schema, keys, bounds, markers and value sets, explicitly refusing forged/malformed/oversized evidence. Hardware evidence is **not proof of physical provenance**.
3. `/v1/keenetic/assess-hardware` accepts an explicitly acknowledged, bounded JSON POST and returns only an in-memory assessment; no report or credentials are persisted.
4. `--assess-hardware-log file.txt` assesses a locally available report without listening, loading DNS configuration or accessing any router.
5. `RehearseKeeneticTransaction` and `/v1/keenetic/rehearsal?failure=verify` emulate a full precheck → snapshot → apply → verify → restore → verify-restore. This is **pure in-memory simulation**, not a Keenetic adapter. Rollback failure/unknown always leads to `AMBIGUOUS`, never PASS.
6. The management UI provides the evidence field and rehearsal button, explicitly labeled offline-only. The original UDP/TCP/DoT/DoH, lifecycle, CAS, and user rules are unchanged.

## Non-negotiable security/compatibility contract

- No `ndmc` mutation commands, changes to port 53, `/etc/resolv.conf`, `opkg dns-override`, firewall, DHCP, routing, Core/Stable/Beta releases, router restarts or NFQUEUE.
- Installing the experimental package does not enable or start a service or change any DNS traffic.
- Successful hardware *inventory* does not mean permission to activate: every assessment always returns `activation_allowed=false`, `hardware_accepted=false`, `changes_applied=false`.
- `PORT53_ENTRIES` is not the owner of the DNS Proxy; success of `show ip name-server` is not proof of firmware support for our integration mode.
- Both IPv4 and IPv6, local names, reverse zones, hardcoded resolvers and selection ordering remain open items.
- The *real* candidate needs a separate reviewed preimage/snapshot design, explicit user authorization, bounded failsafe watchdog, isolated hardware acceptance and restore proof before activation can even be considered.

## Read-only hardware capture

DSTest is the only authorized hardware target. Execution is optional and requires an explicitly configured SSH target (`RF_DPR2G_SSH_TARGET` in the runner); trusted host-key validation is mandatory. The script is piped to `sh -s` over SSH: no installation, daemon lifecycle or file upload. Do not target production.

A local report may contain device details; handle it privately. This implementation does not send reports to an external service.

## CI gate

The experimental matrix step uses `scripts/verify-dns-router-experimental.sh`, which runs `go test`, race, vet, the DPR2-G acceptance suite, shell syntax, read-only CLI assessment, benchmark and IPK build/manifest inspection on aarch64/mips/mipsel. Main/Dev/Stable/Beta are not modified.

## Definition of Done

- API + CLI + evidence parser + collector + rehearsal tests pass on CI.
- Three experimental IPKs compile and preserve independence of DNS Manager.
- `gofmt -l` clean **before** commit and CI.
- CI exact SHA and exact Run ID verified; no public releases.
- Hardware evidence status reported truthfully as `NOT_REQUESTED` or `READ_ONLY_COLLECTED`—**not** `HARDWARE_ACCEPTED`.

**DPR2-H candidate**: isolated real-hardware shadow probing only after a DSTest evidence review; subsequent transactional activation remains separately gated.
