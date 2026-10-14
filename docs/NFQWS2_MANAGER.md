# RouterForge NFQWS2 Manager

RouterForge provides an installed-only management, diagnostics and strategy-selection layer for an existing `nfqws2-keenetic` installation. It does not silently install or replace nfqws2.

## List editor exact-preimage safety (NF5A)

The `/list` read returns the exact SHA256 of its bounded regular-file content.
Every editor save and every cross-list conflict cleanup now supplies that SHA
with `expected_sha256`; the server refuses missing or stale identities before
backup or write, then rechecks after making its safety backup. Symlink and
non-regular list files are rejected. This prevents normal stale-tab/peer-edit
clobbering; no new automatic list mutations or reloads are introduced.
A genuinely simultaneous external writer between the final check and atomic
rename remains outside this optimistic pre-image guard.

## NF5B — referenced-list deletion protection

List deletion now requires the exact SHA256 returned by `/v1/list`, rejects
protected/unsafe paths and refuses removal when either the active nfqws2
configuration or any saved configuration mentions the list. Unreadable or
non-regular configuration files fail closed. A second preimage/reference check
runs after the backup, immediately before deletion. No forced deletion or
automatic edits to dependent profiles are performed.

## NF5C — exclusive list creation (no TOCTOU overwrite)

Creating a new `.list` file now uses `O_CREATE|O_EXCL` against a verified
real list-root directory. An existing filename or symlink causes a conflict,
and simultaneous create attempts have at most one successful writer. This
replaces the former check-then-atomic-rename path which could overwrite a file
created between the check and the rename. No existing files or profiles are
modified, and list save/delete SHA and reference protections remain intact.

## NF5D — CAS guard for upstream list source replacement

The upstream list installer now requires **two distinct SHA256 identities** for
an explicit replacement: the incoming payload SHA256 from the pinned upstream
manifest and the exact *previous local file* SHA256 from the inventory. The
manager rechecks the old hash before backup and immediately before final write.
A missing, stale, symlinked or oversized local file is not replaceable, even
when the downloaded payload matches the remote checksum. Only the old file
is backed up; restart remains disabled. The non-replacement install path is
unchanged and will receive its separate exclusive-create audit next.

## Safety model

- production mutations pass through the Core-guarded Admin boundary;
- candidate tests use isolated NFQUEUE transactions and do not replace the active strategy while measuring;
- Apply is explicit and uses exact config/dependency hashes, backup, post-apply verification and rollback;
- historical evidence may inform what the UI shows, but never bypasses live verification.

## Post-Apply regression checks (NF4D)

A verified AutoTune Apply may select at most two other HTTPS targets from fresh
(under 24 hours), independently measured WORKING target memory. Historical
memory is **never** accepted as evidence of current production reachability:
each control site is resolved to a public IPv4 and must pass a live production
HTTPS precheck immediately before any write. Unreachable controls are skipped.

The successful precheck's exact hostname and IPv4 are pinned for a second
production-routed probe after Smart Apply's existing target verification. If any
prechecked control stops opening, the **same Smart Apply transaction** rolls
back config and list mutations. Legacy Smart Apply callers are unaffected.
No new background job, auto-apply behavior, or production change is introduced
by the delivery patch itself. A result reports how many controls were offered
and verified; zero means no control sites were live-proven.

## Strategy selection

AutoSelect uses the direct selector model.

The selector constructs its direct live queue in this order: explicitly supplied candidates; fresh WORKING evidence for the same target and unchanged configuration/environment; saved strategies; then the portable RouterForge corpus. Candidates are deduplicated by technique fingerprint and executed in isolated NFQUEUE worker sandboxes. History changes execution priority only, never replaces a current live test. No planner, synthesizer or adaptive mutation layer is used.

Selector modes control breadth and stopping behavior, not a separate planning stage. The live result remains authoritative.

Before or alongside AutoSelect, the Property Probe can execute a bounded set of isolated live questions against the target. Every probe uses the same NFQUEUE transaction and cleanup proof as normal candidate testing. Results are recorded as `HELPS`, `MIXED`, `NO_EFFECT`, or `UNMEASURED`; a missing or infrastructure-invalid measurement is never treated as a negative result. Property-probe evidence is exposed as diagnostic context while the direct catalog remains available for live verification.

Historical target memory, strategy scores, insights and recommendations remain separate read-only evidence systems. They describe prior observations and can help an operator understand previously successful strategies, but the direct selector does not treat them as an execution plan and does not skip current live testing because of historical success.

## Memory-aware worker admission (NF3B1)

Before spawning isolated NFQUEUE workers, the direct selector checks `/proc/meminfo` and
caps the requested concurrency using both `MemTotal` and `MemAvailable`.
For routers with up to 192 MiB it permits at most one worker; up to 320 MiB,
two; up to 448 MiB, three; larger routers may use up to the existing maximum
of four, subject to free-RAM thresholds and available NFQUEUE slots.
Less than 24 MiB available stops **a new selector run before creating workers**.
On older kernels without `MemAvailable`, it conservatively uses one worker.
Admission is assessed at the start; it is not a claim of continuous memory
monitoring or a guarantee against later pressure. The effective worker count
and admission reason are displayed in the selector. Live test and explicit
Apply safety gates remain unchanged. There is no automatic production change.

## Bounded live confirmation (NF3B2)

