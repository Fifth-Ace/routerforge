# RouterForge U4 Diagnostic Classifier Parity — provenance lock

Date: 2026-09-21  
RouterForge base SHA: `9559e4ce978595c186032c32f086f6b0a9038c59`

## Purpose

U4 makes the existing DNS -> TCP -> TLS -> HTTP staged detector explicit about *what is proven* and *what is only suspected*.
The legacy `classification` field remains compatible; a new structured `diagnostic` object carries code, fault domain, confidence,
strategy relevance, evidence and next action.

## Upstream semantics reviewed

z2k pinned ref `fca1ed5a452f2554b3dfa1ab18571cee7c505174`:
- `tests/test_http_classifier.lua`: ordinary HTTP errors and bare HTTP 451 remain neutral; a status code alone is not block proof.
- `tests/test_discord_tls_timeout.lua`: ACKed TLS silence is treated as a stronger transport/DPI signal only with bounded observation.
- `tests/test_quic_silence_detector.lua`: silence, progress and success are separate concepts; capture exhaustion is neutral.

Omn1z pinned ref `bf4e810ef22ffb6671e97dc411234ba9430909c9`:
- `internal/services/monitor/trace.go`: distinguishes `unreplied`, `replied`, and `gone` rather than labeling every failed flow as blocked.

## RouterForge classifier policy

- DNS failure is a DNS fault, not an NFQWS strategy recommendation.
- TCP refusal is remote/service evidence, not DPI proof.
- TCP reset is strategy-relevant only as a *candidate signal* and remains medium confidence.
- TCP established + TLS timeout/EOF/reset/alert is strategy-relevant because failure occurs after the TCP gate.
- 12-20 KiB partial response is explicitly `SUSPECTED`, requires revalidation/TCP16 evidence.
- HTTP 4xx/5xx including bare 451 is `HTTP_RESPONSE_RESTRICTED_NOT_BLOCK_PROOF`; transport worked.
- clear end-to-end response is high-confidence clear for the router-side probe.
- unknown/partial evidence remains inconclusive.

No U4 code mutates NFQWS production configuration, lists, firewall, routes, queues, services or processes.
