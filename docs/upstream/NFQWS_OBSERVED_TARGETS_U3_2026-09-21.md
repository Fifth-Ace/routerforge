# RouterForge U3 Observed Targets — provenance and architecture lock

Date: 2026-09-21  
RouterForge base SHA: `4f440a276ead5323cc8d1509ce047c0189808e88`

## Architecture

U3 does not create another packet monitor.

Flow source remains RouterForge Network Tools:

`Network Tools /v1/flow-explorer -> NFQWS observation adapter -> bounded classifier -> observed-target store`

The NFQWS Manager reaches the existing Network Tools runtime through its Unix socket and consumes the already bounded flow-explorer API.

## Upstream references

Omn1z:
- `internal/tools/netmon/conntrack.go`
- `internal/tools/netmon/devices.go`
- `internal/services/monitor/monitor.go`
- `internal/services/monitor/trace.go`

z2k:
- `files/z2k-blocked-monitor.sh`

RouterForge:
- `modules/network-tools/runtime/flow_explorer.go`
- `modules/network-tools/runtime/flow_attribution.go`

## Classifier policy

Visible candidates require at least two distinct occurrences. A still-active conntrack entry is not counted again on every refresh.

Signals:
- TCP SYN_SENT with repeated outbound packets and no reply;
- TCP UNREPLIED with repeated outbound packets and no reply;
- TCP CLOSE with no meaningful reply, labeled conservatively as `tcp_rst_or_early_close`;
- UDP UNREPLIED near expiry with repeated egress and no return.

Controls:
- local/private destinations excluded;
- source must be a private/link-local LAN-side address;
- destination port must be present in active NFQWS TCP/UDP filters/port assignments;
- retention 24 hours;
- maximum 256 persisted entries;
- ignored-target set;
- no candidate is labeled "blocked";
- DNS target correlation is best-effort and bounded, using recent RouterForge target memory and revalidated TCP16 SNI memory.

## Mutation boundary

Scanning writes only RouterForge-owned observation evidence.
Ignoring writes only the RouterForge observed-target store.
No NFQWS config, list, service, firewall, route, NFQUEUE or production process is changed.

The UI's "Add to user.list" action is a separate explicit user mutation through the existing list-save API and its backup/validation path.
