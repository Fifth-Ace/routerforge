# RouterForge NFQWS2 Manager

RouterForge provides an installed-only management, diagnostics and strategy-selection layer for an existing `nfqws2-keenetic` installation. It does not silently install or replace nfqws2.

## Safety model

- production mutations pass through the Core-guarded Admin boundary;
- candidate tests use isolated NFQUEUE transactions and do not replace the active strategy while measuring;
- Apply is explicit and uses exact config/dependency hashes, backup, post-apply verification and rollback;
- historical evidence may inform what the UI shows, but never bypasses live verification.

## Strategy selection

AutoSelect uses the direct selector model.

The selector builds its executable candidate set from the portable RouterForge strategy corpus plus explicitly supplied or saved candidates. Candidates are deduplicated by technique fingerprint and then executed directly through isolated NFQUEUE worker sandboxes. The selector does not use a planner, synthesizer, adaptive mutation layer or generated candidate plan between the corpus and live execution.

Selector modes control breadth and stopping behavior, not a separate planning stage. The live result remains authoritative.

Before or alongside AutoSelect, the Property Probe can execute a bounded set of isolated live questions against the target. Every probe uses the same NFQUEUE transaction and cleanup proof as normal candidate testing. Results are recorded as `HELPS`, `MIXED`, `NO_EFFECT`, or `UNMEASURED`; a missing or infrastructure-invalid measurement is never treated as a negative result. Property-probe evidence is exposed as diagnostic context while the direct catalog remains available for live verification.

Historical target memory, strategy scores, insights and recommendations remain separate read-only evidence systems. They describe prior observations and can help an operator understand previously successful strategies, but the direct selector does not treat them as an execution plan and does not skip current live testing because of historical success.

## Credits / design references

RouterForge's implementation is independent, but the AutoSelect and DPI-analysis work was informed by ideas from:

- `necronicle/z2k` — https://github.com/necronicle/z2k — especially property/probe-driven DPI analysis and measuring transport behavior before choosing techniques; reviewed at `4192519cad13c7253b70fa56cbca05bc83e6dcfb`.
- `Omn1z/nfqws2-keenetic-strategy-selector` — https://github.com/Omn1z/nfqws2-keenetic-strategy-selector — especially isolated NFQUEUE strategy scanning and live result ranking; reviewed at `0514a18209b0f33e68df6f6657339e912d756049`.
- `rndnaame/nfqws-menu` — https://github.com/rndnaame/nfqws-menu — source/reference for strategy, list and blob integration already credited in the NFQWS UI.

No source code from the two AutoSelect reference projects is vendored into this implementation.

## Current architecture

The current NFQWS strategy-selection path is:

1. direct portable strategy corpus;
2. optional explicitly supplied / saved candidates;
3. isolated live candidate execution;
4. live ranking and recommendation;
5. explicit deterministic Apply gate;
6. historical memory / scores / insights retained as separate evidence.

The retired planner, candidate synthesizer and adaptive candidate-plan layers are no longer part of the selector execution path.