The direct selector first tests the ordered candidate queue in its ordinary
isolated NFQUEUE worker pool. When the baseline is not already working, it
re-checks at most **two** successful candidates, one after another, in a fresh
sandbox **after** the original worker rules have been removed. A confirmed
candidate must succeed in **both independent live HTTPS probes**; a single
success or a failed second attempt cannot become a selector recommendation.
A cleanup failure stops the confirmation sequence and fails the selector closed.
This is not candidate generation, mutation, or a trusted-memory shortcut.

One-off `WORKING` results are shown as initial measurements but are excluded
from reusable target-memory evidence. Failed tests can still contribute to
failure history. Any Apply remains a separate, explicit action with its own
verification and rollback. Confirmation does not touch the production config.

## Opt-in ASN community candidates (NF3C1)

The selector may optionally load **up to eight** TLS-only strategy candidates
from the provider-specific `data/AS<number>.json` file published by
`zemidala/nfqws2-strategies`. This feature is **off by default** and does not
vendor or republish community entries. The user may supply an ASN explicitly,
or opt in to RIPEstat detection (which contacts RIPEstat to determine the router's
public IPv4 and its ASN). Only then does the manager fetch the provider file.

Remote data is size/entry/argument bounded, restricted to `--lua-desync`
steps, filtered by provider/protocol/failure reports, and ordered by freshness,
matching site, and confirmations. Unknown Lua dependencies are excluded by
existing local candidate validation; HTTPS tests and independent confirmation
remain mandatory. Nothing is auto-applied. Cache: 24 h on valid results/404,
10 min for errors. Network failure cannot stop the ordinary local candidate
pool. This reproduces the *selection pattern*, not the PHP server or strategy
database: see upstream `src/www/api.php` (`communityLoad`, `communityCandidates`).

## Safe Append Preview/Apply parity (NF4A)

When a confirmed selector winner needs a new narrowly targeted
`NFQWS_ARGS_CUSTOM` profile, the existing Preview and one-shot Apply now
rebuild the **same deterministic append-mode candidate**. Replacement-mode
Apply continues to verify that the original production profile matches the
previewed one exactly. The receipt binds append/replacement mode as well as
configuration and dependency SHA256, and Smart Apply still owns backup,
restart, post-write checks and rollback. This fixes the previous situation
where Preview supported append but Apply incorrectly demanded a source profile.

It does **not** yet alter an existing hostlist; that separate placement
path requires exact list reference/selector analysis before implementation.
No automatic production mutation is performed by strategy selection.

## NF4B: reuse an exclusive existing hostlist profile

When the selected winner has passed two independent live attempts, RouterForge
first checks whether precisely one existing HTTPS production profile uses the
same exact technique and only one file-backed hostlist. The list must have no
other production profile references, the target must not already be routed by
another profile, and the list must be a regular validated local file. Only
then the selector prepares an explicit *add this domain to that list* plan;
otherwise the NF4A create-new-profile path is preserved unchanged.

Preview exposes the exact list name, old/new list SHA256 and the added domain.
The original nfqws2 config SHA is unchanged in this placement. The one-shot
Apply receipt rechecks the profile, list ownership, exact pre-image SHA,
updated bytes and unchanged config, then delegates the list update to the
existing Smart Apply backup/restart/verify/rollback implementation. No silent
write, background Apply, or new deployment code is introduced. Runtime
behavior still requires physical Keenetic verification.

## Post-Apply target verification and rollback (NF4C)

For the explicitly confirmed AutoTune Apply, the same Smart Apply transaction
now checks the selected domain using a real production-routed HTTPS probe after
restart and exact config/list/blob SHA checks. The destination is pinned to the
public IPv4 proven during the live selector session; no new DNS lookup is used.
A transient connection failure may be retried once. If both probes fail or
HTTP progress cannot be proven, the transaction invokes its existing config
and list rollback. Other Smart Apply clients retain their former behavior.
A successful probe validates reachability **from the router**, not every
client/device/QUIC route. Hardware behavior remains HARDWARE_PENDING until a
physical Keenetic acceptance test.

Upstream inspiration: zemidala/nfqws2-ui `autoApply()` (live site check and
rollback). The implementation stays in RouterForge's Go Smart Apply handler.

## Credits / design references

RouterForge's implementation is independent, but the AutoSelect and DPI-analysis work was informed by ideas from:

- `necronicle/z2k` — https://github.com/necronicle/z2k — especially property/probe-driven DPI analysis and measuring transport behavior before choosing techniques; reviewed at `4192519cad13c7253b70fa56cbca05bc83e6dcfb`.
- `Omn1z/nfqws2-keenetic-strategy-selector` — https://github.com/Omn1z/nfqws2-keenetic-strategy-selector — especially isolated NFQUEUE strategy scanning and live result ranking; reviewed at `0514a18209b0f33e68df6f6657339e912d756049`.
- `rndnaame/nfqws-menu` — https://github.com/rndnaame/nfqws-menu — source/reference for strategy, list and blob integration already credited in the NFQWS UI.
- `zemidala/nfqws2-ui` — https://github.com/zemidala/nfqws2-ui — practical inspiration for ordering already proven strategies and bringing user workflows together; NF3A code is independently implemented.

No source code from the two AutoSelect reference projects is vendored into this implementation.

## Current architecture

The current NFQWS strategy-selection path is:

1. explicit candidates → same-environment fresh WORKING memory → saved library → portable corpus;
2. deduplication by technique fingerprint;
3. isolated live candidate execution;
4. live ranking and recommendation;
5. explicit deterministic Apply gate;
6. historical scores / insights retained as separate evidence, never bypassing a live test.

The retired planner, candidate synthesizer and adaptive candidate-plan layers are no longer part of the selector execution path.
