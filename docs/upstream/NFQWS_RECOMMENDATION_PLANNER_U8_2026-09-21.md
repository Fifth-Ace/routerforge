# RouterForge U8 Recommendation-aware Progressive Planner

Base SHA: `a7e7efa051b907622b20585923d02d70c4f7874a`

Stage contract follows the CURRENT NFQWS completion plan:
- baseline remains first and is not part of candidate reordering;
- historical evidence is a hint only;
- diagnostic evidence may alter candidate order only when Diagnostic Classifier marks it strategy-relevant;
- transport compiler compatibility and MaxCandidates remain authoritative;
- duplicate recommendation fingerprints promote the existing candidate in place and preserve its original source identity;
- known weak/degraded history can lower priority but never declares a live failure;
- live selector comparator remains authoritative for the actual winner;
- planner metadata never creates Apply eligibility.

Planner metadata exposed:
- planner version/read-only state;
- diagnostic code/fault-domain/strategy relevance;
- historical compatible count;
- promoted count;
- admitted-from-registry count;
- per-candidate original/effective order and stage;
- source/family/fingerprint;
- historical and diagnostic hint flags;
- explicit planning reason.

UI contract:
- Auto Selector obtains current Diagnostic v1 evidence before planning when possible;
- historical/diagnostic plan is rendered separately from live selector outcomes;
- failure to obtain diagnostic evidence falls back to historical planning rather than fabricating a classification.
