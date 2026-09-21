# RouterForge U10 — End-to-End Automatic Setup UX

Base SHA: `0bf658fc79bff62d97457386dd57aab354c9138f`

U10 is a frontend orchestration layer over already closed RouterForge authorities:
Inspector/Diagnostic Classifier -> optional TCP16 -> Recommendation-aware Planner ->
isolated Selector -> Generic Candidate Safe Apply Preview -> explicit Smart Apply.

Entry points:
- manual target;
- Observed Target one-click Auto Setup;
- list + resolved domain;
- Domain Inspector one-click Auto Setup.

Explicit terminal UI states include:
DEPENDENCY_MISSING, DNS_FAILURE, ROUTING_FAILURE, TARGET_ALREADY_HEALTHY,
NO_CANDIDATES_COMPILE, NO_CANDIDATE_WORKS, CLEANUP_FAILURE,
CONFIG_CHANGED_WHILE_TESTING, WINNER_VERIFIED, PREVIEW_STALE,
APPLY_ROLLBACK, APPLY_SUCCESS.

No hidden mutation is introduced. The flow may collect evidence and build Preview;
production mutation still requires the user's explicit Smart Apply confirmation.
