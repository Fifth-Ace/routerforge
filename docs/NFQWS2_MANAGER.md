# RouterForge NFQWS2 Manager

RouterForge provides an installed-only management, diagnostics and strategy-selection layer for an existing `nfqws2-keenetic` installation. It does not silently install or replace nfqws2.

## Safety model

- production mutations pass through the Core-guarded Admin boundary;
- candidate tests use isolated NFQUEUE transactions and do not replace the active strategy while measuring;
- Apply is explicit and uses exact config/dependency hashes, backup, post-apply verification and rollback;
- historical results can change search priority, but never bypass live verification.

## Strategy selection

AutoSelect is designed as a search system rather than a fixed preset chooser. Its sources are:

1. target-specific verified memory;
2. historically proven recommendations;
3. dynamically synthesized candidates;
4. saved/imported candidates;
5. curated corpus and builtins as deterministic fallbacks.

The Strategy Synthesizer composes candidates from transport-specific technique axes (split/disorder/fake/overlap/fooling/repeats/TTL/length variants) and validates every candidate through the existing RouterForge candidate compiler before it can enter an isolated live test. Selector modes reserve search capacity for synthesis so a large saved/static pool cannot starve newly composed candidates.

Before an AutoSelect run, the Property Probe executes a bounded set of isolated live questions against the target. Every probe uses the same NFQUEUE transaction and cleanup proof as normal candidate testing. Results are recorded as `HELPS`, `MIXED`, `NO_EFFECT`, or `UNMEASURED`; a missing or infrastructure-invalid measurement is never treated as a negative result. Measured-positive families move to the front of the synthesis budget, while negative evidence only de-prioritizes a family and never blacklists combinations that may still work.

The next development stages add evidence-driven mutation of promising candidates and multi-target composition.

## Credits / design references

RouterForge's implementation is independent, but the AutoSelect architecture was informed by ideas from:

- `necronicle/z2k` — https://github.com/necronicle/z2k — especially property/probe-driven DPI analysis and composing a strategy from measured behavior instead of relying only on preset enumeration; reviewed at `4192519cad13c7253b70fa56cbca05bc83e6dcfb`.
- `Omn1z/nfqws2-keenetic-strategy-selector` — https://github.com/Omn1z/nfqws2-keenetic-strategy-selector — especially isolated NFQUEUE strategy scanning and live result ranking; reviewed at `0514a18209b0f33e68df6f6657339e912d756049`.
- `rndnaame/nfqws-menu` — https://github.com/rndnaame/nfqws-menu — source/reference for strategy, list and blob integration already credited in the NFQWS UI.

No source code from the two AutoSelect reference projects is vendored into this implementation.

## Current workstream

P24 Strategy Synthesis:

- P24A — dynamic candidate synthesis and search-budget integration;
- P24B — direct DPI property probing and property-guided synthesis;
- P24C — adaptive mutation/search from live outcomes;
- P24D — multi-target/multi-transport composition;
- P24E — hardware acceptance.
