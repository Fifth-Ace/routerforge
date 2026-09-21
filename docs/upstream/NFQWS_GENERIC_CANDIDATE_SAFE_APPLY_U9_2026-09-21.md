# RouterForge U9 — Generic Candidate Safe Apply

Base SHA: `0b9178588ac795c28659f2980bcce026ff830c7e`

Contract:
- source does not decide Apply eligibility;
- only a live-verified `WORKING` winner with cleanup + infrastructure proof can mint a gate;
- the candidate technique must still compile at gate creation and Preview;
- one deterministic production slot binding is still mandatory; ambiguous or absent target binding remains fail-closed;
- Preview snapshots hashes of every referenced list/blob and includes them in the short-lived server-side receipt;
- receipt binds target, transport, session, candidate source/id/name/fingerprint, exact candidate config, active/candidate SHA, live result, dependency hashes, issue/expiry;
- Apply rechecks active/candidate identity and delegates dependency hash enforcement to existing Smart Apply;
- receipt is consumed before mutation;
- no receipt is persisted in browser durable state.

Existing Smart Apply remains authoritative:
backup -> atomic write -> controlled service action -> runtime/status verification -> rollback on failure.
