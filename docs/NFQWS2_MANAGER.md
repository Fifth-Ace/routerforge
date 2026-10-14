# RouterForge NFQWS2 Manager

RouterForge provides an installed-only management, diagnostics and strategy-selection layer for an existing `nfqws2-keenetic` installation. It does not silently install or replace nfqws2.

## Safety model

- production mutations pass through the Core-guarded Admin boundary;
- candidate tests use isolated NFQUEUE transactions and do not replace the active strategy while measuring;
- Apply is explicit and uses exact config/dependency hashes, backup, post-apply verification and rollback;
- historical evidence may inform what the UI shows, but never bypasses live verification.

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
